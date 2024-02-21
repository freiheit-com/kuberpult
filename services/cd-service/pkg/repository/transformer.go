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

Copyright 2023 freiheit.com*/

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/uuid"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/event"

	"github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/valid"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/freiheit-com/kuberpult/pkg/logger"

	yaml3 "gopkg.in/yaml.v3"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/mapper"
	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
)

const (
	queueFileName       = "queued_version"
	yamlParsingError    = "# yaml parsing error"
	fieldSourceAuthor   = "source_author"
	fieldSourceMessage  = "source_message"
	fieldSourceCommitId = "source_commit_id"
	fieldDisplayVersion = "display_version"
	fieldSourceRepoUrl  = "sourceRepoUrl" // urgh, inconsistent
	fieldCreatedAt      = "created_at"
	fieldTeam           = "team"
	// number of old releases that will ALWAYS be kept in addition to the ones that are deployed:
	keptVersionsOnCleanup = 20
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

func environmentDirectory(fs billy.Filesystem, environment string) string {
	return fs.Join("environments", environment)
}

func environmentApplicationDirectory(fs billy.Filesystem, environment, application string) string {
	return fs.Join("environments", environment, "applications", application)
}

func releasesDirectoryWithVersion(fs billy.Filesystem, application string, version uint64) string {
	return fs.Join(releasesDirectory(fs, application), versionToString(version))
}

func commitDirectory(fs billy.Filesystem, commit string) string {
	return fs.Join("commits", commit[:2], commit[2:])
}

func commitApplicationDirectory(fs billy.Filesystem, commit, application string) string {
	return fs.Join(commitDirectory(fs, commit), "applications", application)
}

func commitEventDir(fs billy.Filesystem, commit, eventId string) string {
	return fs.Join(commitDirectory(fs, commit), "events", eventId)
}

func GetEnvironmentLocksCount(fs billy.Filesystem, env string) float64 {
	envLocksCount := 0
	envDir := environmentDirectory(fs, env)
	locksDir := fs.Join(envDir, "locks")
	if entries, _ := fs.ReadDir(locksDir); entries != nil {
		envLocksCount += len(entries)
	}
	return float64(envLocksCount)
}

func GetEnvironmentApplicationLocksCount(fs billy.Filesystem, environment, application string) float64 {
	envAppLocksCount := 0
	appDir := environmentApplicationDirectory(fs, environment, application)
	locksDir := fs.Join(appDir, "locks")
	if entries, _ := fs.ReadDir(locksDir); entries != nil {
		envAppLocksCount += len(entries)
	}
	return float64(envAppLocksCount)
}

func GaugeEnvLockMetric(fs billy.Filesystem, env string) {
	if ddMetrics != nil {
		ddMetrics.Gauge("env_lock_count", GetEnvironmentLocksCount(fs, env), []string{"env:" + env}, 1)
	}
}

func GaugeEnvAppLockMetric(fs billy.Filesystem, env, app string) {
	if ddMetrics != nil {
		ddMetrics.Gauge("app_lock_count", GetEnvironmentApplicationLocksCount(fs, env, app), []string{"app:" + app, "env:" + env}, 1)
	}
}

func UpdateDatadogMetrics(state *State, changes *TransformerResult) error {
	filesystem := state.Filesystem
	if ddMetrics == nil {
		return nil
	}
	configs, err := state.GetEnvironmentConfigs()
	if err != nil {
		return err
	}
	for env := range configs {
		GaugeEnvLockMetric(filesystem, env)
		appsDir := filesystem.Join(environmentDirectory(filesystem, env), "applications")
		if entries, _ := filesystem.ReadDir(appsDir); entries != nil {
			for _, app := range entries {
				GaugeEnvAppLockMetric(filesystem, env, app.Name())
			}
		}
	}
	now := time.Now() // ensure all events have the same timestamp
	if changes != nil && ddMetrics != nil {
		for i := range changes.ChangedApps {
			oneChange := changes.ChangedApps[i]
			teamMessage := func() string {
				if oneChange.Team != "" {
					return fmt.Sprintf(" for team %s", oneChange.Team)
				}
				return ""
			}()
			event := statsd.Event{
				Title:     "Kuberpult app deployed",
				Text:      fmt.Sprintf("Kuberpult has deployed %s to %s%s", oneChange.App, oneChange.Env, teamMessage),
				Timestamp: now,
				Tags: []string{
					"kuberpult.application:" + oneChange.App,
					"kuberpult.environment:" + oneChange.Env,
					"kuberpult.team:" + oneChange.Team,
				},
			}
			err := ddMetrics.Event(&event)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func RegularlySendDatadogMetrics(repo Repository, interval time.Duration, callBack func(repository Repository)) {
	metricEventTimer := time.NewTicker(interval * time.Second)
	for {
		select {
		case <-metricEventTimer.C:
			callBack(repo)
		}
	}
}

func GetRepositoryStateAndUpdateMetrics(repo Repository) {
	repoState := repo.State()
	if err := UpdateDatadogMetrics(repoState, nil); err != nil {
		panic(err.Error())
	}
}

// A Transformer updates the files in the worktree
type Transformer interface {
	Transform(context.Context, *State, TransformerContext) (commitMsg string, e error)
}

type TransformerContext interface {
	Execute(Transformer) error
	AddAppEnv(app string, env string, team string)
	DeleteEnvFromApp(app string, env string)
}

func RunTransformer(ctx context.Context, t Transformer, s *State) (string, *TransformerResult, error) {
	runner := transformerRunner{
		Context: ctx,
		State:   s,
		Stack:   [][]string{nil},
	}
	if err := runner.Execute(t); err != nil {
		return "", nil, err
	}
	return runner.Stack[0][0], &TransformerResult{
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

func (r *transformerRunner) Execute(t Transformer) error {
	r.Stack = append(r.Stack, nil)
	msg, err := t.Transform(r.Context, r.State, r)
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
		App: app,
		Env: env,
	})
	r.DeletedRootApps = append(r.DeletedRootApps, RootApp{
		Env: env,
	})
}

type CreateApplicationVersion struct {
	Authentication
	Version         uint64
	Application     string
	Manifests       map[string]string
	SourceCommitId  string
	SourceAuthor    string
	SourceMessage   string
	SourceRepoUrl   string
	Team            string
	DisplayVersion  string
	WriteCommitData bool
}

type ctxMarkerGenerateUuid struct{}

var (
	ctxMarkerGenerateUuidKey = &ctxMarkerGenerateUuid{}
)

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

func (c *CreateApplicationVersion) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	version, err := c.calculateVersion(state)
	if err != nil {
		return "", err
	}
	fs := state.Filesystem
	if !valid.ApplicationName(c.Application) {
		return "", GetCreateReleaseAppNameTooLong(c.Application, valid.AppNameRegExp, valid.MaxAppNameLen)
	}
	releaseDir := releasesDirectoryWithVersion(fs, c.Application, version)
	appDir := applicationDirectory(fs, c.Application)
	if err = fs.MkdirAll(releaseDir, 0777); err != nil {
		return "", GetCreateReleaseGeneralFailure(err)
	}

	if !valid.SHA1CommitID(c.SourceCommitId) {
		logger.FromContext(ctx).Sugar().Warnf("commit ID is not a valid SHA1 hash, should be exactly 40 characters [0-9a-fA-F] %s\n", c.SourceCommitId)
	}

	configs, err := state.GetEnvironmentConfigs()
	if err != nil {
		if errors.Is(err, InvalidJson) {
			return "", err
		}
		return "", GetCreateReleaseGeneralFailure(err)
	}

	if c.SourceCommitId != "" {
		c.SourceCommitId = strings.ToLower(c.SourceCommitId)
		if err := util.WriteFile(fs, fs.Join(releaseDir, fieldSourceCommitId), []byte(c.SourceCommitId), 0666); err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
		if valid.SHA1CommitID(c.SourceCommitId) {
			commitDir := commitApplicationDirectory(fs, c.SourceCommitId, c.Application)
			if err := fs.MkdirAll(commitDir, 0777); err != nil {
				return "", GetCreateReleaseGeneralFailure(err)
			}
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
	if err := util.WriteFile(fs, fs.Join(releaseDir, fieldCreatedAt), []byte(getTimeNow(ctx).Format(time.RFC3339)), 0666); err != nil {
		return "", GetCreateReleaseGeneralFailure(err)
	}
	if c.Team != "" {
		if err := util.WriteFile(fs, fs.Join(appDir, fieldTeam), []byte(c.Team), 0666); err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
	}
	if c.SourceRepoUrl != "" {
		if err := util.WriteFile(fs, fs.Join(appDir, fieldSourceRepoUrl), []byte(c.SourceRepoUrl), 0666); err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
	}
	isLatest, err := isLatestsVersion(state, c.Application, version)
	if err != nil {
		return "", GetCreateReleaseGeneralFailure(err)
	}
	if !isLatest {
		// check that we can actually backfill this version
		oldVersions, err := findOldApplicationVersions(state, c.Application)
		if err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
		for _, oldVersion := range oldVersions {
			if version == oldVersion {
				return "", GetCreateReleaseTooOld()
			}
		}
	}

	var allEnvsOfThisApp []string = nil
	for env, man := range c.Manifests {
		allEnvsOfThisApp = append(allEnvsOfThisApp, env)
		err := state.checkUserPermissions(ctx, env, c.Application, auth.PermissionCreateRelease, c.Team, c.RBACConfig)
		if err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
		envDir := fs.Join(releaseDir, "environments", env)

		config, found := configs[env]
		hasUpstream := false
		if found {
			hasUpstream = config.Upstream != nil
		}

		if err = fs.MkdirAll(envDir, 0777); err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
		if err := util.WriteFile(fs, fs.Join(envDir, "manifests.yaml"), []byte(man), 0666); err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
		teamOwner, err := state.GetApplicationTeamOwner(c.Application)
		if err != nil {
			return "", err
		}
		t.AddAppEnv(c.Application, env, teamOwner)
		if hasUpstream && config.Upstream.Latest && isLatest {
			d := &DeployApplicationVersion{
				Environment:    env,
				Application:    c.Application,
				Version:        version, // the train should queue deployments, instead of giving up:
				LockBehaviour:  api.LockBehavior_RECORD,
				Authentication: c.Authentication,
			}
			err := t.Execute(d)
			if err != nil {
				_, ok := err.(*LockedError)
				if ok {
					continue // LockedErrors are expected
				} else {
					return "", GetCreateReleaseGeneralFailure(err)
				}
			}
		}
	}
	gen, ok := getGeneratorFromContext(ctx)
	if !ok || gen == nil {
		logger.FromContext(ctx).Info("using real UUID generator.")
		gen = uuid.RealUUIDGenerator{}
	} else {
		logger.FromContext(ctx).Info("using  UUID generator from context.")
	}
	eventUuid := gen.Generate()
	if c.WriteCommitData {
		err = writeCommitData(ctx, c.SourceCommitId, c.SourceMessage, c.Application, eventUuid, allEnvsOfThisApp, fs)
		if err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
	}
	return fmt.Sprintf("created version %d of %q", version, c.Application), nil
}

func getGeneratorFromContext(ctx context.Context) (uuid.GenerateUUIDs, bool) {
	gen, ok := ctx.Value(ctxMarkerGenerateUuidKey).(uuid.GenerateUUIDs)
	return gen, ok
}

func AddGeneratorToContext(ctx context.Context, gen uuid.GenerateUUIDs) context.Context {
	return context.WithValue(ctx, ctxMarkerGenerateUuidKey, gen)
}

func writeCommitData(ctx context.Context, sourceCommitId string, sourceMessage string, app string, eventId string, environments []string, fs billy.Filesystem) error {
	if !valid.SHA1CommitID(sourceCommitId) {
		return nil
	}
	commitDir := commitDirectory(fs, sourceCommitId)
	if err := util.WriteFile(fs, fs.Join(commitDir, ".empty"), make([]byte, 0), 0666); err != nil {
		return GetCreateReleaseGeneralFailure(err)
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
	err := writeEvent(ctx, eventId, sourceCommitId, fs, environments)
	if err != nil {
		return fmt.Errorf("error while writing event: %v", err)
	}
	return nil
}

func writeEvent(ctx context.Context, eventId string, sourceCommitId string, filesystem billy.Filesystem, envs []string) error {
	eventDir := commitEventDir(filesystem, sourceCommitId, eventId)
	_, err := filesystem.Stat(eventDir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			// anything except "not exists" is an error:
			return fmt.Errorf("event directory %s already existed: %v", eventDir, err)
		}
	}
	if err := filesystem.MkdirAll(eventDir, 0777); err != nil {
		return fmt.Errorf("could not create directory %s: %v", eventDir, err)
	}
	for i := range envs {
		env := envs[i]
		environmentDir := filesystem.Join(eventDir, "environments", env)
		if err := filesystem.MkdirAll(environmentDir, 0777); err != nil {
			return fmt.Errorf("could not create directory %s: %v", environmentDir, err)
		}
		environmentNamePath := filesystem.Join(environmentDir, ".gitkeep")
		if err := util.WriteFile(filesystem, environmentNamePath, make([]byte, 0), 0666); err != nil {
			return fmt.Errorf("could not write file %s: %v", environmentNamePath, err)
		}
	}
	eventTypePath := filesystem.Join(eventDir, "eventType")
	if err := util.WriteFile(filesystem, eventTypePath, []byte(event.NewReleaseEventName), 0666); err != nil {
		return fmt.Errorf("could not write file %s: %v", eventTypePath, err)
	}

	// Note: we do not store the "createAt" date here, because we use UUIDs with timestamp information
	return nil
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
	if c.SourceRepoUrl != "" {
		existingSourceRepoUrl, err := util.ReadFile(fs, fs.Join(releaseDir, fieldSourceCommitId))
		if err != nil {
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_SOURCE_REPO_URL, "")
		}
		existingSourceRepoUrlStr := string(existingSourceRepoUrl)
		if existingSourceRepoUrlStr != c.SourceRepoUrl {
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_SOURCE_REPO_URL, createUnifiedDiff(existingSourceRepoUrlStr, c.SourceRepoUrl, ""))
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

type RawNode struct{ *yaml3.Node }

func (n *RawNode) UnmarshalYAML(node *yaml3.Node) error {
	n.Node = node
	return nil
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
	edits := myers.ComputeEdits(span.URIFromPath(existingFilename), existingValueStr, string(requestValue))
	return fmt.Sprint(gotextdiff.ToUnified(existingFilename, requestFilename, existingValueStr, edits))
}

func isLatestsVersion(state *State, application string, version uint64) (bool, error) {
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

type CreateUndeployApplicationVersion struct {
	Authentication
	Application string
}

func (c *CreateUndeployApplicationVersion) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	fs := state.Filesystem
	lastRelease, err := GetLastRelease(fs, c.Application)
	if err != nil {
		return "", err
	}
	if lastRelease == 0 {
		return "", fmt.Errorf("cannot undeploy non-existing application '%v'", c.Application)
	}

	releaseDir := releasesDirectoryWithVersion(fs, c.Application, lastRelease+1)
	if err = fs.MkdirAll(releaseDir, 0777); err != nil {
		return "", err
	}

	configs, err := state.GetEnvironmentConfigs()
	if err != nil {
		return "", err
	}
	// this is a flag to indicate that this is the special "undeploy" version
	if err := util.WriteFile(fs, fs.Join(releaseDir, "undeploy"), []byte(""), 0666); err != nil {
		return "", err
	}
	if err := util.WriteFile(fs, fs.Join(releaseDir, fieldCreatedAt), []byte(getTimeNow(ctx).Format(time.RFC3339)), 0666); err != nil {
		return "", err
	}
	for env := range configs {
		err := state.checkUserPermissions(ctx, env, c.Application, auth.PermissionCreateUndeploy, "", c.RBACConfig)
		if err != nil {
			return "", err
		}
		envDir := fs.Join(releaseDir, "environments", env)

		config, found := configs[env]
		hasUpstream := false
		if found {
			hasUpstream = config.Upstream != nil
		}

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
		teamOwner, err := state.GetApplicationTeamOwner(c.Application)
		if err != nil {
			return "", err
		}
		t.AddAppEnv(c.Application, env, teamOwner)
		if hasUpstream && config.Upstream.Latest {
			d := &DeployApplicationVersion{
				Environment: env,
				Application: c.Application,
				Version:     lastRelease + 1,
				// the train should queue deployments, instead of giving up:
				LockBehaviour:  api.LockBehavior_RECORD,
				Authentication: c.Authentication,
			}
			err := t.Execute(d)
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
	return fmt.Sprintf("created undeploy-version %d of '%v'", lastRelease+1, c.Application), nil
}

func removeCommit(fs billy.Filesystem, commitID, application string) error {
	errorTemplate := func(message string, err error) error {
		return fmt.Errorf("while removing applicaton %s from commit %s and error was encountered, message: %s, error %w", application, commitID, message, err)
	}

	commitApplicationDir := commitApplicationDirectory(fs, commitID, application)
	if err := fs.Remove(commitApplicationDir); err != nil {
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

type UndeployApplication struct {
	Authentication
	Application string
}

func (u *UndeployApplication) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	fs := state.Filesystem
	lastRelease, err := GetLastRelease(fs, u.Application)
	if err != nil {
		return "", err
	}
	if lastRelease == 0 {
		return "", fmt.Errorf("UndeployApplication: error cannot undeploy non-existing application '%v'", u.Application)
	}
	isUndeploy, err := state.IsUndeployVersion(u.Application, lastRelease)
	if err != nil {
		return "", err
	}
	if !isUndeploy {
		return "", fmt.Errorf("UndeployApplication: error last release is not un-deployed application version of '%v'", u.Application)
	}
	appDir := applicationDirectory(fs, u.Application)
	configs, err := state.GetEnvironmentConfigs()
	for env := range configs {
		err := state.checkUserPermissions(ctx, env, u.Application, auth.PermissionDeployUndeploy, "", u.RBACConfig)
		if err != nil {
			return "", err
		}
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
		undeployFile := fs.Join(versionDir, "undeploy")

		_, err = fs.Stat(versionDir)
		if err != nil && errors.Is(err, os.ErrNotExist) {
			// if the app was never deployed here, that's not a reason to stop
			continue
		}

		_, err = fs.Stat(undeployFile)
		if err != nil && errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("UndeployApplication: error cannot un-deploy application '%v' the release '%v' is not un-deployed: '%v'", u.Application, env, undeployFile)
		}

	}
	// remove application
	releasesDir := fs.Join(appDir, "releases")
	files, err := fs.ReadDir(releasesDir)
	if err != nil {
		return "", fmt.Errorf("could not read the releases directory %s %w", releasesDir, err)
	}
	for _, file := range files {
		if file.IsDir() {
			releaseDir := fs.Join(releasesDir, file.Name())
			commitIDFile := fs.Join(releaseDir, "source_commit_id")
			var commitID string
			dat, err := util.ReadFile(fs, commitIDFile)
			if err != nil {
				// release does not have a corresponding commit, which might be the case if it's an undeploy release, no prob
				continue
			}
			commitID = string(dat)
			if valid.SHA1CommitID(commitID) {
				if err := removeCommit(fs, commitID, u.Application); err != nil {
					return "", fmt.Errorf("could not remove the commit: %w", err)
				}
			}
		}
	}
	if err = fs.Remove(appDir); err != nil {
		return "", err
	}
	for env := range configs {
		appDir := environmentApplicationDirectory(fs, env, u.Application)
		teamOwner, err := state.GetApplicationTeamOwner(u.Application)
		if err != nil {
			return "", err
		}
		t.AddAppEnv(u.Application, env, teamOwner)
		// remove environment application
		if err := fs.Remove(appDir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("UndeployApplication: unexpected error application '%v' environment '%v': '%w'", u.Application, env, err)
		}
	}
	return fmt.Sprintf("application '%v' was deleted successfully", u.Application), nil
}

type DeleteEnvFromApp struct {
	Authentication
	Application string
	Environment string
}

func (u *DeleteEnvFromApp) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	err := state.checkUserPermissions(ctx, u.Environment, u.Application, auth.PermissionDeleteEnvironmentApplication, "", u.RBACConfig)
	if err != nil {
		return "", err
	}
	fs := state.Filesystem
	thisSprintf := func(format string, a ...any) string {
		return fmt.Sprintf("DeleteEnvFromApp app '%s' on env '%s': %s", u.Application, u.Environment, fmt.Sprintf(format, a...))
	}

	if u.Application == "" {
		return "", fmt.Errorf(thisSprintf("Need to provide the application"))
	}

	if u.Environment == "" {
		return "", fmt.Errorf(thisSprintf("Need to provide the environment"))
	}

	envAppDir := environmentApplicationDirectory(fs, u.Environment, u.Application)
	entries, err := fs.ReadDir(envAppDir)
	if err != nil {
		return "", wrapFileError(err, envAppDir, thisSprintf("Could not open application directory. Does the app exist?"))
	}

	if entries == nil {
		// app was never deployed on this env, so that's unusual - but for idempotency we treat it just like a success case:
		return fmt.Sprintf("Attempted to remove environment '%v' from application '%v' but it did not exist.", u.Environment, u.Application), nil
	}

	err = fs.Remove(envAppDir)
	if err != nil {
		return "", wrapFileError(err, envAppDir, thisSprintf("Cannot delete app.'"))
	}

	t.DeleteEnvFromApp(u.Application, u.Environment)
	return fmt.Sprintf("Environment '%v' was removed from application '%v' successfully.", u.Environment, u.Application), nil
}

type CleanupOldApplicationVersions struct {
	Application string
}

// Finds old releases for an application
func findOldApplicationVersions(state *State, name string) ([]uint64, error) {
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

	if positionOfOldestVersion < (keptVersionsOnCleanup - 1) {
		return nil, nil
	}
	return versions[0 : positionOfOldestVersion-(keptVersionsOnCleanup-1)], err
}

func (c *CleanupOldApplicationVersions) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	fs := state.Filesystem
	oldVersions, err := findOldApplicationVersions(state, c.Application)
	if err != nil {
		return "", fmt.Errorf("cleanup: could not get application releases for app '%s': %w", c.Application, err)
	}

	msg := ""
	for _, oldRelease := range oldVersions {
		// delete oldRelease:
		releasesDir := releasesDirectoryWithVersion(fs, c.Application, oldRelease)
		_, err := fs.Stat(releasesDir)
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
	// we only cleanup non-deployed versions, so there are not changes for argoCd here
	return msg, nil
}

func wrapFileError(e error, filename string, message string) error {
	return fmt.Errorf("%s '%s': %w", message, filename, e)
}

type Authentication struct {
	RBACConfig auth.RBACConfig
}

type CreateEnvironmentLock struct {
	Authentication
	Environment string
	LockId      string
	Message     string
}

func (s *State) checkUserPermissions(ctx context.Context, env, application, action, team string, RBACConfig auth.RBACConfig) error {
	if !RBACConfig.DexEnabled {
		return nil
	}
	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("checkUserPermissions: user not found: %v", err))
	}

	envs, err := s.GetEnvironmentConfigs()
	if err != nil {
		return err
	}
	var group string
	for envName, config := range envs {
		if envName == env {
			group = mapper.DeriveGroupName(config, env)
			break
		}
	}
	if group == "" {
		return fmt.Errorf("group not found for environment: %s", env)
	}
	return auth.CheckUserPermissions(RBACConfig, user, env, team, group, application, action)
}

func (s *State) checkUserPermissionsEnvGroup(ctx context.Context, envGroup, application, action, team string, RBACConfig auth.RBACConfig) error {
	if !RBACConfig.DexEnabled {
		return nil
	}
	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("checkUserPermissions: user not found: %v", err))
	}
	return auth.CheckUserPermissions(RBACConfig, user, "*", team, envGroup, application, action)
}

// checkUserPermissionsCreateEnvironment check the permission for the environment creation action.
// This is a "special" case because the environment group is already provided on the request.
func (s *State) checkUserPermissionsCreateEnvironment(ctx context.Context, RBACConfig auth.RBACConfig, envConfig config.EnvironmentConfig) error {
	if !RBACConfig.DexEnabled {
		return nil
	}
	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("checkUserPermissions: user not found: %v", err))
	}
	envGroup := "*"
	// If an env group is provided on the request, use it on the permission.
	if envConfig.EnvironmentGroup != nil {
		envGroup = *(envConfig.EnvironmentGroup)
	}
	return auth.CheckUserPermissions(RBACConfig, user, "*", "", envGroup, "*", auth.PermissionCreateEnvironment)
}

func (c *CreateEnvironmentLock) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	err := state.checkUserPermissions(ctx, c.Environment, "*", auth.PermissionCreateLock, "", c.RBACConfig)
	if err != nil {
		return "", err
	}
	fs := state.Filesystem
	envDir := fs.Join("environments", c.Environment)
	if _, err := fs.Stat(envDir); err != nil {
		return "", fmt.Errorf("error accessing dir %q: %w", envDir, err)
	}
	chroot, err := fs.Chroot(envDir)
	if err != nil {
		return "", err
	}
	if err := createLock(ctx, chroot, c.LockId, c.Message); err != nil {
		return "", err
	}
	GaugeEnvLockMetric(fs, c.Environment)
	return fmt.Sprintf("Created lock %q on environment %q", c.LockId, c.Environment), nil
}

func createLock(ctx context.Context, fs billy.Filesystem, lockId, message string) error {
	locksDir := "locks"
	if err := fs.MkdirAll(locksDir, 0777); err != nil {
		return err
	}

	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return err
	}

	// create lock dir
	newLockDir := fs.Join(locksDir, lockId)
	if err := fs.MkdirAll(newLockDir, 0777); err != nil {
		return err
	}

	// write message
	if err := util.WriteFile(fs, fs.Join(newLockDir, "message"), []byte(message), 0666); err != nil {
		return err
	}

	// write email
	if err := util.WriteFile(fs, fs.Join(newLockDir, "created_by_email"), []byte(user.Email), 0666); err != nil {
		return err
	}

	// write name
	if err := util.WriteFile(fs, fs.Join(newLockDir, "created_by_name"), []byte(user.Name), 0666); err != nil {
		return err
	}

	// write date in iso format
	if err := util.WriteFile(fs, fs.Join(newLockDir, fieldCreatedAt), []byte(getTimeNow(ctx).Format(time.RFC3339)), 0666); err != nil {
		return err
	}
	return nil
}

type DeleteEnvironmentLock struct {
	Authentication
	Environment string
	LockId      string
}

func (c *DeleteEnvironmentLock) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	err := state.checkUserPermissions(ctx, c.Environment, "*", auth.PermissionDeleteLock, "", c.RBACConfig)
	if err != nil {
		return "", err
	}
	fs := state.Filesystem
	s := State{
		Filesystem: fs,
	}
	lockDir := s.GetEnvLockDir(c.Environment, c.LockId)
	_, err = fs.Stat(lockDir)
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

	apps, err := s.GetEnvironmentApplications(c.Environment)
	if err != nil {
		return "", fmt.Errorf("environment applications for %q not found: %v", c.Environment, err.Error())
	}

	additionalMessageFromDeployment := ""
	for _, appName := range apps {
		queueMessage, err := s.ProcessQueue(ctx, fs, c.Environment, appName)
		if err != nil {
			return "", err
		}
		if queueMessage != "" {
			additionalMessageFromDeployment = additionalMessageFromDeployment + "\n" + queueMessage
		}
	}
	GaugeEnvLockMetric(fs, c.Environment)
	return fmt.Sprintf("Deleted lock %q on environment %q%s", c.LockId, c.Environment, additionalMessageFromDeployment), nil
}

type CreateEnvironmentGroupLock struct {
	Authentication
	EnvironmentGroup string
	LockId           string
	Message          string
}

func (c *CreateEnvironmentGroupLock) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	err := state.checkUserPermissions(ctx, c.EnvironmentGroup, "*", auth.PermissionCreateLock, "", c.RBACConfig)
	if err != nil {
		return "", err
	}
	envNamesSorted, err := state.GetEnvironmentConfigsForGroup(c.EnvironmentGroup)
	if err != nil {
		return "", grpc.PublicError(ctx, err)
	}
	for index := range envNamesSorted {
		envName := envNamesSorted[index]
		x := CreateEnvironmentLock{
			Authentication: c.Authentication,
			Environment:    envName,
			LockId:         c.LockId, // the IDs should be the same for all. See `useLocksSimilarTo` in store.tsx
			Message:        c.Message,
		}
		if err := t.Execute(&x); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("Creating locks '%s' for environment group '%s':", c.LockId, c.EnvironmentGroup), nil
}

type DeleteEnvironmentGroupLock struct {
	Authentication
	EnvironmentGroup string
	LockId           string
}

func (c *DeleteEnvironmentGroupLock) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	err := state.checkUserPermissions(ctx, c.EnvironmentGroup, "*", auth.PermissionDeleteLock, "", c.RBACConfig)
	if err != nil {
		return "", err
	}
	envNamesSorted, err := state.GetEnvironmentConfigsForGroup(c.EnvironmentGroup)
	if err != nil {
		return "", grpc.PublicError(ctx, err)
	}
	for index := range envNamesSorted {
		envName := envNamesSorted[index]
		x := DeleteEnvironmentLock{
			Authentication: c.Authentication,
			Environment:    envName,
			LockId:         c.LockId,
		}
		if err := t.Execute(&x); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("Deleting locks '%s' for environment group '%s':", c.LockId, c.EnvironmentGroup), nil
}

type CreateEnvironmentApplicationLock struct {
	Authentication
	Environment string
	Application string
	LockId      string
	Message     string
}

func (c *CreateEnvironmentApplicationLock) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	// Note: it's possible to lock an application BEFORE it's even deployed to the environment.
	err := state.checkUserPermissions(ctx, c.Environment, c.Application, auth.PermissionCreateLock, "", c.RBACConfig)
	if err != nil {
		return "", err
	}
	fs := state.Filesystem
	envDir := fs.Join("environments", c.Environment)
	if _, err := fs.Stat(envDir); err != nil {
		return "", fmt.Errorf("error accessing dir %q: %w", envDir, err)
	}

	appDir := fs.Join(envDir, "applications", c.Application)
	if err := fs.MkdirAll(appDir, 0777); err != nil {
		return "", err
	}
	chroot, err := fs.Chroot(appDir)
	if err != nil {
		return "", err
	}
	if err := createLock(ctx, chroot, c.LockId, c.Message); err != nil {
		return "", err
	}
	GaugeEnvAppLockMetric(fs, c.Environment, c.Application)
	// locks are invisible to argoCd, so no changes here
	return fmt.Sprintf("Created lock %q on environment %q for application %q", c.LockId, c.Environment, c.Application), nil
}

type DeleteEnvironmentApplicationLock struct {
	Authentication
	Environment string
	Application string
	LockId      string
}

func (c *DeleteEnvironmentApplicationLock) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	err := state.checkUserPermissions(ctx, c.Environment, c.Application, auth.PermissionDeleteLock, "", c.RBACConfig)
	if err != nil {
		return "", err
	}
	fs := state.Filesystem
	lockDir := fs.Join("environments", c.Environment, "applications", c.Application, "locks", c.LockId)
	_, err = fs.Stat(lockDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", grpc.FailedPrecondition(ctx, fmt.Errorf("directory %s for app lock does not exist", lockDir))
		}
		return "", err
	}
	if err := fs.Remove(lockDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to delete directory %q: %w", lockDir, err)
	}
	s := State{
		Filesystem: fs,
	}
	if err := s.DeleteAppLockIfEmpty(ctx, c.Environment, c.Application); err != nil {
		return "", err
	}
	queueMessage, err := s.ProcessQueue(ctx, fs, c.Environment, c.Application)
	if err != nil {
		return "", err
	}
	GaugeEnvAppLockMetric(fs, c.Environment, c.Application)
	return fmt.Sprintf("Deleted lock %q on environment %q for application %q%s", c.LockId, c.Environment, c.Application, queueMessage), nil
}

type CreateEnvironment struct {
	Authentication
	Environment string
	Config      config.EnvironmentConfig
}

func (c *CreateEnvironment) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	err := state.checkUserPermissionsCreateEnvironment(ctx, c.RBACConfig, c.Config)
	if err != nil {
		return "", err
	}
	fs := state.Filesystem
	envDir := fs.Join("environments", c.Environment)
	// Creation of environment is possible, but configuring it is not if running in bootstrap mode.
	// Configuration needs to be done by modifying config map in source repo
	if state.BootstrapMode && c.Config != (config.EnvironmentConfig{}) {
		return "", fmt.Errorf("Cannot create or update configuration in bootstrap mode. Please update configuration in config map instead.")
	}
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
	// we do not need to inform argoCd when creating an environment, as there are no apps yet
	return fmt.Sprintf("create environment %q", c.Environment), file.Close()
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

	// TODO SU: maybe check here if that version is already deployed? or somewhere else ... or not at all...
	return fmt.Sprintf("Queued version %d of app %q in env %q", c.Version, c.Application, c.Environment), nil
}

type DeployApplicationVersion struct {
	Authentication
	Environment   string
	Application   string
	Version       uint64
	LockBehaviour api.LockBehavior
}

func (c *DeployApplicationVersion) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	err := state.checkUserPermissions(ctx, c.Environment, c.Application, auth.PermissionDeployRelease, "", c.RBACConfig)
	if err != nil {
		return "", err
	}
	fs := state.Filesystem
	// Check that the release exist and fetch manifest
	releaseDir := releasesDirectoryWithVersion(fs, c.Application, c.Version)
	manifest := fs.Join(releaseDir, "environments", c.Environment, "manifests.yaml")
	manifestContent := []byte{}
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
			envLocks, appLocks map[string]Lock
			err                error
		)
		envLocks, err = state.GetEnvironmentLocks(c.Environment)
		if err != nil {
			return "", err
		}
		appLocks, err = state.GetEnvironmentApplicationLocks(c.Environment, c.Application)
		if err != nil {
			return "", err
		}
		if len(envLocks) > 0 || len(appLocks) > 0 {
			switch c.LockBehaviour {
			case api.LockBehavior_RECORD:
				q := QueueApplicationVersion{
					Environment: c.Environment,
					Application: c.Application,
					Version:     c.Version,
				}
				return q.Transform(ctx, state, t)
			case api.LockBehavior_FAIL:
				return "", &LockedError{
					EnvironmentApplicationLocks: appLocks,
					EnvironmentLocks:            envLocks,
				}
			case api.LockBehavior_IGNORE:
				// just continue
			}
		}
	}
	// Create a symlink to the release
	applicationDir := fs.Join("environments", c.Environment, "applications", c.Application)
	if err := fs.MkdirAll(applicationDir, 0777); err != nil {
		return "", err
	}
	versionFile := fs.Join(applicationDir, "version")
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

	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return "", err
	}

	if err := util.WriteFile(fs, fs.Join(applicationDir, "deployed_by"), []byte(user.Name), 0666); err != nil {
		return "", err
	}
	if err := util.WriteFile(fs, fs.Join(applicationDir, "deployed_by_email"), []byte(user.Email), 0666); err != nil {
		return "", err
	}

	if err := util.WriteFile(fs, fs.Join(applicationDir, "deployed_at_utc"), []byte(getTimeNow(ctx).UTC().String()), 0666); err != nil {
		return "", err
	}

	s := State{
		Filesystem: fs,
	}
	err = s.DeleteQueuedVersionIfExists(c.Environment, c.Application)
	if err != nil {
		return "", err
	}
	d := &CleanupOldApplicationVersions{
		Application: c.Application,
	}
	if err := t.Execute(d); err != nil {
		return "", err
	}

	return fmt.Sprintf("deployed version %d of %q to %q", c.Version, c.Application, c.Environment), nil
}

type ReleaseTrain struct {
	Authentication
	Target string
	Team   string
}

func getEnvironmentGroupsEnvironmentsOrEnvironment(configs map[string]config.EnvironmentConfig, targetGroupName string) map[string]config.EnvironmentConfig {

	envGroupConfigs := make(map[string]config.EnvironmentConfig)

	for env, config := range configs {
		if config.EnvironmentGroup != nil && *config.EnvironmentGroup == targetGroupName {
			envGroupConfigs[env] = config
		}
	}
	if len(envGroupConfigs) == 0 {
		envConfig, ok := configs[targetGroupName]
		if ok {
			envGroupConfigs[targetGroupName] = envConfig
		}
	}
	return envGroupConfigs
}

func (c *ReleaseTrain) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	var targetGroupName = c.Target

	configs, err := state.GetEnvironmentConfigs()
	if err != nil {
		return "", grpc.InternalError(ctx, err)
	}
	var envGroupConfigs = getEnvironmentGroupsEnvironmentsOrEnvironment(configs, targetGroupName)

	if len(envGroupConfigs) == 0 {
		return "", grpc.PublicError(ctx, fmt.Errorf("could not find environment group or environment configs for '%v'", targetGroupName))
	}

	// this to sort the env, to make sure that for the same input we always got the same output
	envGroups := make([]string, 0, len(envGroupConfigs))
	for env := range envGroupConfigs {
		envGroups = append(envGroups, env)
	}
	sort.Strings(envGroups)

	for _, envName := range envGroups {
		if err := t.Execute(&envReleaseTrain{
			Parent:          c,
			Env:             envName,
			EnvConfigs:      configs,
			EnvGroupConfigs: envGroupConfigs,
		}); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf(
		"Release Train to environment/environment group '%s':\n",
		targetGroupName), nil
}

type envReleaseTrain struct {
	Parent          *ReleaseTrain
	Env             string
	EnvConfigs      map[string]config.EnvironmentConfig
	EnvGroupConfigs map[string]config.EnvironmentConfig
}

func (c *envReleaseTrain) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	envConfig := c.EnvGroupConfigs[c.Env]
	if envConfig.Upstream == nil {
		return fmt.Sprintf(
			"Environment '%q' does not have upstream configured - skipping.",
			c.Env), nil
	}
	err := state.checkUserPermissions(
		ctx,
		c.Env,
		"*",
		auth.PermissionDeployReleaseTrain,
		c.Parent.Team,
		c.Parent.RBACConfig,
	)
	if err != nil {
		return "", err
	}
	upstreamLatest := envConfig.Upstream.Latest
	upstreamEnvName := envConfig.Upstream.Environment
	if !upstreamLatest && upstreamEnvName == "" {
		return fmt.Sprintf(
			"Environment %q does not have upstream.latest or upstream.environment configured - skipping.",
			c.Env), nil
	}
	if upstreamLatest && upstreamEnvName != "" {
		return fmt.Sprintf(
			"Environment %q has both upstream.latest and upstream.environment configured - skipping.",
			c.Env), nil
	}
	source := upstreamEnvName
	if upstreamLatest {
		source = "latest"
	} else {
		_, ok := c.EnvConfigs[upstreamEnvName]
		if !ok {
			return fmt.Sprintf(
				"Could not find environment config for upstream env %q. Target env was %q",
				upstreamEnvName, c.Env), nil
		}
	}
	envLocks, err := state.GetEnvironmentLocks(c.Env)
	if err != nil {
		return "", grpc.InternalError(ctx, fmt.Errorf("could not get lock for environment %q: %w", c.Env, err))
	}
	if len(envLocks) > 0 {
		return fmt.Sprintf(
			"Target Environment '%s' is locked - skipping.",
			c.Env), nil
	}
	var apps []string
	if upstreamLatest {
		apps, err = state.GetApplications()
		if err != nil {
			return "", grpc.InternalError(ctx, fmt.Errorf("could not get all applications for %q: %w", source, err))
		}
	} else {
		apps, err = state.GetEnvironmentApplications(upstreamEnvName)
		if err != nil {
			return "", grpc.PublicError(ctx, fmt.Errorf("upstream environment (%q) does not have applications: %w", upstreamEnvName, err))
		}
	}
	sort.Strings(apps)
	// now iterate over all apps, deploying all that are not locked
	numServices := 0
	var skipped []string
	for _, appName := range apps {
		if c.Parent.Team != "" {
			if team, err := state.GetApplicationTeamOwner(appName); err != nil {
				return "", nil
			} else if c.Parent.Team != team {
				continue
			}
		}
		currentlyDeployedVersion, err := state.GetEnvironmentApplicationVersion(c.Env, appName)
		if err != nil {
			return "", grpc.PublicError(ctx, fmt.Errorf("application %q in env %q does not have a version deployed: %w", appName, c.Env, err))
		}
		var versionToDeploy uint64
		if upstreamLatest {
			versionToDeploy, err = GetLastRelease(state.Filesystem, appName)
			if err != nil {
				return "", grpc.PublicError(ctx, fmt.Errorf("application %q does not have a latest deployed: %w", appName, err))
			}
		} else {
			upstreamVersion, err := state.GetEnvironmentApplicationVersion(upstreamEnvName, appName)
			if err != nil {
				return "", grpc.PublicError(ctx, fmt.Errorf("application %q does not have a version deployed in env %q: %w", appName, upstreamEnvName, err))
			}
			if upstreamVersion == nil {
				skipped = append(skipped, fmt.Sprintf(
					"skipping because there is no version for application %q in env %q \n",
					appName, upstreamEnvName))
				continue
			}
			versionToDeploy = *upstreamVersion
		}
		if currentlyDeployedVersion != nil && *currentlyDeployedVersion == versionToDeploy {
			skipped = append(skipped, fmt.Sprintf(
				"skipping %q because it is already in the version %d\n",
				appName, *currentlyDeployedVersion))
			continue
		}

		d := &DeployApplicationVersion{
			Environment:    c.Env, // here we deploy to the next env
			Application:    appName,
			Version:        versionToDeploy,
			LockBehaviour:  api.LockBehavior_RECORD,
			Authentication: c.Parent.Authentication,
		}
		if err := t.Execute(d); err != nil {
			_, ok := err.(*LockedError)
			if ok {
				continue // locked errors are to be expected
			}
			if errors.Is(err, os.ErrNotExist) {
				continue // some apps do not exist on all envs, we ignore those
			}
			return "", grpc.InternalError(ctx, fmt.Errorf("unexpected error while deploying app %q to env %q: %w", appName, c.Env, err))
		}
		numServices += 1
	}
	teamInfo := ""
	if c.Parent.Team != "" {
		teamInfo = " for team '" + c.Parent.Team + "'"
	}
	if err := t.Execute(&skippedServices{
		Messages: skipped,
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Release Train to '%s' environment:\n\n"+
		"The release train deployed %d services from '%s' to '%s'%s",
		c.Env, numServices, source, c.Env, teamInfo,
	), nil
}

// skippedServices is a helper Transformer to generate the "skipped
// services" commit log.
type skippedServices struct {
	Messages []string
}

func (c *skippedServices) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	if len(c.Messages) == 0 {
		return "", nil
	}
	for _, msg := range c.Messages {
		if err := t.Execute(&skippedService{Message: msg}); err != nil {
			return "", err
		}
	}
	return "Skipped services", nil
}

type skippedService struct {
	Message string
}

func (c *skippedService) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	return c.Message, nil
}
