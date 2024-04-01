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

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/freiheit-com/kuberpult/pkg/metrics"
	"github.com/freiheit-com/kuberpult/pkg/ptr"

	"github.com/freiheit-com/kuberpult/pkg/uuid"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/event"
	git "github.com/libgit2/git2go/v34"

	"github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/valid"

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
	queueFileName         = "queued_version"
	yamlParsingError      = "# yaml parsing error"
	fieldSourceAuthor     = "source_author"
	fieldSourceMessage    = "source_message"
	fieldSourceCommitId   = "source_commit_id"
	fieldDisplayVersion   = "display_version"
	fieldSourceRepoUrl    = "sourceRepoUrl" // urgh, inconsistent
	fieldCreatedAt        = "created_at"
	fieldTeam             = "team"
	fieldNextCommidId     = "nextCommit"
	fieldPreviousCommitId = "previousCommit"
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

func manifestDirectoryWithReleasesVersion(fs billy.Filesystem, application string, version uint64) string {
	return fs.Join(releasesDirectoryWithVersion(fs, application, version), "environments")
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
		ddMetrics.Gauge("env_lock_count", GetEnvironmentLocksCount(fs, env), []string{"env:" + env}, 1) //nolint: errcheck
	}
}

func GaugeEnvAppLockMetric(fs billy.Filesystem, env, app string) {
	if ddMetrics != nil {
		ddMetrics.Gauge("app_lock_count", GetEnvironmentApplicationLocksCount(fs, env, app), []string{"app:" + app, "env:" + env}, 1) //nolint: errcheck
	}
}

func GaugeDeploymentMetric(_ context.Context, env, app string, timeInMinutes float64) error {
	if ddMetrics != nil {
		// store the time since the last deployment in minutes:
		err := ddMetrics.Gauge(
			"lastDeployed",
			timeInMinutes,
			[]string{metrics.EventTagApplication + ":" + app, metrics.EventTagEnvironment + ":" + env},
			1)
		return err
	}
	return nil
}

func sortFiles(gs []os.FileInfo) func(i int, j int) bool {
	return func(i, j int) bool {
		iIndex := gs[i].Name()
		jIndex := gs[j].Name()
		return iIndex < jIndex
	}
}

func UpdateDatadogMetrics(ctx context.Context, state *State, changes *TransformerResult, now time.Time) error {
	filesystem := state.Filesystem
	if ddMetrics == nil {
		return nil
	}
	configs, err := state.GetEnvironmentConfigs()
	if err != nil {
		return err
	}
	// sorting the environments to get a deterministic order of events:
	var configKeys []string = nil
	for k := range configs {
		configKeys = append(configKeys, k)
	}
	sort.Strings(configKeys)
	for i := range configKeys {
		env := configKeys[i]
		GaugeEnvLockMetric(filesystem, env)
		appsDir := filesystem.Join(environmentDirectory(filesystem, env), "applications")
		if entries, _ := filesystem.ReadDir(appsDir); entries != nil {
			// according to the docs, entries should already be sorted, but turns out it is not, so we sort it:
			sort.Slice(entries, sortFiles(entries))
			for _, app := range entries {
				GaugeEnvAppLockMetric(filesystem, env, app.Name())

				_, deployedAtTimeUtc, err := state.GetDeploymentMetaData(env, app.Name())
				if err != nil {
					return err
				}
				timeDiff := now.Sub(deployedAtTimeUtc)
				err = GaugeDeploymentMetric(ctx, env, app.Name(), timeDiff.Minutes())
				if err != nil {
					return err
				}
			}
		}
	}
	if changes != nil && ddMetrics != nil {
		for i := range changes.ChangedApps {
			oneChange := changes.ChangedApps[i]
			teamMessage := func() string {
				if oneChange.Team != "" {
					return fmt.Sprintf(" for team %s", oneChange.Team)
				}
				return ""
			}()
			evt := statsd.Event{
				Hostname:       "",
				AggregationKey: "",
				Priority:       "",
				SourceTypeName: "",
				AlertType:      "",
				Title:          "Kuberpult app deployed",
				Text:           fmt.Sprintf("Kuberpult has deployed %s to %s%s", oneChange.App, oneChange.Env, teamMessage),
				Timestamp:      now,
				Tags: []string{
					"kuberpult.application:" + oneChange.App,
					"kuberpult.environment:" + oneChange.Env,
					"kuberpult.team:" + oneChange.Team,
				},
			}
			err := ddMetrics.Event(&evt)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func RegularlySendDatadogMetrics(repo Repository, interval time.Duration, callBack func(repository Repository)) {
	metricEventTimer := time.NewTicker(interval * time.Second)
	for range metricEventTimer.C {
		callBack(repo)
	}
}

func GetRepositoryStateAndUpdateMetrics(ctx context.Context, repo Repository) {
	repoState := repo.State()
	if err := UpdateDatadogMetrics(ctx, repoState, nil, time.Now()); err != nil {
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
		ChangedApps:     nil,
		DeletedRootApps: nil,
		Commits:         nil,
		Context:         ctx,
		State:           s,
		Stack:           [][]string{nil},
	}
	if err := runner.Execute(t); err != nil {
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
		Team: "",
		App:  app,
		Env:  env,
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
	PreviousCommit  string
	NextCommit      string
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

	var checkForInvalidCommitId = func(commitId, helperText string) {
		if !valid.SHA1CommitID(commitId) {
			logger.FromContext(ctx).
				Sugar().
				Warnf("%s commit ID is not a valid SHA1 hash, should be exactly 40 characters [0-9a-fA-F] %s\n", commitId, helperText)
		}
	}

	checkForInvalidCommitId(c.SourceCommitId, "Source")
	checkForInvalidCommitId(c.PreviousCommit, "Previous")
	checkForInvalidCommitId(c.NextCommit, "Next")

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

	for env := range c.Manifests {
		allEnvsOfThisApp = append(allEnvsOfThisApp, env)
	}
	gen := getGenerator(ctx)
	eventUuid := gen.Generate()
	if c.WriteCommitData {
		err = writeCommitData(ctx, c.SourceCommitId, c.SourceMessage, c.Application, eventUuid, allEnvsOfThisApp, c.PreviousCommit, c.NextCommit, fs)
		if err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
	}

	for env, man := range c.Manifests {
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
				SourceTrain:     nil,
				Environment:     env,
				Application:     c.Application,
				Version:         version, // the train should queue deployments, instead of giving up:
				LockBehaviour:   api.LockBehavior_RECORD,
				Authentication:  c.Authentication,
				WriteCommitData: c.WriteCommitData,
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
	return fmt.Sprintf("created version %d of %q", version, c.Application), nil
}

func getGenerator(ctx context.Context) uuid.GenerateUUIDs {
	gen, ok := ctx.Value(ctxMarkerGenerateUuidKey).(uuid.GenerateUUIDs)
	if !ok || gen == nil {
		return uuid.RealUUIDGenerator{}
	}
	return gen
}

func AddGeneratorToContext(ctx context.Context, gen uuid.GenerateUUIDs) context.Context {
	return context.WithValue(ctx, ctxMarkerGenerateUuidKey, gen)
}

func writeCommitData(ctx context.Context, sourceCommitId string, sourceMessage string, app string, eventId string, environments []string, previousCommitId string, nextCommitId string, fs billy.Filesystem) error {
	if !valid.SHA1CommitID(sourceCommitId) {
		return nil
	}
	commitDir := commitDirectory(fs, sourceCommitId)
	if err := fs.MkdirAll(commitDir, 0777); err != nil {
		return GetCreateReleaseGeneralFailure(err)
	}
	if err := util.WriteFile(fs, fs.Join(commitDir, ".empty"), make([]byte, 0), 0666); err != nil {
		return GetCreateReleaseGeneralFailure(err)
	}

	if previousCommitId != "" && valid.SHA1CommitID(previousCommitId) {
		if err := writeNextPrevInfo(ctx, sourceCommitId, strings.ToLower(previousCommitId), fieldPreviousCommitId, app, fs); err != nil {
			return GetCreateReleaseGeneralFailure(err)
		}
	}
	if nextCommitId != "" && valid.SHA1CommitID(nextCommitId) {
		if err := writeNextPrevInfo(ctx, sourceCommitId, strings.ToLower(nextCommitId), fieldNextCommidId, app, fs); err != nil {
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
	envMap := make(map[string]struct{}, len(environments))
	for _, env := range environments {
		envMap[env] = struct{}{}
	}
	err := writeEvent(eventId, sourceCommitId, fs, &event.NewRelease{
		Environments: envMap,
	})
	if err != nil {
		return fmt.Errorf("error while writing event: %v", err)
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

func writeEvent(
	eventId string,
	sourceCommitId string,
	filesystem billy.Filesystem,
	ev event.Event,
) error {
	eventDir := commitEventDir(filesystem, sourceCommitId, eventId)
	if err := event.Write(filesystem, eventDir, ev); err != nil {
		return fmt.Errorf(
			"could not write an event for commit %s for uuid %s, error: %w",
			sourceCommitId, eventId, err)
	}
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
	Application     string
	WriteCommitData bool
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
				SourceTrain: nil,
				Environment: env,
				Application: c.Application,
				Version:     lastRelease + 1,
				// the train should queue deployments, instead of giving up:
				LockBehaviour:   api.LockBehavior_RECORD,
				Authentication:  c.Authentication,
				WriteCommitData: c.WriteCommitData,
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
	if err != nil {
		return "", err
	}
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
		Commit:                 nil,
		BootstrapMode:          false,
		EnvironmentConfigsPath: "",
		Filesystem:             fs,
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
		Commit:                 nil,
		BootstrapMode:          false,
		EnvironmentConfigsPath: "",
		Filesystem:             fs,
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

type CreateEnvironmentTeamLock struct {
	Authentication
	Environment string
	Team        string
	LockId      string
	Message     string
}

func (c *CreateEnvironmentTeamLock) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	// Note: it's possible to lock an application BEFORE it's even deployed to the environment.
	err := state.checkUserPermissions(ctx, c.Environment, "*", auth.PermissionCreateLock, c.Team, c.RBACConfig)
	if err != nil {
		return "", err
	}
	fs := state.Filesystem
	envDir := fs.Join("environments", c.Environment)
	if _, err := fs.Stat(envDir); err != nil {
		return "", fmt.Errorf("error accessing dir %q: %w", envDir, err)
	}

	teamDir := fs.Join(envDir, "teams", c.Team)
	if err := fs.MkdirAll(teamDir, 0777); err != nil {
		return "", err
	}
	chroot, err := fs.Chroot(teamDir)
	if err != nil {
		return "", err
	}
	if err := createLock(ctx, chroot, c.LockId, c.Message); err != nil {
		return "", err
	}
	//GaugeEnvAppLockMetric(fs, c.Environment, c.Application)
	// locks are invisible to argoCd, so no changes here
	return fmt.Sprintf("Created lock %q on environment %q for team %q", c.LockId, c.Environment, c.Team), nil
}

type DeleteEnvironmentTeamLock struct {
	Authentication
	Environment string
	Team        string
	LockId      string
}

func (c *DeleteEnvironmentTeamLock) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	err := state.checkUserPermissions(ctx, c.Environment, "", auth.PermissionDeleteLock, c.Team, c.RBACConfig)
	if err != nil {
		return "", err
	}
	fs := state.Filesystem
	lockDir := fs.Join("environments", c.Environment, "teams", c.Team, "locks", c.LockId)
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
		Commit:                 nil,
		BootstrapMode:          false,
		EnvironmentConfigsPath: "",
		Filesystem:             fs,
	}
	if err := s.DeleteTeamLockIfEmpty(ctx, c.Environment, c.Team); err != nil {
		return "", err
	}

	return fmt.Sprintf("Deleted lock %q on environment %q for team %q", c.LockId, c.Environment, c.Team), nil
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
	//exhaustruct:ignore
	defaultConfig := config.EnvironmentConfig{}
	if state.BootstrapMode && c.Config != defaultConfig {
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
	Environment     string
	Application     string
	Version         uint64
	LockBehaviour   api.LockBehavior
	WriteCommitData bool
	SourceTrain     *DeployApplicationVersionSource
}

type DeployApplicationVersionSource struct {
	TargetGroup *string
	Upstream    string
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

	lockPreventedDeployment := false
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
			teamLocks = map[string]Lock{} //This is a workaround. This transformer now needs a team file to be created on the application folder in order for the team locks to be loaded correctly. It turns out almost no other test that uses the create release transformer (that uses this one) sets the team, so the file wont exist and the tests will fail. IF we can't find the file, simply create an empty map and move on.
		} else {
			teamLocks, err = state.GetEnvironmentTeamLocks(c.Environment, string(team))
			if err != nil {
				return "", err
			}
		}

		if len(envLocks) > 0 || len(appLocks) > 0 || len(teamLocks) > 0 {
			if c.WriteCommitData {
				var lockType, lockMsg string
				if len(envLocks) > 0 {
					lockType = "environment"
					for _, lock := range envLocks {
						lockMsg = lock.Message
						break
					}
				} else {
					if len(appLocks) > 0 {
						lockType = "application"
						for _, lock := range appLocks {
							lockMsg = lock.Message
							break
						}
					} else {
						lockType = "team"
						for _, lock := range teamLocks {
							lockMsg = lock.Message
							break
						}
					}

				}
				if err := addEventForRelease(ctx, fs, releaseDir, &event.LockPreventedDeployment{
					Application: c.Application,
					Environment: c.Environment,
					LockMessage: lockMsg,
					LockType:    lockType,
				}); err != nil {
					return "", err
				}
				lockPreventedDeployment = true
			}
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
			}
		}
	}

	applicationDir := fs.Join("environments", c.Environment, "applications", c.Application)
	firstDeployment := false
	versionFile := fs.Join(applicationDir, "version")
	oldReleaseDir := ""

	//Check if there is a version of target app already deployed on target environment
	if _, err := fs.Lstat(versionFile); err == nil {
		//File Exists
		evaledPath, _ := fs.Readlink(versionFile) //Version is stored as symlink, eval it
		oldReleaseDir = evaledPath
	} else {
		//File does not exist
		firstDeployment = true
	}

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
		Commit:                 nil,
		BootstrapMode:          false,
		EnvironmentConfigsPath: "",
		Filesystem:             fs,
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

	if c.WriteCommitData { // write the corresponding event
		if err := addEventForRelease(ctx, fs, releaseDir, createDeploymentEvent(c.Application, c.Environment, c.SourceTrain)); err != nil {
			return "", err
		}

		if !firstDeployment && !lockPreventedDeployment {
			//If not first deployment and current deployment is successful, signal a new replaced by event
			if newReleaseCommitId, err := getCommitIDFromReleaseDir(ctx, fs, releaseDir); err == nil {
				if !valid.SHA1CommitID(newReleaseCommitId) {
					logger.FromContext(ctx).Sugar().Infof(
						"The source commit ID %s is not a valid/complete SHA1 hash, event cannot be stored.",
						newReleaseCommitId)
				} else {
					if err := addEventForRelease(ctx, fs, oldReleaseDir, createReplacedByEvent(c.Application, c.Environment, newReleaseCommitId)); err != nil {
						return "", err
					}
				}
			}
		} else {
			logger.FromContext(ctx).Sugar().Infof(
				"Release to replace decteted, but could not retrieve new commit information. Replaced-by event not stored.")
		}
	}

	return fmt.Sprintf("deployed version %d of %q to %q", c.Version, c.Application, c.Environment), nil
}

func getCommitIDFromReleaseDir(ctx context.Context, fs billy.Filesystem, releaseDir string) (string, error) {
	commitIdPath := fs.Join(releaseDir, "source_commit_id")

	commitIDBytes, err := util.ReadFile(fs, commitIdPath)
	if err != nil {
		logger.FromContext(ctx).Sugar().Infof(
			"Error while reading source commit ID file at %s, error %w. Deployment event not stored.",
			commitIdPath, err)
		return "", err
	}
	commitID := string(commitIDBytes)
	// if the stored source commit ID is invalid then we will not be able to store the event (simply)
	return commitID, nil
}

func addEventForRelease(ctx context.Context, fs billy.Filesystem, releaseDir string, ev event.Event) error {
	if commitID, err := getCommitIDFromReleaseDir(ctx, fs, releaseDir); err == nil {
		gen := getGenerator(ctx)
		eventUuid := gen.Generate()

		if !valid.SHA1CommitID(commitID) {
			logger.FromContext(ctx).Sugar().Infof(
				"The source commit ID %s is not a valid/complete SHA1 hash, event cannot be stored.",
				commitID)
			return nil
		}

		if err := writeEvent(eventUuid, commitID, fs, ev); err != nil {
			return fmt.Errorf(
				"could not write an event for commit %s, error: %w",
				commitID, err)
			//return fmt.Errorf(
			//	"could not write an event for commit %s with uuid %s, error: %w",
			//	commitID, eventUuid, err)
		}
	}
	return nil
}

func createDeploymentEvent(application, environment string, sourceTrain *DeployApplicationVersionSource) *event.Deployment {
	ev := event.Deployment{
		SourceTrainEnvironmentGroup: nil,
		SourceTrainUpstream:         nil,
		Application:                 application,
		Environment:                 environment,
	}
	if sourceTrain != nil {
		if sourceTrain.TargetGroup != nil {
			ev.SourceTrainEnvironmentGroup = sourceTrain.TargetGroup
		}
		ev.SourceTrainUpstream = &sourceTrain.Upstream
	}
	return &ev
}

func createReplacedByEvent(application, environment, commitId string) *event.ReplacedBy {
	ev := event.ReplacedBy{
		Application:       application,
		Environment:       environment,
		CommitIDtoReplace: commitId,
	}
	return &ev
}

type ReleaseTrain struct {
	Authentication
	Target          string
	Team            string
	CommitHash      string
	WriteCommitData bool
	Repo            Repository
}
type Overview struct {
	App     string
	Version uint64
}

func getEnvironmentInGroup(groups []*api.EnvironmentGroup, groupNameToReturn string, envNameToReturn string) *api.Environment {
	for _, currentGroup := range groups {
		if currentGroup.EnvironmentGroupName == groupNameToReturn {
			for _, currentEnv := range currentGroup.Environments {
				if currentEnv.Name == envNameToReturn {
					return currentEnv
				}
			}
		}
	}
	return nil
}

func getOverrideVersions(commitHash, upstreamEnvName string, repo Repository) (resp []Overview, err error) {
	oid, err := git.NewOid(commitHash)
	if err != nil {
		return nil, fmt.Errorf("Error creating new oid for commitHash %s: %w", commitHash, err)
	}
	s, err := repo.StateAt(oid)
	if err != nil {
		var gerr *git.GitError
		if errors.As(err, &gerr) {
			if gerr.Code == git.ErrorCodeNotFound {
				return nil, fmt.Errorf("ErrNotFound: %w", err)
			}
		}
		return nil, fmt.Errorf("unable to get oid: %w", err)
	}
	envs, err := s.GetEnvironmentConfigs()
	if err != nil {
		return nil, fmt.Errorf("unable to get EnvironmentConfigs for %s: %w", commitHash, err)
	}
	result := mapper.MapEnvironmentsToGroups(envs)
	for envName, config := range envs {
		var groupName = mapper.DeriveGroupName(config, envName)
		var envInGroup = getEnvironmentInGroup(result, groupName, envName)
		if upstreamEnvName != envInGroup.Name || upstreamEnvName != groupName {
			continue
		}
		apps, err := s.GetEnvironmentApplications(envName)
		if err != nil {
			return nil, fmt.Errorf("unable to get EnvironmentApplication for env %s: %w", envName, err)
		}
		for _, appName := range apps {
			app := api.Environment_Application{
				Version:            0,
				Locks:              nil,
				QueuedVersion:      0,
				UndeployVersion:    false,
				ArgoCd:             nil,
				DeploymentMetaData: nil,
				Name:               appName,
			}
			version, err := s.GetEnvironmentApplicationVersion(envName, appName)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("unable to get EnvironmentApplicationVersion for %s: %w", appName, err)
			}
			if version == nil {
				continue
			}
			app.Version = *version
			resp = append(resp, Overview{App: app.Name, Version: app.Version})
		}
	}
	return resp, nil
}

func (c *ReleaseTrain) getUpstreamLatestApp(upstreamLatest bool, state *State, ctx context.Context, upstreamEnvName, source, commitHash string) (apps []string, appVersions []Overview, err error) {
	if commitHash != "" {
		appVersions, err := getOverrideVersions(c.CommitHash, upstreamEnvName, c.Repo)
		if err != nil {
			return nil, nil, grpc.PublicError(ctx, fmt.Errorf("could not get app version for commitHash %s for %s: %w", c.CommitHash, c.Target, err))
		}
		// check that commit hash is not older than 20 commits in the past
		for _, app := range appVersions {
			apps = append(apps, app.App)
			versions, err := findOldApplicationVersions(state, app.App)
			if err != nil {
				return nil, nil, grpc.PublicError(ctx, fmt.Errorf("unable to find findOldApplicationVersions for app %s: %w", app.App, err))
			}
			if len(versions) > 0 && versions[0] > app.Version {
				return nil, nil, grpc.PublicError(ctx, fmt.Errorf("Version for app %s is older than 20 commits when running release train to commitHash %s: %w", app.App, c.CommitHash, err))
			}

		}
		return apps, appVersions, nil
	}
	if upstreamLatest {
		apps, err = state.GetApplications()
		if err != nil {
			return nil, nil, grpc.PublicError(ctx, fmt.Errorf("could not get all applications for %q: %w", source, err))
		}
		return apps, nil, nil
	}
	apps, err = state.GetEnvironmentApplications(upstreamEnvName)
	if err != nil {
		return nil, nil, grpc.PublicError(ctx, fmt.Errorf("upstream environment (%q) does not have applications: %w", upstreamEnvName, err))
	}
	return apps, nil, nil
}

func getEnvironmentGroupsEnvironmentsOrEnvironment(configs map[string]config.EnvironmentConfig, targetGroupName string) (map[string]config.EnvironmentConfig, bool) {
	envGroupConfigs := make(map[string]config.EnvironmentConfig)
	isEnvGroup := false

	for env, config := range configs {
		if config.EnvironmentGroup != nil && *config.EnvironmentGroup == targetGroupName {
			isEnvGroup = true
			envGroupConfigs[env] = config
		}
	}
	if len(envGroupConfigs) == 0 {
		envConfig, ok := configs[targetGroupName]
		if ok {
			envGroupConfigs[targetGroupName] = envConfig
		}
	}
	return envGroupConfigs, isEnvGroup
}

type ReleaseTrainApplicationPrognosis struct {
	SkipCause *api.ReleaseTrainAppPrognosis_SkipCause
	Version   uint64
}

type ReleaseTrainEnvironmentPrognosis struct {
	SkipCause *api.ReleaseTrainEnvPrognosis_SkipCause
	Error     error
	// map key is the name of the app
	AppsPrognoses map[string]ReleaseTrainApplicationPrognosis
}

type ReleaseTrainPrognosisOutcome = uint64

type ReleaseTrainPrognosis struct {
	Error                error
	EnvironmentPrognoses map[string]ReleaseTrainEnvironmentPrognosis
}

func (c *ReleaseTrain) Prognosis(
	ctx context.Context,
	state *State,
) ReleaseTrainPrognosis {
	configs, err := state.GetEnvironmentConfigs()
	if err != nil {
		return ReleaseTrainPrognosis{
			Error:                grpc.InternalError(ctx, err),
			EnvironmentPrognoses: nil,
		}
	}

	var targetGroupName = c.Target
	var envGroupConfigs, isEnvGroup = getEnvironmentGroupsEnvironmentsOrEnvironment(configs, targetGroupName)
	if len(envGroupConfigs) == 0 {
		return ReleaseTrainPrognosis{
			Error:                grpc.PublicError(ctx, fmt.Errorf("could not find environment group or environment configs for '%v'", targetGroupName)),
			EnvironmentPrognoses: nil,
		}
	}

	// this to sort the env, to make sure that for the same input we always got the same output
	envGroups := make([]string, 0, len(envGroupConfigs))
	for env := range envGroupConfigs {
		envGroups = append(envGroups, env)
	}
	sort.Strings(envGroups)

	envPrognoses := make(map[string]ReleaseTrainEnvironmentPrognosis)

	for _, envName := range envGroups {
		var trainGroup *string
		if isEnvGroup {
			trainGroup = ptr.FromString(targetGroupName)
		}

		envReleaseTrain := &envReleaseTrain{
			Parent:          c,
			Env:             envName,
			EnvConfigs:      configs,
			EnvGroupConfigs: envGroupConfigs,
			WriteCommitData: c.WriteCommitData,
			TrainGroup:      trainGroup,
		}

		envPrognosis := envReleaseTrain.prognosis(ctx, state)

		if envPrognosis.Error != nil {
			return ReleaseTrainPrognosis{
				Error:                envPrognosis.Error,
				EnvironmentPrognoses: nil,
			}
		}

		envPrognoses[envName] = envPrognosis
	}

	return ReleaseTrainPrognosis{
		Error:                nil,
		EnvironmentPrognoses: envPrognoses,
	}
}

func (c *ReleaseTrain) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	prognosis := c.Prognosis(ctx, state)

	if prognosis.Error != nil {
		return "", prognosis.Error
	}

	var targetGroupName = c.Target
	configs, _ := state.GetEnvironmentConfigs()
	var envGroupConfigs, isEnvGroup = getEnvironmentGroupsEnvironmentsOrEnvironment(configs, targetGroupName)

	// sorting for determinism
	envNames := make([]string, 0, len(prognosis.EnvironmentPrognoses))
	for envName := range prognosis.EnvironmentPrognoses {
		envNames = append(envNames, envName)
	}
	sort.Strings(envNames)

	for _, envName := range envNames {
		var trainGroup *string
		if isEnvGroup {
			trainGroup = ptr.FromString(targetGroupName)
		}

		if err := t.Execute(&envReleaseTrain{
			Parent:          c,
			Env:             envName,
			EnvConfigs:      configs,
			EnvGroupConfigs: envGroupConfigs,
			WriteCommitData: c.WriteCommitData,
			TrainGroup:      trainGroup,
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
	WriteCommitData bool
	TrainGroup      *string
}

func (c *envReleaseTrain) prognosis(
	ctx context.Context,
	state *State,
) ReleaseTrainEnvironmentPrognosis {
	envConfig := c.EnvGroupConfigs[c.Env]
	if envConfig.Upstream == nil {
		return ReleaseTrainEnvironmentPrognosis{
			SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
				SkipCause: api.ReleaseTrainEnvSkipCause_ENV_HAS_NO_UPSTREAM,
			},
			Error:         nil,
			AppsPrognoses: nil,
		}
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
		return ReleaseTrainEnvironmentPrognosis{
			SkipCause:     nil,
			Error:         err,
			AppsPrognoses: nil,
		}
	}

	upstreamLatest := envConfig.Upstream.Latest
	upstreamEnvName := envConfig.Upstream.Environment
	if !upstreamLatest && upstreamEnvName == "" {
		return ReleaseTrainEnvironmentPrognosis{
			SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
				SkipCause: api.ReleaseTrainEnvSkipCause_ENV_HAS_NO_UPSTREAM_LATEST_OR_UPSTREAM_ENV,
			},
			Error:         nil,
			AppsPrognoses: nil,
		}
	}

	if upstreamLatest && upstreamEnvName != "" {
		return ReleaseTrainEnvironmentPrognosis{
			SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
				SkipCause: api.ReleaseTrainEnvSkipCause_ENV_HAS_BOTH_UPSTREAM_LATEST_AND_UPSTREAM_ENV,
			},
			Error:         nil,
			AppsPrognoses: nil,
		}
	}

	if !upstreamLatest {
		_, ok := c.EnvConfigs[upstreamEnvName]
		if !ok {
			return ReleaseTrainEnvironmentPrognosis{
				SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
					SkipCause: api.ReleaseTrainEnvSkipCause_UPSTREAM_ENV_CONFIG_NOT_FOUND,
				},
				Error:         nil,
				AppsPrognoses: nil,
			}
		}
	}

	envLocks, err := state.GetEnvironmentLocks(c.Env)
	if err != nil {
		return ReleaseTrainEnvironmentPrognosis{
			SkipCause:     nil,
			Error:         grpc.InternalError(ctx, fmt.Errorf("could not get lock for environment %q: %w", c.Env, err)),
			AppsPrognoses: nil,
		}
	}

	if len(envLocks) > 0 {
		return ReleaseTrainEnvironmentPrognosis{
			SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
				SkipCause: api.ReleaseTrainEnvSkipCause_ENV_IS_LOCKED,
			},
			Error:         nil,
			AppsPrognoses: nil,
		}
	}

	source := upstreamEnvName
	if upstreamLatest {
		source = "latest"
	}

	apps, overrideVersions, err := c.Parent.getUpstreamLatestApp(upstreamLatest, state, ctx, upstreamEnvName, source, c.Parent.CommitHash)
	if err != nil {
		return ReleaseTrainEnvironmentPrognosis{
			SkipCause:     nil,
			Error:         err,
			AppsPrognoses: nil,
		}
	}
	sort.Strings(apps)

	appsPrognoses := make(map[string]ReleaseTrainApplicationPrognosis)

	for _, appName := range apps {
		if c.Parent.Team != "" {
			if team, err := state.GetApplicationTeamOwner(appName); err != nil {
				return ReleaseTrainEnvironmentPrognosis{
					SkipCause:     nil,
					Error:         err,
					AppsPrognoses: nil,
				}
			} else if c.Parent.Team != team {
				continue
			}
		}

		currentlyDeployedVersion, err := state.GetEnvironmentApplicationVersion(c.Env, appName)
		if err != nil {
			return ReleaseTrainEnvironmentPrognosis{
				SkipCause:     nil,
				Error:         grpc.PublicError(ctx, fmt.Errorf("application %q in env %q does not have a version deployed: %w", appName, c.Env, err)),
				AppsPrognoses: nil,
			}
		}

		var versionToDeploy uint64
		if overrideVersions != nil {
			for _, override := range overrideVersions {
				if override.App == appName {
					versionToDeploy = override.Version
				}
			}
		} else if upstreamLatest {
			versionToDeploy, err = GetLastRelease(state.Filesystem, appName)
			if err != nil {
				return ReleaseTrainEnvironmentPrognosis{
					SkipCause:     nil,
					Error:         grpc.PublicError(ctx, fmt.Errorf("application %q does not have a latest deployed: %w", appName, err)),
					AppsPrognoses: nil,
				}
			}
		} else {
			upstreamVersion, err := state.GetEnvironmentApplicationVersion(upstreamEnvName, appName)
			if err != nil {
				return ReleaseTrainEnvironmentPrognosis{
					SkipCause:     nil,
					Error:         grpc.PublicError(ctx, fmt.Errorf("application %q does not have a version deployed in env %q: %w", appName, upstreamEnvName, err)),
					AppsPrognoses: nil,
				}
			}
			if upstreamVersion == nil {
				appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
					SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
						SkipCause: api.ReleaseTrainAppSkipCause_APP_HAS_NO_VERSION_IN_UPSTREAM_ENV,
					},
					Version: 0,
				}
				continue
			}
			versionToDeploy = *upstreamVersion
		}
		if currentlyDeployedVersion != nil && *currentlyDeployedVersion == versionToDeploy {
			appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
				SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
					SkipCause: api.ReleaseTrainAppSkipCause_APP_ALREADY_IN_UPSTREAM_VERSION,
				},
				Version: 0,
			}
			continue
		}

		appLocks, err := state.GetEnvironmentApplicationLocks(c.Env, appName)

		if err != nil {
			return ReleaseTrainEnvironmentPrognosis{
				SkipCause:     nil,
				Error:         err,
				AppsPrognoses: nil,
			}
		}

		if len(appLocks) > 0 {
			appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
				SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
					SkipCause: api.ReleaseTrainAppSkipCause_APP_IS_LOCKED,
				},
				Version: 0,
			}
			continue
		}

		fs := state.Filesystem

		releaseDir := releasesDirectoryWithVersion(fs, appName, versionToDeploy)
		manifest := fs.Join(releaseDir, "environments", c.Env, "manifests.yaml")

		if _, err := fs.Stat(manifest); err != nil {
			appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
				SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
					SkipCause: api.ReleaseTrainAppSkipCause_APP_DOES_NOT_EXIST_IN_ENV,
				},
				Version: 0,
			}
			continue
		}

		appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
			SkipCause: nil,
			Version:   versionToDeploy,
		}
	}

	return ReleaseTrainEnvironmentPrognosis{
		SkipCause:     nil,
		Error:         nil,
		AppsPrognoses: appsPrognoses,
	}
}

func (c *envReleaseTrain) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
) (string, error) {
	renderEnvironmentSkipCause := func(SkipCause *api.ReleaseTrainEnvPrognosis_SkipCause) string {
		envConfig := c.EnvGroupConfigs[c.Env]
		upstreamEnvName := envConfig.Upstream.Environment
		switch SkipCause.SkipCause {
		case api.ReleaseTrainEnvSkipCause_ENV_HAS_NO_UPSTREAM:
			return fmt.Sprintf("Environment '%q' does not have upstream configured - skipping.", c.Env)
		case api.ReleaseTrainEnvSkipCause_ENV_HAS_NO_UPSTREAM_LATEST_OR_UPSTREAM_ENV:
			return fmt.Sprintf("Environment %q does not have upstream.latest or upstream.environment configured - skipping.", c.Env)
		case api.ReleaseTrainEnvSkipCause_ENV_HAS_BOTH_UPSTREAM_LATEST_AND_UPSTREAM_ENV:
			return fmt.Sprintf("Environment %q has both upstream.latest and upstream.environment configured - skipping.", c.Env)
		case api.ReleaseTrainEnvSkipCause_UPSTREAM_ENV_CONFIG_NOT_FOUND:
			return fmt.Sprintf("Could not find environment config for upstream env %q. Target env was %q", upstreamEnvName, c.Env)
		case api.ReleaseTrainEnvSkipCause_ENV_IS_LOCKED:
			return fmt.Sprintf("Target Environment '%s' is locked - skipping.", c.Env)
		default:
			return fmt.Sprintf("Environment '%s' is skipped for an unrecognized reason", c.Env)
		}
	}

	renderApplicationSkipCause := func(SkipCause *api.ReleaseTrainAppPrognosis_SkipCause, appName string) string {
		envConfig := c.EnvGroupConfigs[c.Env]
		upstreamEnvName := envConfig.Upstream.Environment
		currentlyDeployedVersion, _ := state.GetEnvironmentApplicationVersion(c.Env, appName)
		switch SkipCause.SkipCause {
		case api.ReleaseTrainAppSkipCause_APP_HAS_NO_VERSION_IN_UPSTREAM_ENV:
			return fmt.Sprintf("skipping because there is no version for application %q in env %q \n", appName, upstreamEnvName)
		case api.ReleaseTrainAppSkipCause_APP_ALREADY_IN_UPSTREAM_VERSION:
			return fmt.Sprintf("skipping %q because it is already in the version %d\n", appName, currentlyDeployedVersion)
		case api.ReleaseTrainAppSkipCause_APP_IS_LOCKED:
			return fmt.Sprintf("skipping application %q in environment %q due to application lock", appName, c.Env)
		case api.ReleaseTrainAppSkipCause_APP_DOES_NOT_EXIST_IN_ENV:
			return fmt.Sprintf("skipping application %q in environment %q because it doesn't exist there", appName, c.Env)
		default:
			return fmt.Sprintf("skipping application %q in environment %q for an unrecognized reason", appName, c.Env)
		}
	}

	prognosis := c.prognosis(ctx, state)

	if prognosis.Error != nil {
		return "", prognosis.Error
	}
	if prognosis.SkipCause != nil {
		return renderEnvironmentSkipCause(prognosis.SkipCause), nil
	}

	envConfig := c.EnvGroupConfigs[c.Env]
	upstreamLatest := envConfig.Upstream.Latest
	upstreamEnvName := envConfig.Upstream.Environment

	source := upstreamEnvName
	if upstreamLatest {
		source = "latest"
	}

	// now iterate over all apps, deploying all that are not locked
	var skipped []string

	// sorting for determinism
	appNames := make([]string, 0, len(prognosis.AppsPrognoses))
	for appName := range prognosis.AppsPrognoses {
		appNames = append(appNames, appName)
	}
	sort.Strings(appNames)

	for _, appName := range appNames {
		appPrognosis := prognosis.AppsPrognoses[appName]
		if appPrognosis.SkipCause != nil {
			skipped = append(skipped, renderApplicationSkipCause(appPrognosis.SkipCause, appName))
			continue
		}
		d := &DeployApplicationVersion{
			Environment:     c.Env, // here we deploy to the next env
			Application:     appName,
			Version:         appPrognosis.Version,
			LockBehaviour:   api.LockBehavior_RECORD,
			Authentication:  c.Parent.Authentication,
			WriteCommitData: c.WriteCommitData,
			SourceTrain: &DeployApplicationVersionSource{
				Upstream:    upstreamEnvName,
				TargetGroup: c.TrainGroup,
			},
		}
		if err := t.Execute(d); err != nil {
			return "", grpc.InternalError(ctx, fmt.Errorf("unexpected error while deploying app %q to env %q: %w", appName, c.Env, err))
		}
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
		c.Env, len(prognosis.AppsPrognoses), source, c.Env, teamInfo,
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
