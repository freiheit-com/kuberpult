/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com*/

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	yaml3 "gopkg.in/yaml.v3"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	queueFileName       = "queued_version"
	fieldCreatedAt      = "created_at"
	fieldCreatedByName  = "created_by_name"
	fieldCreatedByEmail = "created_by_email"
	fieldMessage        = "message"
)

func versionToString(Version uint64) string {
	return strconv.FormatUint(Version, 10)
}

func releasesDirectory(fs billy.Filesystem, application string) string {
	return fs.Join("applications", application, "releases")
}

func applicationDirectory(fs billy.Filesystem, application string) string {
	return fs.Join("applications", application)
}

func releasesDirectoryWithVersion(fs billy.Filesystem, application string, version uint64) string {
	return fs.Join(releasesDirectory(fs, application), versionToString(version))
}

func manifestDirectoryWithReleasesVersion(fs billy.Filesystem, application string, version uint64) string {
	return fs.Join(releasesDirectoryWithVersion(fs, application, version), "environments")
}

// A Transformer updates the files in the worktree
type Transformer interface {
	Transform(ctx context.Context, state *State, t TransformerContext, transaction *sql.Tx) (commitMsg string, e error)
	GetDBEventType() db.EventType
}

type TransformerContext interface {
	Execute(t Transformer, transaction *sql.Tx) error
	AddAppEnv(app string, env string, team string)
	DeleteEnvFromApp(app string, env string)
}

func RunTransformer(ctx context.Context, t Transformer, s *State, transaction *sql.Tx) (string, *TransformerResult, error) {
	runner := transformerRunner{
		ChangedApps:     nil,
		DeletedRootApps: nil,
		Commits:         nil,
		Context:         ctx,
		State:           s,
		Stack:           [][]string{nil},
	}
	if err := runner.Execute(t, transaction); err != nil {
		return "", nil, err
	}
	commitMsg := ""
	if len(runner.Stack[0]) > 0 {
		commitMsg = runner.Stack[0][0]
	}
	return commitMsg, &TransformerResult{
		ChangedApps:     runner.ChangedApps,
		DeletedRootApps: runner.DeletedRootApps,
		Commits:         runner.Commits,
	}, nil
}

type transformerRunner struct {
	Context context.Context
	State   *State
	// Stores the current stack of commit messages. Each entry of
	// the outer slice corresponds to a step being executed. Each
	// entry of the inner slices correspond to a message generated
	// by that step.
	Stack           [][]string
	ChangedApps     []AppEnv
	DeletedRootApps []RootApp
	Commits         *CommitIds
}

func (r *transformerRunner) Execute(t Transformer, transaction *sql.Tx) error {
	r.Stack = append(r.Stack, nil)
	msg, err := t.Transform(r.Context, r.State, r, transaction)
	if err != nil {
		return err
	}
	idx := len(r.Stack) - 1
	if len(r.Stack[idx]) != 0 {
		if msg != "" {
			msg = msg + "\n" + strings.Join(r.Stack[idx], "\n")
		} else {
			msg = strings.Join(r.Stack[idx], "\n")
		}
	}
	if msg != "" {
		r.Stack[idx-1] = append(r.Stack[idx-1], msg)
	}
	r.Stack = r.Stack[:idx]
	return nil
}

func (r *transformerRunner) AddAppEnv(app string, env string, team string) {
	r.ChangedApps = append(r.ChangedApps, AppEnv{
		App:  app,
		Env:  env,
		Team: team,
	})
}

func (r *transformerRunner) DeleteEnvFromApp(app string, env string) {
	r.ChangedApps = append(r.ChangedApps, AppEnv{
		Team: "",
		App:  app,
		Env:  env,
	})
	r.DeletedRootApps = append(r.DeletedRootApps, RootApp{
		Env: env,
	})
}

type RawNode struct{ *yaml3.Node }

func (n *RawNode) UnmarshalYAML(node *yaml3.Node) error {
	n.Node = node
	return nil
}

func wrapFileError(e error, filename string, message string) error {
	return fmt.Errorf("%s '%s': %w", message, filename, e)
}

type Authentication struct {
	RBACConfig auth.RBACConfig
}

type QueueApplicationVersion struct {
	Environment string
	Application string
	Version     uint64
}

func (c *QueueApplicationVersion) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	fs := state.Filesystem
	// Create a symlink to the release
	applicationDir := fs.Join("environments", c.Environment, "applications", c.Application)
	if err := fs.MkdirAll(applicationDir, 0777); err != nil {
		return "", err
	}
	queuedVersionFile := fs.Join(applicationDir, queueFileName)
	if err := fs.Remove(queuedVersionFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	releaseDir := releasesDirectoryWithVersion(fs, c.Application, c.Version)
	if err := fs.Symlink(fs.Join("..", "..", "..", "..", releaseDir), queuedVersionFile); err != nil {
		return "", err
	}

	return fmt.Sprintf("Queued version %d of app %q in env %q", c.Version, c.Application, c.Environment), nil
}

type DeployApplicationVersion struct {
	Authentication  `json:"-"`
	Environment     string                          `json:"env"`
	Application     string                          `json:"app"`
	Version         uint64                          `json:"version"`
	LockBehaviour   api.LockBehavior                `json:"lockBehaviour"`
	WriteCommitData bool                            `json:"writeCommitData"`
	SourceTrain     *DeployApplicationVersionSource `json:"sourceTrain"`
	Author          string                          `json:"author"`
}

func (c *DeployApplicationVersion) GetDBEventType() db.EventType {
	return db.EvtDeployApplicationVersion
}

type DeployApplicationVersionSource struct {
	TargetGroup *string `json:"targetGroup"`
	Upstream    string  `json:"upstream"`
}

func (c *DeployApplicationVersion) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	fs := state.Filesystem
	// Check that the release exist and fetch manifest
	releaseDir := releasesDirectoryWithVersion(fs, c.Application, c.Version)
	manifest := fs.Join(releaseDir, "environments", c.Environment, "manifests.yaml")
	var manifestContent []byte
	if file, err := fs.Open(manifest); err != nil {
		return "", wrapFileError(err, manifest, fmt.Sprintf("deployment failed: could not open manifest for app %s with release %d on env %s", c.Application, c.Version, c.Environment))
	} else {
		if content, err := io.ReadAll(file); err != nil {
			return "", err
		} else {
			manifestContent = content
		}
		file.Close()
	}

	if c.LockBehaviour != api.LockBehavior_IGNORE {
		// Check that the environment is not locked
		var (
			envLocks, appLocks, teamLocks map[string]Lock
			err                           error
		)
		envLocks, err = state.GetEnvironmentLocks(c.Environment)
		if err != nil {
			return "", err
		}
		appLocks, err = state.GetEnvironmentApplicationLocks(c.Environment, c.Application)
		if err != nil {
			return "", err
		}

		appDir := applicationDirectory(fs, c.Application)

		team, err := util.ReadFile(fs, fs.Join(appDir, "team"))

		if errors.Is(err, os.ErrNotExist) {
			teamLocks = map[string]Lock{} //If we dont find the team file, there is no team for application, meaning there can't be any team locks
		} else {
			teamLocks, err = state.GetEnvironmentTeamLocks(c.Environment, string(team))
			if err != nil {
				return "", err
			}
		}
		if len(envLocks) > 0 || len(appLocks) > 0 || len(teamLocks) > 0 {
			switch c.LockBehaviour {
			case api.LockBehavior_RECORD:
				q := QueueApplicationVersion{
					Environment: c.Environment,
					Application: c.Application,
					Version:     c.Version,
				}
				return q.Transform(ctx, state, t, nil)
			case api.LockBehavior_FAIL:
				return "", &LockedError{
					EnvironmentApplicationLocks: appLocks,
					EnvironmentLocks:            envLocks,
					TeamLocks:                   teamLocks,
				}
			}
		}
	}

	applicationDir := fs.Join("environments", c.Environment, "applications", c.Application)
	versionFile := fs.Join(applicationDir, "version")

	// Create a symlink to the release
	if err := fs.MkdirAll(applicationDir, 0777); err != nil {
		return "", err
	}
	if err := fs.Remove(versionFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := fs.Symlink(fs.Join("..", "..", "..", "..", releaseDir), versionFile); err != nil {
		return "", err
	}
	// Copy the manifest for argocd
	manifestsDir := fs.Join(applicationDir, "manifests")
	if err := fs.MkdirAll(manifestsDir, 0777); err != nil {
		return "", err
	}
	manifestFilename := fs.Join(manifestsDir, "manifests.yaml")
	// note that the manifest is empty here!
	// but actually it's not quite empty!
	// The function we are using here is `util.WriteFile`. And that does not allow overwriting files with empty content.
	// We work around this unusual behavior by writing a space into the file
	if len(manifestContent) == 0 {
		manifestContent = []byte(" ")
	}
	if err := util.WriteFile(fs, manifestFilename, manifestContent, 0666); err != nil {
		return "", err
	}
	teamOwner, err := state.GetApplicationTeamOwner(c.Application)
	if err != nil {
		return "", err
	}
	t.AddAppEnv(c.Application, c.Environment, teamOwner)

	existingDeployment, err := state.DBHandler.DBSelectDeployment(ctx, transaction, c.Application, c.Environment)
	if err != nil {
		return "", fmt.Errorf("error while retrieving deployment: %v", err)
	}

	logger.FromContext(ctx).Sugar().Warnf("writing deployed name...")
	if err := util.WriteFile(fs, fs.Join(applicationDir, "deployed_by"), []byte(existingDeployment.Metadata.DeployedByName), 0666); err != nil {
		return "", err
	}

	logger.FromContext(ctx).Sugar().Warnf("writing deployed email...")
	if err := util.WriteFile(fs, fs.Join(applicationDir, "deployed_by_email"), []byte(existingDeployment.Metadata.DeployedByEmail), 0666); err != nil {
		return "", err
	}

	logger.FromContext(ctx).Sugar().Warnf("writing deployed at...")
	if err := util.WriteFile(fs, fs.Join(applicationDir, "deployed_at_utc"), []byte(existingDeployment.Created.UTC().String()), 0666); err != nil {
		return "", err
	}

	s := State{
		Commit:                 nil,
		BootstrapMode:          false,
		EnvironmentConfigsPath: "",
		Filesystem:             fs,
		DBHandler:              state.DBHandler,
	}
	err = s.DeleteQueuedVersionIfExists(c.Environment, c.Application)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("deployed version %d of %q to %q", c.Version, c.Application, c.Environment), nil
}

type CreateEnvironmentLock struct {
	Authentication `json:"-"`
	Environment    string `json:"env"`
	LockId         string `json:"lockId"`
	Message        string `json:"message"`
}

func (c *CreateEnvironmentLock) GetDBEventType() db.EventType {
	return db.EvtCreateEnvironmentLock
}

func (c *CreateEnvironmentLock) Transform(
	ctx context.Context,
	state *State,
	_ TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	fs := state.Filesystem
	envDir := fs.Join("environments", c.Environment)
	if _, err := fs.Stat(envDir); err != nil {
		return "", fmt.Errorf("error accessing dir %q: %w", envDir, err)
	}
	chroot, err := fs.Chroot(envDir)
	if err != nil {
		return "", err
	}

	lock, err := state.DBHandler.DBSelectEnvironmentLock(ctx, transaction, c.Environment, c.LockId)
	if err != nil {
		return "", err
	}

	if lock == nil {
		return "", fmt.Errorf("no lock found")
	}
	if err := createLock(ctx, chroot, lock.LockID, lock.Metadata.Message, lock.Metadata.CreatedByName, lock.Metadata.CreatedByEmail, lock.Created.Format(time.RFC3339)); err != nil {
		return "", err
	}

	return fmt.Sprintf("Created lock %q on environment %q", c.LockId, c.Environment), nil
}

func createLock(ctx context.Context, fs billy.Filesystem, lockId, message, authorName, authorEmail, created string) error {
	locksDir := "locks"
	if err := fs.MkdirAll(locksDir, 0777); err != nil {
		return err
	}

	// create lock dir
	newLockDir := fs.Join(locksDir, lockId)
	if err := fs.MkdirAll(newLockDir, 0777); err != nil {
		return err
	}

	// write message
	if err := util.WriteFile(fs, fs.Join(newLockDir, fieldMessage), []byte(message), 0666); err != nil {
		return err
	}

	// write email
	if err := util.WriteFile(fs, fs.Join(newLockDir, fieldCreatedByEmail), []byte(authorEmail), 0666); err != nil {
		return err
	}

	// write name
	if err := util.WriteFile(fs, fs.Join(newLockDir, fieldCreatedByName), []byte(authorName), 0666); err != nil {
		return err
	}

	// write date in iso format
	if err := util.WriteFile(fs, fs.Join(newLockDir, fieldCreatedAt), []byte(created), 0666); err != nil {
		return err
	}
	return nil
}

type DeleteEnvironmentLock struct {
	Authentication `json:"-"`
	Environment    string `json:"env"`
	LockId         string `json:"lockId"`
}

func (c *DeleteEnvironmentLock) GetDBEventType() db.EventType {
	return db.EvtDeleteEnvironmentLock
}

func (c *DeleteEnvironmentLock) Transform(
	ctx context.Context,
	state *State,
	_ TransformerContext,
	_ *sql.Tx,
) (string, error) {
	fs := state.Filesystem
	s := State{
		Commit:                 nil,
		BootstrapMode:          false,
		EnvironmentConfigsPath: "",
		Filesystem:             fs,
		DBHandler:              state.DBHandler,
	}
	lockDir := s.GetEnvLockDir(c.Environment, c.LockId)
	_, err := fs.Stat(lockDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", grpc.FailedPrecondition(ctx, fmt.Errorf("directory %s for env lock does not exist", lockDir))
		}
		return "", err
	}

	if err := fs.Remove(lockDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to delete directory %q: %w", lockDir, err)
	}
	if err := s.DeleteEnvLockIfEmpty(ctx, c.Environment); err != nil {
		return "", err
	}

	return fmt.Sprintf("Deleted lock %q on environment %q", c.LockId, c.Environment), nil
}
