/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
package repository

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/argocd"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/fs"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/history"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/notify"
	"go.uber.org/zap"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	git "github.com/libgit2/git2go/v33"
)

// A Repository provides a multiple reader / single writer access to a git repository.
type Repository interface {
	Apply(ctx context.Context, transformers ...Transformer) error
	Push(ctx context.Context, pushAction func() error) error
	ApplyTransformersInternal(transformers ...Transformer) ([]string, *State, error)
	State() *State

	Notify() *notify.Notify

	IsReady() (bool, error)
	WaitReady() error
}

type repository struct {
	// Mutex gurading the writer
	writeLock    sync.Mutex
	writesDone   uint
	remote       *git.Remote
	config       *Config
	credentials  *credentialsStore
	certificates *certificateStore

	repository *git.Repository

	// Testing
	nextError error

	// Mutex guarding head
	headLock sync.Mutex

	notify notify.Notify

	// Signaling readyness to allow fetching in the background
	*Readiness
}

type Config struct {
	// Mandatory Config
	URL  string
	Path string
	// Optional Config
	Credentials    Credentials
	Certificates   Certificates
	CommitterEmail string
	CommitterName  string
	// default branch is master
	Branch string
	//
	GcFrequency uint
}

func openOrCreate(path string) (*git.Repository, error) {
	repo2, err := git.OpenRepositoryExtended(path, git.RepositoryOpenNoSearch, path)
	if err != nil {
		var gerr *git.GitError
		if errors.As(err, &gerr) {
			if gerr.Code == git.ErrNotFound {
				os.MkdirAll(path, 0777)
				repo2, err = git.InitRepository(path, true)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return repo2, err
}

// Opens a repository. The repository is initialized and updated in the background.
func New(ctx context.Context, cfg Config) (Repository, error) {
	logger := logger.FromContext(ctx)
	if cfg.Branch == "" {
		cfg.Branch = "master"
	}
	if cfg.CommitterEmail == "" {
		cfg.CommitterEmail = "kuberpult@example.com"
	}
	if cfg.CommitterName == "" {
		cfg.CommitterName = "kuberpult"
	}
	var credentials *credentialsStore
	var certificates *certificateStore
	var err error
	if strings.HasPrefix(cfg.URL, "./") || strings.HasPrefix(cfg.URL, "/") {
		logger.Debug("git url indicates a local directory. Ignoring credentials and certificates.")
	} else {
		credentials, err = cfg.Credentials.load()
		if err != nil {
			return nil, err
		}
		certificates, err = cfg.Certificates.load()
		if err != nil {
			return nil, err
		}
	}

	if repo2, err := openOrCreate(cfg.Path); err != nil {
		return nil, err
	} else {
		// configure remotes
		if remote, err := repo2.Remotes.CreateAnonymous(cfg.URL); err != nil {
			return nil, err
		} else {
			result := &repository{
				remote:       remote,
				config:       &cfg,
				credentials:  credentials,
				certificates: certificates,
				repository:   repo2,
				Readiness:    newReadiness(),
			}
			result.headLock.Lock()

			defer result.headLock.Unlock()
			fetchSpec := fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", cfg.Branch, cfg.Branch)
			fetchOptions := git.FetchOptions{
				RemoteCallbacks: git.RemoteCallbacks{
					UpdateTipsCallback: func(refname string, a *git.Oid, b *git.Oid) error {
						logger.Debug("git.fetched",
							zap.String("refname", refname),
							zap.String("revision.new", b.String()),
						)
						return nil
					},
					CredentialsCallback:      credentials.CredentialsCallback(ctx),
					CertificateCheckCallback: certificates.CertificateCheckCallback(ctx),
				},
			}
			err := remote.Fetch([]string{fetchSpec}, &fetchOptions, "fetching")
			if err != nil {
				return nil, err
			}
			var zero git.Oid
			var rev *git.Oid = &zero
			if remoteRef, err := repo2.References.Lookup(fmt.Sprintf("refs/remotes/origin/%s", cfg.Branch)); err != nil {
				var gerr *git.GitError
				if errors.As(err, &gerr) && gerr.Code == git.ErrNotFound {
					// not found
					// nothing to do
				} else {
					return nil, err
				}
			} else {
				rev = remoteRef.Target()
				if _, err := repo2.References.Create(fmt.Sprintf("refs/heads/%s", cfg.Branch), rev, true, "reset branch"); err != nil {
					return nil, err
				}
			}
			// check that we can build the current state
			if _, err := result.buildState(); err != nil {
				return nil, err
			}

			return result, nil
		}
	}
}

func NewWait(ctx context.Context, cfg Config) (Repository, error) {
	if repo, err := New(ctx, cfg); err != nil {
		return repo, err
	} else {
		return repo, repo.WaitReady()
	}
}

func (r *repository) ApplyTransformersInternal(transformers ...Transformer) ([]string, *State, error) {
	if state, err := r.buildState(); err != nil {
		return nil, nil, &InternalError{inner: err}
	} else {
		commitMsg := []string{}
		for _, t := range transformers {
			if msg, err := t.Transform(state.Filesystem); err != nil {
				return nil, nil, err
			} else {
				commitMsg = append(commitMsg, msg)
			}
		}
		return commitMsg, state, nil
	}
}

func (r *repository) ApplyTransformers(ctx context.Context, transformers ...Transformer) error {
	commitMsg, state, err := r.ApplyTransformersInternal(transformers...)
	if err != nil {
		return err
	}
	if err := r.afterTransform(ctx, state.Filesystem); err != nil {
		return &InternalError{inner: err}
	}
	treeId, err := state.Filesystem.(*fs.TreeBuilderFS).Insert()
	if err != nil {
		return &InternalError{inner: err}
	}
	committer := &git.Signature{
		Name:  r.config.CommitterName,
		Email: r.config.CommitterEmail,
		When:  time.Now(),
	}

	author := &git.Signature{
		Name:  auth.Extract(ctx).Name,
		Email: auth.Extract(ctx).Email,
		When:  time.Now(),
	}

	var rev *git.Oid
	if state.Commit != nil {
		rev = state.Commit.Id()
	}
	if _, err := r.repository.CreateCommitFromIds(
		fmt.Sprintf("refs/heads/%s", r.config.Branch),
		author,
		committer,
		strings.Join(commitMsg, "\n"),
		treeId,
		rev,
	); err != nil {
		return &InternalError{inner: err}
	}
	return nil
}

func (r *repository) FetchAndReset(ctx context.Context) error {
	fetchSpec := fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", r.config.Branch, r.config.Branch)
	logger := logger.FromContext(ctx)
	fetchOptions := git.FetchOptions{
		RemoteCallbacks: git.RemoteCallbacks{
			UpdateTipsCallback: func(refname string, a *git.Oid, b *git.Oid) error {
				logger.Debug("git.fetched",
					zap.String("refname", refname),
					zap.String("revision.new", b.String()),
				)
				return nil
			},
			CredentialsCallback:      r.credentials.CredentialsCallback(ctx),
			CertificateCheckCallback: r.certificates.CertificateCheckCallback(ctx),
		},
	}
	err := r.remote.Fetch([]string{fetchSpec}, &fetchOptions, "fetching")
	if err != nil {
		return &InternalError{inner: err}
	}
	var zero git.Oid
	var rev *git.Oid = &zero
	if remoteRef, err := r.repository.References.Lookup(fmt.Sprintf("refs/remotes/origin/%s", r.config.Branch)); err != nil {
		var gerr *git.GitError
		if errors.As(err, &gerr) && gerr.Code == git.ErrNotFound {
			// not found
			// nothing to do
		} else {
			return err
		}
	} else {
		rev = remoteRef.Target()
		if _, err := r.repository.References.Create(fmt.Sprintf("refs/heads/%s", r.config.Branch), rev, true, "reset branch"); err != nil {
			return err
		}
	}
	obj, err := r.repository.Lookup(rev)
	if err != nil {
		return err
	}
	commit, err := obj.AsCommit()
	if err != nil {
		return err
	}
	err = r.repository.ResetToCommit(commit, git.ResetSoft, &git.CheckoutOptions{Strategy: git.CheckoutForce})
	if err != nil {
		return err
	}
	return nil
}

func (r *repository) Apply(ctx context.Context, transformers ...Transformer) error {
	// Obtain a new worktree
	r.writeLock.Lock()
	defer r.writeLock.Unlock()
	defer func() {
		r.writesDone = r.writesDone + uint(len(transformers))
		r.maybeGc(ctx)
	}()
	err := r.ApplyTransformers(ctx, transformers...)

	pushOptions := git.PushOptions{
		RemoteCallbacks: git.RemoteCallbacks{
			CredentialsCallback:      r.credentials.CredentialsCallback(ctx),
			CertificateCheckCallback: r.certificates.CertificateCheckCallback(ctx),
		},
	}

	pushAction := func() error {
		return r.remote.Push([]string{fmt.Sprintf("refs/heads/%s:refs/heads/%s", r.config.Branch, r.config.Branch)}, &pushOptions)
	}

	if err != nil {
		return err
	} else {

		if err := r.Push(ctx, pushAction); err != nil {
			gerr := err.(*git.GitError)
			if gerr.Code == git.ErrorCodeNonFastForward {
				err = r.FetchAndReset(ctx)
				if err != nil {
					return err
				}
				err = r.ApplyTransformers(ctx, transformers...)
				if err != nil {
					return err
				}
				if err := r.Push(ctx, pushAction); err != nil {
					return &InternalError{inner: err}
				}
			} else {
				return &InternalError{inner: err}
			}
		}
		r.notify.Notify()
	}

	return nil
}

func (r *repository) Push(ctx context.Context, pushAction func() error) error {

	span, ctx := tracer.StartSpanFromContext(ctx, "Apply")
	defer span.Finish()

	eb := backoff.NewExponentialBackOff()
	eb.MaxElapsedTime = 7 * time.Second
	return backoff.Retry(
		func() error {
			span, _ := tracer.StartSpanFromContext(ctx, "Push")
			span.SetTag("elapsedTime", eb.GetElapsedTime())
			defer span.Finish()
			err := pushAction()
			if err != nil {
				gerr := err.(*git.GitError)
				if gerr.Code == git.ErrorCodeNonFastForward {
					return backoff.Permanent(err)
				}
			}
			return err
		},
		backoff.WithMaxRetries(eb, 6),
	)
}

func (r *repository) afterTransform(ctx context.Context, fs billy.Filesystem) error {
	state := State{Filesystem: fs}
	configs, err := state.GetEnvironmentConfigs()
	if err != nil {
		return err
	}
	for env, config := range configs {
		if config.ArgoCd != nil {
			err := r.updateArgoCdApps(ctx, &state, env, config)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *repository) updateArgoCdApps(ctx context.Context, state *State, name string, config config.EnvironmentConfig) error {
	fs := state.Filesystem
	if apps, err := state.GetEnvironmentApplications(name); err != nil {
		return err
	} else {
		appData := []argocd.AppData{}
		sort.Strings(apps)
		for _, appName := range apps {
			isUndeployVersion, err := state.IsLatestUndeployVersion(appName)
			if err != nil {
				return err
			}
			appData = append(appData, argocd.AppData{
				AppName:           appName,
				IsUndeployVersion: isUndeployVersion,
			})
		}
		if manifests, err := argocd.Render(r.config.URL, r.config.Branch, config, name, appData); err != nil {
			return err
		} else {
			for apiVersion, content := range manifests {
				if err := fs.MkdirAll(fs.Join("argocd", string(apiVersion)), 0777); err != nil {
					return err
				}
				target := fs.Join("argocd", string(apiVersion), fmt.Sprintf("%s.yaml", name))
				if err := util.WriteFile(fs, target, content, 0666); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (r *repository) buildState() (*State, error) {
	if obj, err := r.repository.RevparseSingle(fmt.Sprintf("refs/heads/%s", r.config.Branch)); err != nil {
		var gerr *git.GitError
		if errors.As(err, &gerr) {
			if gerr.Code == git.ErrNotFound {
				return &State{Filesystem: fs.NewEmptyTreeBuildFS(r.repository)}, nil
			}
		}
		return nil, err
	} else {
		commit, err := obj.AsCommit()
		if err != nil {
			return nil, err
		}
		return &State{
			Filesystem: fs.NewTreeBuildFS(r.repository, commit.TreeId()),
			Commit:     commit,
			History:    history.NewHistory(r.repository),
		}, nil
	}
}

func (r *repository) State() *State {
	s, err := r.buildState()
	if err != nil {
		panic(err)
	}
	return s
}

func (r *repository) Notify() *notify.Notify {
	return &r.notify
}

type ObjectCount struct {
	Count       uint64
	Size        uint64
	InPack      uint64
	Packs       uint64
	SizePack    uint64
	Garbage     uint64
	SizeGarbage uint64
}

func (r *repository) countObjects(ctx context.Context) (ObjectCount, error) {
	var stats ObjectCount
	/*
		The output of `git count-objects` looks like this:
			count: 0
			size: 0
			in-pack: 635
			packs: 1
			size-pack: 2845
			prune-packable: 0
			garbage: 0
			size-garbage: 0
	*/
	cmd := exec.CommandContext(ctx, "git", "count-objects", "--verbose")
	cmd.Dir = r.config.Path
	out, err := cmd.Output()
	if err != nil {
		return stats, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		var (
			token string
			value uint64
		)
		if _, err := fmt.Sscan(scanner.Text(), &token, &value); err != nil {
			return stats, err
		}
		switch token {
		case "count:":
			stats.Count = value
		case "size:":
			stats.Size = value
		case "in-packs:":
			stats.InPack = value
		case "packs:":
			stats.Packs = value
		case "size-pack:":
			stats.SizePack = value
		case "garbage:":
			stats.Garbage = value
		case "size-garbage":
			stats.SizeGarbage = value
		}
	}
	return stats, nil
}

func (r *repository) maybeGc(ctx context.Context) {
	if r.config.GcFrequency == 0 || r.writesDone < r.config.GcFrequency {
		return
	}
	log := logger.FromContext(ctx)
	r.writesDone = 0
	timeBefore := time.Now()
	statsBefore, _ := r.countObjects(ctx)
	cmd := exec.CommandContext(ctx, "git", "repack", "-a", "-d")
	cmd.Dir = r.config.Path
	err := cmd.Run()
	if err != nil {
		log.Error("git.repack", zap.Error(err))
		return
	}
	statsAfter, _ := r.countObjects(ctx)
	log.Info("git.repack", zap.Duration("duration", time.Now().Sub(timeBefore)), zap.Uint64("collected", statsBefore.Count-statsAfter.Count))
}

type State struct {
	Filesystem billy.Filesystem
	Commit     *git.Commit
	History    *history.History
}

func (s *State) Applications() ([]string, error) {
	if entries, err := s.Filesystem.ReadDir("applications"); err != nil {
		return nil, err
	} else {
		result := make([]string, 0, len(entries))
		for _, e := range entries {
			result = append(result, e.Name())
		}
		return result, nil
	}
}

func (s *State) Releases(application string) ([]uint64, error) {
	if entries, err := s.Filesystem.ReadDir(s.Filesystem.Join("applications", application, "releases")); err != nil {
		return nil, err
	} else {
		result := make([]uint64, 0, len(entries))
		for _, e := range entries {
			if i, err := strconv.ParseUint(e.Name(), 10, 64); err != nil {
				// just skip
			} else {
				result = append(result, i)
			}
		}
		return result, nil
	}
}

func (s *State) ReleaseManifests(application string, release uint64) (map[string]string, error) {
	base := s.Filesystem.Join("applications", application, "releases", strconv.FormatUint(release, 10), "environments")
	if entries, err := s.Filesystem.ReadDir(base); err != nil {
		return nil, err
	} else {
		result := make(map[string]string, len(entries))
		for _, e := range entries {
			if buf, err := readFile(s.Filesystem, s.Filesystem.Join(base, e.Name(), "manifests.yaml")); err != nil {
				return nil, err
			} else {
				result[e.Name()] = string(buf)
			}
		}
		return result, nil
	}
}

type Lock struct {
	Message string
}

func (s *State) GetEnvironmentLocks(environment string) (map[string]Lock, error) {
	base := s.Filesystem.Join("environments", environment, "locks")
	if entries, err := s.Filesystem.ReadDir(base); err != nil {
		return nil, err
	} else {
		result := make(map[string]Lock, len(entries))
		for _, e := range entries {
			if buf, err := readFile(s.Filesystem, s.Filesystem.Join(base, e.Name())); err != nil {
				return nil, err
			} else {
				result[e.Name()] = Lock{
					Message: string(buf),
				}
			}
		}
		return result, nil
	}
}

func (s *State) GetEnvironmentApplicationVersionCommit(environment, application string) (*git.Commit, error) {
	if s.Commit == nil {
		return nil, nil
	} else {
		return s.History.Change(s.Commit,
			[]string{
				"environments", environment,
				"applications", application,
				"version",
			})
	}
}
func (s *State) GetEnvironmentApplicationLocksCommit(environment, application string, lockId string) (*git.Commit, error) {
	if s.Commit == nil {
		return nil, nil
	} else {
		return s.History.Change(s.Commit,
			[]string{
				"environments", environment,
				"applications", application,
				"locks", lockId,
			})
	}
}
func (s *State) GetEnvironmentLocksCommit(environment, lockId string) (*git.Commit, error) {
	if s.Commit == nil {
		return nil, nil
	} else {
		return s.History.Change(s.Commit,
			[]string{
				"environments", environment,
				"locks", lockId,
			})
	}
}

func (s *State) GetEnvironmentApplicationLocks(environment, application string) (map[string]Lock, error) {
	base := s.Filesystem.Join("environments", environment, "applications", application, "locks")
	if entries, err := s.Filesystem.ReadDir(base); err != nil {
		return nil, err
	} else {
		result := make(map[string]Lock, len(entries))
		for _, e := range entries {
			if buf, err := readFile(s.Filesystem, s.Filesystem.Join(base, e.Name())); err != nil {
				return nil, err
			} else {
				result[e.Name()] = Lock{
					Message: string(buf),
				}
			}
		}
		return result, nil
	}
}

func (s *State) GetQueuedVersion(environment string, application string) (*uint64, error) {
	return s.readSymlink(environment, application, queueFileName)
}

func (s *State) DeleteQueuedVersion(environment string, application string) error {
	queuedVersion := s.Filesystem.Join("environments", environment, "applications", application, queueFileName)
	return s.Filesystem.Remove(queuedVersion)
}

func (s *State) DeleteQueuedVersionIfExists(environment string, application string) error {
	queuedVersion, err := s.GetQueuedVersion(environment, application)
	if err != nil {
		return err
	}
	if queuedVersion == nil {
		return nil // nothing to do
	}
	return s.DeleteQueuedVersion(environment, application)
}

func (s *State) GetEnvironmentApplicationVersion(environment, application string) (*uint64, error) {
	return s.readSymlink(environment, application, "version")
}

// returns nil if there is no file
func (s *State) readSymlink(environment string, application string, symlinkName string) (*uint64, error) {
	version := s.Filesystem.Join("environments", environment, "applications", application, symlinkName)
	if lnk, err := s.Filesystem.Readlink(version); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// if the link does not exist, we return nil
			return nil, nil
		}
		return nil, err
	} else {
		if stat, err := s.Filesystem.Stat(s.Filesystem.Join("environments", environment, "applications", application, lnk)); err != nil {
			// if the file that the link points to does not exist, that's an error
			return nil, err
		} else {
			res, err := strconv.ParseUint(stat.Name(), 10, 64)
			return &res, err
		}
	}
}

func (s *State) GetEnvironmentConfigs() (map[string]config.EnvironmentConfig, error) {
	envs, err := s.Filesystem.ReadDir("environments")
	if err != nil {
		return nil, err
	}
	result := map[string]config.EnvironmentConfig{}
	for _, env := range envs {
		fileName := s.Filesystem.Join("environments", env.Name(), "config.json")
		var config config.EnvironmentConfig
		if err := decodeJsonFile(s.Filesystem, fileName, &config); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				result[env.Name()] = config
			} else {
				return nil, err
			}
		} else {
			result[env.Name()] = config
		}
	}
	return result, nil
}

func (s *State) GetEnvironmentApplications(environment string) ([]string, error) {
	appDir := s.Filesystem.Join("environments", environment, "applications")
	return names(s.Filesystem, appDir)
}

func (s *State) GetApplications() ([]string, error) {
	return names(s.Filesystem, "applications")
}

func (s *State) GetApplicationReleases(application string) ([]uint64, error) {
	if ns, err := names(s.Filesystem, s.Filesystem.Join("applications", application, "releases")); err != nil {
		return nil, err
	} else {
		result := make([]uint64, 0, len(ns))
		for _, n := range ns {
			if i, err := strconv.ParseUint(n, 10, 64); err == nil {
				result = append(result, i)
			}
		}
		return result, nil
	}
}

type Release struct {
	Version uint64
	/**
	"UndeployVersion=true" means that this version is empty, and has no manifest that could be deployed.
	It is intended to help cleanup old services within the normal release cycle (e.g. dev->staging->production).
	*/
	UndeployVersion bool
	SourceAuthor    string
	SourceCommitId  string
	SourceMessage   string
}

func (s *State) IsLatestUndeployVersion(application string) (bool, error) {
	version, err := GetLastRelease(s.Filesystem, application)
	if err != nil {
		return false, err
	}
	base := releasesDirectoryWithVersion(s.Filesystem, application, version)
	_, err = s.Filesystem.Stat(base)
	if err != nil {
		return false, wrapFileError(err, base, "could not call stat")
	}
	if _, err := readFile(s.Filesystem, s.Filesystem.Join(base, "undeploy")); err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

func (s *State) IsUndeployVersion(application string, version uint64) (bool, error) {
	base := releasesDirectoryWithVersion(s.Filesystem, application, version)
	_, err := s.Filesystem.Stat(base)
	if err != nil {
		return false, wrapFileError(err, base, "could not call stat")
	}
	if _, err := readFile(s.Filesystem, s.Filesystem.Join(base, "undeploy")); err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

func (s *State) GetApplicationRelease(application string, version uint64) (*Release, error) {
	base := releasesDirectoryWithVersion(s.Filesystem, application, version)
	_, err := s.Filesystem.Stat(base)
	if err != nil {
		return nil, wrapFileError(err, base, "could not call stat")
	}
	release := Release{Version: version}
	if cnt, err := readFile(s.Filesystem, s.Filesystem.Join(base, "source_commit_id")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		release.SourceCommitId = string(cnt)
	}
	if cnt, err := readFile(s.Filesystem, s.Filesystem.Join(base, "source_author")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		release.SourceAuthor = string(cnt)
	}
	if cnt, err := readFile(s.Filesystem, s.Filesystem.Join(base, "source_message")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		release.SourceMessage = string(cnt)
	}
	isUndeploy, err := s.IsUndeployVersion(application, version)
	if err != nil {
		return nil, err
	}
	release.UndeployVersion = isUndeploy
	return &release, nil
}

func (s *State) GetApplicationReleaseCommit(application string, version uint64) (*git.Commit, error) {
	return s.History.Change(s.Commit, []string{
		"applications", application,
		"releases", fmt.Sprintf("%d", version),
	})
}

func names(fs billy.Filesystem, path string) ([]string, error) {
	files, err := fs.ReadDir(path)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(files))
	for _, app := range files {
		result = append(result, app.Name())
	}
	return result, nil
}

func decodeJsonFile(fs billy.Filesystem, path string, out interface{}) error {
	if file, err := fs.Open(path); err != nil {
		return wrapFileError(err, path, "could not decode json file")
	} else {
		defer file.Close()
		dec := json.NewDecoder(file)
		return dec.Decode(out)
	}
}

func readFile(fs billy.Filesystem, path string) ([]byte, error) {
	if file, err := fs.Open(path); err != nil {
		return nil, err
	} else {
		defer file.Close()
		return io.ReadAll(file)
	}
}

// ProcessQueue checks if there is something in the queue
// deploys if necessary
// deletes the queue
func (s *State) ProcessQueue(fs billy.Filesystem, environment string, application string) (string, error) {
	queuedVersion, err := s.GetQueuedVersion(environment, application)
	queueDeploymentMessage := ""
	if err != nil {
		// could not read queued version.
		return "", err
	} else {
		if queuedVersion == nil {
			// if there is no version queued, that's not an issue, just do nothing:
			return "", nil
		}

		currentlyDeployedVersion, err := s.GetEnvironmentApplicationVersion(environment, application)
		if err != nil {
			return "", err
		}

		if currentlyDeployedVersion != nil && *queuedVersion == *currentlyDeployedVersion {
			// delete queue, it's outdated! But if we can't, that's not really a problem, as it would be overwritten
			// whenever the next deployment happens:
			err = s.DeleteQueuedVersion(environment, application)
			return fmt.Sprintf("deleted queued version %d because it was already deployed. app=%q env=%q", *queuedVersion, application, environment), err
		} else {
			// versions are different, deploy!
			d := DeployApplicationVersion{
				Environment:   environment,
				Application:   application,
				Version:       *queuedVersion,
				LockBehaviour: api.LockBehavior_Fail,
			}
			transform, err := d.Transform(fs)
			if err != nil {
				_, ok := err.(*LockedError)
				if ok {
					/**
					  Usually ProcessQueue is only called when an unlock happened -
					  however, we have 2 kinds of locks, so it might be an unlock of the env,
					  and the app is still locked! (or other way around)
					  In this case, we should skip it.
					*/
					return "", nil
				}
				return "", err
			} else {
				queueDeploymentMessage = fmt.Sprintf("\n%s (was queued)", transform)
			}
		}
	}
	return queueDeploymentMessage, nil
}
