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
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"go.uber.org/zap"
	"math"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"

	config "github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/mapper"
	"github.com/freiheit-com/kuberpult/pkg/sorting"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/freiheit-com/kuberpult/pkg/conversion"
	"github.com/freiheit-com/kuberpult/pkg/metrics"

	"github.com/freiheit-com/kuberpult/pkg/uuid"

	"github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/valid"

	"github.com/freiheit-com/kuberpult/pkg/logger"

	yaml3 "gopkg.in/yaml.v3"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	diffspan "github.com/hexops/gotextdiff/span"
	"golang.org/x/sync/errgroup"
)

const (
	queueFileName         = "queued_version"
	yamlParsingError      = "# yaml parsing error"
	fieldSourceAuthor     = "source_author"
	fieldSourceMessage    = "source_message"
	fieldSourceCommitId   = "source_commit_id"
	fieldDisplayVersion   = "display_version"
	fieldCreatedAt        = "created_at"
	fieldTeam             = "team"
	fieldNextCommidId     = "nextCommit"
	fieldPreviousCommitId = "previousCommit"
	// number of old releases that will ALWAYS be kept in addition to the ones that are deployed:
	keptVersionsOnCleanup = 20
)

func (s *State) GetEnvironmentLocksCount(ctx context.Context, transaction *sql.Tx, env string) (float64, error) {
	locks, err := s.GetEnvironmentLocks(ctx, transaction, env)
	if err != nil {
		return -1, err
	}
	return float64(len(locks)), nil
}

func (s *State) GetEnvironmentApplicationLocksCount(ctx context.Context, transaction *sql.Tx, environment, application string) (float64, error) {
	locks, err := s.GetEnvironmentApplicationLocks(ctx, transaction, environment, application)
	if err != nil {
		return -1, err
	}
	return float64(len(locks)), nil
}

func GaugeGitSyncStatus(ctx context.Context, s *State, transaction *sql.Tx) error {
	if ddMetrics != nil {
		numberUnsyncedApps, err := s.DBHandler.DBCountAppsWithStatus(ctx, transaction, db.UNSYNCED)
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("error getting number of unsynced apps: %v\n", err)
			return err
		}
		numberSyncFailedApps, err := s.DBHandler.DBCountAppsWithStatus(ctx, transaction, db.SYNC_FAILED)
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("error getting number of sync failed apps: %v\n", err)
			return err
		}
		err = MeasureGitSyncStatus(numberUnsyncedApps, numberSyncFailedApps)
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("could not send git sync status metrics to datadog: %v\n", err)
			return err
		}

	}
	return nil
}

func GaugeEnvLockMetric(ctx context.Context, s *State, transaction *sql.Tx, env string) {
	if ddMetrics != nil {
		count, err := s.GetEnvironmentLocksCount(ctx, transaction, env)
		if err != nil {
			logger.FromContext(ctx).
				Sugar().
				Warnf("Error when trying to get the number of environment locks: %w\n", err)
			return
		}
		err = ddMetrics.Gauge("environment_lock_count", count, []string{"kuberpult_environment:" + env}, 1)
		if err != nil {
			logger.FromContext(ctx).
				Sugar().
				Warnf("Error when trying to send `environment_lock_count` metric to datadog: %w\n", err)
		}
	}
}
func GaugeEnvAppLockMetric(ctx context.Context, appEnvLocksCount int, env, app string) {
	if ddMetrics != nil {
		err := ddMetrics.Gauge("application_lock_count", float64(appEnvLocksCount), []string{"kuberpult_environment:" + env, "kuberpult_application:" + app}, 1)
		if err != nil {
			logger.FromContext(ctx).
				Sugar().
				Warnf("Error when trying to send `application_lock_count` metric to datadog: %w\n", err)
		}
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

func UpdateDatadogMetrics(ctx context.Context, transaction *sql.Tx, state *State, repo Repository, changes *TransformerResult, now time.Time, even bool) error {
	if ddMetrics == nil {
		return nil
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "UpdateDatadogMetrics")
	defer span.Finish()
	span.SetTag("even", even)

	if state.DBHandler == nil {
		logger.FromContext(ctx).Sugar().Warn("Tried to update datadog metrics without database")
		return nil
	}

	if even {
		repo.(*repository).GaugeQueueSize(ctx)
	}
	err2 := UpdateLockMetrics(ctx, transaction, state, now, even)
	if err2 != nil {
		span.Finish(tracer.WithError(err2))
		return err2
	}

	if even {
		err := UpdateChangedAppMetrics(ctx, changes, now)
		if err != nil {
			span.Finish(tracer.WithError(err))
			return err
		}
		err = GaugeGitSyncStatus(ctx, state, transaction)
		if err != nil {
			span.Finish(tracer.WithError(err))
			return err
		}
	}

	return nil
}

func UpdateChangedAppMetrics(ctx context.Context, changes *TransformerResult, now time.Time) error {
	span, _ := tracer.StartSpanFromContext(ctx, "UpdateChangedAppMetrics")
	defer span.Finish()
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

func UpdateLockMetrics(ctx context.Context, transaction *sql.Tx, state *State, now time.Time, even bool) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "UpdateLockMetrics")
	defer span.Finish()

	envConfigs, _, err := state.GetEnvironmentConfigsSorted(ctx, transaction)
	if err != nil {
		return err
	}

	for envName := range envConfigs {
		if even {
			GaugeEnvLockMetric(ctx, state, transaction, envName)
		}

		envApps, err := state.GetEnvironmentApplicationsFromDB(ctx, transaction, envName)
		if err != nil {
			return fmt.Errorf("failed to read environment from the db: %v", err)
		}

		// in the future apps might be sorted, but currently they aren't
		slices.Sort(envApps)
		var appCounter = 0

		//Get all locks at once, avoid round trips to DB
		allAppLocks, err := state.DBHandler.DBSelectAllActiveAppLocksForSliceApps(ctx, transaction, envApps)
		if err != nil {
			return err
		}

		appLockMap := map[string]int{} //appName -> num of app locks
		//Collect number of appLocks
		for _, currentLock := range allAppLocks {
			appCounter = appCounter + 1
			actualEven := appCounter%2 == 0
			if actualEven == even {
				// iterating over all apps can take a while, so we only do half the apps each run
				continue
			}
			if value, ok := appLockMap[currentLock.App]; !ok {
				appLockMap[currentLock.App] = 1
			} else {
				appLockMap[currentLock.App] = value + 1
			}
		}

		appCounter = 0
		for _, appName := range envApps {
			appCounter = appCounter + 1
			actualEven := appCounter%2 == 0
			if actualEven == even {
				// iterating over all apps can take a while, so we only do half the apps each run
				continue
			}
			GaugeEnvAppLockMetric(ctx, appLockMap[appName], envName, appName)
			_, deployedAtTimeUtc, err := state.GetDeploymentMetaData(ctx, transaction, envName, appName)
			if err != nil {
				return err
			}
			timeDiff := now.Sub(deployedAtTimeUtc)
			err = GaugeDeploymentMetric(ctx, envName, appName, timeDiff.Minutes())
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func RegularlySendDatadogMetrics(repo Repository, interval time.Duration, callBack func(repository Repository, even bool)) {
	metricEventTimer := time.NewTicker(interval * time.Second)
	var even = true
	for range metricEventTimer.C {
		callBack(repo, even)
		even = !even
	}
}

func GetRepositoryStateAndUpdateMetrics(ctx context.Context, repo Repository, even bool) {
	s := repo.State()
	err := s.DBHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
		if err := UpdateDatadogMetrics(ctx, transaction, s, repo, nil, time.Now(), even); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		panic(err.Error())
	}
}

// A Transformer updates the files in the worktree
type Transformer interface {
	Transform(ctx context.Context, state *State, t TransformerContext, transaction *sql.Tx) (commitMsg string, e error)
	GetDBEventType() db.EventType
	SetEslVersion(eslVersion db.TransformerID)
	GetEslVersion() db.TransformerID
}

type TransformerContext interface {
	Execute(ctx context.Context, t Transformer, transaction *sql.Tx) error
	AddAppEnv(app string, env string, team string)
	DeleteEnvFromApp(app string, env string)
}

func RunTransformer(ctx context.Context, t Transformer, s *State, transaction *sql.Tx) (string, *TransformerResult, error) {
	runner := transformerRunner{
		ChangedApps:     nil,
		DeletedRootApps: nil,
		State:           s,
	}
	if err := runner.Execute(ctx, t, transaction); err != nil {
		return "", nil, err
	}
	commitMsg := ""
	return commitMsg, &TransformerResult{
		ChangedApps:     runner.ChangedApps,
		DeletedRootApps: runner.DeletedRootApps,
	}, nil
}

type transformerRunner struct {
	//Context context.Context
	State *State
	// Stores the current stack of commit messages. Each entry of
	// the outer slice corresponds to a step being executed. Each
	// entry of the inner slices correspond to a message generated
	// by that step.
	ChangedApps     []AppEnv
	DeletedRootApps []RootApp
}

func (r *transformerRunner) Execute(ctx context.Context, t Transformer, transaction *sql.Tx) error {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, fmt.Sprintf("Transformer_%s", t.GetDBEventType()))
	defer span.Finish()
	_, err := t.Transform(ctx, r.State, r, transaction)
	return onErr(err)
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
	Authentication        `json:"-"`
	Version               uint64            `json:"version"`
	Application           string            `json:"app"`
	Manifests             map[string]string `json:"manifests"`
	SourceCommitId        string            `json:"sourceCommitId"`
	SourceAuthor          string            `json:"sourceCommitAuthor"`
	SourceMessage         string            `json:"sourceCommitMessage"`
	Team                  string            `json:"team"`
	DisplayVersion        string            `json:"displayVersion"`
	WriteCommitData       bool              `json:"writeCommitData"`
	PreviousCommit        string            `json:"previousCommit"`
	CiLink                string            `json:"ciLink"`
	AllowedDomains        []string          `json:"-"`
	TransformerEslVersion db.TransformerID  `json:"-"`
	IsPrepublish          bool              `json:"isPrepublish"`
}

func (c *CreateApplicationVersion) GetDBEventType() db.EventType {
	return db.EvtCreateApplicationVersion
}

func (c *CreateApplicationVersion) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *CreateApplicationVersion) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

type ctxMarkerGenerateUuid struct{}

var (
	ctxMarkerGenerateUuidKey = &ctxMarkerGenerateUuid{}
)

func (s *State) GetLastRelease(ctx context.Context, transaction *sql.Tx, application string) (uint64, error) {
	releases, err := s.DBHandler.DBSelectAllReleasesOfApp(ctx, transaction, application)
	if err != nil {
		return 0, fmt.Errorf("could not get releases of app %s: %v", application, err)
	}
	if len(releases) == 0 {
		return 0, nil
	}
	l := len(releases)
	return uint64(releases[l-1]), nil
}

func isValidLink(urlToCheck string, allowedDomains []string) bool {
	u, err := url.ParseRequestURI(urlToCheck) //Check if is a valid URL
	if err != nil {
		return false
	}
	return slices.Contains(allowedDomains, u.Hostname())
}

func isValidLifeTime(lifeTime string) bool {
	pattern := `^[1-9][0-9]*(h|d|w)$`
	matched, err := regexp.MatchString(pattern, lifeTime)
	if err != nil {
		return false
	}
	return matched
}

func (c *CreateApplicationVersion) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	version, err := c.calculateVersion(ctx, transaction, state)
	if err != nil {
		return "", err
	}
	if !valid.ApplicationName(c.Application) {
		return "", GetCreateReleaseAppNameTooLong(c.Application, valid.AppNameRegExp, uint32(valid.MaxAppNameLen))
	}
	allApps, err := state.DBHandler.DBSelectAllApplications(ctx, transaction)
	if err != nil {
		return "", GetCreateReleaseGeneralFailure(err)
	}
	if allApps == nil {
		allApps = []string{}
	}

	if !slices.Contains(allApps, c.Application) {
		// this app is new
		//We need to check that this is not an app that has been previously deleted
		app, err := state.DBHandler.DBSelectApp(ctx, transaction, c.Application)
		if err != nil {
			return "", GetCreateReleaseGeneralFailure(fmt.Errorf("could not read apps: %v", err))
		}
		if app != nil && app.StateChange != db.AppStateChangeDelete {
			return "", GetCreateReleaseGeneralFailure(fmt.Errorf("could not write new app, app already exists: %v", err)) //Should never happen
		}

		err = state.DBHandler.DBInsertOrUpdateApplication(
			ctx,
			transaction,
			c.Application,
			db.AppStateChangeCreate,
			db.DBAppMetaData{Team: c.Team},
		)
		if err != nil {
			return "", GetCreateReleaseGeneralFailure(fmt.Errorf("could not write new app: %v", err))
		}
	} else {
		// app is not new, but metadata may have changed
		existingApp, err := state.DBHandler.DBSelectApp(ctx, transaction, c.Application)
		if err != nil {
			return "", err
		}
		if existingApp == nil {
			return "", fmt.Errorf("could not find app '%s'", c.Application)
		}
		newMeta := db.DBAppMetaData{Team: c.Team}
		// only update the app, if something really changed:
		if !cmp.Equal(newMeta, existingApp.Metadata) {
			err = state.DBHandler.DBInsertOrUpdateApplication(
				ctx,
				transaction,
				c.Application,
				db.AppStateChangeUpdate,
				newMeta,
			)
			if err != nil {
				return "", GetCreateReleaseGeneralFailure(fmt.Errorf("could not update app: %v", err))
			}
		}
	}

	if c.SourceCommitId != "" && !valid.SHA1CommitID(c.SourceCommitId) {
		return "", GetCreateReleaseGeneralFailure(fmt.Errorf("Source commit ID is not a valid SHA1 hash, should be exactly 40 characters [0-9a-fA-F] %s\n", c.SourceCommitId))
	}
	if c.PreviousCommit != "" && !valid.SHA1CommitID(c.PreviousCommit) {
		logger.FromContext(ctx).Sugar().Warnf("Previous commit ID %s is invalid", c.PreviousCommit)
	}

	configs, err := state.GetAllEnvironmentConfigs(ctx, transaction)
	if err != nil {
		if errors.Is(err, InvalidJson) {
			return "", err
		}
		return "", GetCreateReleaseGeneralFailure(err)
	}

	if c.CiLink != "" && !isValidLink(c.CiLink, c.AllowedDomains) {
		return "", GetCreateReleaseGeneralFailure(fmt.Errorf("Provided CI Link: %s is not valid or does not match any of the allowed domain", c.CiLink))
	}

	isLatest, err := isLatestVersion(ctx, transaction, state, c.Application, version)
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

	var allEnvsOfThisApp []string = nil

	for env := range c.Manifests {
		allEnvsOfThisApp = append(allEnvsOfThisApp, env)
	}
	slices.Sort(allEnvsOfThisApp)

	gen := getGenerator(ctx)
	eventUuid := gen.Generate()
	if c.WriteCommitData {
		err = writeCommitData(ctx, state.DBHandler, transaction, version, c.TransformerEslVersion, c.SourceCommitId, c.SourceMessage, c.Application, eventUuid, allEnvsOfThisApp, c.PreviousCommit, state)
		if err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
	}
	sortedKeys := sorting.SortKeys(c.Manifests)

	isMinor, err := c.checkMinorFlags(ctx, transaction, state.DBHandler, version, state.MinorRegexes)
	if err != nil {
		return "", err
	}
	manifestsToKeep := c.Manifests
	if c.IsPrepublish {
		manifestsToKeep = make(map[string]string)
	}
	now, err := state.DBHandler.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return "", GetCreateReleaseGeneralFailure(fmt.Errorf("could not get transaction timestamp"))
	}
	release := db.DBReleaseWithMetaData{
		ReleaseNumber: version,
		App:           c.Application,
		Manifests: db.DBReleaseManifests{
			Manifests: manifestsToKeep,
		},
		Metadata: db.DBReleaseMetaData{
			SourceAuthor:    c.SourceAuthor,
			SourceCommitId:  c.SourceCommitId,
			SourceMessage:   c.SourceMessage,
			DisplayVersion:  c.DisplayVersion,
			UndeployVersion: false,
			IsMinor:         isMinor,
			CiLink:          c.CiLink,
			IsPrepublish:    c.IsPrepublish,
		},
		Environments: []string{},
		Created:      *now,
	}
	err = state.DBHandler.DBUpdateOrCreateRelease(ctx, transaction, release)
	if err != nil {
		return "", GetCreateReleaseGeneralFailure(err)
	}

	for i := range sortedKeys {
		env := sortedKeys[i]
		err = state.DBHandler.DBAppendAppToEnvironment(ctx, transaction, env, c.Application)
		if err != nil {
			return "", grpc.PublicError(ctx, err)
		}

		err = state.checkUserPermissions(ctx, transaction, env, c.Application, auth.PermissionCreateRelease, c.Team, c.RBACConfig, true)
		if err != nil {
			return "", err
		}

		config, found := configs[env]
		hasUpstream := false
		if found {
			hasUpstream = config.Upstream != nil
		}

		teamOwner, err := state.GetApplicationTeamOwner(ctx, transaction, c.Application)
		if err != nil {
			return "", err
		}
		t.AddAppEnv(c.Application, env, teamOwner)
		if hasUpstream && config.Upstream.Latest && isLatest && !c.IsPrepublish {
			d := &DeployApplicationVersion{
				SourceTrain:           nil,
				Environment:           env,
				Application:           c.Application,
				Version:               version, // the train should queue deployments, instead of giving up:
				LockBehaviour:         api.LockBehavior_RECORD,
				Authentication:        c.Authentication,
				WriteCommitData:       c.WriteCommitData,
				Author:                c.SourceAuthor,
				CiLink:                c.CiLink,
				TransformerEslVersion: c.TransformerEslVersion,
				SkipCleanup:           false,
			}
			err := t.Execute(ctx, d, transaction)
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

func (c *CreateApplicationVersion) checkMinorFlags(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler, version uint64, minorRegexes []*regexp.Regexp) (bool, error) {
	releaseVersions, err := dbHandler.DBSelectAllReleasesOfApp(ctx, transaction, c.Application)
	if err != nil {
		return false, err
	}
	if releaseVersions == nil {
		return false, nil
	}
	sort.Slice(releaseVersions, func(i, j int) bool { return releaseVersions[i] > releaseVersions[j] })
	nextVersion := int64(-1)
	previousVersion := int64(-1)
	for i := len(releaseVersions) - 1; i >= 0; i-- {
		if releaseVersions[i] > int64(version) {
			nextVersion = releaseVersions[i]
			break
		}
	}
	for i := 0; i < len(releaseVersions); i++ {
		if releaseVersions[i] < int64(version) {
			previousVersion = releaseVersions[i]
			break
		}
	}
	if nextVersion != -1 {
		nextRelease, err := dbHandler.DBSelectReleaseByVersion(ctx, transaction, c.Application, uint64(nextVersion), true)
		if err != nil {
			return false, err
		}
		if nextRelease == nil {
			return false, fmt.Errorf("next release exists in the all releases but not in the release table!")
		}
		nextRelease.Metadata.IsMinor = compareManifests(ctx, c.Manifests, nextRelease.Manifests.Manifests, minorRegexes)
		err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, *nextRelease)
		if err != nil {
			return false, err
		}
	}
	if previousVersion == -1 {
		return false, nil
	}
	previousRelease, err := dbHandler.DBSelectReleaseByVersion(ctx, transaction, c.Application, uint64(previousVersion), true)
	if err != nil {
		return false, err
	}
	return compareManifests(ctx, c.Manifests, previousRelease.Manifests.Manifests, minorRegexes), nil
}

func compareManifests(ctx context.Context, firstManifests map[string]string, secondManifests map[string]string, comparisonRegexes []*regexp.Regexp) bool {
	if len(firstManifests) != len(secondManifests) {
		return false
	}
	for key, manifest1 := range firstManifests {
		manifest2, exists := secondManifests[key]
		if !exists {
			return false
		}
		filteredLines1 := filterManifestLines(ctx, manifest1, comparisonRegexes)
		filteredLines2 := filterManifestLines(ctx, manifest2, comparisonRegexes)
		if !reflect.DeepEqual(filteredLines1, filteredLines2) {
			return false
		}
	}
	return true
}

func filterManifestLines(ctx context.Context, str string, regexes []*regexp.Regexp) []string {
	lines := strings.Split(str, "\n")
	filteredLines := make([]string, 0)
	for _, line := range lines {
		match := false
		for _, regex := range regexes {
			if regex.MatchString(line) {
				match = true
				break
			}
		}
		if !match {
			filteredLines = append(filteredLines, line)
		}
	}
	return filteredLines
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

func writeCommitData(ctx context.Context, h *db.DBHandler, transaction *sql.Tx, releaseVersion uint64, transformerEslVersion db.TransformerID, sourceCommitId string, sourceMessage string, app string, eventId string, environments []string, previousCommitId string, state *State) error {
	if !valid.SHA1CommitID(sourceCommitId) {
		return nil
	}

	envMap := make(map[string]struct{}, len(environments))
	for _, env := range environments {
		envMap[env] = struct{}{}
	}

	ev := &event.NewRelease{
		Environments: envMap,
	}
	var writeError error
	gen := getGenerator(ctx)
	eventUuid := gen.Generate()
	writeError = state.DBHandler.DBWriteNewReleaseEvent(ctx, transaction, transformerEslVersion, releaseVersion, eventUuid, sourceCommitId, ev)

	if writeError != nil {
		return fmt.Errorf("error while writing event: %v", writeError)
	}
	return nil
}

func (c *CreateApplicationVersion) calculateVersion(ctx context.Context, transaction *sql.Tx, state *State) (uint64, error) {
	if c.Version == 0 {
		return 0, fmt.Errorf("version is required when using the database")
	} else {
		metaData, err := state.DBHandler.DBSelectReleaseByVersion(ctx, transaction, c.Application, c.Version, true)
		if err != nil {
			return 0, fmt.Errorf("could not calculate version, error: %v", err)
		}
		if metaData == nil {
			logger.FromContext(ctx).Sugar().Infof("could not calculate version, no metadata on app %s with version %v", c.Application, c.Version)
			return c.Version, nil
		}
		logger.FromContext(ctx).Sugar().Warnf("release exists already %v: %v", metaData.ReleaseNumber, metaData)

		existingRelease := metaData.ReleaseNumber
		logger.FromContext(ctx).Sugar().Warnf("comparing release %v: %v", c.Version, existingRelease)
		// check if version differs, if it's the same, that's ok
		return 0, c.sameAsExistingDB(ctx, transaction, state, metaData)
	}
}

func (c *CreateApplicationVersion) sameAsExistingDB(ctx context.Context, transaction *sql.Tx, state *State, metadata *db.DBReleaseWithMetaData) error {
	if c.SourceCommitId != "" {
		existingSourceCommitIdStr := metadata.Metadata.SourceCommitId
		if existingSourceCommitIdStr != c.SourceCommitId {
			logger.FromContext(ctx).Sugar().Warnf("SourceCommitId is different1 '%s'!='%s'", c.SourceCommitId, existingSourceCommitIdStr)
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_SOURCE_COMMIT_ID, createUnifiedDiff(existingSourceCommitIdStr, c.SourceCommitId, ""))
		}
	}
	if c.SourceAuthor != "" {
		existingSourceAuthorStr := metadata.Metadata.SourceAuthor
		if existingSourceAuthorStr != c.SourceAuthor {
			logger.FromContext(ctx).Sugar().Warnf("SourceAuthor is different1 '%s'!='%s'", c.SourceAuthor, existingSourceAuthorStr)
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_SOURCE_AUTHOR, createUnifiedDiff(existingSourceAuthorStr, c.SourceAuthor, ""))
		}
	}
	if c.SourceMessage != "" {
		existingSourceMessageStr := metadata.Metadata.SourceMessage
		if existingSourceMessageStr != c.SourceMessage {
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_SOURCE_MESSAGE, createUnifiedDiff(existingSourceMessageStr, c.SourceMessage, ""))
		}
	}
	if c.DisplayVersion != "" {
		existingDisplayVersionStr := metadata.Metadata.DisplayVersion
		if existingDisplayVersionStr != c.DisplayVersion {
			logger.FromContext(ctx).Sugar().Warnf("displayVersion is different1 '%s'!='%s'", c.DisplayVersion, metadata.Metadata.DisplayVersion)
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_DISPLAY_VERSION, createUnifiedDiff(existingDisplayVersionStr, c.DisplayVersion, ""))
		}
	}
	if c.Team != "" {
		appData, err := state.DBHandler.DBSelectApp(ctx, transaction, c.Application)
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("team is different1 '%s'!='%s'", c.Team, appData.Metadata.Team)
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_TEAM, "")
		}
		existingTeamStr := appData.Metadata.Team
		if existingTeamStr != c.Team {
			logger.FromContext(ctx).Sugar().Warnf("team is different2 '%s'!='%s'", c.Team, appData.Metadata.Team)
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_TEAM, createUnifiedDiff(existingTeamStr, c.Team, ""))
		}
	}
	metaData, err := state.DBHandler.DBSelectReleaseByVersion(ctx, transaction, c.Application, c.Version, true)
	if err != nil {
		return fmt.Errorf("could not calculate version, error: %v", err)
	}
	if metaData == nil {
		return fmt.Errorf("could not calculate version, no metadata on app %s", c.Application)
	}
	if err != nil {
		return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_MANIFESTS, fmt.Sprintf("manifest missing for app %s", c.Application))
	}
	for env, man := range c.Manifests {
		existingManStr := metaData.Manifests.Manifests[env]
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
	edits := myers.ComputeEdits(diffspan.URIFromPath(existingFilename), existingValueStr, string(requestValue))
	return fmt.Sprint(gotextdiff.ToUnified(existingFilename, requestFilename, existingValueStr, edits))
}

func isLatestVersion(ctx context.Context, transaction *sql.Tx, state *State, application string, version uint64) (bool, error) {
	var rels []uint64
	var err error
	all, err := state.DBHandler.DBSelectAllReleasesOfApp(ctx, transaction, application)
	if err != nil {
		return false, err
	}
	//Convert
	if all == nil {
		rels = make([]uint64, 0)
	} else {
		rels = make([]uint64, len(all))
		for idx, rel := range all {
			rels[idx] = uint64(rel)
		}
	}

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
	Authentication        `json:"-"`
	Application           string           `json:"app"`
	WriteCommitData       bool             `json:"writeCommitData"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (c *CreateUndeployApplicationVersion) GetDBEventType() db.EventType {
	return db.EvtCreateUndeployApplicationVersion
}

func (c *CreateUndeployApplicationVersion) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *CreateUndeployApplicationVersion) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *CreateUndeployApplicationVersion) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	lastRelease, err := state.GetLastRelease(ctx, transaction, c.Application)
	if err != nil {
		return "", err
	}
	if lastRelease == 0 {
		return "", fmt.Errorf("cannot undeploy non-existing application '%v'", c.Application)
	}
	now, err := state.DBHandler.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return "", fmt.Errorf("could not get transaction timestamp")
	}
	envManifests := make(map[string]string)
	envs := []string{}
	allEnvsApps, err := state.DBHandler.FindEnvsAppsFromReleases(ctx, transaction)
	if err != nil {
		return "", fmt.Errorf("error while getting envs for this apps: %v", err)
	}
	for env, apps := range allEnvsApps {
		if slices.Contains(apps, c.Application) {
			envManifests[env] = "" //empty manifest
			envs = append(envs, env)
		}
	}
	release := db.DBReleaseWithMetaData{
		ReleaseNumber: lastRelease + 1,
		App:           c.Application,
		Manifests: db.DBReleaseManifests{
			Manifests: envManifests,
		},
		Metadata: db.DBReleaseMetaData{
			SourceAuthor:    "",
			SourceCommitId:  "",
			SourceMessage:   "",
			DisplayVersion:  "",
			UndeployVersion: true,
			IsMinor:         false,
			IsPrepublish:    false,
			CiLink:          "",
		},
		Environments: envs,
		Created:      *now,
	}
	err = state.DBHandler.DBUpdateOrCreateRelease(ctx, transaction, release)
	if err != nil {
		return "", GetCreateReleaseGeneralFailure(err)
	}

	configs, err := state.GetAllEnvironmentConfigs(ctx, transaction)
	if err != nil {
		return "", fmt.Errorf("error while getting environment configs, error: %w", err)
	}
	for env := range configs {
		err := state.checkUserPermissions(ctx, transaction, env, c.Application, auth.PermissionCreateUndeploy, "", c.RBACConfig, true)
		if err != nil {
			return "", err
		}
		config, found := configs[env]
		hasUpstream := false
		if found {
			hasUpstream = config.Upstream != nil
		}
		teamOwner, err := state.GetApplicationTeamOwner(ctx, transaction, c.Application)
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
				LockBehaviour:         api.LockBehavior_RECORD,
				Authentication:        c.Authentication,
				WriteCommitData:       c.WriteCommitData,
				Author:                "",
				TransformerEslVersion: c.TransformerEslVersion,
				CiLink:                "",
				SkipCleanup:           false,
			}
			err := t.Execute(ctx, d, transaction)
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

type UndeployApplication struct {
	Authentication        `json:"-"`
	Application           string           `json:"app"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (u *UndeployApplication) GetDBEventType() db.EventType {
	return db.EvtUndeployApplication
}

func (u *UndeployApplication) SetEslVersion(id db.TransformerID) {
	u.TransformerEslVersion = id
}

func (c *UndeployApplication) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (u *UndeployApplication) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	lastRelease, err := state.GetLastRelease(ctx, transaction, u.Application)
	if err != nil {
		return "", err
	}
	if lastRelease == 0 {
		return "", fmt.Errorf("UndeployApplication: error cannot undeploy non-existing application '%v'", u.Application)
	}

	isUndeploy, err := state.IsUndeployVersion(ctx, transaction, u.Application, lastRelease)
	if err != nil {
		return "", err
	}
	if !isUndeploy {
		return "", fmt.Errorf("UndeployApplication: error last release is not un-deployed application version of '%v'", u.Application)
	}
	configs, err := state.GetAllEnvironmentConfigs(ctx, transaction)
	if err != nil {
		return "", err
	}
	for env := range configs {
		err := state.checkUserPermissions(ctx, transaction, env, u.Application, auth.PermissionDeployUndeploy, "", u.RBACConfig, true)
		if err != nil {
			return "", err
		}
		deployment, err := state.DBHandler.DBSelectLatestDeployment(ctx, transaction, u.Application, env)
		if err != nil {
			return "", fmt.Errorf("UndeployApplication(db): error cannot un-deploy application '%v' the release '%v' cannot be found", u.Application, env)
		}

		var isUndeploy bool
		if deployment != nil && deployment.Version != nil {
			release, err := state.DBHandler.DBSelectReleaseByVersion(ctx, transaction, u.Application, uint64(*deployment.Version), true)
			if err != nil {
				return "", err
			}
			isUndeploy = release.Metadata.UndeployVersion
			if !isUndeploy {
				return "", fmt.Errorf("UndeployApplication(db): error cannot un-deploy application '%v' the current release '%v' is not un-deployed", u.Application, env)
			}
			//Delete deployment (register a new deployment by deleting version)
			user, err := auth.ReadUserFromContext(ctx)
			if err != nil {
				return "", err
			}
			deployment.Version = nil
			deployment.Metadata.DeployedByName = user.Name
			deployment.Metadata.DeployedByEmail = user.Email
			err = state.DBHandler.DBUpdateOrCreateDeployment(ctx, transaction, *deployment)
			if err != nil {
				return "", err
			}
		}
		if deployment == nil || deployment.Version == nil || isUndeploy {
			locks, err := state.DBHandler.DBSelectAllAppLocks(ctx, transaction, env, u.Application)
			if err != nil {
				return "", err
			}
			if locks == nil {
				continue
			}
			for _, currentLockID := range locks {
				err := state.DBHandler.DBDeleteApplicationLock(ctx, transaction, env, u.Application, currentLockID)
				if err != nil {
					return "", err
				}
			}
			continue
		}
		return "", fmt.Errorf("UndeployApplication(db): error cannot un-deploy application '%v' the release '%v' is not un-deployed", u.Application, env)
	}
	dbApp, err := state.DBHandler.DBSelectExistingApp(ctx, transaction, u.Application)
	if err != nil {
		return "", fmt.Errorf("UndeployApplication: could not select app '%s': %v", u.Application, err)
	}
	err = state.DBHandler.DBInsertOrUpdateApplication(ctx, transaction, dbApp.App, db.AppStateChangeDelete, db.DBAppMetaData{Team: dbApp.Metadata.Team})
	if err != nil {
		return "", fmt.Errorf("UndeployApplication: could not insert app '%s': %v", u.Application, err)
	}

	err = state.DBHandler.DBClearReleases(ctx, transaction, u.Application)

	if err != nil {
		return "", fmt.Errorf("UndeployApplication: could not clear releases for app '%s': %v", u.Application, err)
	}

	allEnvs, err := state.DBHandler.DBSelectAllEnvironments(ctx, transaction)
	if err != nil {
		return "", fmt.Errorf("UndeployApplication: could not get all environments: %v", err)
	}
	if allEnvs == nil {
		return "", fmt.Errorf("UndeployApplication: all environments nil")
	}
	for _, envName := range allEnvs {
		t.AddAppEnv(u.Application, envName, dbApp.Metadata.Team)
		err = state.DBHandler.DBRemoveAppFromEnvironment(ctx, transaction, envName, u.Application)
		if err != nil {
			return "", fmt.Errorf("UndeployApplication: could not write environment: %v", err)
		}
	}

	return fmt.Sprintf("application '%v' was deleted successfully", u.Application), nil
}

type DeleteEnvFromApp struct {
	Authentication        `json:"-"`
	Application           string           `json:"app"`
	Environment           string           `json:"env"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (u *DeleteEnvFromApp) GetDBEventType() db.EventType {
	return db.EvtDeleteEnvFromApp
}

func (u *DeleteEnvFromApp) SetEslVersion(id db.TransformerID) {
	u.TransformerEslVersion = id
}

func (u *DeleteEnvFromApp) GetEslVersion() db.TransformerID {
	return u.TransformerEslVersion
}

func (u *DeleteEnvFromApp) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	err := state.checkUserPermissions(ctx, transaction, u.Environment, u.Application, auth.PermissionDeleteEnvironmentApplication, "", u.RBACConfig, true)
	if err != nil {
		return "", err
	}
	releases, err := state.DBHandler.DBSelectReleasesByAppLatestEslVersion(ctx, transaction, u.Application, true)
	if err != nil {
		return "", err
	}

	now, err := state.DBHandler.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return "", fmt.Errorf("could not get transaction timestamp")
	}
	for _, dbReleaseWithMetadata := range releases {
		newManifests := make(map[string]string)
		for envName, manifest := range dbReleaseWithMetadata.Manifests.Manifests {
			if envName != u.Environment {
				newManifests[envName] = manifest
			}
		}
		newRelease := db.DBReleaseWithMetaData{
			ReleaseNumber: dbReleaseWithMetadata.ReleaseNumber,
			App:           dbReleaseWithMetadata.App,
			Created:       *now,
			Manifests:     db.DBReleaseManifests{Manifests: newManifests},
			Metadata:      dbReleaseWithMetadata.Metadata,
			Environments:  []string{},
		}
		err = state.DBHandler.DBUpdateOrCreateRelease(ctx, transaction, newRelease)
		if err != nil {
			return "", err
		}
	}

	err = state.DBHandler.DBRemoveAppFromEnvironment(ctx, transaction, u.Environment, u.Application)
	if err != nil {
		return "", fmt.Errorf("Couldn't write environment '%s' into environments table, error: %w", u.Environment, err)
	}
	t.DeleteEnvFromApp(u.Application, u.Environment)
	return fmt.Sprintf("Environment '%v' was removed from application '%v' successfully.", u.Environment, u.Application), nil
}

type CleanupOldApplicationVersions struct {
	Application           string
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
}

func (c *CleanupOldApplicationVersions) GetDBEventType() db.EventType {
	return db.EvtCleanupOldApplicationVersions
}

func (c *CleanupOldApplicationVersions) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *CleanupOldApplicationVersions) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

// Finds old releases for an application
func findOldApplicationVersions(ctx context.Context, transaction *sql.Tx, state *State, name string) ([]uint64, error) {
	// 1) get release in each env:
	versions, err := state.GetAllApplicationReleases(ctx, transaction, name)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, err
	}
	sort.Slice(versions, func(i, j int) bool {
		return versions[i] < versions[j]
	})

	deployments, err := state.GetAllDeploymentsForApp(ctx, transaction, name)
	if err != nil {
		return nil, err
	}

	var oldestDeployedVersion uint64
	deployedVersions := []int64{}
	for _, version := range deployments {
		deployedVersions = append(deployedVersions, version)
	}

	if len(deployedVersions) == 0 {
		// Use the latest version as oldest deployed version
		oldestDeployedVersion = versions[len(versions)-1]
	} else {
		oldestDeployedVersion = uint64(slices.Min(deployedVersions))
	}

	positionOfOldestVersion := sort.Search(len(versions), func(i int) bool {
		return versions[i] >= oldestDeployedVersion
	})

	if positionOfOldestVersion < (int(state.ReleaseVersionsLimit) - 1) {
		return nil, nil
	}
	return versions[0 : positionOfOldestVersion-(int(state.ReleaseVersionsLimit)-1)], err
}

func (c *CleanupOldApplicationVersions) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	oldVersions, err := findOldApplicationVersions(ctx, transaction, state, c.Application)
	if err != nil {
		return "", fmt.Errorf("cleanup: could not get application releases for app '%s': %w", c.Application, err)
	}

	msg := ""
	for _, oldRelease := range oldVersions {
		//'Delete' from releases table
		if err := state.DBHandler.DBDeleteFromReleases(ctx, transaction, c.Application, oldRelease); err != nil {
			return "", err
		}
		msg = fmt.Sprintf("%sremoved version %d of app %v as cleanup\n", msg, oldRelease, c.Application)
	}
	// we only cleanup non-deployed versions, so there are not changes for argoCd here
	return msg, nil
}

type Authentication struct {
	RBACConfig auth.RBACConfig
}

type CreateEnvironmentLock struct {
	Authentication        `json:"-"`
	Environment           string           `json:"env"`
	LockId                string           `json:"lockId"`
	Message               string           `json:"message"`
	CiLink                string           `json:"ciLink"`
	AllowedDomains        []string         `json:"-"`
	SuggestedLifeTime     *string          `json:"suggestedLifeTime"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (c *CreateEnvironmentLock) GetDBEventType() db.EventType {
	return db.EvtCreateEnvironmentLock
}

func (c *CreateEnvironmentLock) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *CreateEnvironmentLock) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (s *State) checkUserPermissionsFromConfig(ctx context.Context, transaction *sql.Tx, env, application, action, team string, RBACConfig auth.RBACConfig, checkTeam bool, config *config.EnvironmentConfig) error {
	if !RBACConfig.DexEnabled {
		return nil
	}
	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return fmt.Errorf("checkUserPermissions: user not found: %v", err)
	}

	if config == nil {
		return fmt.Errorf("checkUserPermissions: environment not found: %s", env)
	}
	group := mapper.DeriveGroupName(*config, env)

	if group == "" {
		return fmt.Errorf("group not found for environment: %s", env)
	}
	err = auth.CheckUserPermissions(RBACConfig, user, env, team, group, application, action)
	if err != nil {
		return err
	}
	if checkTeam {
		if team == "" && application != "*" {
			team, err = s.GetTeamName(ctx, transaction, application)

			if err != nil {
				return err
			}
		}
		err = auth.CheckUserTeamPermissions(RBACConfig, user, team, action)
	}
	return err
}

func (s *State) checkUserPermissions(ctx context.Context, transaction *sql.Tx, env, application, action, team string, RBACConfig auth.RBACConfig, checkTeam bool) error {
	cfg, err := s.GetEnvironmentConfigFromDB(ctx, transaction, env)
	if err != nil {
		return err
	}
	return s.checkUserPermissionsFromConfig(ctx, transaction, env, application, action, team, RBACConfig, checkTeam, cfg)
}

// checkUserPermissionsCreateEnvironment check the permission for the environment creation action.
// This is a "special" case because the environment group is already provided on the request.
func (s *State) checkUserPermissionsCreateEnvironment(ctx context.Context, RBACConfig auth.RBACConfig, envConfig config.EnvironmentConfig) error {
	if !RBACConfig.DexEnabled {
		return nil
	}
	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return fmt.Errorf("checkUserPermissions: user not found: %v", err)
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
	transaction *sql.Tx,
) (string, error) {
	err := state.checkUserPermissions(ctx, transaction, c.Environment, "*", auth.PermissionCreateLock, "", c.RBACConfig, false)
	if err != nil {
		return "", err
	}

	if c.CiLink != "" && !isValidLink(c.CiLink, c.AllowedDomains) {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Provided CI Link: %s is not valid or does not match any of the allowed domain", c.CiLink))
	}
	if c.SuggestedLifeTime != nil && *c.SuggestedLifeTime != "" && !isValidLifeTime(*c.SuggestedLifeTime) {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Suggested life time: %s is not a valid lifetime. It should be a number followed by h, d or w.", *c.SuggestedLifeTime))
	}
	now, err := state.DBHandler.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return "", fmt.Errorf("could not get transaction timestamp")
	}

	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return "", err
	}
	//Write to locks table
	metadata := db.LockMetadata{
		Message:           c.Message,
		CreatedByName:     user.Name,
		CreatedByEmail:    user.Email,
		CiLink:            c.CiLink,
		CreatedAt:         *now,
		SuggestedLifeTime: "",
	}
	if c.SuggestedLifeTime != nil {
		metadata.SuggestedLifeTime = *c.SuggestedLifeTime
	}

	errW := state.DBHandler.DBWriteEnvironmentLock(ctx, transaction, c.LockId, c.Environment, metadata)
	if errW != nil {
		return "", errW
	}

	//Add it to all locks
	allEnvLocks, err := state.DBHandler.DBSelectAllEnvironmentLocks(ctx, transaction, c.Environment)
	if err != nil {
		return "", err
	}

	if allEnvLocks == nil {
		allEnvLocks = &db.AllEnvLocksGo{
			Version: 1,
			AllEnvLocksJson: db.AllEnvLocksJson{
				EnvLocks: []string{},
			},
			Created:     *now,
			Environment: c.Environment,
		}
	}

	if !slices.Contains(allEnvLocks.EnvLocks, c.LockId) {
		allEnvLocks.EnvLocks = append(allEnvLocks.EnvLocks, c.LockId)
		err := state.DBHandler.DBWriteAllEnvironmentLocks(ctx, transaction, allEnvLocks.Version, c.Environment, allEnvLocks.EnvLocks)
		if err != nil {
			return "", err
		}
	}
	GaugeEnvLockMetric(ctx, state, transaction, c.Environment)
	return fmt.Sprintf("Created lock %q on environment %q", c.LockId, c.Environment), nil
}

type DeleteEnvironmentLock struct {
	Authentication        `json:"-"`
	Environment           string           `json:"env"`
	LockId                string           `json:"lockId"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (c *DeleteEnvironmentLock) GetDBEventType() db.EventType {
	return db.EvtDeleteEnvironmentLock
}

func (c *DeleteEnvironmentLock) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *DeleteEnvironmentLock) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *DeleteEnvironmentLock) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	err := state.checkUserPermissions(ctx, transaction, c.Environment, "*", auth.PermissionDeleteLock, "", c.RBACConfig, false)
	if err != nil {
		return "", err
	}
	s := State{
		MinorRegexes:              state.MinorRegexes,
		MaxNumThreads:             state.MaxNumThreads,
		DBHandler:                 state.DBHandler,
		ReleaseVersionsLimit:      state.ReleaseVersionsLimit,
		CloudRunClient:            state.CloudRunClient,
		ParallelismOneTransaction: state.ParallelismOneTransaction,
	}
	err = s.DBHandler.DBDeleteEnvironmentLock(ctx, transaction, c.Environment, c.LockId)
	if err != nil {
		return "", err
	}
	allEnvLocks, err := state.DBHandler.DBSelectAllEnvironmentLocks(ctx, transaction, c.Environment)
	if err != nil {
		return "", fmt.Errorf("DeleteEnvironmentLock: could not select all env locks '%v': '%w'", c.Environment, err)
	}
	var locks []string
	if allEnvLocks != nil {
		locks = db.Remove(allEnvLocks.EnvLocks, c.LockId)
	}

	err = state.DBHandler.DBWriteAllEnvironmentLocks(ctx, transaction, allEnvLocks.Version, c.Environment, locks)
	if err != nil {
		return "", fmt.Errorf("DeleteEnvironmentLock: could not write env locks '%v': '%w'", c.Environment, err)
	}

	additionalMessageFromDeployment, err := s.ProcessQueueAllApps(ctx, transaction, c.Environment)
	if err != nil {
		return "", err
	}
	GaugeEnvLockMetric(ctx, state, transaction, c.Environment)
	return fmt.Sprintf("Deleted lock %q on environment %q%s", c.LockId, c.Environment, additionalMessageFromDeployment), nil
}

type CreateEnvironmentGroupLock struct {
	Authentication        `json:"-"`
	EnvironmentGroup      string           `json:"env"`
	LockId                string           `json:"lockId"`
	Message               string           `json:"message"`
	CiLink                string           `json:"ciLink"`
	SuggestedLifeTime     *string          `json:"suggestedLifeTime"`
	AllowedDomains        []string         `json:"-"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (c *CreateEnvironmentGroupLock) GetDBEventType() db.EventType {
	return db.EvtCreateEnvironmentGroupLock
}

func (c *CreateEnvironmentGroupLock) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *CreateEnvironmentGroupLock) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *CreateEnvironmentGroupLock) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	err := state.checkUserPermissions(ctx, transaction, c.EnvironmentGroup, "*", auth.PermissionCreateLock, "", c.RBACConfig, false)
	if err != nil {
		return "", err
	}
	if c.CiLink != "" && !isValidLink(c.CiLink, c.AllowedDomains) {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Provided CI Link: %s is not valid or does not match any of the allowed domain", c.CiLink))
	}
	if c.SuggestedLifeTime != nil && *c.SuggestedLifeTime != "" && !isValidLifeTime(*c.SuggestedLifeTime) {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Suggested life time: %s is not a valid lifetime. It should be a number followed by h, d or w.", *c.SuggestedLifeTime))
	}
	envNamesSorted, err := state.GetEnvironmentConfigsForGroup(ctx, transaction, c.EnvironmentGroup)
	if err != nil {
		return "", grpc.PublicError(ctx, err)
	}
	logger.FromContext(ctx).Sugar().Warnf("envNamesSorted: %v", envNamesSorted)
	for index := range envNamesSorted {
		envName := envNamesSorted[index]
		x := CreateEnvironmentLock{
			Authentication:        c.Authentication,
			Environment:           envName,
			LockId:                c.LockId, // the IDs should be the same for all. See `useLocksSimilarTo` in store.tsx
			Message:               c.Message,
			TransformerEslVersion: c.TransformerEslVersion,
			CiLink:                c.CiLink,
			AllowedDomains:        c.AllowedDomains,
			SuggestedLifeTime:     c.SuggestedLifeTime,
		}
		if err := t.Execute(ctx, &x, transaction); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("Creating locks '%s' for environment group '%s':", c.LockId, c.EnvironmentGroup), nil
}

type DeleteEnvironmentGroupLock struct {
	Authentication        `json:"-"`
	EnvironmentGroup      string           `json:"envGroup"`
	LockId                string           `json:"lockId"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (c *DeleteEnvironmentGroupLock) GetDBEventType() db.EventType {
	return db.EvtDeleteEnvironmentGroupLock
}

func (c *DeleteEnvironmentGroupLock) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *DeleteEnvironmentGroupLock) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *DeleteEnvironmentGroupLock) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	err := state.checkUserPermissions(ctx, transaction, c.EnvironmentGroup, "*", auth.PermissionDeleteLock, "", c.RBACConfig, false)
	if err != nil {
		return "", err
	}
	envNamesSorted, err := state.GetEnvironmentConfigsForGroup(ctx, transaction, c.EnvironmentGroup)
	if err != nil {
		return "", grpc.PublicError(ctx, err)
	}
	for index := range envNamesSorted {
		envName := envNamesSorted[index]
		x := DeleteEnvironmentLock{
			Authentication:        c.Authentication,
			Environment:           envName,
			LockId:                c.LockId,
			TransformerEslVersion: c.TransformerEslVersion,
		}
		if err := t.Execute(ctx, &x, transaction); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("Deleting locks '%s' for environment group '%s':", c.LockId, c.EnvironmentGroup), nil
}

type CreateEnvironmentApplicationLock struct {
	Authentication        `json:"-"`
	Environment           string           `json:"env"`
	Application           string           `json:"app"`
	LockId                string           `json:"lockId"`
	Message               string           `json:"message"`
	CiLink                string           `json:"ciLink"`
	SuggestedLifeTime     *string          `json:"suggestedLifeTime"`
	AllowedDomains        []string         `json:"-"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (c *CreateEnvironmentApplicationLock) GetDBEventType() db.EventType {
	return db.EvtCreateEnvironmentApplicationLock
}

func (c *CreateEnvironmentApplicationLock) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *CreateEnvironmentApplicationLock) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *CreateEnvironmentApplicationLock) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	// Note: it's possible to lock an application BEFORE it's even deployed to the environment.
	err := state.checkUserPermissions(ctx, transaction, c.Environment, c.Application, auth.PermissionCreateLock, "", c.RBACConfig, true)
	if err != nil {
		return "", err
	}
	if c.CiLink != "" && !isValidLink(c.CiLink, c.AllowedDomains) {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Provided CI Link: %s is not valid or does not match any of the allowed domain", c.CiLink))
	}
	if c.SuggestedLifeTime != nil && *c.SuggestedLifeTime != "" && !isValidLifeTime(*c.SuggestedLifeTime) {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Suggested life time: %s is not a valid lifetime. It should be a number followed by h, d or w.", *c.SuggestedLifeTime))
	}
	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return "", err
	}
	now, err := state.DBHandler.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return "", fmt.Errorf("could not get transaction timestamp: %v", err)
	}
	if now == nil {
		return "", fmt.Errorf("could not get transaction timestamp: nil")
	}
	//Write to locks table
	metadata := db.LockMetadata{
		CreatedByName:     user.Name,
		CreatedByEmail:    user.Email,
		Message:           c.Message,
		CiLink:            c.CiLink,
		CreatedAt:         *now,
		SuggestedLifeTime: "",
	}
	if c.SuggestedLifeTime != nil {
		metadata.SuggestedLifeTime = *c.SuggestedLifeTime
	}

	errW := state.DBHandler.DBWriteApplicationLock(ctx, transaction, c.LockId, c.Environment, c.Application, metadata)
	if errW != nil {
		return "", errW
	}

	//Add it to all locks
	allAppLocks, err := state.DBHandler.DBSelectAllAppLocks(ctx, transaction, c.Environment, c.Application)
	if err != nil {
		return "", err
	}
	if allAppLocks == nil {
		allAppLocks = make([]string, 0)
	}

	if !slices.Contains(allAppLocks, c.LockId) {
		allAppLocks = append(allAppLocks, c.LockId)
	}
	GaugeEnvAppLockMetric(ctx, len(allAppLocks), c.Environment, c.Application)

	// locks are invisible to argoCd, so no changes here
	return fmt.Sprintf("Created lock %q on environment %q for application %q", c.LockId, c.Environment, c.Application), nil
}

type DeleteEnvironmentApplicationLock struct {
	Authentication        `json:"-"`
	Environment           string           `json:"env"`
	Application           string           `json:"app"`
	LockId                string           `json:"lockId"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (c *DeleteEnvironmentApplicationLock) GetDBEventType() db.EventType {
	return db.EvtDeleteEnvironmentApplicationLock
}

func (c *DeleteEnvironmentApplicationLock) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *DeleteEnvironmentApplicationLock) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *DeleteEnvironmentApplicationLock) Transform(
	ctx context.Context,
	state *State,
	_ TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	err := state.checkUserPermissions(ctx, transaction, c.Environment, c.Application, auth.PermissionDeleteLock, "", c.RBACConfig, true)

	if err != nil {
		return "", err
	}
	queueMessage := ""
	err = state.DBHandler.DBDeleteApplicationLock(ctx, transaction, c.Environment, c.Application, c.LockId)
	if err != nil {
		return "", err
	}
	allAppLocks, err := state.DBHandler.DBSelectAllAppLocks(ctx, transaction, c.Environment, c.Application)
	if err != nil {
		return "", fmt.Errorf("DeleteEnvironmentApplicationLock: could not select all env app locks for app '%v' on '%v': '%w'", c.Application, c.Environment, err)
	}
	var locks []string
	if allAppLocks != nil {
		locks = db.Remove(allAppLocks, c.LockId)
	}

	GaugeEnvAppLockMetric(ctx, len(locks), c.Environment, c.Application)
	return fmt.Sprintf("Deleted lock %q on environment %q for application %q%s", c.LockId, c.Environment, c.Application, queueMessage), nil
}

type CreateEnvironmentTeamLock struct {
	Authentication        `json:"-"`
	Environment           string           `json:"env"`
	Team                  string           `json:"team"`
	LockId                string           `json:"lockId"`
	Message               string           `json:"message"`
	CiLink                string           `json:"ciLink"`
	SuggestedLifeTime     *string          `json:"suggestedLifeTime"`
	AllowedDomains        []string         `json:"-"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (c *CreateEnvironmentTeamLock) GetDBEventType() db.EventType {
	return db.EvtCreateEnvironmentTeamLock
}

func (c *CreateEnvironmentTeamLock) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *CreateEnvironmentTeamLock) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *CreateEnvironmentTeamLock) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	// Note: it's possible to lock an application BEFORE it's even deployed to the environment.
	err := state.checkUserPermissions(ctx, transaction, c.Environment, "*", auth.PermissionCreateLock, c.Team, c.RBACConfig, true)

	if err != nil {
		return "", err
	}

	if c.CiLink != "" && !isValidLink(c.CiLink, c.AllowedDomains) {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Provided CI Link: %s is not valid or does not match any of the allowed domain", c.CiLink))
	}
	if c.SuggestedLifeTime != nil && *c.SuggestedLifeTime != "" && !isValidLifeTime(*c.SuggestedLifeTime) {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Suggested life time: %s is not a valid lifetime. It should be a number followed by h, d or w.", *c.SuggestedLifeTime))
	}
	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return "", err
	}
	//Write to locks table
	now, err := state.DBHandler.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return "", fmt.Errorf("could not get transaction timestamp")
	}
	metadata := db.LockMetadata{
		CreatedByName:     user.Name,
		CreatedByEmail:    user.Email,
		Message:           c.Message,
		CiLink:            c.CiLink,
		CreatedAt:         *now,
		SuggestedLifeTime: "",
	}
	if c.SuggestedLifeTime != nil {
		metadata.SuggestedLifeTime = *c.SuggestedLifeTime
	}

	errW := state.DBHandler.DBWriteTeamLock(ctx, transaction, c.LockId, c.Environment, c.Team, metadata)

	if errW != nil {
		return "", errW
	}

	return fmt.Sprintf("Created lock %q on environment %q for team %q", c.LockId, c.Environment, c.Team), nil
}

type DeleteEnvironmentTeamLock struct {
	Authentication        `json:"-"`
	Environment           string           `json:"env"`
	Team                  string           `json:"team"`
	LockId                string           `json:"lockId"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
}

func (c *DeleteEnvironmentTeamLock) GetDBEventType() db.EventType {
	return db.EvtDeleteEnvironmentTeamLock
}

func (c *DeleteEnvironmentTeamLock) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *DeleteEnvironmentTeamLock) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *DeleteEnvironmentTeamLock) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	err := state.checkUserPermissions(ctx, transaction, c.Environment, "", auth.PermissionDeleteLock, c.Team, c.RBACConfig, true)

	if err != nil {
		return "", err
	}
	err = state.DBHandler.DBDeleteTeamLock(ctx, transaction, c.Environment, c.Team, c.LockId)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Deleted lock %q on environment %q for team %q", c.LockId, c.Environment, c.Team), nil
}

// Creates or Update an Environment
type CreateEnvironment struct {
	Authentication        `json:"-"`
	Environment           string                   `json:"env"`
	Config                config.EnvironmentConfig `json:"config"`
	TransformerEslVersion db.TransformerID         `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (c *CreateEnvironment) GetDBEventType() db.EventType {
	return db.EvtCreateEnvironment
}

func (c *CreateEnvironment) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *CreateEnvironment) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *CreateEnvironment) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	err := state.checkUserPermissionsCreateEnvironment(ctx, c.RBACConfig, c.Config)
	if err != nil {
		return "", err
	}
	// first read the env to see if it has applications:
	env, err := state.DBHandler.DBSelectEnvironment(ctx, transaction, c.Environment)
	if err != nil {
		return "", fmt.Errorf("could not select environment %s from database, error: %w", c.Environment, err)
	}

	// write to environments table
	// We don't use environment applications column anymore, but for backward compatibility we keep it updated
	environmentApplications := make([]string, 0)
	if env != nil {
		environmentApplications = env.Applications
	}
	err = state.DBHandler.DBWriteEnvironment(ctx, transaction, c.Environment, c.Config, environmentApplications)
	if err != nil {
		return "", fmt.Errorf("unable to write to the environment table, error: %w", err)
	}

	//Should be empty on new environments
	envApps, err := state.GetEnvironmentApplications(ctx, transaction, c.Environment)
	if err != nil {
		return "", fmt.Errorf("Unable to read environment, error: %w", err)

	}
	for _, app := range envApps {
		t.AddAppEnv(app, c.Environment, "")
	}

	// we do not need to inform argoCd when creating an environment, as there are no apps yet
	return fmt.Sprintf("create environment %q", c.Environment), nil
}

type DeleteEnvironment struct {
	Authentication        `json:"-"`
	Environment           string           `json:"env"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (c *DeleteEnvironment) GetDBEventType() db.EventType {
	return db.EvtDeleteEnvironment
}

func (c *DeleteEnvironment) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *DeleteEnvironment) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *DeleteEnvironment) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	err := state.checkUserPermissions(ctx, transaction, c.Environment, "*", auth.PermissionDeleteEnvironment, "", c.RBACConfig, false)
	if err != nil {
		return "", err
	}

	allEnvs, err := state.DBHandler.DBSelectAllEnvironments(ctx, transaction)
	if err != nil || allEnvs == nil {
		return "", fmt.Errorf("error getting all environments %v", err)
	}

	/*Check for locks*/
	envLocks, err := state.DBHandler.DBSelectAllEnvironmentLocks(ctx, transaction, c.Environment)
	if err != nil {
		return "", err
	}
	if envLocks != nil && len(envLocks.EnvLocks) != 0 {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Could not delete environment '%s'. Environment locks for this environment exist.", c.Environment))
	}

	appLocksForEnv, err := state.DBHandler.DBSelectAllAppLocksForEnv(ctx, transaction, c.Environment)
	if err != nil {
		return "", err
	}
	if len(appLocksForEnv) != 0 {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Could not delete environment '%s'. Application locks for this environment exist.", c.Environment))
	}

	teamLocksForEnv, err := state.DBHandler.DBSelectTeamLocksForEnv(ctx, transaction, c.Environment)
	if err != nil {
		return "", err
	}
	if len(teamLocksForEnv) != 0 {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Could not delete environment '%s'. Team locks for this environment exist.", c.Environment))
	}

	/* Check that no environment has the one we are trying to delete as upstream */
	allEnvConfigs, err := state.GetAllEnvironmentConfigs(ctx, transaction)
	if err != nil {
		return "", err
	}

	envConfigToDelete := allEnvConfigs[c.Environment]

	//Find out if env to delete is last of its group, might be useful next
	var envToDeleteGroupName = mapper.DeriveGroupName(envConfigToDelete, c.Environment)

	var allEnvGroups = mapper.MapEnvironmentsToGroups(allEnvConfigs)
	lastEnvOfGroup := false
	for _, currGroup := range allEnvGroups {
		if currGroup.EnvironmentGroupName == envToDeleteGroupName {
			lastEnvOfGroup = (len(currGroup.Environments) == 1)
		}
	}

	for envName, envConfig := range allEnvConfigs {
		if envConfig.Upstream != nil && envConfig.Upstream.Environment == c.Environment {
			return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Could not delete environment '%s'. Environment '%s' is upstream from '%s'", c.Environment, c.Environment, envName))
		}

		//If we are deleting an environment and it is the last one on the group, we are also deleting the group.
		//If this group is upstream from another env, we need to block it aswell
		if envConfig.Upstream != nil && envConfig.Upstream.Environment == envToDeleteGroupName && lastEnvOfGroup {
			return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Could not delete environment '%s'. '%s' is part of environment group '%s', "+
				"which is upstream from '%s' and deleting '%s' would result in environment group deletion.",
				c.Environment,
				c.Environment,
				envToDeleteGroupName,
				envName,
				c.Environment,
			))
		}
	}

	/*Remove environment from all apps*/
	allAppsForEnv, err := state.GetEnvironmentApplications(ctx, transaction, c.Environment)
	if err != nil {
		return "", err
	}

	//Delete env from apps
	for _, app := range allAppsForEnv {
		logger.FromContext(ctx).Sugar().Infof("Deleting environment '%s' from '%s'.", c.Environment, app)
		deleteEnvFromAppTransformer := DeleteEnvFromApp{
			Authentication:        c.Authentication,
			TransformerEslVersion: c.TransformerEslVersion,
			Environment:           c.Environment,
			Application:           app,
		}
		err = t.Execute(ctx, &deleteEnvFromAppTransformer, transaction)
		if err != nil {
			return "", err
		}
	}

	//Delete env from environments table
	err = state.DBHandler.DBDeleteEnvironment(ctx, transaction, c.Environment)

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Successfully deleted environment '%s'", c.Environment), nil
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
	version := int64(c.Version)
	err := state.DBHandler.DBWriteDeploymentAttempt(ctx, transaction, c.Environment, c.Application, &version)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Queued version %d of app %q in env %q", c.Version, c.Application, c.Environment), nil
}

type DeployApplicationVersion struct {
	Authentication        `json:"-"`
	Environment           string                          `json:"env"`
	Application           string                          `json:"app"`
	Version               uint64                          `json:"version"`
	LockBehaviour         api.LockBehavior                `json:"lockBehaviour"`
	WriteCommitData       bool                            `json:"writeCommitData"`
	SourceTrain           *DeployApplicationVersionSource `json:"sourceTrain"`
	Author                string                          `json:"author"`
	CiLink                string                          `json:"cilink"`
	TransformerEslVersion db.TransformerID                `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	SkipCleanup           bool                            `json:"-"`
}

func (c *DeployApplicationVersion) GetDBEventType() db.EventType {
	return db.EvtDeployApplicationVersion
}

func (c *DeployApplicationVersion) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *DeployApplicationVersion) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
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
	err := state.checkUserPermissions(ctx, transaction, c.Environment, c.Application, auth.PermissionDeployRelease, "", c.RBACConfig, true)
	if err != nil {
		return "", err
	}

	var manifestContent []byte
	version, err := state.DBHandler.DBSelectReleaseByVersion(ctx, transaction, c.Application, c.Version, true)
	if err != nil {
		return "", err
	}
	if version == nil {
		return "", fmt.Errorf("could not find version %d for app %s", c.Version, c.Application)
	}
	manifestContent = []byte(version.Manifests.Manifests[c.Environment])
	lockPreventedDeployment := false
	team, err := state.GetApplicationTeamOwner(ctx, transaction, c.Application)
	if err != nil {
		return "", fmt.Errorf("could not determine team for deployment: %w", err)
	}
	if c.LockBehaviour != api.LockBehavior_IGNORE {
		// Check that the environment is not locked
		var (
			envLocks, appLocks, teamLocks map[string]Lock
			err                           error
		)
		envLocks, err = state.GetEnvironmentLocks(ctx, transaction, c.Environment)
		if err != nil {
			return "", err
		}
		appLocks, err = state.GetEnvironmentApplicationLocks(ctx, transaction, c.Environment, c.Application)
		if err != nil {
			return "", err
		}
		teamLocks, err = state.GetEnvironmentTeamLocks(ctx, transaction, c.Environment, string(team))
		if err != nil {
			return "", err
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
				ev := createLockPreventedDeploymentEvent(c.Application, c.Environment, lockMsg, lockType)
				newReleaseCommitId, err := getCommitID(ctx, transaction, state, c.Version, c.Application)
				if err != nil {
					logger.FromContext(ctx).Sugar().Warnf("could not write event data - continuing. %v", fmt.Errorf("getCommitIDFromReleaseDir %v", err))
				} else {
					gen := getGenerator(ctx)
					eventUuid := gen.Generate()
					err = state.DBHandler.DBWriteLockPreventedDeploymentEvent(ctx, transaction, c.TransformerEslVersion, eventUuid, newReleaseCommitId, ev)
					if err != nil {
						return "", GetCreateReleaseGeneralFailure(err)
					}
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
				return q.Transform(ctx, state, t, transaction)
			case api.LockBehavior_FAIL:
				return "", &LockedError{
					EnvironmentApplicationLocks: appLocks,
					EnvironmentLocks:            envLocks,
					TeamLocks:                   teamLocks,
				}
			}
		}
	}

	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return "", err
	}

	firstDeployment := false
	var oldVersion *int64

	if state.CloudRunClient != nil {
		err := state.CloudRunClient.DeployApplicationVersion(ctx, manifestContent)
		if err != nil {
			return "", err
		}
	}
	existingDeployment, err := state.DBHandler.DBSelectLatestDeployment(ctx, transaction, c.Application, c.Environment)
	if err != nil {
		return "", err
	}
	if existingDeployment == nil || existingDeployment.Version == nil {
		firstDeployment = true
	} else {
		oldVersion = existingDeployment.Version
	}
	if err != nil {
		return "", fmt.Errorf("could not find deployment for app %s and env %s", c.Application, c.Environment)
	}
	var v = int64(c.Version)
	newDeployment := db.Deployment{
		Created:       time.Time{},
		App:           c.Application,
		Env:           c.Environment,
		Version:       &v,
		TransformerID: c.TransformerEslVersion,
		Metadata: db.DeploymentMetadata{
			DeployedByEmail: user.Email,
			DeployedByName:  user.Name,
			CiLink:          c.CiLink,
		},
	}
	err = state.DBHandler.DBUpdateOrCreateDeployment(ctx, transaction, newDeployment)
	if err != nil {
		return "", fmt.Errorf("could not write deployment for %v - %v", newDeployment, err)
	}
	t.AddAppEnv(c.Application, c.Environment, team)
	s := State{
		MinorRegexes:              state.MinorRegexes,
		MaxNumThreads:             state.MaxNumThreads,
		DBHandler:                 state.DBHandler,
		ReleaseVersionsLimit:      state.ReleaseVersionsLimit,
		CloudRunClient:            state.CloudRunClient,
		ParallelismOneTransaction: state.ParallelismOneTransaction,
	}
	err = s.DeleteQueuedVersionIfExists(ctx, transaction, c.Environment, c.Application)
	if err != nil {
		return "", err
	}
	if !c.SkipCleanup {
		d := &CleanupOldApplicationVersions{
			Application:           c.Application,
			TransformerEslVersion: c.TransformerEslVersion,
		}

		if err := t.Execute(ctx, d, transaction); err != nil {
			return "", err
		}
	}
	if c.WriteCommitData { // write the corresponding event
		newReleaseCommitId, err := getCommitID(ctx, transaction, state, c.Version, c.Application)
		deploymentEvent := createDeploymentEvent(c.Application, c.Environment, c.SourceTrain)
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("could not write event data - continuing. %v", fmt.Errorf("getCommitIDFromReleaseDir %v", err))
		} else {
			if !valid.SHA1CommitID(newReleaseCommitId) {
				logger.FromContext(ctx).Sugar().Warnf("skipping event because commit id was not found")
			} else {
				gen := getGenerator(ctx)
				eventUuid := gen.Generate()
				err = state.DBHandler.DBWriteDeploymentEvent(ctx, transaction, c.TransformerEslVersion, eventUuid, newReleaseCommitId, deploymentEvent)
				if err != nil {
					return "", GetCreateReleaseGeneralFailure(err)
				}
			}
		}

		if !firstDeployment && !lockPreventedDeployment {
			//If not first deployment and current deployment is successful, signal a new replaced by event
			if !valid.SHA1CommitID(newReleaseCommitId) {
				logger.FromContext(ctx).Sugar().Infof(
					"The source commit ID %s is not a valid/complete SHA1 hash, event cannot be stored.",
					newReleaseCommitId)
			} else {
				ev := createReplacedByEvent(c.Application, c.Environment, newReleaseCommitId)
				if oldVersion == nil {
					logger.FromContext(ctx).Sugar().Errorf("did not find old version of app %s - skipping replaced-by event", c.Application)
				} else {
					gen := getGenerator(ctx)
					eventUuid := gen.Generate()
					v := uint64(*oldVersion)
					oldReleaseCommitId, err := getCommitID(ctx, transaction, state, v, c.Application)
					if err != nil {
						logger.FromContext(ctx).Sugar().Warnf("could not find commit for release %d of app %s - skipping replaced-by event", v, c.Application)
					} else {
						err = state.DBHandler.DBWriteReplacedByEvent(ctx, transaction, c.TransformerEslVersion, eventUuid, oldReleaseCommitId, ev)
						if err != nil {
							return "", err
						}
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

func getCommitID(ctx context.Context, transaction *sql.Tx, state *State, release uint64, app string) (string, error) {
	tmp, err := state.DBHandler.DBSelectReleaseByVersion(ctx, transaction, app, release, true)
	if err != nil {
		return "", err
	}
	if tmp == nil {
		return "", fmt.Errorf("release %v not found for app %s", release, app)
	}
	if tmp.Metadata.SourceCommitId == "" {
		return "", fmt.Errorf("Found release %v for app %s, but commit id was empty", release, app)
	}
	return tmp.Metadata.SourceCommitId, nil
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

func createLockPreventedDeploymentEvent(application, environment, lockMsg, lockType string) *event.LockPreventedDeployment {
	ev := event.LockPreventedDeployment{
		Application: application,
		Environment: environment,
		LockMessage: lockMsg,
		LockType:    lockType,
	}
	return &ev
}

type ReleaseTrain struct {
	Authentication        `json:"-"`
	Target                string           `json:"target"`
	Team                  string           `json:"team,omitempty"`
	CommitHash            string           `json:"commitHash"`
	WriteCommitData       bool             `json:"writeCommitData"`
	Repo                  Repository       `json:"-"`
	TransformerEslVersion db.TransformerID `json:"-"`
	TargetType            string           `json:"targetType"`
	CiLink                string           `json:"-"`
	AllowedDomains        []string         `json:"-"`
}

func (c *ReleaseTrain) GetDBEventType() db.EventType {
	return db.EvtReleaseTrain
}

func (c *ReleaseTrain) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *ReleaseTrain) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

type Overview struct {
	App     string
	Version uint64
}

func getOverrideVersions(ctx context.Context, transaction *sql.Tx, commitHash, upstreamEnvName string, state *State) (resp []Overview, err error) {
	dbHandler := state.DBHandler
	ts, err := dbHandler.DBReadCommitHashTransactionTimestamp(ctx, transaction, commitHash)
	if err != nil {
		return nil, fmt.Errorf("unable to get manifest repo timestamp that corresponds to provided commit Hash %v", err)
	} else if ts == nil {
		return nil, fmt.Errorf("timestamp for the provided commit hash %q does not exist.", commitHash)
	}

	apps, err := state.GetEnvironmentApplicationsAtTimestamp(ctx, transaction, upstreamEnvName, *ts)
	if err != nil {
		return nil, fmt.Errorf("unable to get applications for environment %s at timestamp %s: %w", upstreamEnvName, *ts, err)
	}

	for _, appName := range apps {
		currentAppDeployments, err := state.GetAllDeploymentsForAppAtTimestamp(ctx, transaction, appName, *ts)
		if err != nil {
			return nil, fmt.Errorf("unable to get GetAllDeploymentsForAppAtTimestamp  %v", err)
		}

		if version, ok := currentAppDeployments[upstreamEnvName]; ok {
			resp = append(resp, Overview{App: appName, Version: uint64(version)})
		}
	}

	return resp, nil
}

func (c *ReleaseTrain) getUpstreamLatestApp(ctx context.Context, transaction *sql.Tx, upstreamLatest bool, state *State, upstreamEnvName, source, commitHash string, targetEnv string) (apps []string, appVersions []Overview, err error) {
	if commitHash != "" {
		appVersions, err := getOverrideVersions(ctx, transaction, c.CommitHash, upstreamEnvName, state)
		if err != nil {
			return nil, nil, grpc.PublicError(ctx, fmt.Errorf("could not get app version for commitHash %s for %s: %w", c.CommitHash, c.Target, err))
		}
		// check that commit hash is not older than 20 commits in the past
		for _, app := range appVersions {
			apps = append(apps, app.App)
			versions, err := findOldApplicationVersions(ctx, transaction, state, app.App)
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
		// For "upstreamlatest" we cannot get the source environment, because it's not a real environment
		// but since we only care about the names of the apps, we can just get the apps for the target env.
		apps, err = state.GetEnvironmentApplications(ctx, transaction, targetEnv)
		if err != nil {
			return nil, nil, grpc.PublicError(ctx, fmt.Errorf("could not get all applications for %q: %w", source, err))
		}
		return apps, nil, nil
	}
	apps, err = state.GetEnvironmentApplications(ctx, transaction, upstreamEnvName)
	if err != nil {
		return nil, nil, grpc.PublicError(ctx, fmt.Errorf("upstream environment (%q) does not have applications: %w", upstreamEnvName, err))
	}
	return apps, nil, nil
}

func getEnvironmentGroupsEnvironmentsOrEnvironment(configs map[string]config.EnvironmentConfig, targetName string, targetType string) (map[string]config.EnvironmentConfig, bool) {
	envGroupConfigs := make(map[string]config.EnvironmentConfig)
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
			envConfig, ok := configs[targetName]
			if ok {
				envGroupConfigs[targetName] = envConfig
			}
		}
	}
	return envGroupConfigs, isEnvGroup
}

type ReleaseTrainApplicationPrognosis struct {
	SkipCause *api.ReleaseTrainAppPrognosis_SkipCause
	Locks     []*api.Lock
	Version   uint64
	Team      string
}

type ReleaseTrainEnvironmentPrognosis struct {
	SkipCause *api.ReleaseTrainEnvPrognosis_SkipCause
	Error     error
	Locks     []*api.Lock
	// map key is the name of the app
	AppsPrognoses map[string]ReleaseTrainApplicationPrognosis
}

type ReleaseTrainPrognosisOutcome = uint64

type ReleaseTrainPrognosis struct {
	Error                error
	EnvironmentPrognoses map[string]ReleaseTrainEnvironmentPrognosis
}

func failedPrognosis(err error) *ReleaseTrainEnvironmentPrognosis {
	return &ReleaseTrainEnvironmentPrognosis{
		SkipCause:     nil,
		Error:         err,
		Locks:         nil,
		AppsPrognoses: nil,
	}
}

func (c *ReleaseTrain) Prognosis(
	ctx context.Context,
	state *State,
	transaction *sql.Tx,
	configs map[string]config.EnvironmentConfig,
) ReleaseTrainPrognosis {
	span, ctx := tracer.StartSpanFromContext(ctx, "ReleaseTrain Prognosis")
	defer span.Finish()
	span.SetTag("targetEnv", c.Target)
	span.SetTag("targetType", c.TargetType)
	span.SetTag("team", c.Team)

	var targetGroupName = c.Target
	var envGroupConfigs, isEnvGroup = getEnvironmentGroupsEnvironmentsOrEnvironment(configs, targetGroupName, c.TargetType)
	if len(envGroupConfigs) == 0 {
		if c.TargetType == api.ReleaseTrainRequest_ENVIRONMENT.String() || c.TargetType == api.ReleaseTrainRequest_ENVIRONMENTGROUP.String() {
			return ReleaseTrainPrognosis{
				Error:                grpc.PublicError(ctx, fmt.Errorf("could not find target of type %v and name '%v'", c.TargetType, targetGroupName)),
				EnvironmentPrognoses: nil,
			}
		}
		return ReleaseTrainPrognosis{
			Error:                grpc.PublicError(ctx, fmt.Errorf("could not find environment group or environment configs for '%v'", targetGroupName)),
			EnvironmentPrognoses: nil,
		}
	}

	allLatestReleases, err := state.GetAllLatestReleases(ctx, transaction, nil)
	if err != nil {
		return ReleaseTrainPrognosis{
			Error:                grpc.PublicError(ctx, fmt.Errorf("could not get all releases of all apps %w", err)),
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
			trainGroup = conversion.FromString(targetGroupName)
		}

		envReleaseTrain := &envReleaseTrain{
			Parent:                       c,
			Env:                          envName,
			EnvConfigs:                   configs,
			EnvGroupConfigs:              envGroupConfigs,
			WriteCommitData:              c.WriteCommitData,
			TrainGroup:                   trainGroup,
			TransformerEslVersion:        c.TransformerEslVersion,
			CiLink:                       c.CiLink,
			AllLatestReleasesCache:       allLatestReleases,
			AllLatestReleaseEnvironments: nil,
		}

		envPrognosis := envReleaseTrain.prognosis(ctx, state, transaction, allLatestReleases)

		if envPrognosis.Error != nil {
			return ReleaseTrainPrognosis{
				Error:                envPrognosis.Error,
				EnvironmentPrognoses: nil,
			}
		}

		envPrognoses[envName] = *envPrognosis
	}

	return ReleaseTrainPrognosis{
		Error:                nil,
		EnvironmentPrognoses: envPrognoses,
	}
}

func (c *ReleaseTrain) Transform(
	ctx context.Context,
	state *State,
	transformerContext TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "ReleaseTrain")
	defer span.Finish()
	//Prognosis can be a costly operation. Abort straight away if ci link is not valid
	if c.CiLink != "" && !isValidLink(c.CiLink, c.AllowedDomains) {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Provided CI Link: %s is not valid or does not match any of the allowed domain", c.CiLink))
	}
	configs, err := state.GetAllEnvironmentConfigs(ctx, transaction)
	if err != nil {
		return "", err
	}

	var targetGroupName = c.Target
	var envGroupConfigs, isEnvGroup = getEnvironmentGroupsEnvironmentsOrEnvironment(configs, targetGroupName, c.TargetType)
	if len(envGroupConfigs) == 0 {
		if c.TargetType == api.ReleaseTrainRequest_ENVIRONMENT.String() || c.TargetType == api.ReleaseTrainRequest_ENVIRONMENTGROUP.String() {
			return "", grpc.PublicError(ctx, fmt.Errorf("could not find target of type %v and name '%v'", c.TargetType, targetGroupName))
		}
		return "", grpc.PublicError(ctx, fmt.Errorf("could not find environment group or environment configs for '%v'", targetGroupName))
	}

	// sorting for determinism
	envNames := make([]string, 0, len(envGroupConfigs))
	for env := range envGroupConfigs {
		envNames = append(envNames, env)
	}
	sort.Strings(envNames)
	span.SetTag("environments", len(envNames))

	allLatestReleases, err := state.GetAllLatestReleases(ctx, transaction, nil)
	if err != nil {
		return "", grpc.PublicError(ctx, fmt.Errorf("could not get all releases of all apps %w", err))
	}

	if isEnvGroup && state.DBHandler.AllowParallelTransactions() {
		releaseTrainErrGroup, ctx := errgroup.WithContext(ctx)
		if state.MaxNumThreads > 0 {
			releaseTrainErrGroup.SetLimit(state.MaxNumThreads)
		}
		if state.ParallelismOneTransaction {
			span, ctx, onErr := tracing.StartSpanFromContext(ctx, "EnvReleaseTrain Parallel")
			defer span.Finish()
			s, err2 := c.runWithNewGoRoutines(
				envNames,
				targetGroupName,
				state.MaxNumThreads,
				ctx,
				configs,
				envGroupConfigs,
				allLatestReleases,
				state,
				transformerContext,
				transaction)
			if err2 != nil {
				return s, onErr(err2)
			} else {
				span.Finish()
			}
		} else {
			for _, envName := range envNames {
				trainGroup := conversion.FromString(targetGroupName)
				envNameLocal := envName
				releaseTrainErrGroup.Go(func() error {
					return c.runEnvReleaseTrainBackground(ctx, state, transformerContext, envNameLocal, trainGroup, envGroupConfigs, configs, allLatestReleases)
				})
			}
		}
		err := releaseTrainErrGroup.Wait()
		if err != nil {
			return "", err
		}
	} else {
		for _, envName := range envNames {
			var trainGroup *string
			if isEnvGroup {
				trainGroup = conversion.FromString(targetGroupName)
			}
			err = transformerContext.Execute(ctx, &envReleaseTrain{
				Parent:                c,
				Env:                   envName,
				EnvConfigs:            configs,
				EnvGroupConfigs:       envGroupConfigs,
				WriteCommitData:       c.WriteCommitData,
				TrainGroup:            trainGroup,
				TransformerEslVersion: c.TransformerEslVersion,
				CiLink:                c.CiLink,

				AllLatestReleasesCache:       allLatestReleases,
				AllLatestReleaseEnvironments: nil,
			}, transaction)
			if err != nil {
				return "", err
			}
		}
	}

	return fmt.Sprintf(
		"Release Train to environment/environment group '%s':\n",
		targetGroupName), nil
}

func (c *ReleaseTrain) runWithNewGoRoutines(
	envNames []string,
	targetGroupName string,
	maxThreads int,
	parentCtx context.Context,
	configs map[string]config.EnvironmentConfig,
	envGroupConfigs map[string]config.EnvironmentConfig,
	allLatestReleases map[string][]int64,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (
	string, error,
) {
	type ChannelData struct {
		prognosis *ReleaseTrainEnvironmentPrognosis
		train     *envReleaseTrain
		error     error
	}
	spanManifests, ctxManifests, onErr := tracing.StartSpanFromContext(parentCtx, "Load Manifests")
	var allReleasesManifests db.AppVersionManifests
	var err error
	allReleasesManifests, err = state.DBHandler.DBSelectAllManifestsForAllReleases(ctxManifests, transaction)
	if err != nil {
		return "", onErr(err)
	}
	spanManifests.Finish()

	var prognosisResultChannel = make(chan *ChannelData, maxThreads)
	go func() {
		spanSpawnAll, ctxSpawnAll := tracer.StartSpanFromContext(parentCtx, "Spawn Go Routines")
		spanSpawnAll.SetTag("numEnvironments", len(envNames))
		group, ctxRoutines := errgroup.WithContext(ctxSpawnAll)
		if maxThreads > 0 {
			group.SetLimit(maxThreads)
		}
		for _, envName := range envNames {
			trainGroup := conversion.FromString(targetGroupName)
			envNameLocal := envName
			// we want to schedule all go routines,
			// but still have the limit set, so "group.Go" would block
			group.Go(func() error {
				span, ctx, onErr := tracing.StartSpanFromContext(ctxRoutines, "EnvReleaseTrain Transform")
				defer span.Finish()

				train := &envReleaseTrain{
					Parent:                c,
					Env:                   envName,
					EnvConfigs:            configs,
					EnvGroupConfigs:       envGroupConfigs,
					WriteCommitData:       c.WriteCommitData,
					TrainGroup:            trainGroup,
					TransformerEslVersion: c.TransformerEslVersion,
					CiLink:                c.CiLink,

					AllLatestReleasesCache:       allLatestReleases,
					AllLatestReleaseEnvironments: allReleasesManifests,
				}

				prognosis, err := train.runEnvPrognosisBackground(ctx, state, envNameLocal, allLatestReleases)

				spanCh, _ := tracer.StartSpanFromContext(ctx, "WriteToChannel")
				defer spanCh.Finish()
				if err != nil {
					prognosisResultChannel <- &ChannelData{
						error:     onErr(err),
						prognosis: prognosis,
						train:     train,
					}
				} else {
					prognosisResultChannel <- &ChannelData{
						error:     nil,
						prognosis: prognosis,
						train:     train,
					}
				}
				return nil
			})
		}
		spanSpawnAll.Finish()

		err := group.Wait()
		if err != nil {
			logger.FromContext(parentCtx).Sugar().Error("waitgroup.error", zap.Error(err))
		}
		close(prognosisResultChannel)
	}()

	spanApplyAll, ctxApplyAll, onErrAll := tracing.StartSpanFromContext(parentCtx, "ApplyAllPrognoses")
	expectedNumPrognoses := len(envNames)
	spanApplyAll.SetTag("numPrognoses", expectedNumPrognoses)
	defer spanApplyAll.Finish()

	var i = 0
	for result := range prognosisResultChannel {
		spanOne, ctxOne, onErrOne := tracing.StartSpanFromContext(ctxApplyAll, "ApplyOnePrognosis")
		spanOne.SetTag("index", i)
		if result.error != nil {
			return "", onErrOne(onErrAll(fmt.Errorf("prognosis could not be applied for env '%s': %w", result.train.Env, result.error)))
		}
		spanOne.SetTag("environment", result.train.Env)
		_, err := result.train.applyPrognosis(ctxOne, state, t, transaction, result.prognosis, spanOne)
		if err != nil {
			return "", onErrOne(onErrAll(fmt.Errorf("prognosis could not be applied for env '%s': %w", result.train.Env, result.error)))
		}
		i++
		spanOne.Finish()
	}

	return "", nil
}

func (c *ReleaseTrain) runEnvReleaseTrainBackground(ctx context.Context, state *State, t TransformerContext, envName string, trainGroup *string, envGroupConfigs map[string]config.EnvironmentConfig, configs map[string]config.EnvironmentConfig, releases map[string][]int64) error {
	err := state.DBHandler.WithTransactionR(ctx, 2, false, func(ctx context.Context, transaction2 *sql.Tx) error {
		err := t.Execute(ctx, &envReleaseTrain{
			Parent:                c,
			Env:                   envName,
			EnvConfigs:            configs,
			EnvGroupConfigs:       envGroupConfigs,
			WriteCommitData:       c.WriteCommitData,
			TrainGroup:            trainGroup,
			TransformerEslVersion: c.TransformerEslVersion,
			CiLink:                c.CiLink,

			AllLatestReleasesCache:       releases,
			AllLatestReleaseEnvironments: nil,
		}, transaction2)
		return err
	})
	return err
}

func (c *envReleaseTrain) runEnvPrognosisBackground(
	ctx context.Context,
	state *State,
	envName string,
	releases map[string][]int64,
) (*ReleaseTrainEnvironmentPrognosis, error) {
	result, err := db.WithTransactionT(state.DBHandler, ctx, 2, false, func(ctx context.Context, transaction *sql.Tx) (*ReleaseTrainEnvironmentPrognosis, error) {
		prognosis := c.prognosis(ctx, state, transaction, releases)
		return prognosis, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run background prognosis for env %s: %w", envName, err)
	}
	return result, err
}

type AllLatestReleasesCache map[string][]int64

type envReleaseTrain struct {
	Parent                *ReleaseTrain
	Env                   string
	EnvConfigs            map[string]config.EnvironmentConfig
	EnvGroupConfigs       map[string]config.EnvironmentConfig
	WriteCommitData       bool
	TrainGroup            *string
	TransformerEslVersion db.TransformerID
	CiLink                string

	AllLatestReleasesCache       AllLatestReleasesCache
	AllLatestReleaseEnvironments db.AppVersionManifests // can be prefilled with all manifests, so that each envReleaseTrain does not need to fetch them again
}

func (c *envReleaseTrain) GetDBEventType() db.EventType {
	return db.EvtEnvReleaseTrain
}

func (c *envReleaseTrain) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *envReleaseTrain) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *envReleaseTrain) prognosis(ctx context.Context, state *State, transaction *sql.Tx, allLatestReleases AllLatestReleasesCache) *ReleaseTrainEnvironmentPrognosis {
	span, ctx := tracer.StartSpanFromContext(ctx, "EnvReleaseTrain Prognosis")
	defer span.Finish()
	span.SetTag("env", c.Env)

	envConfig := c.EnvGroupConfigs[c.Env]
	if envConfig.Upstream == nil {
		return &ReleaseTrainEnvironmentPrognosis{
			SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
				SkipCause: api.ReleaseTrainEnvSkipCause_ENV_HAS_NO_UPSTREAM,
			},
			Error:         nil,
			Locks:         nil,
			AppsPrognoses: nil,
		}
	}

	err := state.checkUserPermissions(
		ctx,
		transaction,
		c.Env,
		"*",
		auth.PermissionDeployReleaseTrain,
		c.Parent.Team,
		c.Parent.RBACConfig,
		false,
	)

	if err != nil {
		return failedPrognosis(err)
	}

	upstreamLatest := envConfig.Upstream.Latest
	upstreamEnvName := envConfig.Upstream.Environment
	if !upstreamLatest && upstreamEnvName == "" {
		return &ReleaseTrainEnvironmentPrognosis{
			SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
				SkipCause: api.ReleaseTrainEnvSkipCause_ENV_HAS_NO_UPSTREAM_LATEST_OR_UPSTREAM_ENV,
			},
			Error:         nil,
			Locks:         nil,
			AppsPrognoses: nil,
		}
	}

	if upstreamLatest && upstreamEnvName != "" {
		return &ReleaseTrainEnvironmentPrognosis{
			SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
				SkipCause: api.ReleaseTrainEnvSkipCause_ENV_HAS_BOTH_UPSTREAM_LATEST_AND_UPSTREAM_ENV,
			},
			Error:         nil,
			Locks:         nil,
			AppsPrognoses: nil,
		}
	}

	if !upstreamLatest {
		_, ok := c.EnvConfigs[upstreamEnvName]
		if !ok {
			return &ReleaseTrainEnvironmentPrognosis{
				SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
					SkipCause: api.ReleaseTrainEnvSkipCause_UPSTREAM_ENV_CONFIG_NOT_FOUND,
				},
				Error:         nil,
				Locks:         nil,
				AppsPrognoses: nil,
			}
		}
	}
	envLocks, err := state.GetEnvironmentLocks(ctx, transaction, c.Env)
	if err != nil {
		return failedPrognosis(grpc.InternalError(ctx, fmt.Errorf("could not get lock for environment %q: %w", c.Env, err)))
	}

	source := upstreamEnvName
	if upstreamLatest {
		source = "latest"
	}

	apps, overrideVersions, err := c.Parent.getUpstreamLatestApp(ctx, transaction, upstreamLatest, state, upstreamEnvName, source, c.Parent.CommitHash, c.Env)
	if err != nil {
		return failedPrognosis(err)
	}
	sort.Strings(apps)

	appsPrognoses := make(map[string]ReleaseTrainApplicationPrognosis)
	if len(envLocks) > 0 {
		locksList := []*api.Lock{}
		sortedKeys := sorting.SortKeys(envLocks)
		for _, lockId := range sortedKeys {
			newLock := &api.Lock{
				Message:   envLocks[lockId].Message,
				CreatedAt: timestamppb.New(envLocks[lockId].CreatedAt),
				CreatedBy: &api.Actor{
					Email: envLocks[lockId].CreatedBy.Email,
					Name:  envLocks[lockId].CreatedBy.Name,
				},
				LockId:            lockId,
				CiLink:            envLocks[lockId].CiLink,
				SuggestedLifetime: envLocks[lockId].SuggestedLifetime,
			}
			locksList = append(locksList, newLock)
		}

		for _, appName := range apps {
			appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
				SkipCause: nil,
				Locks:     locksList,
				Version:   0,
				Team:      "",
			}
		}
		return &ReleaseTrainEnvironmentPrognosis{
			SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
				SkipCause: api.ReleaseTrainEnvSkipCause_ENV_IS_LOCKED,
			},
			Error:         nil,
			Locks:         locksList,
			AppsPrognoses: appsPrognoses,
		}
	}
	allLatestDeploymentsTargetEnv, err := state.GetAllLatestDeployments(ctx, transaction, c.Env, apps)
	if err != nil {
		return failedPrognosis(grpc.PublicError(ctx, fmt.Errorf("Could not obtain latest deployments for env %s: %w", c.Env, err)))
	}

	allLatestDeploymentsUpstreamEnv, err := state.GetAllLatestDeployments(ctx, transaction, upstreamEnvName, apps)

	if err != nil {
		return failedPrognosis(grpc.PublicError(ctx, fmt.Errorf("Could not obtain latest deployments for env %s: %w", c.Env, err)))
	}

	var allLatestReleaseEnvironments db.AppVersionManifests
	if c.AllLatestReleaseEnvironments == nil {
		allLatestReleaseEnvironments, err = state.DBHandler.DBSelectAllManifestsForAllReleases(ctx, transaction)
		if err != nil {
			return failedPrognosis(grpc.PublicError(ctx, fmt.Errorf("Error getting all releases of all apps: %w", err)))
		}
	} else {
		allLatestReleaseEnvironments = c.AllLatestReleaseEnvironments
	}
	span.SetTag("ConsideredApps", len(apps))
	allTeams, err := state.GetAllApplicationsTeamOwner(ctx, transaction)
	if err != nil {
		return failedPrognosis(err)
	}
	for _, appName := range apps {
		if c.Parent.Team != "" {
			team, ok := allTeams[appName]
			if !ok {
				// If we cannot find the app in all teams, we cannot determine the team of the app.
				// This indicates an incorrect db state, and we just skip it:
				appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
					SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
						SkipCause: api.ReleaseTrainAppSkipCause_APP_WITHOUT_TEAM,
					},
					Locks:   nil,
					Version: 0,
					Team:    team,
				}
				continue
			}
			// If we found the team name, but it's not the given team,
			// then it's not really worthy of "SkipCause", because we shouldn't even consider this app.
			if c.Parent.Team != team {
				continue
			}
		}
		currentlyDeployedVersion := allLatestDeploymentsTargetEnv[appName]

		var versionToDeploy uint64
		if overrideVersions != nil {
			for _, override := range overrideVersions {
				if override.App == appName {
					versionToDeploy = override.Version
				}
			}
		} else if upstreamLatest {
			l := len(allLatestReleases[appName])
			if allLatestReleases == nil || allLatestReleases[appName] == nil || l == 0 {
				logger.FromContext(ctx).Sugar().Warnf("app %s appears to have no releases on env=%s, so it is skipped", appName, c.Env)
				continue
			}
			versionToDeploy = uint64(allLatestReleases[appName][int(math.Max(0, float64(l-1)))])
		} else {
			upstreamVersion := allLatestDeploymentsUpstreamEnv[appName]

			if upstreamVersion == nil {
				appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
					SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
						SkipCause: api.ReleaseTrainAppSkipCause_APP_HAS_NO_VERSION_IN_UPSTREAM_ENV,
					},
					Locks:   nil,
					Version: 0,
					Team:    "",
				}
				continue
			}
			versionToDeploy = uint64(*upstreamVersion)
		}
		if currentlyDeployedVersion != nil && *currentlyDeployedVersion == int64(versionToDeploy) {
			appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
				SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
					SkipCause: api.ReleaseTrainAppSkipCause_APP_ALREADY_IN_UPSTREAM_VERSION,
				},
				Locks:   nil,
				Version: 0,
				Team:    "",
			}
			continue
		}

		appLocks, err := state.GetEnvironmentApplicationLocks(ctx, transaction, c.Env, appName)

		if err != nil {
			return failedPrognosis(err)
		}

		if len(appLocks) > 0 {
			locksList := []*api.Lock{}
			sortedKeys := sorting.SortKeys(appLocks)
			for _, lockId := range sortedKeys {
				newLock := &api.Lock{
					Message:   appLocks[lockId].Message,
					CreatedAt: timestamppb.New(appLocks[lockId].CreatedAt),
					CreatedBy: &api.Actor{
						Email: appLocks[lockId].CreatedBy.Email,
						Name:  appLocks[lockId].CreatedBy.Name,
					},
					LockId:            lockId,
					CiLink:            appLocks[lockId].CiLink,
					SuggestedLifetime: appLocks[lockId].SuggestedLifetime,
				}
				locksList = append(locksList, newLock)
			}

			appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
				SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
					SkipCause: api.ReleaseTrainAppSkipCause_APP_IS_LOCKED,
				},
				Locks:   locksList,
				Version: 0,
				Team:    "",
			}
			continue
		}

		releaseEnvs, exists := allLatestReleaseEnvironments[appName][versionToDeploy]
		if !exists {
			return failedPrognosis(fmt.Errorf("No release found for app %s and versionToDeploy %d", appName, versionToDeploy))
		}

		found := false
		for _, env := range releaseEnvs {
			if env == c.Env {
				found = true
				break
			}
		}
		if !found {
			appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
				SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
					SkipCause: api.ReleaseTrainAppSkipCause_APP_DOES_NOT_EXIST_IN_ENV,
				},
				Locks:   nil,
				Version: 0,
				Team:    "",
			}
			continue
		}

		teamName, ok := allTeams[appName]

		if ok { //IF we find information for team
			envConfig, ok := c.EnvConfigs[c.Env]
			if !ok {
				err = state.checkUserPermissions(ctx, transaction, c.Env, "*", auth.PermissionDeployReleaseTrain, teamName, c.Parent.RBACConfig, true)
			} else {
				err = state.checkUserPermissionsFromConfig(ctx, transaction, c.Env, "*", auth.PermissionDeployReleaseTrain, teamName, c.Parent.RBACConfig, true, &envConfig)
			}

			if err != nil {
				appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
					SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
						SkipCause: api.ReleaseTrainAppSkipCause_NO_TEAM_PERMISSION,
					},
					Locks:   nil,
					Version: 0,
					Team:    teamName,
				}
				continue
			}

			teamLocks, err := state.GetEnvironmentTeamLocks(ctx, transaction, c.Env, teamName)

			if err != nil {
				return failedPrognosis(err)
			}

			if len(teamLocks) > 0 {
				locksList := []*api.Lock{}
				sortedKeys := sorting.SortKeys(teamLocks)
				for _, lockId := range sortedKeys {
					newLock := &api.Lock{
						Message:   teamLocks[lockId].Message,
						CreatedAt: timestamppb.New(teamLocks[lockId].CreatedAt),
						CreatedBy: &api.Actor{
							Email: teamLocks[lockId].CreatedBy.Email,
							Name:  teamLocks[lockId].CreatedBy.Name,
						},
						LockId:            lockId,
						CiLink:            teamLocks[lockId].CiLink,
						SuggestedLifetime: teamLocks[lockId].SuggestedLifetime,
					}
					locksList = append(locksList, newLock)

				}

				appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
					SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
						SkipCause: api.ReleaseTrainAppSkipCause_TEAM_IS_LOCKED,
					},
					Locks:   locksList,
					Version: 0,
					Team:    teamName,
				}
				continue
			}
		}
		appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
			SkipCause: nil,
			Locks:     nil,
			Version:   versionToDeploy,
			Team:      "",
		}
	}
	return &ReleaseTrainEnvironmentPrognosis{
		SkipCause:     nil,
		Error:         nil,
		Locks:         nil,
		AppsPrognoses: appsPrognoses,
	}
}

func (c *envReleaseTrain) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "EnvReleaseTrain Transform")
	defer span.Finish()

	prognosis := c.prognosis(ctx, state, transaction, c.AllLatestReleasesCache)

	return c.applyPrognosis(ctx, state, t, transaction, prognosis, span)
}

func (c *envReleaseTrain) applyPrognosis(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
	prognosis *ReleaseTrainEnvironmentPrognosis,
	span tracer.Span,
) (string, error) {
	allApps, err := state.GetApplications(ctx, transaction)
	if err != nil {
		return "", err
	}
	allLatestDeployments, err := state.GetAllLatestDeployments(ctx, transaction, c.Env, allApps)
	if err != nil {
		return "", err
	}

	renderApplicationSkipCause := c.renderApplicationSkipCause(ctx, allLatestDeployments)
	renderEnvironmentSkipCause := c.renderEnvironmentSkipCause()

	if prognosis.Error != nil {
		return "", prognosis.Error
	}
	if prognosis.SkipCause != nil {
		if !c.WriteCommitData {
			return renderEnvironmentSkipCause(prognosis.SkipCause), nil
		}
		for appName := range prognosis.AppsPrognoses {
			releases := c.AllLatestReleasesCache[appName]
			var release uint64
			if releases == nil {
				release = 0
			} else {
				release = uint64(releases[len(releases)-1])
			}
			eventMessage := ""
			if len(prognosis.Locks) > 0 {
				eventMessage = prognosis.Locks[0].Message
			}
			newEvent := createLockPreventedDeploymentEvent(appName, c.Env, eventMessage, "environment")
			commitID, err := getCommitID(ctx, transaction, state, release, appName)
			if err != nil {
				logger.FromContext(ctx).Sugar().Warnf("could not write event data - continuing. %v", fmt.Errorf("getCommitIDFromReleaseDir %v", err))
			} else {
				gen := getGenerator(ctx)
				eventUuid := gen.Generate()
				err = state.DBHandler.DBWriteLockPreventedDeploymentEvent(ctx, transaction, c.TransformerEslVersion, eventUuid, commitID, newEvent)
				if err != nil {
					return "", GetCreateReleaseGeneralFailure(err)
				}
			}
		}

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

	span.SetTag("ConsideredApps", len(appNames))
	var deployCounter uint = 0
	for _, appName := range appNames {
		appPrognosis := prognosis.AppsPrognoses[appName]
		if appPrognosis.SkipCause != nil {
			skipped = append(skipped, renderApplicationSkipCause(&appPrognosis, appName))
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
			Author:                "",
			TransformerEslVersion: c.TransformerEslVersion,
			CiLink:                c.CiLink,
			SkipCleanup:           true,
		}
		if err := t.Execute(ctx, d, transaction); err != nil {
			return "", grpc.InternalError(ctx, fmt.Errorf("unexpected error while deploying app %q to env %q: %w", appName, c.Env, err))
		}
		deployCounter++
	}
	span.SetTag("DeployedApps", deployCounter)

	teamInfo := ""
	if c.Parent.Team != "" {
		teamInfo = " for team '" + c.Parent.Team + "'"
	}
	if err := t.Execute(ctx, &skippedServices{
		Messages:              skipped,
		TransformerEslVersion: c.TransformerEslVersion,
	}, transaction); err != nil {
		return "", err
	}
	deployedApps := 0
	for _, checker := range prognosis.AppsPrognoses {
		if checker.SkipCause != nil {
			deployedApps += 1
		}
	}

	return fmt.Sprintf("Release Train to '%s' environment:\n\n"+
		"The release train deployed %d services from '%s' to '%s'%s",
		c.Env, deployedApps, source, c.Env, teamInfo,
	), nil
}

func (c *envReleaseTrain) renderEnvironmentSkipCause() func(SkipCause *api.ReleaseTrainEnvPrognosis_SkipCause) string {
	return func(SkipCause *api.ReleaseTrainEnvPrognosis_SkipCause) string {
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
}

func (c *envReleaseTrain) renderApplicationSkipCause(
	_ context.Context,
	allLatestDeployments map[string]*int64,
) func(Prognosis *ReleaseTrainApplicationPrognosis, appName string) string {
	return func(Prognosis *ReleaseTrainApplicationPrognosis, appName string) string {
		envConfig := c.EnvGroupConfigs[c.Env]
		upstreamEnvName := envConfig.Upstream.Environment
		var currentlyDeployedVersion uint64
		if latestDeploymentVersion, found := allLatestDeployments[appName]; found && latestDeploymentVersion != nil {
			currentlyDeployedVersion = uint64(*latestDeploymentVersion)
		}
		switch Prognosis.SkipCause.SkipCause {
		case api.ReleaseTrainAppSkipCause_APP_HAS_NO_VERSION_IN_UPSTREAM_ENV:
			return fmt.Sprintf("skipping because there is no version for application %q in env %q \n", appName, upstreamEnvName)
		case api.ReleaseTrainAppSkipCause_APP_ALREADY_IN_UPSTREAM_VERSION:
			return fmt.Sprintf("skipping %q because it is already in the version %d\n", appName, currentlyDeployedVersion)
		case api.ReleaseTrainAppSkipCause_APP_IS_LOCKED:
			return fmt.Sprintf("skipping application %q in environment %q due to application lock", appName, c.Env)
		case api.ReleaseTrainAppSkipCause_APP_DOES_NOT_EXIST_IN_ENV:
			return fmt.Sprintf("skipping application %q in environment %q because it doesn't exist there", appName, c.Env)
		case api.ReleaseTrainAppSkipCause_TEAM_IS_LOCKED:
			return fmt.Sprintf("skipping application %q in environment %q due to team lock on team %q", appName, c.Env, Prognosis.Team)
		case api.ReleaseTrainAppSkipCause_NO_TEAM_PERMISSION:
			return fmt.Sprintf("skipping application %q in environment %q because the user team %q is not the same as the apllication", appName, c.Env, Prognosis.Team)
		default:
			return fmt.Sprintf("skipping application %q in environment %q for an unrecognized reason", appName, c.Env)
		}
	}
}

// skippedServices is a helper Transformer to generate the "skipped
// services" commit log.
type skippedServices struct {
	Messages              []string
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (c *skippedServices) GetDBEventType() db.EventType {
	return db.EvtSkippedServices
}

func (c *skippedServices) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *skippedServices) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *skippedServices) Transform(
	ctx context.Context,
	_ *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	if len(c.Messages) == 0 {
		return "", nil
	}
	for _, msg := range c.Messages {
		if err := t.Execute(ctx, &skippedService{Message: msg, TransformerEslVersion: c.TransformerEslVersion}, transaction); err != nil {
			return "", err
		}
	}
	return "Skipped services", nil
}

type skippedService struct {
	Message               string
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (c *skippedService) GetDBEventType() db.EventType {
	return db.EvtSkippedServices
}

func (c *skippedService) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *skippedService) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *skippedService) Transform(_ context.Context, _ *State, _ TransformerContext, _ *sql.Tx) (string, error) {
	return c.Message, nil
}
