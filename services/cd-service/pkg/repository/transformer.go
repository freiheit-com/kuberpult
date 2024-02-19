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
	"k8s.io/utils/strings/slices"

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
	Transform(context.Context, *State) (commitMsg string, changes *TransformerResult, e error)
}

type TransformerFunc func(context.Context, *State) (string, *TransformerResult, error)

func (t TransformerFunc) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	return (t)(ctx, state)
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

func (c *CreateApplicationVersion) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	version, err := c.calculateVersion(state)
	if err != nil {
		return "", nil, err
	}
	fs := state.Filesystem
	if !valid.ApplicationName(c.Application) {
		return "", nil, GetCreateReleaseAppNameTooLong(c.Application, valid.AppNameRegExp, valid.MaxAppNameLen)
	}
	releaseDir := releasesDirectoryWithVersion(fs, c.Application, version)
	appDir := applicationDirectory(fs, c.Application)
	if err = fs.MkdirAll(releaseDir, 0777); err != nil {
		return "", nil, GetCreateReleaseGeneralFailure(err)
	}

	if !valid.SHA1CommitID(c.SourceCommitId) {
		logger.FromContext(ctx).Sugar().Warnf("commit ID is not a valid SHA1 hash, should be exactly 40 characters [0-9a-fA-F] %s\n", c.SourceCommitId)
	}

	configs, err := state.GetEnvironmentConfigs()
	if err != nil {
		if errors.Is(err, InvalidJson) {
			return "", nil, err
		}
		return "", nil, GetCreateReleaseGeneralFailure(err)
	}

	if c.SourceCommitId != "" {
		c.SourceCommitId = strings.ToLower(c.SourceCommitId)
		if err := util.WriteFile(fs, fs.Join(releaseDir, fieldSourceCommitId), []byte(c.SourceCommitId), 0666); err != nil {
			return "", nil, GetCreateReleaseGeneralFailure(err)
		}
		if valid.SHA1CommitID(c.SourceCommitId) {
			commitDir := commitApplicationDirectory(fs, c.SourceCommitId, c.Application)
			if err := fs.MkdirAll(commitDir, 0777); err != nil {
				return "", nil, GetCreateReleaseGeneralFailure(err)
			}
		}
	}
	if c.SourceAuthor != "" {
		if err := util.WriteFile(fs, fs.Join(releaseDir, fieldSourceAuthor), []byte(c.SourceAuthor), 0666); err != nil {
			return "", nil, GetCreateReleaseGeneralFailure(err)
		}
	}
	if c.SourceMessage != "" {
		if err := util.WriteFile(fs, fs.Join(releaseDir, fieldSourceMessage), []byte(c.SourceMessage), 0666); err != nil {
			return "", nil, GetCreateReleaseGeneralFailure(err)
		}
	}
	if c.DisplayVersion != "" {
		if err := util.WriteFile(fs, fs.Join(releaseDir, fieldDisplayVersion), []byte(c.DisplayVersion), 0666); err != nil {
			return "", nil, GetCreateReleaseGeneralFailure(err)
		}
	}
	if err := util.WriteFile(fs, fs.Join(releaseDir, fieldCreatedAt), []byte(getTimeNow(ctx).Format(time.RFC3339)), 0666); err != nil {
		return "", nil, GetCreateReleaseGeneralFailure(err)
	}
	if c.Team != "" {
		if err := util.WriteFile(fs, fs.Join(appDir, fieldTeam), []byte(c.Team), 0666); err != nil {
			return "", nil, GetCreateReleaseGeneralFailure(err)
		}
	}
	if c.SourceRepoUrl != "" {
		if err := util.WriteFile(fs, fs.Join(appDir, fieldSourceRepoUrl), []byte(c.SourceRepoUrl), 0666); err != nil {
			return "", nil, GetCreateReleaseGeneralFailure(err)
		}
	}
	result := ""
	isLatest, err := isLatestsVersion(state, c.Application, version)
	if err != nil {
		return "", nil, GetCreateReleaseGeneralFailure(err)
	}
	if !isLatest {
		// check that we can actually backfill this version
		oldVersions, err := findOldApplicationVersions(state, c.Application)
		if err != nil {
			return "", nil, GetCreateReleaseGeneralFailure(err)
		}
		for _, oldVersion := range oldVersions {
			if version == oldVersion {
				return "", nil, GetCreateReleaseTooOld()
			}
		}
	}

	changes := &TransformerResult{}
	var allEnvsOfThisApp []string = nil
	for env, man := range c.Manifests {
		allEnvsOfThisApp = append(allEnvsOfThisApp, env)
		err := state.checkUserPermissions(ctx, env, c.Application, auth.PermissionCreateRelease, c.Team, c.RBACConfig)
		if err != nil {
			return "", nil, GetCreateReleaseGeneralFailure(err)
		}
		envDir := fs.Join(releaseDir, "environments", env)

		config, found := configs[env]
		hasUpstream := false
		if found {
			hasUpstream = config.Upstream != nil
		}

		if err = fs.MkdirAll(envDir, 0777); err != nil {
			return "", nil, GetCreateReleaseGeneralFailure(err)
		}
		if err := util.WriteFile(fs, fs.Join(envDir, "manifests.yaml"), []byte(man), 0666); err != nil {
			return "", nil, GetCreateReleaseGeneralFailure(err)
		}
		teamOwner, err := state.GetApplicationTeamOwner(c.Application)
		if err != nil {
			return "", nil, err
		}
		changes.AddAppEnv(c.Application, env, teamOwner)
		if hasUpstream && config.Upstream.Latest && isLatest {
			d := &DeployApplicationVersion{
				Environment:     env,
				Application:     c.Application,
				Version:         version, // the train should queue deployments, instead of giving up:
				LockBehaviour:   api.LockBehavior_RECORD,
				Authentication:  c.Authentication,
				WriteCommitData: c.WriteCommitData,
			}
			deployResult, subChanges, err := d.Transform(ctx, state)
			changes.Combine(subChanges)
			if err != nil {
				_, ok := err.(*LockedError)
				if ok {
					continue // LockedErrors are expected
				} else {
					return "", nil, GetCreateReleaseGeneralFailure(err)
				}
			}
			result = result + deployResult + "\n"
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
			return "", nil, GetCreateReleaseGeneralFailure(err)
		}
	}
	return fmt.Sprintf("created version %d of %q\n%s", version, c.Application, result), changes, nil
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
	err := writeNewReleaseEvent(ctx, eventId, sourceCommitId, fs, environments)
	if err != nil {
		return fmt.Errorf("error while writing event: %v", err)
	}
	return nil
}

func writeNewReleaseEvent(ctx context.Context, eventId string, sourceCommitId string, filesystem billy.Filesystem, envs []string) error {
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
	Application     string
	WriteCommitData bool
}

func (c *CreateUndeployApplicationVersion) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	fs := state.Filesystem
	lastRelease, err := GetLastRelease(fs, c.Application)
	if err != nil {
		return "", nil, err
	}
	changes := &TransformerResult{}
	if lastRelease == 0 {
		return "", nil, fmt.Errorf("cannot undeploy non-existing application '%v'", c.Application)
	}

	releaseDir := releasesDirectoryWithVersion(fs, c.Application, lastRelease+1)
	if err = fs.MkdirAll(releaseDir, 0777); err != nil {
		return "", nil, err
	}

	configs, err := state.GetEnvironmentConfigs()
	if err != nil {
		return "", nil, err
	}
	// this is a flag to indicate that this is the special "undeploy" version
	if err := util.WriteFile(fs, fs.Join(releaseDir, "undeploy"), []byte(""), 0666); err != nil {
		return "", nil, err
	}
	if err := util.WriteFile(fs, fs.Join(releaseDir, fieldCreatedAt), []byte(getTimeNow(ctx).Format(time.RFC3339)), 0666); err != nil {
		return "", nil, err
	}
	result := ""
	for env := range configs {
		err := state.checkUserPermissions(ctx, env, c.Application, auth.PermissionCreateUndeploy, "", c.RBACConfig)
		if err != nil {
			return "", nil, err
		}
		envDir := fs.Join(releaseDir, "environments", env)

		config, found := configs[env]
		hasUpstream := false
		if found {
			hasUpstream = config.Upstream != nil
		}

		if err = fs.MkdirAll(envDir, 0777); err != nil {
			return "", nil, err
		}
		// note that the manifest is empty here!
		// but actually it's not quite empty!
		// The function we are using in DeployApplication version is `util.WriteFile`. And that does not allow overwriting files with empty content.
		// We work around this unusual behavior by writing a space into the file
		if err := util.WriteFile(fs, fs.Join(envDir, "manifests.yaml"), []byte(" "), 0666); err != nil {
			return "", nil, err
		}
		teamOwner, err := state.GetApplicationTeamOwner(c.Application)
		if err != nil {
			return "", nil, err
		}
		changes.AddAppEnv(c.Application, env, teamOwner)
		if hasUpstream && config.Upstream.Latest {
			d := &DeployApplicationVersion{
				Environment: env,
				Application: c.Application,
				Version:     lastRelease + 1,
				// the train should queue deployments, instead of giving up:
				LockBehaviour:   api.LockBehavior_RECORD,
				Authentication:  c.Authentication,
				WriteCommitData: c.WriteCommitData,
			}
			deployResult, subChanges, err := d.Transform(ctx, state)
			changes.Combine(subChanges)
			if err != nil {
				_, ok := err.(*LockedError)
				if ok {
					continue // locked error are expected
				} else {
					return "", nil, err
				}
			}
			result = result + deployResult + "\n"
		}
	}
	return fmt.Sprintf("created undeploy-version %d of '%v'\n%s", lastRelease+1, c.Application, result), changes, nil
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

func (u *UndeployApplication) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	fs := state.Filesystem
	lastRelease, err := GetLastRelease(fs, u.Application)
	if err != nil {
		return "", nil, err
	}
	if lastRelease == 0 {
		return "", nil, fmt.Errorf("UndeployApplication: error cannot undeploy non-existing application '%v'", u.Application)
	}
	isUndeploy, err := state.IsUndeployVersion(u.Application, lastRelease)
	if err != nil {
		return "", nil, err
	}
	if !isUndeploy {
		return "", nil, fmt.Errorf("UndeployApplication: error last release is not un-deployed application version of '%v'", u.Application)
	}
	appDir := applicationDirectory(fs, u.Application)
	configs, err := state.GetEnvironmentConfigs()
	for env := range configs {
		err := state.checkUserPermissions(ctx, env, u.Application, auth.PermissionDeployUndeploy, "", u.RBACConfig)
		if err != nil {
			return "", nil, err
		}
		envAppDir := environmentApplicationDirectory(fs, env, u.Application)
		entries, err := fs.ReadDir(envAppDir)
		if err != nil {
			return "", nil, wrapFileError(err, envAppDir, "UndeployApplication: Could not open application directory. Does the app exist?")
		}
		if entries == nil {
			// app was never deployed on this env, so we must ignore it!
			continue
		}

		appLocksDir := fs.Join(envAppDir, "locks")
		err = fs.Remove(appLocksDir)
		if err != nil {
			return "", nil, fmt.Errorf("UndeployApplication: cannot delete app locks '%v'", appLocksDir)
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
			return "", nil, fmt.Errorf("UndeployApplication: error cannot un-deploy application '%v' the release '%v' is not un-deployed: '%v'", u.Application, env, undeployFile)
		}

	}
	// remove application
	releasesDir := fs.Join(appDir, "releases")
	files, err := fs.ReadDir(releasesDir)
	if err != nil {
		return "", nil, fmt.Errorf("could not read the releases directory %s %w", releasesDir, err)
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
					return "", nil, fmt.Errorf("could not remove the commit: %w", err)
				}
			}
		}
	}
	if err = fs.Remove(appDir); err != nil {
		return "", nil, err
	}
	changes := &TransformerResult{}
	for env := range configs {
		appDir := environmentApplicationDirectory(fs, env, u.Application)
		teamOwner, err := state.GetApplicationTeamOwner(u.Application)
		if err != nil {
			return "", nil, err
		}
		changes.AddAppEnv(u.Application, env, teamOwner)
		// remove environment application
		if err := fs.Remove(appDir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", nil, fmt.Errorf("UndeployApplication: unexpected error application '%v' environment '%v': '%w'", u.Application, env, err)
		}
	}
	return fmt.Sprintf("application '%v' was deleted successfully", u.Application), changes, nil
}

type DeleteEnvFromApp struct {
	Authentication
	Application string
	Environment string
}

func (u *DeleteEnvFromApp) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	err := state.checkUserPermissions(ctx, u.Environment, u.Application, auth.PermissionDeleteEnvironmentApplication, "", u.RBACConfig)
	if err != nil {
		return "", nil, err
	}
	fs := state.Filesystem
	thisSprintf := func(format string, a ...any) string {
		return fmt.Sprintf("DeleteEnvFromApp app '%s' on env '%s': %s", u.Application, u.Environment, fmt.Sprintf(format, a...))
	}

	if u.Application == "" {
		return "", nil, fmt.Errorf(thisSprintf("Need to provide the application"))
	}

	if u.Environment == "" {
		return "", nil, fmt.Errorf(thisSprintf("Need to provide the environment"))
	}

	envAppDir := environmentApplicationDirectory(fs, u.Environment, u.Application)
	entries, err := fs.ReadDir(envAppDir)
	if err != nil {
		return "", nil, wrapFileError(err, envAppDir, thisSprintf("Could not open application directory. Does the app exist?"))
	}

	if entries == nil {
		// app was never deployed on this env, so that's unusual - but for idempotency we treat it just like a success case:
		return fmt.Sprintf("Attempted to remove environment '%v' from application '%v' but it did not exist.", u.Environment, u.Application), nil, nil
	}

	err = fs.Remove(envAppDir)
	if err != nil {
		return "", nil, wrapFileError(err, envAppDir, thisSprintf("Cannot delete app.'"))
	}

	changes := &TransformerResult{
		ChangedApps: []AppEnv{
			{
				App: u.Application,
				Env: u.Environment,
			},
		},
		DeletedRootApps: []RootApp{
			{
				Env: u.Environment,
			},
		},
	}
	return fmt.Sprintf("Environment '%v' was removed from application '%v' successfully.", u.Environment, u.Application), changes, nil
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

func (c *CleanupOldApplicationVersions) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	fs := state.Filesystem
	oldVersions, err := findOldApplicationVersions(state, c.Application)
	if err != nil {
		return "", nil, fmt.Errorf("cleanup: could not get application releases for app '%s': %w", c.Application, err)
	}

	msg := ""
	for _, oldRelease := range oldVersions {
		// delete oldRelease:
		releasesDir := releasesDirectoryWithVersion(fs, c.Application, oldRelease)
		_, err := fs.Stat(releasesDir)
		if err != nil {
			return "", nil, wrapFileError(err, releasesDir, "CleanupOldApplicationVersions: could not stat")
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
						return "", nil, wrapFileError(err, releasesDir, "CleanupOldApplicationVersions: could not remove commit path")
					}
				}
			}
		}

		err = fs.Remove(releasesDir)
		if err != nil {
			return "", nil, fmt.Errorf("CleanupOldApplicationVersions: Unexpected error app %s: %w",
				c.Application, err)
		}
		msg = fmt.Sprintf("%sremoved version %d of app %v as cleanup\n", msg, oldRelease, c.Application)
	}
	changes := &TransformerResult{} // we only cleanup non-deployed versions, so there are not changes for argoCd here
	return msg, changes, nil
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

func (c *CreateEnvironmentLock) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	err := state.checkUserPermissions(ctx, c.Environment, "*", auth.PermissionCreateLock, "", c.RBACConfig)
	if err != nil {
		return "", nil, err
	}
	fs := state.Filesystem
	envDir := fs.Join("environments", c.Environment)
	if _, err := fs.Stat(envDir); err != nil {
		return "", nil, fmt.Errorf("error accessing dir %q: %w", envDir, err)
	} else {
		if chroot, err := fs.Chroot(envDir); err != nil {
			return "", nil, err
		} else {
			if err := createLock(ctx, chroot, c.LockId, c.Message); err != nil {
				return "", nil, err
			} else {
				GaugeEnvLockMetric(fs, c.Environment)
				changes := &TransformerResult{}
				return fmt.Sprintf("Created lock %q on environment %q", c.LockId, c.Environment), changes, nil
			}
		}
	}
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

func (c *DeleteEnvironmentLock) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	err := state.checkUserPermissions(ctx, c.Environment, "*", auth.PermissionDeleteLock, "", c.RBACConfig)
	if err != nil {
		return "", nil, err
	}
	fs := state.Filesystem
	s := State{
		Filesystem: fs,
	}
	lockDir := s.GetEnvLockDir(c.Environment, c.LockId)
	_, err = fs.Stat(lockDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, grpc.FailedPrecondition(ctx, fmt.Errorf("directory %s for env lock does not exist", lockDir))
		}
		return "", nil, err
	}

	if err := fs.Remove(lockDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", nil, fmt.Errorf("failed to delete directory %q: %w", lockDir, err)
	} else {
		err := s.DeleteEnvLockIfEmpty(ctx, c.Environment)
		if err != nil {
			return "", nil, err
		}

		apps, err := s.GetEnvironmentApplications(c.Environment)
		if err != nil {
			return "", nil, fmt.Errorf("environment applications for %q not found: %v", c.Environment, err.Error())
		}

		additionalMessageFromDeployment := ""
		for _, appName := range apps {
			queueMessage, err := s.ProcessQueue(ctx, fs, c.Environment, appName)
			if err != nil {
				return "", nil, err
			}
			if queueMessage != "" {
				additionalMessageFromDeployment = additionalMessageFromDeployment + "\n" + queueMessage
			}
		}
		GaugeEnvLockMetric(fs, c.Environment)
		changes := &TransformerResult{}
		return fmt.Sprintf("Deleted lock %q on environment %q%s", c.LockId, c.Environment, additionalMessageFromDeployment), changes, nil
	}
}

type CreateEnvironmentGroupLock struct {
	Authentication
	EnvironmentGroup string
	LockId           string
	Message          string
}

func (c *CreateEnvironmentGroupLock) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	err := state.checkUserPermissions(ctx, c.EnvironmentGroup, "*", auth.PermissionCreateLock, "", c.RBACConfig)
	if err != nil {
		return "", nil, err
	}
	envNamesSorted, err := state.GetEnvironmentConfigsForGroup(c.EnvironmentGroup)
	if err != nil {
		return "", nil, grpc.PublicError(ctx, err)
	}
	changes := &TransformerResult{}
	message := fmt.Sprintf("Creating locks '%s' for environment group '%s':", c.LockId, c.EnvironmentGroup)
	for index := range envNamesSorted {
		envName := envNamesSorted[index]
		x := CreateEnvironmentLock{
			Authentication: c.Authentication,
			Environment:    envName,
			LockId:         c.LockId, // the IDs should be the same for all. See `useLocksSimilarTo` in store.tsx
			Message:        c.Message,
		}
		subMessage, subChanges, err := x.Transform(ctx, state)
		if err != nil {
			return "", nil, err
		}
		changes.Combine(subChanges)
		message = message + "\n" + subMessage
	}
	return message, changes, nil
}

type DeleteEnvironmentGroupLock struct {
	Authentication
	EnvironmentGroup string
	LockId           string
}

func (c *DeleteEnvironmentGroupLock) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	err := state.checkUserPermissions(ctx, c.EnvironmentGroup, "*", auth.PermissionDeleteLock, "", c.RBACConfig)
	if err != nil {
		return "", nil, err
	}
	envNamesSorted, err := state.GetEnvironmentConfigsForGroup(c.EnvironmentGroup)
	if err != nil {
		return "", nil, grpc.PublicError(ctx, err)
	}
	changes := &TransformerResult{}
	message := fmt.Sprintf("Deleting locks '%s' for environment group '%s':", c.LockId, c.EnvironmentGroup)

	for index := range envNamesSorted {
		envName := envNamesSorted[index]
		x := DeleteEnvironmentLock{
			Authentication: c.Authentication,
			Environment:    envName,
			LockId:         c.LockId,
		}
		subMessage, subChanges, err := x.Transform(ctx, state)
		if err != nil {
			return "", nil, err
		}
		changes.Combine(subChanges)
		message = message + "\n" + subMessage
	}
	return message, changes, nil
}

type CreateEnvironmentApplicationLock struct {
	Authentication
	Environment string
	Application string
	LockId      string
	Message     string
}

func (c *CreateEnvironmentApplicationLock) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	// Note: it's possible to lock an application BEFORE it's even deployed to the environment.
	err := state.checkUserPermissions(ctx, c.Environment, c.Application, auth.PermissionCreateLock, "", c.RBACConfig)
	if err != nil {
		return "", nil, err
	}
	fs := state.Filesystem
	envDir := fs.Join("environments", c.Environment)
	if _, err := fs.Stat(envDir); err != nil {
		return "", nil, fmt.Errorf("error accessing dir %q: %w", envDir, err)
	} else {
		appDir := fs.Join(envDir, "applications", c.Application)
		if err := fs.MkdirAll(appDir, 0777); err != nil {
			return "", nil, err
		}
		if chroot, err := fs.Chroot(appDir); err != nil {
			return "", nil, err
		} else {
			if err := createLock(ctx, chroot, c.LockId, c.Message); err != nil {
				return "", nil, err
			} else {
				GaugeEnvAppLockMetric(fs, c.Environment, c.Application)
				changes := &TransformerResult{} // locks are invisible to argoCd, so no changes here
				return fmt.Sprintf("Created lock %q on environment %q for application %q", c.LockId, c.Environment, c.Application), changes, nil
			}
		}
	}
}

type DeleteEnvironmentApplicationLock struct {
	Authentication
	Environment string
	Application string
	LockId      string
}

func (c *DeleteEnvironmentApplicationLock) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	err := state.checkUserPermissions(ctx, c.Environment, c.Application, auth.PermissionDeleteLock, "", c.RBACConfig)
	if err != nil {
		return "", nil, err
	}
	fs := state.Filesystem
	lockDir := fs.Join("environments", c.Environment, "applications", c.Application, "locks", c.LockId)
	_, err = fs.Stat(lockDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, grpc.FailedPrecondition(ctx, fmt.Errorf("directory %s for app lock does not exist", lockDir))
		}
		return "", nil, err
	}
	if err := fs.Remove(lockDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", nil, fmt.Errorf("failed to delete directory %q: %w", lockDir, err)
	} else {
		s := State{
			Filesystem: fs,
		}
		err := s.DeleteAppLockIfEmpty(ctx, c.Environment, c.Application)
		if err != nil {
			return "", nil, err
		}
		queueMessage, err := s.ProcessQueue(ctx, fs, c.Environment, c.Application)
		if err != nil {
			return "", nil, err
		}
		GaugeEnvAppLockMetric(fs, c.Environment, c.Application)
		changes := &TransformerResult{}
		return fmt.Sprintf("Deleted lock %q on environment %q for application %q%s", c.LockId, c.Environment, c.Application, queueMessage), changes, nil
	}
}

type CreateEnvironment struct {
	Authentication
	Environment string
	Config      config.EnvironmentConfig
}

func (c *CreateEnvironment) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	err := state.checkUserPermissionsCreateEnvironment(ctx, c.RBACConfig, c.Config)
	if err != nil {
		return "", nil, err
	}
	fs := state.Filesystem
	envDir := fs.Join("environments", c.Environment)
	// Creation of environment is possible, but configuring it is not if running in bootstrap mode.
	// Configuration needs to be done by modifying config map in source repo
	if state.BootstrapMode && c.Config != (config.EnvironmentConfig{}) {
		return "", nil, fmt.Errorf("Cannot create or update configuration in bootstrap mode. Please update configuration in config map instead.")
	}
	if err := fs.MkdirAll(envDir, 0777); err != nil {
		return "", nil, err
	} else {
		configFile := fs.Join(envDir, "config.json")
		file, err := fs.OpenFile(configFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
		if err != nil {
			return "", nil, fmt.Errorf("error creating config: %w", err)
		}
		enc := json.NewEncoder(file)
		enc.SetIndent("", "  ")
		if err := enc.Encode(c.Config); err != nil {
			return "", nil, fmt.Errorf("error writing json: %w", err)
		}
		changes := &TransformerResult{} // we do not need to inform argoCd when creating an environment, as there are no apps yet
		return fmt.Sprintf("create environment %q", c.Environment), changes, file.Close()
	}
}

type QueueApplicationVersion struct {
	Environment string
	Application string
	Version     uint64
}

func (c *QueueApplicationVersion) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	fs := state.Filesystem
	// Create a symlink to the release
	applicationDir := fs.Join("environments", c.Environment, "applications", c.Application)
	if err := fs.MkdirAll(applicationDir, 0777); err != nil {
		return "", nil, err
	}
	queuedVersionFile := fs.Join(applicationDir, queueFileName)
	if err := fs.Remove(queuedVersionFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", nil, err
	}
	releaseDir := releasesDirectoryWithVersion(fs, c.Application, c.Version)
	if err := fs.Symlink(fs.Join("..", "..", "..", "..", releaseDir), queuedVersionFile); err != nil {
		return "", nil, err
	}

	// TODO SU: maybe check here if that version is already deployed? or somewhere else ... or not at all...
	changes := &TransformerResult{}
	return fmt.Sprintf("Queued version %d of app %q in env %q", c.Version, c.Application, c.Environment), changes, nil
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

func (c *DeployApplicationVersion) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	err := state.checkUserPermissions(ctx, c.Environment, c.Application, auth.PermissionDeployRelease, "", c.RBACConfig)
	if err != nil {
		return "", nil, err
	}
	fs := state.Filesystem
	// Check that the release exist and fetch manifest
	releaseDir := releasesDirectoryWithVersion(fs, c.Application, c.Version)
	manifest := fs.Join(releaseDir, "environments", c.Environment, "manifests.yaml")
	manifestContent := []byte{}
	if file, err := fs.Open(manifest); err != nil {
		return "", nil, wrapFileError(err, manifest, fmt.Sprintf("deployment failed: could not open manifest for app %s with release %d on env %s", c.Application, c.Version, c.Environment))
	} else {
		if content, err := io.ReadAll(file); err != nil {
			return "", nil, err
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
			return "", nil, err
		}
		appLocks, err = state.GetEnvironmentApplicationLocks(c.Environment, c.Application)
		if err != nil {
			return "", nil, err
		}
		if len(envLocks) > 0 || len(appLocks) > 0 {
			switch c.LockBehaviour {
			case api.LockBehavior_RECORD:
				q := QueueApplicationVersion{
					Environment: c.Environment,
					Application: c.Application,
					Version:     c.Version,
				}
				return q.Transform(ctx, state)
			case api.LockBehavior_FAIL:
				return "", nil, &LockedError{
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
		return "", nil, err
	}
	versionFile := fs.Join(applicationDir, "version")
	if err := fs.Remove(versionFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", nil, err
	}
	if err := fs.Symlink(fs.Join("..", "..", "..", "..", releaseDir), versionFile); err != nil {
		return "", nil, err
	}
	// Copy the manifest for argocd
	manifestsDir := fs.Join(applicationDir, "manifests")
	if err := fs.MkdirAll(manifestsDir, 0777); err != nil {
		return "", nil, err
	}
	changes := &TransformerResult{}
	manifestFilename := fs.Join(manifestsDir, "manifests.yaml")
	// note that the manifest is empty here!
	// but actually it's not quite empty!
	// The function we are using here is `util.WriteFile`. And that does not allow overwriting files with empty content.
	// We work around this unusual behavior by writing a space into the file
	if len(manifestContent) == 0 {
		manifestContent = []byte(" ")
	}
	if err := util.WriteFile(fs, manifestFilename, manifestContent, 0666); err != nil {
		return "", nil, err
	}
	teamOwner, err := state.GetApplicationTeamOwner(c.Application)
	if err != nil {
		return "", nil, err
	}
	changes.AddAppEnv(c.Application, c.Environment, teamOwner)

	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return "", nil, err
	}

	if err := util.WriteFile(fs, fs.Join(applicationDir, "deployed_by"), []byte(user.Name), 0666); err != nil {
		return "", nil, err
	}
	if err := util.WriteFile(fs, fs.Join(applicationDir, "deployed_by_email"), []byte(user.Email), 0666); err != nil {
		return "", nil, err
	}

	if err := util.WriteFile(fs, fs.Join(applicationDir, "deployed_at_utc"), []byte(getTimeNow(ctx).UTC().String()), 0666); err != nil {
		return "", nil, err
	}

	s := State{
		Filesystem: fs,
	}
	err = s.DeleteQueuedVersionIfExists(c.Environment, c.Application)
	if err != nil {
		return "", nil, err
	}
	d := &CleanupOldApplicationVersions{
		Application: c.Application,
	}
	transform, subChanges, err := d.Transform(ctx, state)
	logger.FromContext(ctx).Info(fmt.Sprintf("DeployApp: sub changes: %v+", subChanges))
	changes.Combine(subChanges)
	if err != nil {
		return "", nil, err
	}

	logger.FromContext(ctx).Info(fmt.Sprintf("DeployApp: combined changes: %v+", changes))

	if c.WriteCommitData { // write the corresponding event
		commitIdPath := fs.Join(releaseDir, "source_commit_id")

		var commitId string
		if data, err := util.ReadFile(fs, commitIdPath); err != nil {
			logger.FromContext(ctx).Sugar().Infof("Error while reading source commit ID file at %s, error %w. Deployment event not stored.", commitIdPath, err)
			goto event_not_loggable
			// return "", nil, fmt.Errorf("Error while reading source commit ID file at %s, error %w", commitIdPath, err)
		} else {
			commitId = string(data)
		}

		// if the stored source commit ID is invalid then we will not be able to store the event (simply)
		if !valid.SHA1CommitID(commitId) {
			logger.FromContext(ctx).Sugar().Infof("The source commit ID %s is not a valid/complete SHA1 hash, event cannot be stored.", commitId)
			goto event_not_loggable
			// return "", nil, fmt.Errorf("The source commit ID %s is not a valid/complete SHA1 hash, event cannot be stored", commitId)
		}

		gen, ok := getGeneratorFromContext(ctx)
		if !ok || gen == nil {
			logger.FromContext(ctx).Info("using real UUID generator.")
			gen = uuid.RealUUIDGenerator{}
		} else {
			logger.FromContext(ctx).Info("using  UUID generator from context.")
		}
		eventUuid := gen.Generate()

		if err := writeDeploymentEvent(fs, commitId, eventUuid, c.Application, c.Environment, c.SourceTrain); err != nil {
			return "", nil, fmt.Errorf("could not write the deployment event for commit %s, error: %w", commitId, err)
		}
	}
event_not_loggable:

	return fmt.Sprintf("deployed version %d of %q to %q\n%s", c.Version, c.Application, c.Environment, transform), changes, nil
}

func writeDeploymentEvent(fs billy.Filesystem, commitId, eventId, application, environment string, sourceTrain *DeployApplicationVersionSource) error {
	eventPath := commitEventDir(fs, commitId, eventId)

	if err := fs.MkdirAll(eventPath, 0777); err != nil {
		return fmt.Errorf("could not create event directory at %s, error: %w", eventPath, err)
	}

	eventTypePath := fs.Join(eventPath, "eventType")
	if err := util.WriteFile(fs, eventTypePath, []byte(event.DeploymentEventName), 0666); err != nil {
		return fmt.Errorf("could not write the event type file at %s, error: %w", eventTypePath, err)
	}

	eventApplicationPath := fs.Join(eventPath, "application")
	if err := util.WriteFile(fs, eventApplicationPath, []byte(application), 0666); err != nil {
		return fmt.Errorf("could not write the application file at %s, error: %w", eventApplicationPath, err)
	}

	eventEnvironmentPath := fs.Join(eventPath, "environment")
	if err := util.WriteFile(fs, eventEnvironmentPath, []byte(environment), 0666); err != nil {
		return fmt.Errorf("could not write the environment file at %s, error: %w", eventEnvironmentPath, err)
	}

	if sourceTrain != nil {
		eventTrainPath := fs.Join(eventPath, "source_train")

		if sourceTrain.TargetGroup != nil {
			eventTrainGroupPath := fs.Join(eventTrainPath, "source_train_group")
			if err := util.WriteFile(fs, eventTrainGroupPath, []byte(*sourceTrain.TargetGroup), 0666); err != nil {
				return fmt.Errorf("could not write source train group file at %s, error: %w", eventTrainGroupPath, err)
			}
		}
		eventTrainUpstreamPath := fs.Join(eventTrainPath, "source_train_upstream")
		if err := util.WriteFile(fs, eventTrainUpstreamPath, []byte(sourceTrain.Upstream), 0666); err != nil {
			return fmt.Errorf("could not write source train upstream file at %s, error: %w", eventTrainUpstreamPath, err)
		}
	}

	return nil
}

type ReleaseTrain struct {
	Authentication
	Target          string
	Team            string
	WriteCommitData bool
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

func generateReleaseTrainResponse(envDeployedMsg, envSkippedMsg map[string]string, targetGroupName string) string {
	resp := fmt.Sprintf("Release Train to environment/environment group '%s':\n\n", targetGroupName)

	// this to sort the env groups, to make sure that for the same input we always got the same output
	envGroups := make([]string, 0, len(envDeployedMsg))
	for env := range envDeployedMsg {
		envGroups = append(envGroups, env)
	}
	for env := range envSkippedMsg {
		if !slices.Contains(envGroups, env) {
			envGroups = append(envGroups, env)
		}
	}
	sort.Strings(envGroups)

	for _, env := range envGroups {
		msg := envDeployedMsg[env]
		resp += fmt.Sprintf("Release Train to '%s' environment:\n\n", env)
		resp += msg
		if skippedMsg, ok := envSkippedMsg[env]; ok {
			resp += "Skipped services:\n"
			resp += skippedMsg
		}
		resp += "\n\n"
	}
	return resp
}

func (c *ReleaseTrain) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	var targetGroupName = c.Target

	configs, err := state.GetEnvironmentConfigs()
	if err != nil {
		return "", nil, grpc.InternalError(ctx, err)
	}
	var envGroupConfigs, isEnvGroup = getEnvironmentGroupsEnvironmentsOrEnvironment(configs, targetGroupName)

	if len(envGroupConfigs) == 0 {
		return "", nil, grpc.PublicError(ctx, fmt.Errorf("could not find environment group or environment configs for '%v'", targetGroupName))
	}

	// this to sort the env, to make sure that for the same input we always got the same output
	envGroups := make([]string, 0, len(envGroupConfigs))
	for env := range envGroupConfigs {
		envGroups = append(envGroups, env)
	}
	sort.Strings(envGroups)

	envDeployedMsg := make(map[string]string)
	envSkippedMsg := make(map[string]string)
	changes := &TransformerResult{}
	for _, envName := range envGroups {
		envConfig := envGroupConfigs[envName]
		if envConfig.Upstream == nil {
			envSkippedMsg[envName] = fmt.Sprintf("Environment '%q' does not have upstream configured - skipping.", envName)
			continue
		}
		err := state.checkUserPermissions(ctx, envName, "*", auth.PermissionDeployReleaseTrain, c.Team, c.RBACConfig)
		if err != nil {
			return "", nil, err
		}

		var upstreamLatest = envConfig.Upstream.Latest
		var upstreamEnvName = envConfig.Upstream.Environment

		if !upstreamLatest && upstreamEnvName == "" {
			envSkippedMsg[envName] = fmt.Sprintf("Environment %q does not have upstream.latest or upstream.environment configured - skipping.", envName)
			continue
		}
		if upstreamLatest && upstreamEnvName != "" {
			envSkippedMsg[envName] = fmt.Sprintf("Environment %q has both upstream.latest and upstream.environment configured - skipping.", envName)
			continue
		}
		source := upstreamEnvName
		if upstreamLatest {
			source = "latest"
		}

		if !upstreamLatest {
			_, ok := configs[upstreamEnvName]
			if !ok {
				return fmt.Sprintf("Could not find environment config for upstream env %q. Target env was %q", upstreamEnvName, envName), nil, err
			}
		}

		envLocks, err := state.GetEnvironmentLocks(envName)
		if err != nil {
			return "", nil, grpc.InternalError(ctx, fmt.Errorf("could not get lock for environment %q: %w", envName, err))
		}
		if len(envLocks) > 0 {
			envSkippedMsg[envName] = fmt.Sprintf("Target Environment '%s' is locked - skipping.\n", envName)
			continue
		}

		var apps []string
		if upstreamLatest {
			apps, err = state.GetApplications()
			if err != nil {
				return "", nil, grpc.InternalError(ctx, fmt.Errorf("could not get all applications for %q: %w", source, err))
			}
		} else {
			apps, err = state.GetEnvironmentApplications(upstreamEnvName)
			if err != nil {
				return "", nil, grpc.PublicError(ctx, fmt.Errorf("upstream environment (%q) does not have applications: %w", upstreamEnvName, err))
			}
		}
		sort.Strings(apps)

		// now iterate over all apps, deploying all that are not locked
		numServices := 0
		completeMessage := ""
		for _, appName := range apps {
			if c.Team != "" {
				if team, err := state.GetApplicationTeamOwner(appName); err != nil {
					return "", nil, nil
				} else if c.Team != team {
					continue
				}
			}
			currentlyDeployedVersion, err := state.GetEnvironmentApplicationVersion(envName, appName)
			if err != nil {
				return "", nil, grpc.PublicError(ctx, fmt.Errorf("application %q in env %q does not have a version deployed: %w", appName, envName, err))
			}
			var versionToDeploy uint64
			if upstreamLatest {
				versionToDeploy, err = GetLastRelease(state.Filesystem, appName)
				if err != nil {
					return "", nil, grpc.PublicError(ctx, fmt.Errorf("application %q does not have a latest deployed: %w", appName, err))
				}
			} else {
				upstreamVersion, err := state.GetEnvironmentApplicationVersion(upstreamEnvName, appName)
				if err != nil {
					return "", nil, grpc.PublicError(ctx, fmt.Errorf("application %q does not have a version deployed in env %q: %w", appName, upstreamEnvName, err))
				}
				if upstreamVersion == nil {
					envSkippedMsg[envName] += fmt.Sprintf("skipping because there is no version for application %q in env %q \n", appName, upstreamEnvName)
					continue
				}
				versionToDeploy = *upstreamVersion
			}
			if currentlyDeployedVersion != nil && *currentlyDeployedVersion == versionToDeploy {
				envSkippedMsg[envName] += fmt.Sprintf("%sskipping %q because it is already in the version %d\n", completeMessage, appName, *currentlyDeployedVersion)
				continue
			}

			sourceTrain := &DeployApplicationVersionSource{}

			if isEnvGroup {
				sourceTrain.TargetGroup = &c.Target
			}
			sourceTrain.Upstream = source

			d := &DeployApplicationVersion{
				Environment:     envName, // here we deploy to the next env
				Application:     appName,
				Version:         versionToDeploy,
				LockBehaviour:   api.LockBehavior_RECORD,
				Authentication:  c.Authentication,
				WriteCommitData: c.WriteCommitData,
				SourceTrain:     sourceTrain,
			}
			transform, subChanges, err := d.Transform(ctx, state)
			if err != nil {
				_, ok := err.(*LockedError)
				if ok {
					continue // locked errors are to be expected
				}
				if errors.Is(err, os.ErrNotExist) {
					continue // some apps do not exist on all envs, we ignore those
				}
				return "", nil, grpc.InternalError(ctx, fmt.Errorf("unexpected error while deploying app %q to env %q: %w", appName, envName, err))
			}
			changes.Combine(subChanges)
			numServices += 1
			completeMessage = completeMessage + transform + "\n"
		}
		teamInfo := ""
		if c.Team != "" {
			teamInfo = " for team '" + c.Team + "'"
		}
		envDeployedMsg[envName] = fmt.Sprintf("The release train deployed %d services from '%s' to '%s'%s\n%s\n", numServices, source, envName, teamInfo, completeMessage)
	}

	return generateReleaseTrainResponse(envDeployedMsg, envSkippedMsg, targetGroupName), changes, nil
}
