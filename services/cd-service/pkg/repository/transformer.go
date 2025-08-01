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
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"github.com/freiheit-com/kuberpult/pkg/types"

	"github.com/google/go-cmp/cmp"

	config "github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/mapper"
	"github.com/freiheit-com/kuberpult/pkg/sorting"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/DataDog/datadog-go/v5/statsd"
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
)

const (
	yamlParsingError = "# yaml parsing error"
	// number of old releases that will ALWAYS be kept in addition to the ones that are deployed:
	keptVersionsOnCleanup = 20
)

func (s *State) GetEnvironmentLocksCount(ctx context.Context, transaction *sql.Tx, env types.EnvName) (float64, error) {
	locks, err := s.GetEnvironmentLocksFromDB(ctx, transaction, env)
	if err != nil {
		return -1, err
	}
	return float64(len(locks)), nil
}

func (s *State) GetEnvironmentApplicationLocksCount(ctx context.Context, transaction *sql.Tx, environment types.EnvName, application string) (float64, error) {
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
			logger.FromContext(ctx).Sugar().Warnf("error getting number of unsynced apps: %v", err)
			return err
		}
		numberSyncFailedApps, err := s.DBHandler.DBCountAppsWithStatus(ctx, transaction, db.SYNC_FAILED)
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("error getting number of sync failed apps: %v", err)
			return err
		}
		err = MeasureGitSyncStatus(numberUnsyncedApps, numberSyncFailedApps)
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("could not send git sync status metrics to datadog: %v", err)
			return err
		}

	}
	return nil
}

func GaugeEnvLockMetric(ctx context.Context, s *State, transaction *sql.Tx, env types.EnvName) {
	if ddMetrics != nil {
		count, err := s.GetEnvironmentLocksCount(ctx, transaction, env)
		if err != nil {
			logger.FromContext(ctx).
				Sugar().
				Warnf("Error when trying to get the number of environment locks: %w", err)
			return
		}
		err = ddMetrics.Gauge("environment_lock_count", count, []string{"kuberpult_environment:" + string(env)}, 1)
		if err != nil {
			logger.FromContext(ctx).
				Sugar().
				Warnf("Error when trying to send `environment_lock_count` metric to datadog: %w", err)
		}
	}
}
func GaugeEnvAppLockMetric(ctx context.Context, appEnvLocksCount int, env types.EnvName, app string) {
	if ddMetrics != nil {
		err := ddMetrics.Gauge("application_lock_count", float64(appEnvLocksCount), []string{"kuberpult_environment:" + string(env), "kuberpult_application:" + app}, 1)
		if err != nil {
			logger.FromContext(ctx).
				Sugar().
				Warnf("Error when trying to send `application_lock_count` metric to datadog: %w", err)
		}
	}
}

func GaugeDeploymentMetric(_ context.Context, env types.EnvName, app string, timeInMinutes float64) error {
	if ddMetrics != nil {
		// store the time since the last deployment in minutes:
		err := ddMetrics.Gauge(
			"lastDeployed",
			timeInMinutes,
			[]string{metrics.EventTagApplication + ":" + app, metrics.EventTagEnvironment + ":" + string(env)},
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
					"kuberpult.environment:" + string(oneChange.Env),
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
	AddAppEnv(app string, env types.EnvName, team string)
	DeleteEnvFromApp(app string, env types.EnvName)
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

func (r *transformerRunner) AddAppEnv(app string, env types.EnvName, team string) {
	r.ChangedApps = append(r.ChangedApps, AppEnv{
		App:  app,
		Env:  env,
		Team: team,
	})
}

func (r *transformerRunner) DeleteEnvFromApp(app string, env types.EnvName) {
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
	Authentication                 `json:"-"`
	Version                        uint64                   `json:"version"`
	Application                    string                   `json:"app"`
	Manifests                      map[types.EnvName]string `json:"manifests"`
	SourceCommitId                 string                   `json:"sourceCommitId"`
	SourceAuthor                   string                   `json:"sourceCommitAuthor"`
	SourceMessage                  string                   `json:"sourceCommitMessage"`
	Team                           string                   `json:"team"`
	DisplayVersion                 string                   `json:"displayVersion"`
	WriteCommitData                bool                     `json:"writeCommitData"`
	PreviousCommit                 string                   `json:"previousCommit"`
	CiLink                         string                   `json:"ciLink"`
	AllowedDomains                 []string                 `json:"-"`
	TransformerEslVersion          db.TransformerID         `json:"-"`
	IsPrepublish                   bool                     `json:"isPrepublish"`
	DeployToDownstreamEnvironments []types.EnvName          `json:"deployToDownstreamEnvironments"`
	Revision                       uint64                   `json:"revision"`
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

func (s *State) GetLastRelease(ctx context.Context, transaction *sql.Tx, application string) (types.ReleaseNumbers, error) {
	releases, err := s.DBHandler.DBSelectAllReleasesOfApp(ctx, transaction, application)
	if err != nil {
		return types.MakeEmptyReleaseNumbers(), fmt.Errorf("could not get releases of app %s: %v", application, err)
	}
	if len(releases) == 0 {
		return types.MakeEmptyReleaseNumbers(), nil
	}
	l := len(releases)
	return releases[l-1], nil
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
		return "", GetCreateReleaseGeneralFailure(fmt.Errorf("source commit ID is not a valid SHA1 hash, should be exactly 40 characters [0-9a-fA-F], but is %d characters: '%s'", len(c.SourceCommitId), c.SourceCommitId))
	}
	if c.PreviousCommit != "" && !valid.SHA1CommitID(c.PreviousCommit) {
		logger.FromContext(ctx).Sugar().Warnf("Previous commit ID %s is invalid", c.PreviousCommit)
	}

	configs, err := state.GetAllEnvironmentConfigs(ctx, transaction)
	if err != nil {
		if errors.Is(err, ErrInvalidJson) {
			return "", err
		}
		return "", GetCreateReleaseGeneralFailure(err)
	}

	if c.CiLink != "" && !isValidLink(c.CiLink, c.AllowedDomains) {
		return "", GetCreateReleaseGeneralFailure(fmt.Errorf("provided CI Link: %s is not valid or does not match any of the allowed domain", c.CiLink))
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

	var allEnvsOfThisApp []types.EnvName = nil

	for env := range c.Manifests {
		allEnvsOfThisApp = append(allEnvsOfThisApp, types.EnvName(env))
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
	sortedEnvs := sorting.SortKeys(c.Manifests)
	if errDownstream := validateDownstreamEnvs(c.DeployToDownstreamEnvironments, sortedEnvs, configs); errDownstream != nil {
		return "", errDownstream
	}

	isMinor, err := c.checkMinorFlags(ctx, transaction, state.DBHandler, version, state.MinorRegexes)
	if err != nil {
		return "", err
	}
	manifestsToKeep := c.Manifests
	if c.IsPrepublish {
		manifestsToKeep = make(map[types.EnvName]string)
	}
	now, err := state.DBHandler.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return "", GetCreateReleaseGeneralFailure(fmt.Errorf("could not get transaction timestamp"))
	}
	release := db.DBReleaseWithMetaData{
		ReleaseNumbers: types.ReleaseNumbers{
			Revision: c.Revision,
			Version:  version.Version,
		},
		App: c.Application,
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
		Environments: []types.EnvName{},
		Created:      *now,
	}
	err = state.DBHandler.DBUpdateOrCreateRelease(ctx, transaction, release)
	if err != nil {
		return "", GetCreateReleaseGeneralFailure(err)
	}

	for i := range sortedEnvs {
		env := sortedEnvs[i]
		err = state.DBHandler.DBAppendAppToEnvironment(ctx, transaction, env, c.Application)
		if err != nil {
			return "", grpc.PublicError(ctx, err)
		}

		config := configs[env]
		hasUpstream := false
		if config.Upstream != nil {
			hasUpstream = true
		}
		err = state.checkUserPermissionsFromConfig(ctx,
			transaction,
			env,
			c.Application,
			auth.PermissionCreateRelease,
			c.Team,
			c.RBACConfig,
			true,
			&config,
		)
		if err != nil {
			return "", err
		}

		teamOwner, err := state.GetApplicationTeamOwner(ctx, transaction, c.Application)
		if err != nil {
			return "", err
		}
		t.AddAppEnv(c.Application, env, teamOwner)
		if ((hasUpstream && config.Upstream.Latest) || slices.Contains(c.DeployToDownstreamEnvironments, env)) && isLatest && !c.IsPrepublish {
			d := &DeployApplicationVersion{
				SourceTrain:           nil,
				Environment:           env,
				Application:           c.Application,
				Version:               *version.Version, // the train should queue deployments, instead of giving up:
				Revision:              c.Revision,
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

func validateDownstreamEnvs(downstreamEnvs []types.EnvName, sortedEnvs []types.EnvName, configs map[types.EnvName]config.EnvironmentConfig) error {
	notDownstreamEnvs := []types.EnvName{}
	missingManifestEnvs := []types.EnvName{}
	for i := range downstreamEnvs {
		downEnv := downstreamEnvs[i]
		if !slices.Contains(sortedEnvs, downEnv) {
			missingManifestEnvs = append(missingManifestEnvs, downEnv)
		}
		config := configs[downEnv]
		if config.Upstream != nil && config.Upstream.Latest {
			notDownstreamEnvs = append(notDownstreamEnvs, downEnv)
		}
	}
	if len(missingManifestEnvs) > 0 {
		return GetCreateReleaseMissingManifest(missingManifestEnvs)
	}
	if len(notDownstreamEnvs) > 0 {
		return GetCreateReleaseIsNoDownstream(notDownstreamEnvs)
	}
	return nil
}

func (c *CreateApplicationVersion) checkMinorFlags(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler, version types.ReleaseNumbers, minorRegexes []*regexp.Regexp) (bool, error) {
	releaseVersions, err := dbHandler.DBSelectAllReleasesOfApp(ctx, transaction, c.Application)
	if err != nil {
		return false, err
	}
	if releaseVersions == nil {
		return false, nil
	}
	sort.Slice(releaseVersions, func(i, j int) bool {
		return *releaseVersions[i].Version > *releaseVersions[j].Version || (*releaseVersions[i].Version == *releaseVersions[j].Version && releaseVersions[i].Revision > releaseVersions[j].Revision)
	})
	nextVersion := types.MakeEmptyReleaseNumbers()
	previousVersion := types.MakeEmptyReleaseNumbers()
	for i := len(releaseVersions) - 1; i >= 0; i-- {
		if types.Greater(releaseVersions[i], version) {
			nextVersion = releaseVersions[i]
			break
		}
	}
	for i := 0; i < len(releaseVersions); i++ {
		if types.Greater(version, releaseVersions[i]) {
			previousVersion = releaseVersions[i]
			break
		}
	}
	if nextVersion.Version != nil {
		nextRelease, err := dbHandler.DBSelectReleaseByVersion(ctx, transaction, c.Application, nextVersion, true)
		if err != nil {
			return false, err
		}
		if nextRelease == nil {
			return false, fmt.Errorf("next release (%d) exists in the all releases but not in the release table", nextVersion)
		}
		nextRelease.Metadata.IsMinor = compareManifests(ctx, c.Manifests, nextRelease.Manifests.Manifests, minorRegexes)
		err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, *nextRelease)
		if err != nil {
			return false, err
		}
	}
	if previousVersion.Version == nil {
		return false, nil
	}
	previousRelease, err := dbHandler.DBSelectReleaseByVersion(ctx, transaction, c.Application, previousVersion, true)
	if err != nil {
		return false, err
	}
	return compareManifests(ctx, c.Manifests, previousRelease.Manifests.Manifests, minorRegexes), nil
}

func compareManifests(ctx context.Context, firstManifests map[types.EnvName]string, secondManifests map[types.EnvName]string, comparisonRegexes []*regexp.Regexp) bool {
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

func writeCommitData(ctx context.Context, h *db.DBHandler, transaction *sql.Tx, releaseVersion types.ReleaseNumbers, transformerEslVersion db.TransformerID, sourceCommitId string, sourceMessage string, app string, eventId string, environments []types.EnvName, previousCommitId string, state *State) error {
	if !valid.SHA1CommitID(sourceCommitId) {
		return nil
	}

	envMap := make(map[string]struct{}, len(environments))
	for _, env := range environments {
		envMap[string(env)] = struct{}{}
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

func (c *CreateApplicationVersion) calculateVersion(ctx context.Context, transaction *sql.Tx, state *State) (types.ReleaseNumbers, error) {
	if c.Version == 0 {
		return types.MakeEmptyReleaseNumbers(), fmt.Errorf("version is required when using the database")
	} else {
		metaData, err := state.DBHandler.DBSelectReleaseByReleaseNumbers(ctx, transaction, c.Application, types.ReleaseNumbers{Version: &c.Version, Revision: c.Revision}, true)
		if err != nil {
			return types.MakeEmptyReleaseNumbers(), fmt.Errorf("could not calculate version, error: %v", err)
		}
		if metaData == nil {
			logger.FromContext(ctx).Sugar().Infof("could not calculate version, no metadata on app %s with version %v.%v", c.Application, c.Version, c.Revision)
			return types.ReleaseNumbers{Version: &c.Version, Revision: c.Revision}, nil
		}
		logger.FromContext(ctx).Sugar().Warnf("release exists already %v: %v", metaData.ReleaseNumbers, metaData)

		existingRelease := metaData.ReleaseNumbers
		logger.FromContext(ctx).Sugar().Warnf("comparing release %v.%v: %v", c.Version, c.Revision, existingRelease)
		// check if version differs, if it's the same, that's ok
		return types.MakeEmptyReleaseNumbers(), c.sameAsExistingDB(ctx, transaction, state, metaData)
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
	metaData, err := state.DBHandler.DBSelectReleaseByVersion(ctx, transaction, c.Application, types.ReleaseNumbers{Version: &c.Version, Revision: c.Revision}, true)
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
		existingManStr := metaData.Manifests.Manifests[types.EnvName(env)]
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

func isLatestVersion(ctx context.Context, transaction *sql.Tx, state *State, application string, version types.ReleaseNumbers) (bool, error) {
	rels, err := state.DBHandler.DBSelectAllReleasesOfApp(ctx, transaction, application)
	if err != nil {
		return false, err
	}

	for _, r := range rels {
		if types.Greater(r, version) {
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
	if lastRelease.Version == nil {
		return "", fmt.Errorf("cannot undeploy non-existing application '%v'", c.Application)
	}
	now, err := state.DBHandler.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return "", fmt.Errorf("could not get transaction timestamp")
	}
	envManifests := make(map[types.EnvName]string)
	envs := []types.EnvName{}
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
	v := uint64(*lastRelease.Version + 1)
	release := db.DBReleaseWithMetaData{
		ReleaseNumbers: types.ReleaseNumbers{
			Revision: 0, //Undeploy versions always have revision 0
			Version:  &v,
		},
		App: c.Application,
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
				Version:     *lastRelease.Version + 1,
				// the train should queue deployments, instead of giving up:
				LockBehaviour:         api.LockBehavior_RECORD,
				Authentication:        c.Authentication,
				WriteCommitData:       c.WriteCommitData,
				Author:                "",
				TransformerEslVersion: c.TransformerEslVersion,
				CiLink:                "",
				SkipCleanup:           false,
				Revision:              0,
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
	return fmt.Sprintf("created undeploy-version %d of '%v'", *lastRelease.Version+1, c.Application), nil
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
	if lastRelease.Version == nil {
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
		if deployment != nil && deployment.ReleaseNumbers.Version != nil {
			release, err := state.DBHandler.DBSelectReleaseByVersion(ctx, transaction, u.Application, deployment.ReleaseNumbers, true)
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
			deployment.ReleaseNumbers.Version = nil
			deployment.Metadata.DeployedByName = user.Name
			deployment.Metadata.DeployedByEmail = user.Email
			err = state.DBHandler.DBUpdateOrCreateDeployment(ctx, transaction, *deployment)
			if err != nil {
				return "", err
			}
		}
		if deployment == nil || deployment.ReleaseNumbers.Version == nil || isUndeploy {
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
	Environment           types.EnvName    `json:"env"`
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
	envName := types.EnvName(u.Environment)
	err := state.checkUserPermissions(ctx, transaction, envName, u.Application, auth.PermissionDeleteEnvironmentApplication, "", u.RBACConfig, true)
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
		newManifests := make(map[types.EnvName]string)
		for e, manifest := range dbReleaseWithMetadata.Manifests.Manifests {
			if e != envName {
				newManifests[e] = manifest
			}
		}

		newRelease := db.DBReleaseWithMetaData{
			ReleaseNumbers: types.ReleaseNumbers{
				Revision: dbReleaseWithMetadata.ReleaseNumbers.Revision,
				Version:  dbReleaseWithMetadata.ReleaseNumbers.Version,
			},
			App:          dbReleaseWithMetadata.App,
			Created:      *now,
			Manifests:    db.DBReleaseManifests{Manifests: newManifests},
			Metadata:     dbReleaseWithMetadata.Metadata,
			Environments: []types.EnvName{},
		}
		err = state.DBHandler.DBUpdateOrCreateRelease(ctx, transaction, newRelease)
		if err != nil {
			return "", err
		}
	}

	err = state.DBHandler.DBRemoveAppFromEnvironment(ctx, transaction, envName, u.Application)
	if err != nil {
		return "", fmt.Errorf("couldn't write environment '%s' into environments table, error: %w", u.Environment, err)
	}
	t.DeleteEnvFromApp(u.Application, envName)
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
func findOldApplicationVersions(ctx context.Context, transaction *sql.Tx, state *State, name string) ([]types.ReleaseNumbers, error) {
	// 1) get release in each env:
	versions, err := state.GetAllApplicationReleases(ctx, transaction, name)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, err
	}
	sort.Slice(versions, func(i, j int) bool {
		return types.Greater(versions[j], versions[i])
	})

	deployments, err := state.GetAllDeploymentsForAppFromDB(ctx, transaction, name)
	if err != nil {
		return nil, err
	}

	var oldestDeployedVersion types.ReleaseNumbers
	deployedVersions := []types.ReleaseNumbers{}
	for _, version := range deployments {
		deployedVersions = append(deployedVersions, version)
	}

	if len(deployedVersions) == 0 {
		// Use the latest version as oldest deployed version
		oldestDeployedVersion = versions[len(versions)-1]
	} else {
		oldestDeployedVersion = slices.MinFunc(deployedVersions, types.CompareReleaseNumbers)
	}

	positionOfOldestVersion := sort.Search(len(versions), func(i int) bool {
		return types.GreaterOrEqual(versions[i], oldestDeployedVersion)
	})

	if positionOfOldestVersion < (int(state.ReleaseVersionsLimit) - 1) {
		return nil, nil
	}
	return versions[0 : positionOfOldestVersion-(int(state.ReleaseVersionsLimit)-1)], err
}

func (c *CleanupOldApplicationVersions) Transform(
	ctx context.Context,
	state *State,
	_ TransformerContext,
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
	// we only clean up non-deployed versions, so there are no changes for argoCd here
	return msg, nil
}

type Authentication struct {
	RBACConfig auth.RBACConfig
}

type CreateEnvironmentLock struct {
	Authentication        `json:"-"`
	Environment           types.EnvName    `json:"env"`
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

func (s *State) checkUserPermissionsFromConfig(ctx context.Context, transaction *sql.Tx, env types.EnvName, application, action, team string, RBACConfig auth.RBACConfig, checkTeam bool, config *config.EnvironmentConfig) error {
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
			team, err = s.GetApplicationTeamOwner(ctx, transaction, application)

			if err != nil {
				return err
			}
		}
		err = auth.CheckUserTeamPermissions(RBACConfig, user, team, action)
	}
	return err
}

func (s *State) checkUserPermissions(ctx context.Context, transaction *sql.Tx, env types.EnvName, application, action, team string, RBACConfig auth.RBACConfig, checkTeam bool) error {
	cfg, err := s.GetEnvironmentConfigFromDB(ctx, transaction, env)
	if err != nil {
		return err
	}
	return s.checkUserPermissionsFromConfig(ctx, transaction, env, application, action, team, RBACConfig, checkTeam, cfg)
}

// CheckUserPermissionsCreateEnvironment checks the permission for the environment creation action.
// This is a "special" case because the environment group is already provided on the request.
func CheckUserPermissionsCreateEnvironment(ctx context.Context, RBACConfig auth.RBACConfig, envConfig config.EnvironmentConfig) error {
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
	envName := types.EnvName(c.Environment)
	err := state.checkUserPermissions(ctx, transaction, envName, "*", auth.PermissionCreateLock, "", c.RBACConfig, false)
	if err != nil {
		return "", err
	}

	if c.CiLink != "" && !isValidLink(c.CiLink, c.AllowedDomains) {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("provided CI Link: %s is not valid or does not match any of the allowed domain", c.CiLink))
	}
	if c.SuggestedLifeTime != nil && *c.SuggestedLifeTime != "" && !isValidLifeTime(*c.SuggestedLifeTime) {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("uggested life time: %s is not a valid lifetime. It should be a number followed by h, d or w", *c.SuggestedLifeTime))
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

	errW := state.DBHandler.DBWriteEnvironmentLock(ctx, transaction, c.LockId, envName, metadata)
	if errW != nil {
		return "", errW
	}

	//Add it to all locks
	allEnvLocks, err := state.DBHandler.DBSelectAllEnvironmentLocks(ctx, transaction, envName)
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
		err := state.DBHandler.DBWriteAllEnvironmentLocks(ctx, transaction, allEnvLocks.Version, envName, allEnvLocks.EnvLocks)
		if err != nil {
			return "", err
		}
	}
	GaugeEnvLockMetric(ctx, state, transaction, envName)
	return fmt.Sprintf("Created lock %q on environment %q", c.LockId, c.Environment), nil
}

type DeleteEnvironmentLock struct {
	Authentication        `json:"-"`
	Environment           types.EnvName    `json:"env"`
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
	envName := types.EnvName(c.Environment)
	err := state.checkUserPermissions(ctx, transaction, envName, "*", auth.PermissionDeleteLock, "", c.RBACConfig, false)
	if err != nil {
		return "", err
	}
	s := State{
		MinorRegexes:              state.MinorRegexes,
		MaxNumThreads:             state.MaxNumThreads,
		DBHandler:                 state.DBHandler,
		ReleaseVersionsLimit:      state.ReleaseVersionsLimit,
		ParallelismOneTransaction: state.ParallelismOneTransaction,
	}
	err = s.DBHandler.DBDeleteEnvironmentLock(ctx, transaction, envName, c.LockId)
	if err != nil {
		return "", err
	}
	allEnvLocks, err := state.DBHandler.DBSelectAllEnvironmentLocks(ctx, transaction, envName)
	if err != nil {
		return "", fmt.Errorf("DeleteEnvironmentLock: could not select all env locks '%v': '%w'", envName, err)
	}
	var locks []string
	if allEnvLocks != nil {
		locks = db.Remove(allEnvLocks.EnvLocks, c.LockId)
		err = state.DBHandler.DBWriteAllEnvironmentLocks(ctx, transaction, allEnvLocks.Version, envName, locks)
		if err != nil {
			return "", fmt.Errorf("DeleteEnvironmentLock: could not write env locks '%v': '%w'", c.Environment, err)
		}
	}

	additionalMessageFromDeployment, err := s.ProcessQueueAllApps(ctx, transaction, envName)
	if err != nil {
		return "", err
	}
	GaugeEnvLockMetric(ctx, state, transaction, envName)
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
	err := state.checkUserPermissions(ctx, transaction, types.EnvName(c.EnvironmentGroup), "*", auth.PermissionCreateLock, "", c.RBACConfig, false)
	if err != nil {
		return "", err
	}
	if c.CiLink != "" && !isValidLink(c.CiLink, c.AllowedDomains) {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("provided CI Link: %s is not valid or does not match any of the allowed domain", c.CiLink))
	}
	if c.SuggestedLifeTime != nil && *c.SuggestedLifeTime != "" && !isValidLifeTime(*c.SuggestedLifeTime) {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("suggested life time: %s is not a valid lifetime. It should be a number followed by h, d or w", *c.SuggestedLifeTime))
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

	err := state.checkUserPermissions(ctx, transaction, types.EnvName(c.EnvironmentGroup), "*", auth.PermissionDeleteLock, "", c.RBACConfig, false)
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
	Environment           types.EnvName    `json:"env"`
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
	envName := types.EnvName(c.Environment)
	// Note: it's possible to lock an application BEFORE it's even deployed to the environment.
	err := state.checkUserPermissions(ctx, transaction, envName, c.Application, auth.PermissionCreateLock, "", c.RBACConfig, true)
	if err != nil {
		return "", err
	}
	if c.CiLink != "" && !isValidLink(c.CiLink, c.AllowedDomains) {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("provided CI Link: %s is not valid or does not match any of the allowed domain", c.CiLink))
	}
	if c.SuggestedLifeTime != nil && *c.SuggestedLifeTime != "" && !isValidLifeTime(*c.SuggestedLifeTime) {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("suggested life time: %s is not a valid lifetime. It should be a number followed by h, d or w", *c.SuggestedLifeTime))
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

	errW := state.DBHandler.DBWriteApplicationLock(ctx, transaction, c.LockId, envName, c.Application, metadata)
	if errW != nil {
		return "", errW
	}

	//Add it to all locks
	allAppLocks, err := state.DBHandler.DBSelectAllAppLocks(ctx, transaction, envName, c.Application)
	if err != nil {
		return "", err
	}
	if allAppLocks == nil {
		allAppLocks = make([]string, 0)
	}

	if !slices.Contains(allAppLocks, c.LockId) {
		allAppLocks = append(allAppLocks, c.LockId)
	}
	GaugeEnvAppLockMetric(ctx, len(allAppLocks), envName, c.Application)

	// locks are invisible to argoCd, so no changes here
	return fmt.Sprintf("Created lock %q on environment %q for application %q", c.LockId, c.Environment, c.Application), nil
}

type DeleteEnvironmentApplicationLock struct {
	Authentication        `json:"-"`
	Environment           types.EnvName    `json:"env"`
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
	envName := types.EnvName(c.Environment)
	err := state.checkUserPermissions(ctx, transaction, envName, c.Application, auth.PermissionDeleteLock, "", c.RBACConfig, true)

	if err != nil {
		return "", err
	}
	queueMessage := ""
	err = state.DBHandler.DBDeleteApplicationLock(ctx, transaction, envName, c.Application, c.LockId)
	if err != nil {
		return "", err
	}
	allAppLocks, err := state.DBHandler.DBSelectAllAppLocks(ctx, transaction, envName, c.Application)
	if err != nil {
		return "", fmt.Errorf("DeleteEnvironmentApplicationLock: could not select all env app locks for app '%v' on '%v': '%w'", c.Application, c.Environment, err)
	}
	var locks []string
	if allAppLocks != nil {
		locks = db.Remove(allAppLocks, c.LockId)
	}

	GaugeEnvAppLockMetric(ctx, len(locks), envName, c.Application)
	return fmt.Sprintf("Deleted lock %q on environment %q for application %q%s", c.LockId, c.Environment, c.Application, queueMessage), nil
}

type CreateEnvironmentTeamLock struct {
	Authentication        `json:"-"`
	Environment           types.EnvName    `json:"env"`
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
	envName := types.EnvName(c.Environment)
	// Note: it's possible to lock an application BEFORE it's even deployed to the environment.
	err := state.checkUserPermissions(ctx, transaction, envName, "*", auth.PermissionCreateLock, c.Team, c.RBACConfig, true)

	if err != nil {
		return "", err
	}

	if c.CiLink != "" && !isValidLink(c.CiLink, c.AllowedDomains) {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("provided CI Link: %s is not valid or does not match any of the allowed domain", c.CiLink))
	}
	if c.SuggestedLifeTime != nil && *c.SuggestedLifeTime != "" && !isValidLifeTime(*c.SuggestedLifeTime) {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("suggested life time: %s is not a valid lifetime. It should be a number followed by h, d or w", *c.SuggestedLifeTime))
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

	errW := state.DBHandler.DBWriteTeamLock(ctx, transaction, c.LockId, envName, c.Team, metadata)

	if errW != nil {
		return "", errW
	}

	return fmt.Sprintf("Created lock %q on environment %q for team %q", c.LockId, c.Environment, c.Team), nil
}

type DeleteEnvironmentTeamLock struct {
	Authentication        `json:"-"`
	Environment           types.EnvName    `json:"env"`
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
	envName := types.EnvName(c.Environment)
	err := state.checkUserPermissions(ctx, transaction, envName, "", auth.PermissionDeleteLock, c.Team, c.RBACConfig, true)

	if err != nil {
		return "", err
	}
	err = state.DBHandler.DBDeleteTeamLock(ctx, transaction, envName, c.Team, c.LockId)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Deleted lock %q on environment %q for team %q", c.LockId, c.Environment, c.Team), nil
}

// Creates or Update an Environment
type CreateEnvironment struct {
	Authentication        `json:"-"`
	Environment           types.EnvName            `json:"env"`
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
	envName := types.EnvName(c.Environment)
	err := CheckUserPermissionsCreateEnvironment(ctx, c.RBACConfig, c.Config)
	if err != nil {
		return "", err
	}
	// first read the env to see if it has applications:
	env, err := state.DBHandler.DBSelectEnvironment(ctx, transaction, envName)
	if err != nil {
		return "", fmt.Errorf("could not select environment %s from database, error: %w", c.Environment, err)
	}

	// write to environments table
	// We don't use environment applications column anymore, but for backward compatibility we keep it updated
	environmentApplications := make([]string, 0)
	if env != nil {
		environmentApplications = env.Applications
	}
	err = state.DBHandler.DBWriteEnvironment(ctx, transaction, envName, c.Config, environmentApplications)
	if err != nil {
		return "", fmt.Errorf("unable to write to the environment table, error: %w", err)
	}

	//Should be empty on new environments
	envApps, err := state.GetEnvironmentApplications(ctx, transaction, envName)
	if err != nil {
		return "", fmt.Errorf("unable to read environment, error: %w", err)

	}
	for _, app := range envApps {
		t.AddAppEnv(app, envName, "")
	}

	// we do not need to inform argoCd when creating an environment, as there are no apps yet
	return fmt.Sprintf("create environment %q", c.Environment), nil
}

type DeleteEnvironment struct {
	Authentication        `json:"-"`
	Environment           types.EnvName    `json:"env"`
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
	envName := types.EnvName(c.Environment)
	err := state.checkUserPermissions(ctx, transaction, envName, "*", auth.PermissionDeleteEnvironment, "", c.RBACConfig, false)
	if err != nil {
		return "", err
	}

	allEnvs, err := state.DBHandler.DBSelectAllEnvironments(ctx, transaction)
	if err != nil || allEnvs == nil {
		return "", fmt.Errorf("error getting all environments %v", err)
	}

	/*Check for locks*/
	envLocks, err := state.DBHandler.DBSelectAllEnvironmentLocks(ctx, transaction, envName)
	if err != nil {
		return "", err
	}
	if envLocks != nil && len(envLocks.EnvLocks) != 0 {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("could not delete environment '%s'. Environment locks for this environment exist", c.Environment))
	}

	appLocksForEnv, err := state.DBHandler.DBSelectAllAppLocksForEnv(ctx, transaction, c.Environment)
	if err != nil {
		return "", err
	}
	if len(appLocksForEnv) != 0 {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("could not delete environment '%s'. Application locks for this environment exist", c.Environment))
	}

	teamLocksForEnv, err := state.DBHandler.DBSelectTeamLocksForEnv(ctx, transaction, c.Environment)
	if err != nil {
		return "", err
	}
	if len(teamLocksForEnv) != 0 {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("could not delete environment '%s'. Team locks for this environment exist", c.Environment))
	}

	/* Check that no environment has the one we are trying to delete as upstream */
	allEnvConfigs, err := state.GetAllEnvironmentConfigs(ctx, transaction)
	if err != nil {
		return "", err
	}

	envConfigToDelete := allEnvConfigs[envName]

	//Find out if env to delete is last of its group, might be useful next
	var envToDeleteGroupName = mapper.DeriveGroupName(envConfigToDelete, envName)

	var allEnvGroups = mapper.MapEnvironmentsToGroups(allEnvConfigs)
	lastEnvOfGroup := false
	for _, currGroup := range allEnvGroups {
		if currGroup.EnvironmentGroupName == envToDeleteGroupName {
			lastEnvOfGroup = (len(currGroup.Environments) == 1)
		}
	}

	for envName, envConfig := range allEnvConfigs {
		if envConfig.Upstream != nil && envConfig.Upstream.Environment == c.Environment {
			return "", grpc.FailedPrecondition(ctx, fmt.Errorf("could not delete environment '%s'. Environment '%s' is upstream from '%s'", c.Environment, c.Environment, envName))
		}

		//If we are deleting an environment and it is the last one on the group, we are also deleting the group.
		//If this group is upstream from another env, we need to block it aswell
		if envConfig.Upstream != nil && envConfig.Upstream.Environment == types.EnvName(envToDeleteGroupName) && lastEnvOfGroup {
			return "", grpc.FailedPrecondition(ctx, fmt.Errorf("could not delete environment '%s'. '%s' is part of environment group '%s', "+
				"which is upstream from '%s' and deleting '%s' would result in environment group deletion",
				c.Environment,
				c.Environment,
				envToDeleteGroupName,
				envName,
				c.Environment,
			))
		}
	}

	/*Remove environment from all apps*/
	allAppsForEnv, err := state.GetEnvironmentApplications(ctx, transaction, envName)
	if err != nil {
		return "", err
	}

	//Delete env from apps
	for _, app := range allAppsForEnv {
		logger.FromContext(ctx).Sugar().Infof("Deleting environment '%s' from '%s'", c.Environment, app)
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
	err = state.DBHandler.DBDeleteEnvironment(ctx, transaction, envName)

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Successfully deleted environment '%s'", c.Environment), nil
}

type ExtendAAEnvironment struct {
	Authentication        `json:"-"`
	Environment           types.EnvName                  `json:"env"`
	ArgoCDConfig          config.EnvironmentConfigArgoCd `json:"config"`
	TransformerEslVersion db.TransformerID               `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
}

func (c *ExtendAAEnvironment) GetDBEventType() db.EventType {
	return db.EvtExtendAAEnvironment
}

func (c *ExtendAAEnvironment) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *ExtendAAEnvironment) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

func (c *ExtendAAEnvironment) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	envName := types.EnvName(c.Environment)
	err := state.checkUserPermissions(ctx, transaction, envName, "*", auth.PermissionCreateEnvironment, "", c.RBACConfig, false)
	if err != nil {
		return "", err
	}
	env, err := state.DBHandler.DBSelectEnvironment(ctx, transaction, envName)
	if err != nil {
		return "", err
	}
	if env == nil {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Environment with name %q not found", envName))
	} else if !isAAEnv(&env.Config) {
		return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Environment with name %q is not an Active/Active environment", envName))
	}

	configs := env.Config.ArgoCdConfigs.ArgoCdConfigurations
	var found bool
	for idx, currentConfig := range env.Config.ArgoCdConfigs.ArgoCdConfigurations {
		if currentConfig.ConcreteEnvName == c.ArgoCDConfig.ConcreteEnvName {
			logger.FromContext(ctx).Sugar().Infof("Argo CD configuration with for %q found. Updating existing configuration.", c.ArgoCDConfig.ConcreteEnvName)
			env.Config.ArgoCdConfigs.ArgoCdConfigurations[idx] = &c.ArgoCDConfig
			found = true
		}
	}
	if !found {
		configs = append(configs, &c.ArgoCDConfig)
	}

	env.Config.ArgoCdConfigs.ArgoCdConfigurations = configs
	err = state.DBHandler.DBWriteEnvironment(ctx, transaction, envName, env.Config, env.Applications)
	if err != nil {
		return "", fmt.Errorf("Could not extend Active/Active environment: %q. %w", envName, err)
	}
	return fmt.Sprintf("Successfully added ArgoCD configuration '%s'", c.Environment), nil
}

func isAAEnv(config *config.EnvironmentConfig) bool {
	return config.ArgoCdConfigs != nil
}

type QueueApplicationVersion struct {
	Environment types.EnvName
	Application string
	Version     uint64
}

func (c *QueueApplicationVersion) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	err := state.DBHandler.DBWriteDeploymentAttempt(ctx, transaction, types.EnvName(c.Environment), c.Application, types.MakeReleaseNumberVersion(c.Version))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Queued version %d of app %q in env %q", c.Version, c.Application, c.Environment), nil
}

type Overview struct {
	App     string
	Version types.ReleaseNumbers
}

func getOverrideVersions(ctx context.Context, transaction *sql.Tx, commitHash string, upstreamEnvName types.EnvName, state *State) (resp []Overview, err error) {
	dbHandler := state.DBHandler
	ts, err := dbHandler.DBReadCommitHashTransactionTimestamp(ctx, transaction, commitHash)
	if err != nil {
		return nil, fmt.Errorf("unable to get manifest repo timestamp that corresponds to provided commit Hash %v", err)
	} else if ts == nil {
		return nil, fmt.Errorf("timestamp for the provided commit hash %q does not exist", commitHash)
	}

	apps, err := state.GetEnvironmentApplicationsAtTimestamp(ctx, transaction, upstreamEnvName, *ts)
	if err != nil {
		return nil, fmt.Errorf("unable to get applications for environment %s at timestamp %s: %w", upstreamEnvName, *ts, err)
	}

	for _, appName := range apps {
		currentAppDeployments, err := state.GetAllDeploymentsForAppFromDBAtTimestamp(ctx, transaction, appName, *ts)
		if err != nil {
			return nil, fmt.Errorf("unable to get GetAllDeploymentsForAppAtTimestamp  %v", err)
		}

		if version, ok := currentAppDeployments[upstreamEnvName]; ok {
			resp = append(resp, Overview{App: appName, Version: version})
		}
	}

	return resp, nil
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
