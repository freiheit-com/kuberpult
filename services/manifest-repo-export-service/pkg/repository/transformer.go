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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	yaml3 "gopkg.in/yaml.v3"

	"github.com/freiheit-com/kuberpult/pkg/valid"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	diffspan "github.com/hexops/gotextdiff/span"

	"io"
	"io/fs"
	"os"
	"sort"
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

const (
	yamlParsingError = "# yaml parsing error"

	fieldSourceAuthor   = "source_author"
	fieldSourceMessage  = "source_message"
	fieldSourceCommitId = "source_commit_id"
	fieldDisplayVersion = "display_version"
	fieldTeam           = "team"
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
	teamOwner, err := state.GetApplicationTeamOwner(ctx, transaction, c.Application)
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

	if err != nil && !errors.Is(err, os.ErrNotExist) {
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

type CreateEnvironmentApplicationLock struct {
	Authentication `json:"-"`
	Environment    string `json:"env"`
	Application    string `json:"app"`
	LockId         string `json:"lockId"`
	Message        string `json:"message"`
}

func (c *CreateEnvironmentApplicationLock) GetDBEventType() db.EventType {
	return db.EvtCreateEnvironmentApplicationLock
}

func (c *CreateEnvironmentApplicationLock) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	fs := state.Filesystem
	envDir := fs.Join("environments", c.Environment)
	if _, err := fs.Stat(envDir); err != nil {
		return "", fmt.Errorf("error accessing dir %q: %w", envDir, err)
	}

	appDir := fs.Join(envDir, "applications", c.Application)
	if err := fs.MkdirAll(appDir, 0777); err != nil {
		return "", err
	}

	lock, err := state.DBHandler.DBSelectAppLock(ctx, transaction, c.Environment, c.Application, c.LockId)

	if err != nil {
		return "", err
	}

	if lock == nil {
		return "", fmt.Errorf("no application lock found to create with lock id '%s', for application '%s' on environment '%s'.\n", c.LockId, c.Application, c.Environment)
	}

	chroot, err := fs.Chroot(appDir)
	if err != nil {
		return "", err
	}

	if err := createLock(ctx, chroot, lock.LockID, lock.Metadata.Message, lock.Metadata.CreatedByName, lock.Metadata.CreatedByEmail, lock.Created.Format(time.RFC3339)); err != nil {
		return "", err
	}

	// locks are invisible to argoCd, so no changes here
	return fmt.Sprintf("Created lock %q on environment %q for application %q", c.LockId, c.Environment, c.Application), nil
}

type DeleteEnvironmentApplicationLock struct {
	Authentication `json:"-"`
	Environment    string `json:"env"`
	Application    string `json:"app"`
	LockId         string `json:"lockId"`
}

func (c *DeleteEnvironmentApplicationLock) GetDBEventType() db.EventType {
	return db.EvtDeleteEnvironmentApplicationLock
}

func (c *DeleteEnvironmentApplicationLock) Transform(
	ctx context.Context,
	state *State,
	_ TransformerContext,
	_ *sql.Tx,
) (string, error) {

	fs := state.Filesystem
	queueMessage := ""
	lockDir := fs.Join("environments", c.Environment, "applications", c.Application, "locks", c.LockId)
	_, err := fs.Stat(lockDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := fs.Remove(lockDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to delete directory %q: %w.", lockDir, err)
	}

	queueMessage, err = state.ProcessQueue(ctx, fs, c.Environment, c.Application)
	if err != nil {
		return "", err
	}
	if err := state.DeleteAppLockIfEmpty(ctx, c.Environment, c.Application); err != nil {
		return "", err
	}

	return fmt.Sprintf("Deleted lock %q on environment %q for application %q%s", c.LockId, c.Environment, c.Application, queueMessage), nil
}

type CreateApplicationVersion struct {
	Authentication  `json:"-"`
	Version         uint64            `json:"version"`
	Application     string            `json:"app"`
	Manifests       map[string]string `json:"manifests"`
	SourceCommitId  string            `json:"sourceCommitId"`
	SourceAuthor    string            `json:"sourceCommitAuthor"`
	SourceMessage   string            `json:"sourceCommitMessage"`
	Team            string            `json:"team"`
	DisplayVersion  string            `json:"displayVersion"`
	WriteCommitData bool              `json:"writeCommitData"`
	PreviousCommit  string            `json:"previousCommit"`
}

func (c *CreateApplicationVersion) GetDBEventType() db.EventType {
	return db.EvtCreateApplicationVersion
}

func (c *CreateApplicationVersion) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	version, err := c.calculateVersion(state)
	if err != nil {
		return "", err
	}
	fs := state.Filesystem
	if !valid.ApplicationName(c.Application) {
		return "", GetCreateReleaseAppNameTooLong(c.Application, valid.AppNameRegExp, uint32(valid.MaxAppNameLen))
	}

	releaseDir := releasesDirectoryWithVersion(fs, c.Application, version)
	appDir := applicationDirectory(fs, c.Application)
	if err = fs.MkdirAll(releaseDir, 0777); err != nil {
		return "", GetCreateReleaseGeneralFailure(err)
	}

	var checkForInvalidCommitId = func(commitId, helperText string) {
		if !valid.SHA1CommitID(commitId) {
			logger.FromContext(ctx).
				Sugar().
				Warnf("%s commit ID is not a valid SHA1 hash, should be exactly 40 characters [0-9a-fA-F] %s\n", commitId, helperText)
		}
	}

	checkForInvalidCommitId(c.SourceCommitId, "Source")
	checkForInvalidCommitId(c.PreviousCommit, "Previous")

	if c.Team != "" {
		//util.WriteFile has a bug where it does not truncate the old file content. If two application versions with the same
		//team are deployed, team names simply get concatenated. Just remove the file beforehand.
		//This bug can't be fixed because it is part of the util library
		teamFileLoc := fs.Join(appDir, fieldTeam)
		if _, err := fs.Stat(teamFileLoc); err == nil { //If path to file exists
			err := fs.Remove(teamFileLoc)
			if err != nil {
				return "", GetCreateReleaseGeneralFailure(err)
			}
		}
		if err := util.WriteFile(fs, teamFileLoc, []byte(c.Team), 0666); err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
	}
	isLatest, err := isLatestVersion(state, c.Application, version)
	if err != nil {
		return "", GetCreateReleaseGeneralFailure(err)
	}
	if !isLatest {
		// check that we can actually backfill this version
		oldVersions, err := findOldApplicationVersions(ctx, transaction, state, c.Application)
		if err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
		for _, oldVersion := range oldVersions {
			if version == oldVersion {
				return "", GetCreateReleaseTooOld()
			}
		}
	}

	return fmt.Sprintf("created version %d of %q", version, c.Application), nil
}

func (c *CreateApplicationVersion) calculateVersion(state *State) (uint64, error) {
	bfs := state.Filesystem
	if c.Version == 0 {
		lastRelease, err := GetLastRelease(bfs, c.Application)
		if err != nil {
			return 0, err
		}
		return lastRelease + 1, nil
	} else {
		// check that the version doesn't already exist
		dir := releasesDirectoryWithVersion(bfs, c.Application, c.Version)
		_, err := bfs.Stat(dir)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return 0, err
			}
		} else {
			// check if version differs
			return 0, c.sameAsExisting(state, c.Version)
		}
		// TODO: check GC here
		return c.Version, nil
	}
}

func (c *CreateApplicationVersion) sameAsExisting(state *State, version uint64) error {
	fs := state.Filesystem
	releaseDir := releasesDirectoryWithVersion(fs, c.Application, version)
	appDir := applicationDirectory(fs, c.Application)
	if c.SourceCommitId != "" {
		existingSourceCommitId, err := util.ReadFile(fs, fs.Join(releaseDir, fieldSourceCommitId))
		if err != nil {
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_SOURCE_COMMIT_ID, "")
		}
		existingSourceCommitIdStr := string(existingSourceCommitId)
		if existingSourceCommitIdStr != c.SourceCommitId {
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_SOURCE_COMMIT_ID, createUnifiedDiff(existingSourceCommitIdStr, c.SourceCommitId, ""))
		}
	}
	if c.SourceAuthor != "" {
		existingSourceAuthor, err := util.ReadFile(fs, fs.Join(releaseDir, fieldSourceAuthor))
		if err != nil {
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_SOURCE_AUTHOR, "")
		}
		existingSourceAuthorStr := string(existingSourceAuthor)
		if existingSourceAuthorStr != c.SourceAuthor {
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_SOURCE_AUTHOR, createUnifiedDiff(existingSourceAuthorStr, c.SourceAuthor, ""))
		}
	}
	if c.SourceMessage != "" {
		existingSourceMessage, err := util.ReadFile(fs, fs.Join(releaseDir, fieldSourceMessage))
		if err != nil {
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_SOURCE_MESSAGE, "")
		}
		existingSourceMessageStr := string(existingSourceMessage)
		if existingSourceMessageStr != c.SourceMessage {
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_SOURCE_MESSAGE, createUnifiedDiff(existingSourceMessageStr, c.SourceMessage, ""))
		}
	}
	if c.DisplayVersion != "" {
		existingDisplayVersion, err := util.ReadFile(fs, fs.Join(releaseDir, fieldDisplayVersion))
		if err != nil {
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_DISPLAY_VERSION, "")
		}
		existingDisplayVersionStr := string(existingDisplayVersion)
		if existingDisplayVersionStr != c.DisplayVersion {
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_DISPLAY_VERSION, createUnifiedDiff(existingDisplayVersionStr, c.DisplayVersion, ""))
		}
	}
	if c.Team != "" {
		existingTeam, err := util.ReadFile(fs, fs.Join(appDir, fieldTeam))
		if err != nil {
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_TEAM, "")
		}
		existingTeamStr := string(existingTeam)
		if existingTeamStr != c.Team {
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_TEAM, createUnifiedDiff(existingTeamStr, c.Team, ""))
		}
	}
	for env, man := range c.Manifests {
		envDir := fs.Join(releaseDir, "environments", env)
		existingMan, err := util.ReadFile(fs, fs.Join(envDir, "manifests.yaml"))
		if err != nil {
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_MANIFESTS, fmt.Sprintf("manifest missing for env %s", env))
		}
		existingManStr := string(existingMan)
		if canonicalizeYaml(existingManStr) != canonicalizeYaml(man) {
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_MANIFESTS, createUnifiedDiff(existingManStr, man, fmt.Sprintf("%s-", env)))
		}
	}
	return GetCreateReleaseAlreadyExistsSame()
}

func canonicalizeYaml(unformatted string) string {
	var target RawNode
	if errDeserial := yaml3.Unmarshal([]byte(unformatted), &target); errDeserial != nil {
		return yamlParsingError // we only use this for comparisons
	}
	if canonicalData, errSerial := yaml3.Marshal(target.Node); errSerial == nil {
		return string(canonicalData)
	} else {
		return yamlParsingError // only for comparisons
	}
}

func createUnifiedDiff(existingValue string, requestValue string, prefix string) string {
	existingValueStr := string(existingValue)
	existingFilename := fmt.Sprintf("%sexisting", prefix)
	requestFilename := fmt.Sprintf("%srequest", prefix)
	edits := myers.ComputeEdits(diffspan.URIFromPath(existingFilename), existingValueStr, string(requestValue))
	return fmt.Sprint(gotextdiff.ToUnified(existingFilename, requestFilename, existingValueStr, edits))
}

func isLatestVersion(state *State, application string, version uint64) (bool, error) {
	rels, err := state.GetApplicationReleases(application)
	if err != nil {
		return false, err
	}
	for _, r := range rels {
		if r > version {
			return false, nil
		}
	}
	return true, nil
}

const (
	releaseVersionsLimit = 20
)

// Finds old releases for an application
func findOldApplicationVersions(ctx context.Context, transaction *sql.Tx, state *State, name string) ([]uint64, error) {
	// 1) get release in each env:
	envConfigs, err := state.GetEnvironmentConfigs()
	if err != nil {
		return nil, err
	}
	versions, err := state.GetApplicationReleases(name)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, err
	}
	sort.Slice(versions, func(i, j int) bool {
		return versions[i] < versions[j]
	})
	// Use the latest version as oldest deployed version
	oldestDeployedVersion := versions[len(versions)-1]
	for env := range envConfigs {
		version, err := state.GetEnvironmentApplicationVersion(env, name)
		if err != nil {
			return nil, err
		}
		if version != nil {
			if *version < oldestDeployedVersion {
				oldestDeployedVersion = *version
			}
		}
	}
	positionOfOldestVersion := sort.Search(len(versions), func(i int) bool {
		return versions[i] >= oldestDeployedVersion
	})

	if positionOfOldestVersion < (int(releaseVersionsLimit) - 1) {
		return nil, nil
	}
	return versions[0 : positionOfOldestVersion-(int(releaseVersionsLimit)-1)], err
}

func GetLastRelease(fs billy.Filesystem, application string) (uint64, error) {
	var err error
	releasesDir := releasesDirectory(fs, application)
	err = fs.MkdirAll(releasesDir, 0777)
	if err != nil {
		return 0, err
	}
	if entries, err := fs.ReadDir(releasesDir); err != nil {
		return 0, err
	} else {
		var lastRelease uint64 = 0
		for _, e := range entries {
			if i, err := strconv.ParseUint(e.Name(), 10, 64); err != nil {
				//TODO(HVG): decide what to do with bad named releases
			} else {
				if i > lastRelease {
					lastRelease = i
				}
			}
		}
		return lastRelease, nil
	}
}

type CreateEnvironmentTeamLock struct {
	Authentication `json:"-"`
	Environment    string `json:"env"`
	Team           string `json:"team"`
	LockId         string `json:"lockId"`
	Message        string `json:"message"`
}

func (c *CreateEnvironmentTeamLock) GetDBEventType() db.EventType {
	return db.EvtCreateEnvironmentTeamLock
}

func (c *CreateEnvironmentTeamLock) Transform(
	ctx context.Context,
	state *State,
	_ TransformerContext,
	tx *sql.Tx,
) (string, error) {

	if !valid.EnvironmentName(c.Environment) {
		return "", status.Error(codes.InvalidArgument, fmt.Sprintf("cannot create environment team lock: invalid environment: '%s'", c.Environment))
	}
	if !valid.TeamName(c.Team) {
		return "", status.Error(codes.InvalidArgument, fmt.Sprintf("cannot create environment team lock: invalid team: '%s'", c.Team))
	}
	if !valid.LockId(c.LockId) {
		return "", status.Error(codes.InvalidArgument, fmt.Sprintf("cannot create environment team lock: invalid lock id: '%s'", c.LockId))
	}

	fs := state.Filesystem

	foundTeam := false
	var err error
	if apps, err := state.GetApplicationsFromFile(); err == nil {
		for _, currentApp := range apps {
			currentTeamName, err := state.GetTeamName(currentApp)
			if err != nil {
				logger.FromContext(ctx).Sugar().Warnf("CreateEnvironmentTeamLock: Could not find team for application: %s.", currentApp)
			} else {
				if c.Team == currentTeamName {
					foundTeam = true
					break
				}
			}
		}
	}
	if err != nil || !foundTeam { //Not found team or apps dir doesn't exist
		return "", &TeamNotFoundErr{err: fmt.Errorf("team '%s' does not exist", c.Team)}
	}

	envDir := fs.Join("environments", c.Environment)
	if _, err := fs.Stat(envDir); err != nil {
		return "", fmt.Errorf("error environment not found dir %q: %w", envDir, err)
	}

	teamDir := fs.Join(envDir, "teams", c.Team)
	if err := fs.MkdirAll(teamDir, 0777); err != nil {
		return "", fmt.Errorf("error could not create teams directory %q: %w", envDir, err)
	}
	chroot, err := fs.Chroot(teamDir)
	if err != nil {
		return "", fmt.Errorf("error changing root of fs to  %s: %w", teamDir, err)
	}

	lock, err := state.DBHandler.DBSelectTeamLock(ctx, tx, c.Environment, c.Team, c.LockId)
	if err != nil {
		return "", err
	}

	if lock == nil {
		return "", fmt.Errorf("could not write team lock information to manifest. No team lock found on database for team '%s' on environment '%s' with ID '%s'.\n", c.Team, c.Environment, c.LockId)
	}

	if err := createLock(ctx, chroot, lock.LockID, lock.Metadata.Message, lock.Metadata.CreatedByName, lock.Metadata.CreatedByEmail, lock.Created.Format(time.RFC3339)); err != nil {
		return "", err
	}

	return fmt.Sprintf("Created lock %q on environment %q for team %q", c.LockId, c.Environment, c.Team), nil
}

type DeleteEnvironmentTeamLock struct {
	Authentication `json:"-"`
	Environment    string `json:"env"`
	Team           string `json:"team"`
	LockId         string `json:"lockId"`
}

func (c *DeleteEnvironmentTeamLock) GetDBEventType() db.EventType {
	return db.EvtDeleteEnvironmentTeamLock
}

func (c *DeleteEnvironmentTeamLock) Transform(
	ctx context.Context,
	state *State,
	_ TransformerContext,
	_ *sql.Tx,
) (string, error) {
	if !valid.EnvironmentName(c.Environment) {
		return "", status.Error(codes.InvalidArgument, fmt.Sprintf("cannot delete environment team lock: invalid environment: '%s'", c.Environment))
	}
	if !valid.TeamName(c.Team) {
		return "", status.Error(codes.InvalidArgument, fmt.Sprintf("cannot delete environment team lock: invalid team: '%s'", c.Team))
	}
	if !valid.LockId(c.LockId) {
		return "", status.Error(codes.InvalidArgument, fmt.Sprintf("cannot delete environment team lock: invalid lock id: '%s'", c.LockId))
	}
	fs := state.Filesystem

	lockDir := fs.Join("environments", c.Environment, "teams", c.Team, "locks", c.LockId)
	_, err := fs.Stat(lockDir)

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if err := fs.Remove(lockDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to delete directory %q: %w", lockDir, err)
	}

	if err := state.DeleteTeamLockIfEmpty(ctx, c.Environment, c.Team); err != nil {
		return "", err
	}

	return fmt.Sprintf("Deleted lock %q on environment %q for team %q", c.LockId, c.Environment, c.Team), nil
}
