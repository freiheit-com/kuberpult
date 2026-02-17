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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	yaml3 "gopkg.in/yaml.v3"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/conversion"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/sorting"
	time2 "github.com/freiheit-com/kuberpult/pkg/time"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"github.com/freiheit-com/kuberpult/pkg/uuid"
	"github.com/freiheit-com/kuberpult/pkg/valid"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/argocd"
)

const (
	queueFileName         = "queued_version"
	fieldCreatedAt        = "created_at"
	fieldCreatedByName    = "created_by_name"
	fieldCreatedByEmail   = "created_by_email"
	fieldSourceCommitId   = "source_commit_id"
	fieldDisplayVersion   = "display_version"
	fieldMessage          = "message"
	fieldSourceMessage    = "source_message"
	fieldSourceAuthor     = "source_author"
	fieldNextCommidId     = "nextCommit"
	fieldPreviousCommitId = "previousCommit"
	keptVersionsOnCleanup = 20
)

const (
	fieldTeam = "team"
)

type ctxMarkerGenerateUuid struct{}

var (
	ctxMarkerGenerateUuidKey = &ctxMarkerGenerateUuid{}
)

func versionToString(Version types.ReleaseNumbers) string {
	return fmt.Sprintf("%v", Version)
}

// releasesDirectory returns applications/<app>/releases/
func releasesDirectory(fs billy.Filesystem, application string) string {
	return fs.Join("applications", application, "releases")
}

// applicationDirectory returns applications/<app>/
func applicationDirectory(fs billy.Filesystem, application string) string {
	return fs.Join("applications", application)
}

// environmentApplicationDirectory returns environments/<env>/applications/<app>/
func environmentApplicationDirectory(fs billy.Filesystem, environment types.EnvName, application string) string {
	return fs.Join("environments", string(environment), "applications", application)
}

// releasesDirectoryWithVersion returns applications/<app>/releases/<version>
func releasesDirectoryWithVersion(fs billy.Filesystem, application string, version types.ReleaseNumbers) string {
	return fs.Join(releasesDirectory(fs, application), versionToString(version))
}

// releasesDirectoryWithVersion returns applications/<app>/releases/<version>
func releasesDirectoryWithVersionNumber(fs billy.Filesystem, application string, version string) string {
	return fs.Join(releasesDirectory(fs, application), version)
}

func (s *State) checkWhichVersionDirectoryExists(fs billy.Filesystem, application string, version types.ReleaseNumbers) (string, error) {
	revisionedVersion := releasesDirectoryWithVersion(fs, application, version)
	_, err := s.Filesystem.Stat(revisionedVersion)
	if err != nil {
		num := strconv.Itoa(int(*version.Version)) //Check for old release version
		v := releasesDirectoryWithVersionNumber(fs, application, num)
		_, err = fs.Stat(v)
		if err != nil {
			return "", err
		}
		return v, nil
	}
	return revisionedVersion, nil
}

func releasesDirectoryWithVersionWithoutRevision(fs billy.Filesystem, application string, version string) string {
	return fs.Join(releasesDirectory(fs, application), version)
}

// environmentApplicationDirectory returns applications/<app>/releases/<version>/environments/
func manifestDirectoryWithReleasesVersion(fs billy.Filesystem, application string, version types.ReleaseNumbers) string {
	return fs.Join(releasesDirectoryWithVersion(fs, application, version), "environments")
}

func AddGeneratorToContext(ctx context.Context, gen uuid.GenerateUUIDs) context.Context {
	return context.WithValue(ctx, ctxMarkerGenerateUuidKey, gen)
}

// A Transformer updates the files in the worktree
type Transformer interface {
	Transform(ctx context.Context, state *State, t TransformerContext, transaction *sql.Tx) (commitMsg string, e error)
	GetDBEventType() db.EventType
	GetMetadata() *TransformerMetadata
	GetEslVersion() db.TransformerID
	SetEslVersion(id db.TransformerID)
	GetGitTag() types.GitTag
	SetCreationTimestamp(ts time.Time)
	GetCreationTimestamp() time.Time
}

type TransformerContext interface {
	Execute(t Transformer, transaction *sql.Tx) error
	AddAppEnv(app string, env types.EnvName)
	DeleteEnvFromApp(app string, env types.EnvName)
	ShouldMinimizeGitData() bool
	ShouldMaximizeGitData() bool
	ChangeEnvironment(environment types.EnvName)
}

type TransformerMetadata struct {
	AuthorName  string `json:"authorName,omitempty"`
	AuthorEmail string `json:"authorEmail,omitempty"`
}

func (t *TransformerMetadata) GetMetadata() *TransformerMetadata {
	return t
}

const NoOpMessage = "Empty Commit\nNo files changed in"

func GetNoOpMessage(t Transformer) (string, error) {
	evt := t.GetDBEventType()
	return fmt.Sprintf("%s %s", NoOpMessage, evt), nil
}

func RunTransformer(ctx context.Context, t Transformer, s *State, transaction *sql.Tx, minimizeExportedData bool) (string, *TransformerResult, error) {
	runner := transformerRunner{
		ChangedApps:         nil,
		ChangedEnvironments: nil,
		Commits:             nil,
		Context:             ctx,
		State:               s,
		Stack:               [][]string{nil},

		MinimizeGitData: minimizeExportedData,
	}
	if err := runner.Execute(t, transaction); err != nil {
		return "", nil, err
	}

	commitMsg := ""
	if len(runner.Stack[0]) > 0 {
		commitMsg = runner.Stack[0][0]
	}
	return commitMsg, &TransformerResult{
		AppEnvsToRender:      runner.ChangedApps,
		EnvironmentsToRender: runner.ChangedEnvironments,
		Commits:              runner.Commits,
	}, nil
}

type transformerRunner struct {
	Context context.Context
	State   *State
	// Stores the current stack of commit messages. Each entry of
	// the outer slice corresponds to a step being executed. Each
	// entry of the inner slices correspond to a message generated
	// by that step.
	Stack       [][]string
	ChangedApps []AppEnvToRender

	// ChangedEnvironments contains all environments that were added or changed, but not deleted
	ChangedEnvironments []EnvironmentToRender
	Commits             *CommitIds

	MinimizeGitData bool
}

var _ TransformerContext = &transformerRunner{} // ensure interface is implemented

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

func (r *transformerRunner) ChangeEnvironment(environment types.EnvName) {
	r.ChangedEnvironments = append(r.ChangedEnvironments, EnvironmentToRender{environment})
}

func (r *transformerRunner) AddAppEnv(app string, env types.EnvName) {
	r.ChangedApps = append(r.ChangedApps, AppEnvToRender{
		App: app,
		Env: env,
	})
}

func (r *transformerRunner) DeleteEnvFromApp(app string, env types.EnvName) {
	r.ChangedApps = append(r.ChangedApps, AppEnvToRender{
		App: app,
		Env: env,
	})
}

func (r *transformerRunner) ShouldMinimizeGitData() bool {
	return r.MinimizeGitData
}

func (r *transformerRunner) ShouldMaximizeGitData() bool {
	return !r.MinimizeGitData
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
	Environment           types.EnvName
	Application           string
	Version               uint64
	Revision              uint64
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
}

func (c *QueueApplicationVersion) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *QueueApplicationVersion) SetEslVersion(eslVersion db.TransformerID) {
	c.TransformerEslVersion = eslVersion
}

func (c *QueueApplicationVersion) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	fs := state.Filesystem
	// Create a symlink to the release
	applicationDir := fs.Join("environments", string(c.Environment), "applications", c.Application)
	if err := fs.MkdirAll(applicationDir, 0777); err != nil {
		return "", err
	}
	queuedVersionFile := fs.Join(applicationDir, queueFileName)
	if err := fs.Remove(queuedVersionFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	releaseDir := releasesDirectoryWithVersion(fs, c.Application, types.MakeReleaseNumbers(c.Version, c.Revision))
	if err := fs.Symlink(fs.Join("..", "..", "..", "..", releaseDir), queuedVersionFile); err != nil {
		return "", err
	}

	return fmt.Sprintf("Queued version %d of app %q in env %q", c.Version, c.Application, c.Environment), nil
}

type DeployApplicationVersion struct {
	Authentication        `json:"-"`
	TransformerMetadata   `json:"metadata"`
	Environment           types.EnvName                   `json:"env"`
	Application           string                          `json:"app"`
	Version               uint64                          `json:"version"`
	Revision              uint64                          `json:"revision"`
	LockBehaviour         api.LockBehavior                `json:"lockBehaviour"`
	WriteCommitData       bool                            `json:"writeCommitData"`
	SourceTrain           *DeployApplicationVersionSource `json:"sourceTrain"`
	Author                string                          `json:"author"`
	TransformerEslVersion db.TransformerID                `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	CreationTimestamp     time.Time                       `json:"-"`
}

func (c *DeployApplicationVersion) GetCreationTimestamp() time.Time {
	return c.CreationTimestamp
}

func (c *DeployApplicationVersion) SetCreationTimestamp(ts time.Time) {
	c.CreationTimestamp = ts
}

var _ Transformer = &DeployApplicationVersion{} // ensure it implements Transformer

func (c *DeployApplicationVersion) GetGitTag() types.GitTag {
	return ""
}

func (c *DeployApplicationVersion) GetDBEventType() db.EventType {
	return db.EvtDeployApplicationVersion
}

func (c *DeployApplicationVersion) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *DeployApplicationVersion) SetEslVersion(eslVersion db.TransformerID) {
	c.TransformerEslVersion = eslVersion
}

type DeployApplicationVersionSource struct {
	TargetGroup *string `json:"targetGroup"`
	Upstream    string  `json:"upstream"`
}

func (c *DeployApplicationVersion) Transform(
	ctx context.Context,
	state *State,
	tCtx TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DeployApplicationVersion::Transform")
	defer span.Finish()
	span.SetTag("application", c.Application)
	span.SetTag("version", c.Version)
	span.SetTag("revision", c.Revision)
	span.SetTag("environment", c.Environment)
	envName := c.Environment
	fsys := state.Filesystem
	// Check that the release exist and fetch manifest
	releaseDir := releasesDirectoryWithVersion(fsys, c.Application, types.MakeReleaseNumbers(c.Version, c.Revision))
	version, err := state.DBHandler.DBSelectReleaseByVersion(ctx, transaction, types.AppName(c.Application), types.ReleaseNumbers{Version: &c.Version, Revision: c.Revision}, true)
	if err != nil {
		return "", err
	}
	if version == nil {
		return "", fmt.Errorf("release of app %s with version %v (revision %v) not found", c.Application, c.Version, c.Revision)
	}
	var manifestContent = []byte(version.Manifests.Manifests[envName])

	applicationDir := fsys.Join("environments", string(c.Environment), "applications", c.Application)
	// Create a symlink to the release
	if err := fsys.MkdirAll(applicationDir, 0777); err != nil {
		return "", err
	}
	versionFile := fsys.Join(applicationDir, "version")
	if err := fsys.Remove(versionFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if tCtx.ShouldMaximizeGitData() {
		if err := fsys.Symlink(fsys.Join("..", "..", "..", "..", releaseDir), versionFile); err != nil {
			return "", err
		}
	}

	// Copy the manifest for argocd
	manifestsDir := fsys.Join(applicationDir, "manifests")
	if err := fsys.MkdirAll(manifestsDir, 0777); err != nil {
		return "", err
	}
	manifestFilename := fsys.Join(manifestsDir, "manifests.yaml")
	// note that the manifest is empty here!
	// but actually it's not quite empty!
	// The function we are using here is `util.WriteFile`. And that does not allow overwriting files with empty content.
	// We work around this unusual behavior by writing a space into the file
	if len(manifestContent) == 0 {
		manifestContent = []byte(" ")
	}
	if err := util.WriteFile(fsys, manifestFilename, manifestContent, 0666); err != nil {
		return "", err
	}

	tCtx.AddAppEnv(c.Application, c.Environment)

	existingDeployment, err := state.DBHandler.DBSelectLatestDeployment(ctx, transaction, types.AppName(c.Application), envName)
	if err != nil {
		return "", fmt.Errorf("error while retrieving deployment: %v", err)
	}
	if existingDeployment == nil {
		return "", nil
	}
	if tCtx.ShouldMaximizeGitData() {
		if err := util.WriteFile(fsys, fsys.Join(applicationDir, "deployed_by"), []byte(existingDeployment.Metadata.DeployedByName), 0666); err != nil {
			return "", err
		}

		if err := util.WriteFile(fsys, fsys.Join(applicationDir, "deployed_by_email"), []byte(existingDeployment.Metadata.DeployedByEmail), 0666); err != nil {
			return "", err
		}

		if err := util.WriteFile(fsys, fsys.Join(applicationDir, "deployed_at_utc"), []byte(existingDeployment.Created.UTC().String()), 0666); err != nil {
			return "", err
		}
	}

	err = state.DeleteQueuedVersionIfExists(envName, c.Application)
	if err != nil {
		return "", err
	}

	d := &CleanupOldApplicationVersions{
		Application: c.Application,
		TransformerMetadata: TransformerMetadata{
			AuthorName:  existingDeployment.Metadata.DeployedByName,
			AuthorEmail: existingDeployment.Metadata.DeployedByEmail,
		},
		TransformerEslVersion: c.TransformerEslVersion,
	}
	if err := tCtx.Execute(d, transaction); err != nil {
		return "", err
	}
	return fmt.Sprintf("deployed version %v of %q to %q", types.MakeReleaseNumbers(c.Version, c.Revision), c.Application, c.Environment), nil
}

type CreateEnvironmentLock struct {
	Authentication        `json:"-"`
	TransformerMetadata   `json:"metadata"`
	Environment           types.EnvName    `json:"env"`
	LockId                string           `json:"lockId"`
	Message               string           `json:"message"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	CreationTimestamp     time.Time        `json:"-"`
}

func (c *CreateEnvironmentLock) GetCreationTimestamp() time.Time {
	return c.CreationTimestamp
}

func (c *CreateEnvironmentLock) SetCreationTimestamp(ts time.Time) {
	c.CreationTimestamp = ts
}

var _ Transformer = &CreateEnvironmentLock{} // ensure it implements Transformer

func (c *CreateEnvironmentLock) GetGitTag() types.GitTag {
	return ""
}

func (c *CreateEnvironmentLock) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *CreateEnvironmentLock) SetEslVersion(eslVersion db.TransformerID) {
	c.TransformerEslVersion = eslVersion
}

func (c *CreateEnvironmentLock) GetDBEventType() db.EventType {
	return db.EvtCreateEnvironmentLock
}

func (c *CreateEnvironmentLock) Transform(
	ctx context.Context,
	state *State,
	tCtx TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	if tCtx.ShouldMinimizeGitData() {
		return GetNoOpMessage(c)
	}
	fs := state.Filesystem
	envDir := fs.Join("environments", string(c.Environment))
	if _, err := fs.Stat(envDir); err != nil {
		return "", fmt.Errorf("could not access environment information on: '%s': %w", envDir, err)
	}
	chroot, err := fs.Chroot(envDir)
	if err != nil {
		return "", err
	}

	lock, err := state.DBHandler.DBSelectEnvLock(ctx, transaction, c.Environment, c.LockId)
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
	Authentication        `json:"-"`
	TransformerMetadata   `json:"metadata"`
	Environment           types.EnvName    `json:"env"`
	LockId                string           `json:"lockId"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	CreationTimestamp     time.Time        `json:"-"`
}

func (c *DeleteEnvironmentLock) GetCreationTimestamp() time.Time {
	return c.CreationTimestamp
}

func (c *DeleteEnvironmentLock) SetCreationTimestamp(ts time.Time) {
	c.CreationTimestamp = ts
}

var _ Transformer = &DeleteEnvironmentLock{} // ensure it implements Transformer

func (c *DeleteEnvironmentLock) GetGitTag() types.GitTag {
	return ""
}

func (c *DeleteEnvironmentLock) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *DeleteEnvironmentLock) SetEslVersion(eslVersion db.TransformerID) {
	c.TransformerEslVersion = eslVersion
}

func (c *DeleteEnvironmentLock) GetDBEventType() db.EventType {
	return db.EvtDeleteEnvironmentLock
}

func (c *DeleteEnvironmentLock) Transform(
	ctx context.Context,
	state *State,
	tCtx TransformerContext,
	_ *sql.Tx,
) (string, error) {
	envName := c.Environment
	if tCtx.ShouldMinimizeGitData() {
		return GetNoOpMessage(c)
	}
	fs := state.Filesystem
	s := State{
		Commit:               nil,
		Filesystem:           fs,
		ReleaseVersionsLimit: state.ReleaseVersionsLimit,
		DBHandler:            state.DBHandler,
	}
	lockDir := s.GetEnvLockDir(envName, c.LockId)
	_, err := fs.Stat(lockDir)

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if err := fs.Remove(lockDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to delete directory %q: %w", lockDir, err)
	}
	if err := s.DeleteEnvLockIfEmpty(ctx, envName); err != nil {
		return "", err
	}

	return fmt.Sprintf("Deleted lock %q on environment %q", c.LockId, c.Environment), nil
}

type CreateEnvironmentApplicationLock struct {
	Authentication        `json:"-"`
	TransformerMetadata   `json:"metadata"`
	Environment           types.EnvName    `json:"env"`
	Application           string           `json:"app"`
	LockId                string           `json:"lockId"`
	Message               string           `json:"message"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	CreationTimestamp     time.Time        `json:"-"`
}

func (c *CreateEnvironmentApplicationLock) GetCreationTimestamp() time.Time {
	return c.CreationTimestamp
}

func (c *CreateEnvironmentApplicationLock) SetCreationTimestamp(ts time.Time) {
	c.CreationTimestamp = ts
}

var _ Transformer = &CreateEnvironmentApplicationLock{} // ensure it implements Transformer

func (c *CreateEnvironmentApplicationLock) GetGitTag() types.GitTag {
	return ""
}

func (c *CreateEnvironmentApplicationLock) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *CreateEnvironmentApplicationLock) SetEslVersion(eslVersion db.TransformerID) {
	c.TransformerEslVersion = eslVersion
}

func (c *CreateEnvironmentApplicationLock) GetDBEventType() db.EventType {
	return db.EvtCreateEnvironmentApplicationLock
}

func (c *CreateEnvironmentApplicationLock) Transform(
	ctx context.Context,
	state *State,
	tCtx TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	if tCtx.ShouldMinimizeGitData() {
		return GetNoOpMessage(c)
	}
	env := c.Environment
	fs := state.Filesystem
	envDir := fs.Join("environments", string(c.Environment))
	if _, err := fs.Stat(envDir); err != nil {
		return "", fmt.Errorf("error accessing dir %q: %w", envDir, err)
	}

	appDir := fs.Join(envDir, "applications", c.Application)
	if err := fs.MkdirAll(appDir, 0777); err != nil {
		return "", err
	}

	lock, err := state.DBHandler.DBSelectAppLock(ctx, transaction, env, types.AppName(c.Application), c.LockId)

	if err != nil {
		return "", err
	}

	if lock == nil {
		return "", fmt.Errorf("no application lock found to create with lock id '%s', for application '%s' on environment '%s'", c.LockId, c.Application, c.Environment)
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
	Authentication        `json:"-"`
	TransformerMetadata   `json:"metadata"`
	Environment           types.EnvName    `json:"env"`
	Application           string           `json:"app"`
	LockId                string           `json:"lockId"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	CreationTimestamp     time.Time        `json:"-"`
}

func (c *DeleteEnvironmentApplicationLock) GetCreationTimestamp() time.Time {
	return c.CreationTimestamp
}

func (c *DeleteEnvironmentApplicationLock) SetCreationTimestamp(ts time.Time) {
	c.CreationTimestamp = ts
}

var _ Transformer = &DeleteEnvironmentApplicationLock{} // ensure it implements Transformer

func (c *DeleteEnvironmentApplicationLock) GetGitTag() types.GitTag {
	return ""
}

func (c *DeleteEnvironmentApplicationLock) GetDBEventType() db.EventType {
	return db.EvtDeleteEnvironmentApplicationLock
}

func (c *DeleteEnvironmentApplicationLock) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *DeleteEnvironmentApplicationLock) SetEslVersion(eslVersion db.TransformerID) {
	c.TransformerEslVersion = eslVersion
}

func (c *DeleteEnvironmentApplicationLock) Transform(
	ctx context.Context,
	state *State,
	tCtx TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	if tCtx.ShouldMinimizeGitData() {
		return GetNoOpMessage(c)
	}
	fs := state.Filesystem
	queueMessage := ""
	lockDir := fs.Join("environments", string(c.Environment), "applications", c.Application, "locks", c.LockId)
	_, err := fs.Stat(lockDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := fs.Remove(lockDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to delete directory %q: %w", lockDir, err)
	}

	queueMessage, err = state.ProcessQueue(ctx, transaction, fs, c.Environment, c.Application)
	if err != nil {
		return "", err
	}
	if err := state.DeleteAppLockIfEmpty(ctx, c.Environment, c.Application); err != nil {
		return "", err
	}

	return fmt.Sprintf("Deleted lock %q on environment %q for application %q%s", c.LockId, c.Environment, c.Application, queueMessage), nil
}

type CreateApplicationVersion struct {
	Authentication        `json:"-"`
	TransformerMetadata   `json:"metadata"`
	Version               uint64                   `json:"version"`
	Revision              uint64                   `json:"revision"`
	Application           string                   `json:"app"`
	Manifests             map[types.EnvName]string `json:"manifests"`
	SourceCommitId        string                   `json:"sourceCommitId"`
	SourceAuthor          string                   `json:"sourceCommitAuthor"`
	SourceMessage         string                   `json:"sourceCommitMessage"`
	Team                  string                   `json:"team"`
	DisplayVersion        string                   `json:"displayVersion"`
	WriteCommitData       bool                     `json:"writeCommitData"`
	PreviousCommit        string                   `json:"previousCommit"`
	TransformerEslVersion db.TransformerID         `json:"-"`
	CreationTimestamp     time.Time                `json:"-"`
}

func (c *CreateApplicationVersion) GetCreationTimestamp() time.Time {
	return c.CreationTimestamp
}

func (c *CreateApplicationVersion) SetCreationTimestamp(ts time.Time) {
	c.CreationTimestamp = ts
}

var _ Transformer = &CreateApplicationVersion{} // ensure it implements Transformer

func (c *CreateApplicationVersion) GetGitTag() types.GitTag {
	return ""
}

func (c *CreateApplicationVersion) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *CreateApplicationVersion) SetEslVersion(eslVersion db.TransformerID) {
	c.TransformerEslVersion = eslVersion
}

func (c *CreateApplicationVersion) GetDBEventType() db.EventType {
	return db.EvtCreateApplicationVersion
}

func (c *CreateApplicationVersion) Transform(
	ctx context.Context,
	state *State,
	tCtx TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	version := types.MakeReleaseNumbers(c.Version, c.Revision)
	fs := state.Filesystem

	releaseDir := releasesDirectoryWithVersion(fs, c.Application, version)
	appDir := applicationDirectory(fs, c.Application)

	if tCtx.ShouldMaximizeGitData() {
		if err := fs.MkdirAll(releaseDir, 0777); err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
	}

	var checkForInvalidCommitId = func(commitId, helperText string) {
		if !valid.SHA1CommitID(commitId) {
			logger.FromContext(ctx).
				Sugar().
				Warnf("%s commit ID is not a valid SHA1 hash, should be exactly 40 characters [0-9a-fA-F] %s", commitId, helperText)
		}
	}

	checkForInvalidCommitId(c.SourceCommitId, "Source")
	checkForInvalidCommitId(c.PreviousCommit, "Previous")

	if tCtx.ShouldMaximizeGitData() {
		if c.SourceCommitId != "" {
			c.SourceCommitId = strings.ToLower(c.SourceCommitId)
			if err := util.WriteFile(fs, fs.Join(releaseDir, fieldSourceCommitId), []byte(c.SourceCommitId), 0666); err != nil {
				return "", GetCreateReleaseGeneralFailure(err)
			}
		}

		if c.SourceAuthor != "" {
			if err := util.WriteFile(fs, fs.Join(releaseDir, fieldSourceAuthor), []byte(c.SourceAuthor), 0666); err != nil {
				return "", GetCreateReleaseGeneralFailure(err)
			}
		}
		if c.SourceMessage != "" {
			if err := util.WriteFile(fs, fs.Join(releaseDir, fieldSourceMessage), []byte(c.SourceMessage), 0666); err != nil {
				return "", GetCreateReleaseGeneralFailure(err)
			}
		}
		if c.DisplayVersion != "" {
			if err := util.WriteFile(fs, fs.Join(releaseDir, fieldDisplayVersion), []byte(c.DisplayVersion), 0666); err != nil {
				return "", GetCreateReleaseGeneralFailure(err)
			}
		}
		if err := util.WriteFile(fs, fs.Join(releaseDir, fieldCreatedAt), []byte(time2.GetTimeNow(ctx).Format(time.RFC3339)), 0666); err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}

		if c.Team != "" {
			//util.WriteFile has a bug where it does not truncate the old file content. If two application versions with the same
			//team are deployed, team names simply get concatenated. Just remove the file beforehand.
			//This bug can'tCtx be fixed because it is part of the util library
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
	}

	var allEnvsOfThisApp []types.EnvName = nil

	for env := range c.Manifests {
		allEnvsOfThisApp = append(allEnvsOfThisApp, env)
	}
	slices.Sort(allEnvsOfThisApp)

	if c.WriteCommitData && tCtx.ShouldMaximizeGitData() {
		ev, err := state.DBHandler.DBSelectAllCommitEventsForTransformer(ctx, transaction, c.TransformerEslVersion, event.EventTypeNewRelease, 1)
		if err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
		if len(ev) == 0 {
			return "", fmt.Errorf("no new release event to read from database for application '%s'", c.Application)
		}

		err = writeCommitData(ctx, c.SourceCommitId, c.SourceMessage, c.Application, c.PreviousCommit, state)
		if err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
	}

	deploymentsMap, err := state.DBHandler.MapEnvNamesToDeployment(ctx, transaction, c.TransformerEslVersion)
	if err != nil {
		return "", err
	}
	sortedKeys := sorting.SortKeys(c.Manifests)
	for i := range sortedKeys {
		env := sortedKeys[i]
		man := c.Manifests[env]

		envDir := fs.Join(releaseDir, "environments", string(env))

		if tCtx.ShouldMaximizeGitData() {
			if err = fs.MkdirAll(envDir, 0777); err != nil {
				return "", GetCreateReleaseGeneralFailure(err)
			}
			if err := util.WriteFile(fs, fs.Join(envDir, "manifests.yaml"), []byte(man), 0666); err != nil {
				return "", GetCreateReleaseGeneralFailure(err)
			}
		}

		if _, exists := deploymentsMap[env]; exists { //If this transformer did not generate any deployments, skip the deployment transformer
			d := &DeployApplicationVersion{
				SourceTrain:           nil,
				Environment:           env,
				Application:           c.Application,
				Version:               *version.Version,
				LockBehaviour:         api.LockBehavior_IGNORE,
				Authentication:        c.Authentication,
				WriteCommitData:       c.WriteCommitData,
				Author:                c.SourceAuthor,
				TransformerEslVersion: c.TransformerEslVersion,
				TransformerMetadata: TransformerMetadata{
					AuthorName:  c.SourceAuthor,
					AuthorEmail: "",
				},
				Revision: version.Revision,
			}
			err = tCtx.Execute(d, transaction)
			if err != nil {
				return "", GetCreateReleaseGeneralFailure(err)
			}
		}
	}

	if tCtx.ShouldMinimizeGitData() && len(deploymentsMap) == 0 {
		return GetNoOpMessage(c)
	}

	return fmt.Sprintf("created version %v of %q", version, c.Application), nil
}

func writeCommitData(ctx context.Context, sourceCommitId string, sourceMessage string, app string, previousCommitId string, state *State) error {
	fs := state.Filesystem
	if !valid.SHA1CommitID(sourceCommitId) {
		return nil
	}
	commitDir := commitDirectory(fs, sourceCommitId)
	if err := fs.MkdirAll(commitDir, 0777); err != nil {
		return GetCreateReleaseGeneralFailure(err)
	}

	if previousCommitId != "" && valid.SHA1CommitID(previousCommitId) {
		if err := writeNextPrevInfo(ctx, sourceCommitId, strings.ToLower(previousCommitId), fieldPreviousCommitId, app, fs); err != nil {
			return GetCreateReleaseGeneralFailure(err)
		}
	}

	commitAppDir := commitApplicationDirectory(fs, sourceCommitId, app)
	if err := fs.MkdirAll(commitAppDir, 0777); err != nil {
		return GetCreateReleaseGeneralFailure(err)
	}
	if err := util.WriteFile(fs, fs.Join(commitDir, ".gitkeep"), make([]byte, 0), 0666); err != nil {
		return err
	}
	if err := util.WriteFile(fs, fs.Join(commitDir, "source_message"), []byte(sourceMessage), 0666); err != nil {
		return GetCreateReleaseGeneralFailure(err)
	}

	if err := util.WriteFile(fs, fs.Join(commitAppDir, ".gitkeep"), make([]byte, 0), 0666); err != nil {
		return GetCreateReleaseGeneralFailure(err)
	}
	return nil
}

func writeNextPrevInfo(ctx context.Context, sourceCommitId string, otherCommitId string, fieldSource string, application string, fs billy.Filesystem) error {

	otherCommitId = strings.ToLower(otherCommitId)
	sourceCommitDir := commitDirectory(fs, sourceCommitId)

	otherCommitDir := commitDirectory(fs, otherCommitId)

	if _, err := fs.Stat(otherCommitDir); err != nil {
		logger.FromContext(ctx).Sugar().Warnf(
			"Could not find the previous commit while trying to create a new release for commit %s and application %s. This is expected when `git.enableWritingCommitData` was just turned on, however it should not happen multiple times.", otherCommitId, application, otherCommitDir)
		return nil
	}

	if err := util.WriteFile(fs, fs.Join(sourceCommitDir, fieldSource), []byte(otherCommitId), 0666); err != nil {
		return err
	}
	fieldOther := ""
	if otherCommitId != "" {

		if fieldSource == fieldPreviousCommitId {
			fieldOther = fieldNextCommidId
		} else {
			fieldOther = fieldPreviousCommitId
		}

		//This is a workaround. util.WriteFile does NOT truncate file contents, so we simply delete the file before writing.
		if err := fs.Remove(fs.Join(otherCommitDir, fieldOther)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}

		if err := util.WriteFile(fs, fs.Join(otherCommitDir, fieldOther), []byte(sourceCommitId), 0666); err != nil {
			return err
		}
	}
	return nil
}

// Finds old releases for an application: Checks for the oldest release that is currently deployed on any environment
// Releases older that the oldest deployed release are eligible for deletion. releaseVersionsLimit
func findOldApplicationVersions(ctx context.Context, transaction *sql.Tx, state *State, appName string) ([]types.ReleaseNumbers, error) {
	// 1) get release in each env:
	envConfigs, err := state.GetAllEnvironmentConfigsFromDB(ctx, transaction)
	//envConfigs, err := state.GetEnvironmentConfigs()
	if err != nil {
		return nil, err
	}
	versions, err := state.GetApplicationReleasesFromFile(appName)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, err
	}

	oldestDeployedVersion := versions[len(versions)-1]
	for env := range envConfigs {
		version, err := state.GetEnvironmentApplicationVersion(ctx, transaction, env, appName)
		if err != nil {
			return nil, err
		}
		if version.Version != nil {
			if types.Greater(oldestDeployedVersion, version) {
				oldestDeployedVersion = version
			}
		}
	}
	positionOfOldestVersion := sort.Search(len(versions), func(i int) bool {
		return types.GreaterOrEqual(versions[i], oldestDeployedVersion)
	})

	if positionOfOldestVersion < (int(state.ReleaseVersionsLimit) - 1) {
		return nil, nil
	}
	indexToKeep := positionOfOldestVersion - 1
	majorsCount := 0
	for ; indexToKeep >= 0; indexToKeep-- {
		release, err := state.DBHandler.DBSelectReleaseByVersion(ctx, transaction, types.AppName(appName), types.ReleaseNumbers{Version: versions[indexToKeep].Version, Revision: versions[indexToKeep].Revision}, false)
		if err != nil {
			return nil, err
		}
		if release == nil {
			majorsCount += 1
			logger.FromContext(ctx).Warn("Release not found in database")
		} else if !release.Metadata.IsMinor && !release.Metadata.IsPrepublish {
			majorsCount += 1
		}
		if majorsCount >= int(state.ReleaseVersionsLimit) {
			break
		}
	}
	if indexToKeep < 0 {
		return nil, nil
	}
	return versions[0:indexToKeep], nil
}

type CreateEnvironmentTeamLock struct {
	Authentication        `json:"-"`
	TransformerMetadata   `json:"metadata"`
	Environment           types.EnvName    `json:"env"`
	Team                  string           `json:"team"`
	LockId                string           `json:"lockId"`
	Message               string           `json:"message"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	CreationTimestamp     time.Time        `json:"-"`
}

func (c *CreateEnvironmentTeamLock) GetCreationTimestamp() time.Time {
	return c.CreationTimestamp
}

func (c *CreateEnvironmentTeamLock) SetCreationTimestamp(ts time.Time) {
	c.CreationTimestamp = ts
}

var _ Transformer = &CreateEnvironmentTeamLock{} // ensure it implements Transformer

func (c *CreateEnvironmentTeamLock) GetGitTag() types.GitTag {
	return ""
}

func (c *CreateEnvironmentTeamLock) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *CreateEnvironmentTeamLock) SetEslVersion(eslVersion db.TransformerID) {
	c.TransformerEslVersion = eslVersion
}

func (c *CreateEnvironmentTeamLock) GetDBEventType() db.EventType {
	return db.EvtCreateEnvironmentTeamLock
}

func (c *CreateEnvironmentTeamLock) Transform(
	ctx context.Context,
	state *State,
	tCtx TransformerContext,
	tx *sql.Tx,
) (string, error) {
	env := c.Environment
	if tCtx.ShouldMinimizeGitData() {
		return GetNoOpMessage(c)
	}

	if !valid.EnvironmentName(env) {
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
				logger.FromContext(ctx).Sugar().Warnf("CreateEnvironmentTeamLock: Could not find team for application: %s", currentApp)
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

	envDir := fs.Join("environments", string(c.Environment))
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

	lock, err := state.DBHandler.DBSelectTeamLock(ctx, tx, env, c.Team, c.LockId)
	if err != nil {
		return "", err
	}

	if lock == nil {
		return "", fmt.Errorf("could not write team lock information to manifest. No team lock found on database for team '%s' on environment '%s' with ID '%s'", c.Team, c.Environment, c.LockId)
	}

	if err := createLock(ctx, chroot, lock.LockID, lock.Metadata.Message, lock.Metadata.CreatedByName, lock.Metadata.CreatedByEmail, lock.Created.Format(time.RFC3339)); err != nil {
		return "", err
	}

	return fmt.Sprintf("Created lock %q on environment %q for team %q.", c.LockId, c.Environment, c.Team), nil
}

type DeleteEnvironmentTeamLock struct {
	Authentication        `json:"-"`
	TransformerMetadata   `json:"metadata"`
	Environment           types.EnvName    `json:"env"`
	Team                  string           `json:"team"`
	LockId                string           `json:"lockId"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	CreationTimestamp     time.Time        `json:"-"`
}

func (c *DeleteEnvironmentTeamLock) GetCreationTimestamp() time.Time {
	return c.CreationTimestamp
}

func (c *DeleteEnvironmentTeamLock) SetCreationTimestamp(ts time.Time) {
	c.CreationTimestamp = ts
}

var _ Transformer = &DeleteEnvironmentTeamLock{} // ensure it implements Transformer

func (c *DeleteEnvironmentTeamLock) GetGitTag() types.GitTag {
	return ""
}

func (c *DeleteEnvironmentTeamLock) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *DeleteEnvironmentTeamLock) SetEslVersion(eslVersion db.TransformerID) {
	c.TransformerEslVersion = eslVersion
}

func (c *DeleteEnvironmentTeamLock) GetDBEventType() db.EventType {
	return db.EvtDeleteEnvironmentTeamLock
}

func (c *DeleteEnvironmentTeamLock) Transform(
	ctx context.Context,
	state *State,
	tCtx TransformerContext,
	_ *sql.Tx,
) (string, error) {
	if tCtx.ShouldMinimizeGitData() {
		return GetNoOpMessage(c)
	}
	envName := c.Environment

	if !valid.EnvironmentName(envName) {
		return "", status.Error(codes.InvalidArgument, fmt.Sprintf("cannot delete environment team lock: invalid environment: '%s'", c.Environment))
	}
	if !valid.TeamName(c.Team) {
		return "", status.Error(codes.InvalidArgument, fmt.Sprintf("cannot delete environment team lock: invalid team: '%s'", c.Team))
	}
	if !valid.LockId(c.LockId) {
		return "", status.Error(codes.InvalidArgument, fmt.Sprintf("cannot delete environment team lock: invalid lock id: '%s'", c.LockId))
	}
	fs := state.Filesystem

	lockDir := fs.Join("environments", string(c.Environment), "teams", c.Team, "locks", c.LockId)
	_, err := fs.Stat(lockDir)

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if err := fs.Remove(lockDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to delete directory %q: %w", lockDir, err)
	}

	if err := state.DeleteTeamLockIfEmpty(ctx, envName, c.Team); err != nil {
		return "", err
	}

	return fmt.Sprintf("Deleted lock %q on environment %q for team %q", c.LockId, c.Environment, c.Team), nil
}

type CreateEnvironment struct {
	Authentication        `json:"-"`
	TransformerMetadata   `json:"metadata"`
	Environment           types.EnvName            `json:"env"`
	Config                config.EnvironmentConfig `json:"config"`
	TransformerEslVersion db.TransformerID         `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	Dryrun                bool                     `json:"dryrun"`
	CreationTimestamp     time.Time                `json:"-"`
}

func (c *CreateEnvironment) GetCreationTimestamp() time.Time {
	return c.CreationTimestamp
}

func (c *CreateEnvironment) SetCreationTimestamp(ts time.Time) {
	c.CreationTimestamp = ts
}

var _ Transformer = &CreateEnvironment{} // ensure it implements Transformer

func (c *CreateEnvironment) GetGitTag() types.GitTag {
	return ""
}

func (c *CreateEnvironment) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *CreateEnvironment) SetEslVersion(eslVersion db.TransformerID) {
	c.TransformerEslVersion = eslVersion
}

func (c *CreateEnvironment) GetDBEventType() db.EventType {
	return db.EvtCreateEnvironment
}

func (c *CreateEnvironment) Transform(
	_ context.Context,
	state *State,
	tCtx TransformerContext,
	_ *sql.Tx,
) (string, error) {
	if c.Dryrun {
		return GetNoOpMessage(c)
	}
	fs := state.Filesystem
	envDir := fs.Join("environments", string(c.Environment))
	if err := fs.MkdirAll(envDir, 0777); err != nil {
		return "", err
	}
	configFile := fs.Join(envDir, "config.json")
	file, err := fs.OpenFile(configFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return "", fmt.Errorf("error creating config: %w", err)
	}
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err := enc.Encode(c.Config); err != nil {
		return "", fmt.Errorf("error writing json: %w", err)
	}
	err = file.Close()
	if err != nil {
		return "", fmt.Errorf("error closing environment config file %s, error: %w", configFile, err)
	}

	tCtx.ChangeEnvironment(c.Environment)

	// we do not need to inform argoCd when creating an environment, as there are no apps yet
	return fmt.Sprintf("create environment %q", c.Environment), nil
}

func commitDirectory(fs billy.Filesystem, commit string) string {
	return fs.Join("commits", commit[:2], commit[2:])
}

func commitApplicationDirectory(fs billy.Filesystem, commit, application string) string {
	return fs.Join(commitDirectory(fs, commit), "applications", application)
}

func removeCommit(fs billy.Filesystem, commitID, application string) error {
	errorTemplate := func(message string, err error) error {
		return fmt.Errorf("while removing applicaton %s from commit %s and error was encountered, message: %s, error %w", application, commitID, message, err)
	}

	commitApplicationDir := commitApplicationDirectory(fs, commitID, application)
	if err := fs.Remove(commitApplicationDir); err != nil {
		if os.IsNotExist(err) {
			// could not read the directory commitApplicationDir - but that's ok, because we don't know
			// if the kuberpult version that accepted this commit in the release endpoint, did already have commit writing enabled.
			// So there's no guarantee that this file ever existed
			return nil
		}
		return errorTemplate(fmt.Sprintf("could not remove the application directory %s", commitApplicationDir), err)
	}
	// check if there are no other services updated by this commit
	// if there are none, start removing the entire branch of the commit

	deleteDirIfEmpty := func(dir string) error {
		files, err := fs.ReadDir(dir)
		if err != nil {
			return errorTemplate(fmt.Sprintf("could not read the directory %s", dir), err)
		}
		if len(files) == 0 {
			if err = fs.Remove(dir); err != nil {
				return errorTemplate(fmt.Sprintf("could not remove the directory %s", dir), err)
			}
		}
		return nil
	}

	commitApplicationsDir := path.Dir(commitApplicationDir)
	if err := deleteDirIfEmpty(commitApplicationsDir); err != nil {
		return errorTemplate(fmt.Sprintf("could not remove directory %s", commitApplicationsDir), err)
	}
	commitDir2 := path.Dir(commitApplicationsDir)

	// if there are no more apps in the "applications" dir, then remove the commit message file and continue cleaning going up
	if _, err := fs.Stat(commitApplicationsDir); err != nil {
		if os.IsNotExist(err) {
			if err := fs.Remove(fs.Join(commitDir2)); err != nil {
				return errorTemplate(fmt.Sprintf("could not remove commit dir %s file", commitDir2), err)
			}
		} else {
			return errorTemplate(fmt.Sprintf("could not stat directory %s with an unexpected error", commitApplicationsDir), err)
		}
	}

	commitDir1 := path.Dir(commitDir2)
	if err := deleteDirIfEmpty(commitDir1); err != nil {
		return errorTemplate(fmt.Sprintf("could not remove directory %s", commitDir2), err)
	}

	return nil
}

type CleanupOldApplicationVersions struct {
	Application           string
	TransformerMetadata   `json:"metadata"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	CreationTimestamp     time.Time        `json:"-"`
}

func (c *CleanupOldApplicationVersions) GetCreationTimestamp() time.Time {
	return c.CreationTimestamp
}

func (c *CleanupOldApplicationVersions) SetCreationTimestamp(ts time.Time) {
	c.CreationTimestamp = ts
}

var _ Transformer = &CleanupOldApplicationVersions{} // ensure it implements Transformer

func (c *CleanupOldApplicationVersions) GetGitTag() types.GitTag {
	return ""
}

func (c *CleanupOldApplicationVersions) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *CleanupOldApplicationVersions) SetEslVersion(eslVersion db.TransformerID) {
	c.TransformerEslVersion = eslVersion
}

func (c *CleanupOldApplicationVersions) GetDBEventType() db.EventType {
	panic("CleanupOldApplicationVersions GetDBEventType")
}

func (c *CleanupOldApplicationVersions) Transform(
	ctx context.Context,
	state *State,
	_ TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	fs := state.Filesystem
	var err error
	var oldVersions []types.ReleaseNumbers
	oldVersions, err = findOldApplicationVersions(ctx, transaction, state, c.Application)
	if err != nil {
		return "", fmt.Errorf("cleanup: could not get application releases for app '%s': %w", c.Application, err)
	}

	msg := ""
	for _, oldRelease := range oldVersions {
		// delete oldRelease:
		releasesDir, err := state.checkWhichVersionDirectoryExists(fs, c.Application, oldRelease)
		if err != nil {
			return "", wrapFileError(err, releasesDir, "CleanupOldApplicationVersions: could not stat")
		}
		{
			commitIDFile := fs.Join(releasesDir, fieldSourceCommitId)
			dat, err := util.ReadFile(fs, commitIDFile)
			if err != nil {
				// not a problem, might be the undeploy commit or the commit has was not specified in CreateApplicationVersion
			} else {
				commitID := string(dat)
				if valid.SHA1CommitID(commitID) {
					if err := removeCommit(fs, commitID, c.Application); err != nil {
						return "", wrapFileError(err, releasesDir, "CleanupOldApplicationVersions: could not remove commit path")
					}
				}
			}
		}
		err = fs.Remove(releasesDir)
		if err != nil {
			return "", fmt.Errorf("CleanupOldApplicationVersions: Unexpected error app %s: %w",
				c.Application, err)
		}
		msg = fmt.Sprintf("%sremoved version %d of app %v as cleanup\n", msg, oldRelease, c.Application)
	}
	return msg, nil
}

type ReleaseTrain struct {
	Authentication        `json:"-"`
	TransformerMetadata   `json:"metadata"`
	Target                string           `json:"target"`
	Team                  string           `json:"team,omitempty"`
	CommitHash            string           `json:"commitHash"`
	WriteCommitData       bool             `json:"writeCommitData"`
	Repo                  Repository       `json:"-"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	TargetType            string           `json:"targetType"`
	GitTag                types.GitTag     `json:"gitTag"`
	CreationTimestamp     time.Time        `json:"-"`
}

func (u *ReleaseTrain) GetCreationTimestamp() time.Time {
	return u.CreationTimestamp
}

func (u *ReleaseTrain) SetCreationTimestamp(ts time.Time) {
	u.CreationTimestamp = ts
}

var _ Transformer = &ReleaseTrain{} // ensure it implements Transformer

func (u *ReleaseTrain) GetGitTag() types.GitTag {
	return u.GitTag
}

func (u *ReleaseTrain) GetEslVersion() db.TransformerID {
	return u.TransformerEslVersion
}

func (u *ReleaseTrain) SetEslVersion(eslVersion db.TransformerID) {
	u.TransformerEslVersion = eslVersion
}

func (u *ReleaseTrain) GetDBEventType() db.EventType {
	return db.EvtReleaseTrain
}

func getEnvironmentGroupsEnvironmentsOrEnvironment(configs map[types.EnvName]config.EnvironmentConfig, targetName string, targetType string) (map[types.EnvName]config.EnvironmentConfig, bool) {
	envGroupConfigs := make(map[types.EnvName]config.EnvironmentConfig)
	isEnvGroup := false

	if targetType != api.ReleaseTrainRequest_ENVIRONMENT.String() {
		for env, config := range configs {
			if config.EnvironmentGroup != nil && *config.EnvironmentGroup == targetName {
				isEnvGroup = true
				envGroupConfigs[env] = config
			}
		}
	}
	if targetType != api.ReleaseTrainRequest_ENVIRONMENTGROUP.String() {
		if len(envGroupConfigs) == 0 {
			envConfig, ok := configs[types.EnvName(targetName)]
			if ok {
				envGroupConfigs[types.EnvName(targetName)] = envConfig
			}
		}
	}
	return envGroupConfigs, isEnvGroup
}

func (u *ReleaseTrain) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "ReleaseTrain::Transform")
	defer span.Finish()
	span.SetTag("target", u.Target)
	//Gets deployments generated by the releasetrain with elsVersion u.TransformerEslVersion from the database and simply deploys them
	deployments, err := state.DBHandler.DBSelectDeploymentsByTransformerID(ctx, transaction, u.TransformerEslVersion)
	if err != nil {
		return "", err
	}
	span.SetTag("deployments", len(deployments))
	skippedDeployments, err := state.DBHandler.DBSelectAllLockPreventedEventsForTransformerID(ctx, transaction, u.TransformerEslVersion)
	if err != nil {
		return "", err
	}

	var targetGroupName = u.Target
	configs, err := state.GetAllEnvironmentConfigsFromDB(ctx, transaction)
	if err != nil {
		return "", err
	}
	var envGroupConfigs, isEnvGroup = getEnvironmentGroupsEnvironmentsOrEnvironment(configs, targetGroupName, u.TargetType)
	for _, currentDeployment := range deployments {
		loopSpan, _ := tracer.StartSpanFromContext(ctx, "ReleaseTrain::Transform Deploy One Application")
		envConfig := envGroupConfigs[currentDeployment.Env]
		if envConfig.Upstream == nil || (envConfig.Upstream.Environment == "" && !envConfig.Upstream.Latest) {
			loopSpan.Finish()
			return "", fmt.Errorf("could not find upstream config for env '%s'", currentDeployment.Env)
		}
		upstreamEnvName := envConfig.Upstream.Environment
		if envConfig.Upstream.Latest {
			upstreamEnvName = currentDeployment.Env
		}
		var trainGroup *string
		if isEnvGroup {
			trainGroup = conversion.FromString(targetGroupName)
		}
		if err := t.Execute(&DeployApplicationVersion{
			Authentication:      u.Authentication,
			TransformerMetadata: u.TransformerMetadata,
			Environment:         currentDeployment.Env,
			Application:         string(currentDeployment.App),
			Version:             *currentDeployment.ReleaseNumbers.Version,
			LockBehaviour:       api.LockBehavior_RECORD,
			WriteCommitData:     u.WriteCommitData,
			SourceTrain: &DeployApplicationVersionSource{
				Upstream:    string(upstreamEnvName),
				TargetGroup: trainGroup,
			},
			TransformerEslVersion: u.TransformerEslVersion,
			Author:                "",
			Revision:              currentDeployment.ReleaseNumbers.Revision,
		}, transaction); err != nil {
			loopSpan.Finish()
			return "", err
		}
		loopSpan.Finish()
	}

	if len(deployments) == 0 {
		return GetNoOpMessage(u)
	}

	commitMessage := fmt.Sprintf("Release Train to %s environment/environment group '%s':\n", u.CommitHash, targetGroupName)
	for _, skipped := range skippedDeployments {
		eventData, err := event.UnMarshallEvent("lock-prevented-deployment", skipped.EventJson)
		if err != nil {
			return "", err
		}
		lockPreventedEvent := eventData.EventData.(*event.LockPreventedDeployment)
		commitMessage += fmt.Sprintf("skipped application %s on environment %s", lockPreventedEvent.Application, lockPreventedEvent.Environment)
	}
	return commitMessage, nil
}

type MigrationTransformer struct {
	TransformerMetadata   `json:"metadata"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	CreationTimestamp     time.Time        `json:"-"`
}

func (u *MigrationTransformer) GetCreationTimestamp() time.Time {
	return u.CreationTimestamp
}

func (u *MigrationTransformer) SetCreationTimestamp(ts time.Time) {
	u.CreationTimestamp = ts
}

var _ Transformer = &MigrationTransformer{} // ensure it implements Transformer

func (c *MigrationTransformer) GetGitTag() types.GitTag {
	return ""
}

func (c *MigrationTransformer) GetDBEventType() db.EventType {
	return db.EvtMigrationTransformer
}
func (c *MigrationTransformer) Transform(_ context.Context, _ *State, _ TransformerContext, _ *sql.Tx) (string, error) {
	return "Migration Transformer", nil
}

func (c *MigrationTransformer) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *MigrationTransformer) SetEslVersion(eslVersion db.TransformerID) {
	c.TransformerEslVersion = eslVersion
}

type DeleteEnvFromApp struct {
	Authentication        `json:"-"`
	TransformerMetadata   `json:"metadata"`
	Application           string           `json:"app"`
	Environment           types.EnvName    `json:"env"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	CreationTimestamp     time.Time        `json:"-"`
}

func (c *DeleteEnvFromApp) GetCreationTimestamp() time.Time {
	return c.CreationTimestamp
}

func (c *DeleteEnvFromApp) SetCreationTimestamp(ts time.Time) {
	c.CreationTimestamp = ts
}

var _ Transformer = &DeleteEnvFromApp{} // ensure it implements Transformer

func (c *DeleteEnvFromApp) GetGitTag() types.GitTag {
	return ""
}

func (c *DeleteEnvFromApp) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *DeleteEnvFromApp) SetEslVersion(eslVersion db.TransformerID) {
	c.TransformerEslVersion = eslVersion
}

func (c *DeleteEnvFromApp) GetDBEventType() db.EventType {
	return db.EvtDeleteEnvFromApp
}

func (c *DeleteEnvFromApp) Transform(
	ctx context.Context,
	state *State,
	tCtx TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	envName := c.Environment
	fs := state.Filesystem

	thisSprintf := func(format string, a ...any) string {
		return fmt.Sprintf("DeleteEnvFromApp app '%s' on env '%s': %s", c.Application, c.Environment, fmt.Sprintf(format, a...))
	}

	thisErrorf := func(format string, a ...any) error {
		return fmt.Errorf("DeleteEnvFromApp app '%s' on env '%s': %s", c.Application, c.Environment, fmt.Sprintf(format, a...))
	}

	if c.Application == "" {
		return "", thisErrorf("Need to provide the application")
	}
	if c.Environment == "" {
		return "", thisErrorf("Need to provide the environment")
	}

	envAppDir := environmentApplicationDirectory(fs, envName, c.Application)
	if entries, err := fs.ReadDir(envAppDir); err != nil {
		return "", wrapFileError(err, envAppDir, thisSprintf("Could not open application directory"))
	} else if entries == nil {
		// app was never deployed on this env, so that's unusual - but for idempotency we treat it just like a success case:
		msg := thisSprintf("environment does not exist.")
		logger.FromContext(ctx).Warn(msg)
		return msg, nil
	}

	appLocksDir := fs.Join(envAppDir, "locks")
	if err := fs.Remove(appLocksDir); err != nil {
		return "", thisErrorf("cannot delete app locks '%v'", appLocksDir)
	}

	if err := fs.Remove(envAppDir); err != nil {
		return "", wrapFileError(err, envAppDir, thisSprintf("Cannot delete app."))
	}

	tCtx.DeleteEnvFromApp(c.Application, c.Environment)

	configs, err := state.GetAllEnvironmentConfigsFromDB(ctx, transaction)
	if err != nil {
		return "", thisErrorf("could not get environment configs: %v", err)
	}

	if deployed, err := isApplicationDeployedAnywhere(fs, types.AppName(c.Application), &configs); err == nil {
		if !deployed {
			_ = removeApplication(fs, c.Application)
			_, _ = removeApplicationFromEnvs(fs, types.AppName(c.Application), &configs)
		}
	} else {
		return "", thisErrorf("error checking if we are removing the last env: %v", err)
	}
	return fmt.Sprintf("Environment '%v' was removed from application '%v' successfully.", c.Environment, c.Application), nil
}

type CreateUndeployApplicationVersion struct {
	Authentication        `json:"-"`
	TransformerMetadata   `json:"metadata"`
	Application           string           `json:"app"`
	WriteCommitData       bool             `json:"writeCommitData"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	CreationTimestamp     time.Time        `json:"-"`
}

func (c *CreateUndeployApplicationVersion) GetCreationTimestamp() time.Time {
	return c.CreationTimestamp
}

func (c *CreateUndeployApplicationVersion) SetCreationTimestamp(ts time.Time) {
	c.CreationTimestamp = ts
}

var _ Transformer = &CreateUndeployApplicationVersion{} // ensure it implements Transformer

func (c *CreateUndeployApplicationVersion) GetGitTag() types.GitTag {
	return ""
}

func (c *CreateUndeployApplicationVersion) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *CreateUndeployApplicationVersion) SetEslVersion(eslVersion db.TransformerID) {
	c.TransformerEslVersion = eslVersion
}

func (c *CreateUndeployApplicationVersion) GetDBEventType() db.EventType {
	return db.EvtCreateUndeployApplicationVersion
}

func (c *CreateUndeployApplicationVersion) Transform(
	ctx context.Context,
	state *State,
	tCtx TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	fs := state.Filesystem
	lastRelease, err := state.DBHandler.DBSelectReleasesByAppLatestEslVersion(ctx, transaction, types.AppName(c.Application), false)
	if err != nil {
		return "", fmt.Errorf("could not get last relase for app '%v': %v", c.Application, err)
	}
	var nextReleaseNumber types.ReleaseNumbers
	if len(lastRelease) == 0 {
		return "", fmt.Errorf("cannot undeploy application '%v'", c.Application)
	}
	nextReleaseNumber = lastRelease[0].ReleaseNumbers

	releaseDir := releasesDirectoryWithVersion(fs, c.Application, nextReleaseNumber)

	configs, err := state.GetAllEnvironmentConfigsFromDB(ctx, transaction)
	if err != nil {
		return "", err
	}
	if tCtx.ShouldMaximizeGitData() {
		if err = fs.MkdirAll(releaseDir, 0777); err != nil {
			return "", err
		}
		// this is a flag to indicate that this is the special "undeploy" version
		if err := util.WriteFile(fs, fs.Join(releaseDir, "undeploy"), []byte(""), 0666); err != nil {
			return "", err
		}
		if err := util.WriteFile(fs, fs.Join(releaseDir, fieldCreatedAt), []byte(time2.GetTimeNow(ctx).Format(time.RFC3339)), 0666); err != nil {
			return "", err
		}
	}
	deploymentsMap, err := state.DBHandler.MapEnvNamesToDeployment(ctx, transaction, c.TransformerEslVersion)
	if err != nil {
		return "", err
	}
	for env := range configs {
		envDir := fs.Join(releaseDir, "environments", string(env))

		config, found := configs[env]
		hasUpstream := false
		if found {
			hasUpstream = config.Upstream != nil
		}
		if tCtx.ShouldMaximizeGitData() {
			if err = fs.MkdirAll(envDir, 0777); err != nil {
				return "", err
			}
			// note that the manifest is empty here!
			// but actually it's not quite empty!
			// The function we are using in DeployApplication version is `util.WriteFile`. And that does not allow overwriting files with empty content.
			// We work around this unusual behavior by writing a space into the file
			if err := util.WriteFile(fs, fs.Join(envDir, "manifests.yaml"), []byte(" "), 0666); err != nil {
				return "", err
			}
		}

		tCtx.AddAppEnv(c.Application, env)
		if _, exists := deploymentsMap[env]; !exists { //If this transformer did not generate any deployments, skip the deployment transformer
			continue
		}
		if hasUpstream && config.Upstream.Latest {
			d := &DeployApplicationVersion{
				SourceTrain: nil,
				Environment: env,
				Application: c.Application,
				Version:     *nextReleaseNumber.Version,
				// the train should queue deployments, instead of giving up:
				LockBehaviour:         api.LockBehavior_RECORD,
				Authentication:        c.Authentication,
				WriteCommitData:       c.WriteCommitData,
				Author:                "",
				TransformerEslVersion: c.TransformerEslVersion,
				TransformerMetadata: TransformerMetadata{
					AuthorName:  "",
					AuthorEmail: "",
				},
				Revision: nextReleaseNumber.Revision,
			}
			err := tCtx.Execute(d, transaction)
			if err != nil {
				_, ok := err.(*LockedError)
				if ok {
					continue // locked error are expected
				} else {
					return "", err
				}
			}
		}
	}
	return fmt.Sprintf("created undeploy-version %d of '%v'", nextReleaseNumber, c.Application), nil
}

type UndeployApplication struct {
	Authentication        `json:"-"`
	TransformerMetadata   `json:"metadata"`
	Application           string           `json:"app"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	CreationTimestamp     time.Time        `json:"-"`
}

func (c *UndeployApplication) GetCreationTimestamp() time.Time {
	return c.CreationTimestamp
}

func (c *UndeployApplication) SetCreationTimestamp(ts time.Time) {
	c.CreationTimestamp = ts
}

var _ Transformer = &UndeployApplication{} // ensure it implements Transformer

func (c *UndeployApplication) GetGitTag() types.GitTag {
	return ""
}

func (u *UndeployApplication) GetEslVersion() db.TransformerID {
	return u.TransformerEslVersion
}
func (u *UndeployApplication) GetDBEventType() db.EventType {
	return db.EvtUndeployApplication
}

func (u *UndeployApplication) SetEslVersion(id db.TransformerID) {
	u.TransformerEslVersion = id
}

func removeApplication(fs billy.Filesystem, application string) error {
	appDir := applicationDirectory(fs, application)
	releasesDir := fs.Join(appDir, "releases")
	files, err := fs.ReadDir(releasesDir)
	if err != nil {
		return fmt.Errorf("could not read the releases directory %s %w", releasesDir, err)
	}
	for _, file := range files {
		if file.IsDir() {
			releaseDir := fs.Join(releasesDir, file.Name())
			commitIDFile := fs.Join(releaseDir, "source_commit_id")
			content, err := util.ReadFile(fs, commitIDFile)
			if err != nil {
				// release does not have a corresponding commit, which might be the case if it's an undeploy release, no prob
				if errors.Is(err, os.ErrNotExist) {
					continue
				} else {
					return err
				}
			}
			if commitID := string(content); valid.SHA1CommitID(commitID) {
				if err := removeCommit(fs, commitID, application); err != nil {
					return fmt.Errorf("could not remove the commit: %w", err)
				}
			}
		}
	}
	if err = fs.Remove(appDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return wrapFileError(err, appDir, "could not remove application directory")
	}
	return nil
}

func removeApplicationFromEnvs(fs billy.Filesystem, application types.AppName, configs *map[types.EnvName]config.EnvironmentConfig) ([]types.EnvName, error) {
	result := make([]types.EnvName, 0)
	for env := range *configs {
		appDir := environmentApplicationDirectory(fs, env, string(application))
		result := append(result, env)
		if err := fs.Remove(appDir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return result, fmt.Errorf("unexpected error application '%v' environment '%v': '%w'", application, env, err)
		}
	}
	return result, nil
}

func isApplicationDeployedAnywhere(fs billy.Filesystem, application types.AppName, configs *map[types.EnvName]config.EnvironmentConfig) (bool, error) {
	for env := range *configs {
		envAppDir := environmentApplicationDirectory(fs, env, string(application))
		versionDir := fs.Join(envAppDir, "version")
		if _, err := fs.Stat(versionDir); err != nil {
			if !errors.Is(err, os.ErrNotExist) { // only errors other that "not exist" are unexpected => propagate
				return false, err
			}
		} else {
			return true, nil // found one
		}
	}
	return false, nil // found none and didnt see any other errors
}

func (u *UndeployApplication) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	//All verifications were already done by the cd-service. This transformer should just blindly delete the affected files
	fs := state.Filesystem
	configs, err := state.GetAllEnvironmentConfigsFromDB(ctx, transaction) // we use ALL envs, to be sure
	if err != nil {
		return "", fmt.Errorf("could not get environment configs: %w", err)
	}
	for env := range configs {
		envAppDir := environmentApplicationDirectory(fs, env, u.Application)
		entries, err := fs.ReadDir(envAppDir)
		if err != nil {
			return "", wrapFileError(err, envAppDir, "UndeployApplication: Could not open application directory. Does the app exist?")
		}
		if entries == nil {
			// app was never deployed on this env, so we must ignore it!
			continue
		}

		appLocksDir := fs.Join(envAppDir, "locks")
		err = fs.Remove(appLocksDir)
		if err != nil {
			return "", fmt.Errorf("UndeployApplication: cannot delete app locks '%v'", appLocksDir)
		}

		versionDir := fs.Join(envAppDir, "version")

		_, err = fs.Stat(versionDir)
		if err != nil && errors.Is(err, os.ErrNotExist) {
			// if the app was never deployed here, that's not a reason to stop
			continue
		}

		undeployFile := fs.Join(versionDir, "undeploy")
		_, err = fs.Stat(undeployFile)
		if err != nil { //Undeploy version does not exist if we minimize git data
			if errors.Is(err, os.ErrNotExist) {
				if t.ShouldMaximizeGitData() {
					logger.FromContext(ctx).Sugar().Warnf("Maximize git data is enabled but could not find undeploy file %q for application %q on environment %q.", undeployFile, u.Application, env)
				}
			} else {
				return "", fmt.Errorf("UndeployApplication: Error while checking for undeploy file: %w", err)
			}
		}

	}

	if err := removeApplication(fs, u.Application); err != nil {
		return "", err
	}

	if envs, err := removeApplicationFromEnvs(fs, types.AppName(u.Application), &configs); err == nil {
		for _, env := range envs {
			t.AddAppEnv(u.Application, env)
		}
	} else {
		return "", err
	}
	return fmt.Sprintf("application '%v' was deleted successfully", u.Application), nil
}

type CreateEnvironmentGroupLock struct {
	Authentication        `json:"-"`
	TransformerMetadata   `json:"metadata"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	CreationTimestamp     time.Time        `json:"-"`
}

func (c *CreateEnvironmentGroupLock) GetCreationTimestamp() time.Time {
	return c.CreationTimestamp
}

func (c *CreateEnvironmentGroupLock) SetCreationTimestamp(ts time.Time) {
	c.CreationTimestamp = ts
}

var _ Transformer = &CreateEnvironmentGroupLock{} // ensure it implements Transformer

func (c *CreateEnvironmentGroupLock) GetGitTag() types.GitTag {
	return ""
}

func (c *CreateEnvironmentGroupLock) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *CreateEnvironmentGroupLock) SetEslVersion(eslVersion db.TransformerID) {
	c.TransformerEslVersion = eslVersion
}

func (c *CreateEnvironmentGroupLock) GetDBEventType() db.EventType {
	return db.EvtCreateEnvironmentGroupLock
}

func (c *CreateEnvironmentGroupLock) Transform(
	_ context.Context,
	_ *State,
	_ TransformerContext,
	_ *sql.Tx,
) (string, error) {
	// group locks are handled on the cd-service, and split into environment locks
	return GetNoOpMessage(c)
}

type DeleteEnvironmentGroupLock struct {
	Authentication        `json:"-"`
	TransformerMetadata   `json:"metadata"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	CreationTimestamp     time.Time        `json:"-"`
}

func (c *DeleteEnvironmentGroupLock) GetCreationTimestamp() time.Time {
	return c.CreationTimestamp
}

func (c *DeleteEnvironmentGroupLock) SetCreationTimestamp(ts time.Time) {
	c.CreationTimestamp = ts
}

var _ Transformer = &DeleteEnvironmentGroupLock{} // ensure it implements Transformer

func (c *DeleteEnvironmentGroupLock) GetGitTag() types.GitTag {
	return ""
}

func (c *DeleteEnvironmentGroupLock) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *DeleteEnvironmentGroupLock) SetEslVersion(eslVersion db.TransformerID) {
	c.TransformerEslVersion = eslVersion
}

func (c *DeleteEnvironmentGroupLock) GetDBEventType() db.EventType {
	return db.EvtDeleteEnvironmentGroupLock
}

func (c *DeleteEnvironmentGroupLock) Transform(
	_ context.Context,
	_ *State,
	_ TransformerContext,
	_ *sql.Tx,
) (string, error) {
	// group locks are handled on the cd-service, and split into environment locks
	return GetNoOpMessage(c)
}

type DeleteEnvironment struct {
	TransformerMetadata   `json:"metadata"`
	Environment           types.EnvName    `json:"env"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	Dryrun                bool             `json:"dryrun"`
	CreationTimestamp     time.Time        `json:"-"`
}

func (c *DeleteEnvironment) GetCreationTimestamp() time.Time {
	return c.CreationTimestamp
}

func (c *DeleteEnvironment) SetCreationTimestamp(ts time.Time) {
	c.CreationTimestamp = ts
}

var _ Transformer = &DeleteEnvironment{} // ensure it implements Transformer

func (d *DeleteEnvironment) GetGitTag() types.GitTag {
	return ""
}

func (d *DeleteEnvironment) GetEslVersion() db.TransformerID {
	return d.TransformerEslVersion
}

func (d *DeleteEnvironment) SetEslVersion(id db.TransformerID) {
	d.TransformerEslVersion = id
}

func (d *DeleteEnvironment) GetDBEventType() db.EventType {
	return db.EvtDeleteEnvironment
}

func (d *DeleteEnvironment) Transform(ctx context.Context, state *State, t TransformerContext, transaction *sql.Tx) (string, error) {
	if d.Dryrun {
		return GetNoOpMessage(d)
	}
	fs := state.Filesystem

	configs, err := state.GetAllEnvironmentConfigsFromDB(ctx, transaction)
	if err != nil {
		return "", err
	}
	config := configs[d.Environment]

	if isAAEnv(config) {
		for _, currentConfig := range config.ArgoCdConfigs.ArgoCdConfigurations {
			if err := deleteAAEnvironment(ctx, fs, d.Environment, types.EnvName(currentConfig.ConcreteEnvName)); err != nil {
				return "", err
			}
		}
	} else {
		argoCdAppFile := fs.Join("argocd", string(argocd.V1Alpha1), fmt.Sprintf("%s.yaml", d.Environment))
		err := fs.Remove(argoCdAppFile)
		if errors.Is(err, os.ErrNotExist) {
			logger.FromContext(ctx).Sugar().Warnf("DeleteEnvironment: environment's argocd app file %q does not exist.", argoCdAppFile)
		} else if err != nil {
			return "", fmt.Errorf("error deleting the environment's argocd app file %q: %w", argoCdAppFile, err)
		}
	}
	envDir := fs.Join("environments", string(d.Environment))
	err = fs.Remove(envDir)
	if errors.Is(err, os.ErrNotExist) {
		logger.FromContext(ctx).Sugar().Warnf("DeleteEnvironment: environment directory %q does not exist.", envDir)
	} else if err != nil {
		return "", fmt.Errorf("error deleting the environment directory %q: %w", envDir, err)
	}

	return fmt.Sprintf("delete environment %q", d.Environment), nil
}

type ExtendAAEnvironment struct {
	Authentication        `json:"-"`
	TransformerMetadata   `json:"metadata"`
	Environment           types.EnvName                  `json:"env"`
	ArgoCDConfig          config.EnvironmentConfigArgoCd `json:"config"`
	TransformerEslVersion db.TransformerID               `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	CreationTimestamp     time.Time                      `json:"-"`
}

func (c *ExtendAAEnvironment) GetCreationTimestamp() time.Time {
	return c.CreationTimestamp
}

func (c *ExtendAAEnvironment) SetCreationTimestamp(ts time.Time) {
	c.CreationTimestamp = ts
}

var _ Transformer = &ExtendAAEnvironment{} // ensure we implement the interface

func (c *ExtendAAEnvironment) GetGitTag() types.GitTag {
	return ""
}

func (c *ExtendAAEnvironment) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *ExtendAAEnvironment) SetEslVersion(eslVersion db.TransformerID) {
	c.TransformerEslVersion = eslVersion
}

func (c *ExtendAAEnvironment) GetDBEventType() db.EventType {
	return db.EvtExtendAAEnvironment
}

func (c *ExtendAAEnvironment) Transform(
	_ context.Context,
	state *State,
	tCtx TransformerContext,
	_ *sql.Tx,
) (string, error) {
	if tCtx.ShouldMinimizeGitData() {
		//This cannot be a NO-OP, as we need to generate the argocd files after the transformer is executed
		return fmt.Sprintf("added configuration for AA environment %q - %q", c.Environment, c.ArgoCDConfig.ConcreteEnvName), nil
	}
	fs := state.Filesystem
	envDir := fs.Join("environments", string(c.Environment))
	if err := fs.MkdirAll(envDir, 0777); err != nil {
		return "", err
	}

	configFile := fs.Join(envDir, "config.json")

	data, err := util.ReadFile(fs, configFile)
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	var envConfig config.EnvironmentConfig

	err = json.Unmarshal(data, &envConfig)
	if err != nil {
		return "", err
	}

	envConfig.ArgoCdConfigs.ArgoCdConfigurations = append(envConfig.ArgoCdConfigs.ArgoCdConfigurations, &c.ArgoCDConfig)

	err = writeEnvironmentConfigurationToManifestRepo(fs, configFile, envConfig)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("added configuration for AA environment %q - %q", c.Environment, c.ArgoCDConfig.ConcreteEnvName), nil
}

type DeleteAAEnvironmentConfig struct {
	Authentication          `json:"-"`
	TransformerMetadata     `json:"metadata"`
	Environment             types.EnvName    `json:"env"`
	ConcreteEnvironmentName types.EnvName    `json:"concreteEnvName"`
	TransformerEslVersion   db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	CreationTimestamp       time.Time        `json:"-"`
}

func (c *DeleteAAEnvironmentConfig) GetCreationTimestamp() time.Time {
	return c.CreationTimestamp
}

func (c *DeleteAAEnvironmentConfig) SetCreationTimestamp(ts time.Time) {
	c.CreationTimestamp = ts
}

var _ Transformer = &DeleteAAEnvironmentConfig{} // ensure we implement the interface

func (c *DeleteAAEnvironmentConfig) GetGitTag() types.GitTag {
	return ""
}

func (c *DeleteAAEnvironmentConfig) GetDBEventType() db.EventType {
	return db.EvtDeleteAAEnvironmentConfig
}

func (c *DeleteAAEnvironmentConfig) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *DeleteAAEnvironmentConfig) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func deleteAAEnvironment(ctx context.Context, fs billy.Filesystem, env types.EnvName, concreteEnv types.EnvName) error {
	envDir := fs.Join("environments", string(env))

	configFile := fs.Join(envDir, "config.json")

	data, err := util.ReadFile(fs, configFile)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	var envConfig config.EnvironmentConfig

	err = json.Unmarshal(data, &envConfig)
	if err != nil {
		return err
	}

	if envConfig.ArgoCdConfigs.CommonEnvPrefix == nil {
		return fmt.Errorf("could not read CommonEnvPrefix for AA environment %q", env)
	}
	argoCdAppFile := getArgoCdAAEnvFileName(fs, types.EnvName(*envConfig.ArgoCdConfigs.CommonEnvPrefix), env, concreteEnv, true)
	err = fs.Remove(argoCdAppFile)
	if errors.Is(err, os.ErrNotExist) {
		logger.FromContext(ctx).Sugar().Warnf("AA environment argocd app file %q does not exist.", argoCdAppFile)
	} else if err != nil {
		return fmt.Errorf("error deleting AA environment's argocd app file %q: %w", argoCdAppFile, err)
	}
	for idx, currentConfig := range envConfig.ArgoCdConfigs.ArgoCdConfigurations {
		if types.EnvName(currentConfig.ConcreteEnvName) == concreteEnv {
			envConfig.ArgoCdConfigs.ArgoCdConfigurations = append(envConfig.ArgoCdConfigs.ArgoCdConfigurations[:idx], envConfig.ArgoCdConfigs.ArgoCdConfigurations[idx+1:]...)
			break
		}
	}
	err = writeEnvironmentConfigurationToManifestRepo(fs, configFile, envConfig)
	if err != nil {
		return fmt.Errorf("error writing environment configuration to manifest repository: %w", err)
	}
	return nil
}

func (c *DeleteAAEnvironmentConfig) Transform(
	ctx context.Context,
	state *State,
	tCtx TransformerContext,
	_ *sql.Tx,
) (string, error) {
	if err := deleteAAEnvironment(ctx, state.Filesystem, c.Environment, c.ConcreteEnvironmentName); err != nil {
		return "", err
	}
	return fmt.Sprintf("removed configuration for AA environment %q - %q", c.Environment, c.ConcreteEnvironmentName), nil
}

func writeEnvironmentConfigurationToManifestRepo(fs billy.Filesystem, configFile string, envConfig config.EnvironmentConfig) error {
	err := fs.Remove(configFile) // If we do not remove the file, it behaves like an "append" not an "overwrite". Removing the file and recreating it resulted in the correct output in the file.
	if err != nil {
		return fmt.Errorf("error removing environment config file: %w", err)
	}
	file, err := fs.OpenFile(configFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("error creating config: %w", err)
	}
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")

	if err := enc.Encode(envConfig); err != nil {
		return fmt.Errorf("error writing json: %w", err)
	}
	err = file.Close()
	if err != nil {
		return fmt.Errorf("error closing environment config file %s, error: %w", configFile, err)
	}
	return nil
}
