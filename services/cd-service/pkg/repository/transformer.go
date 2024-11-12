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
	"io"
	"io/fs"
	"math"
	"net/url"
	"os"
	"path"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	time2 "github.com/freiheit-com/kuberpult/pkg/time"
	"github.com/google/go-cmp/cmp"

	config "github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/mapper"
	"github.com/freiheit-com/kuberpult/pkg/sorting"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/freiheit-com/kuberpult/pkg/conversion"
	"github.com/freiheit-com/kuberpult/pkg/metrics"

	"github.com/freiheit-com/kuberpult/pkg/uuid"
	git "github.com/libgit2/git2go/v34"

	"github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/valid"

	"github.com/freiheit-com/kuberpult/pkg/logger"

	yaml3 "gopkg.in/yaml.v3"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	diffspan "github.com/hexops/gotextdiff/span"
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

func versionToString(Version uint64) string {
	return strconv.FormatUint(Version, 10)
}

func releasesDirectory(fs billy.Filesystem, application string) string {
	return fs.Join("applications", application, "releases")
}

func applicationDirectory(fs billy.Filesystem, application string) string {
	return fs.Join("applications", application)
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

func GaugeEnvLockMetric(ctx context.Context, s *State, transaction *sql.Tx, env string) {
	if ddMetrics != nil {
		count, err := s.GetEnvironmentLocksCount(ctx, transaction, env)
		if err != nil {
			logger.FromContext(ctx).
				Sugar().
				Warnf("Error when trying to get the number of environment locks: %w\n", err)
			return
		}
		err = ddMetrics.Gauge("env_lock_count", count, []string{"env:" + env}, 1)
		if err != nil {
			logger.FromContext(ctx).
				Sugar().
				Warnf("Error when trying to send `env_lock_count` metric to datadog: %w\n", err)
		}
		err = ddMetrics.Gauge("environment_lock_count", count, []string{"kuberpult_environment:" + env}, 1)
		if err != nil {
			logger.FromContext(ctx).
				Sugar().
				Warnf("Error when trying to send `environment_lock_count` metric to datadog: %w\n", err)
		}
	}
}
func GaugeEnvAppLockMetric(ctx context.Context, s *State, transaction *sql.Tx, env, app string) {
	if ddMetrics != nil {
		count, err := s.GetEnvironmentApplicationLocksCount(ctx, transaction, env, app)
		if err != nil {
			logger.FromContext(ctx).
				Sugar().
				Warnf("Error when trying to get the number of application locks: %w\n", err)
			return
		}
		err = ddMetrics.Gauge("app_lock_count", count, []string{"app:" + app, "env:" + env}, 1)
		if err != nil {
			logger.FromContext(ctx).
				Sugar().
				Warnf("Error when trying to send `app_lock_count` metric to datadog: %w\n", err)
		}
		err = ddMetrics.Gauge("application_lock_count", count, []string{"kuberpult_environment:" + env, "kuberpult_application:" + app}, 1)
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

func UpdateDatadogMetrics(ctx context.Context, transaction *sql.Tx, state *State, repo Repository, changes *TransformerResult, now time.Time) error {
	if ddMetrics == nil {
		return nil
	}

	if state.DBHandler == nil {
		logger.FromContext(ctx).Sugar().Warn("Tried to update datadog metrics without database")
		return nil
	}

	_, envNames, err := state.GetEnvironmentConfigsSorted(ctx, transaction)
	if err != nil {
		return err
	}
	repo.(*repository).GaugeQueueSize(ctx)
	for _, envName := range envNames {
		GaugeEnvLockMetric(ctx, state, transaction, envName)

		env, err := state.DBHandler.DBSelectEnvironment(ctx, transaction, envName)
		if err != nil {
			return fmt.Errorf("failed to read environment from the db: %v", err)
		}

		// in the future apps might be sorted, but currently they aren't
		slices.Sort(env.Applications)
		for _, appName := range env.Applications {
			GaugeEnvAppLockMetric(ctx, state, transaction, envName, appName)

			// 2024-11-08 17:09:03 +0000 UTC
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
	s := repo.State()
	if s.DBHandler.ShouldUseOtherTables() {
		err := s.DBHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
			if err := UpdateDatadogMetrics(ctx, transaction, s, repo, nil, time.Now()); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			panic(err.Error())
		}
	} else {
		if err := UpdateDatadogMetrics(ctx, nil, s, repo, nil, time.Now()); err != nil {
			panic(err.Error())
		}
	}
}

func RegularlyCleanupOverviewCache(ctx context.Context, repo Repository, interval time.Duration, cacheTtlHours uint) {
	cleanupEventTimer := time.NewTicker(interval * time.Second)
	for range cleanupEventTimer.C {
		logger.FromContext(ctx).Sugar().Warn("Cleaning up old overview caches")
		s := repo.State()
		if s.DBHandler.ShouldUseOtherTables() {
			err := s.DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := s.DBHandler.DBDeleteOldOverviews(ctx, transaction, 5, time.Now().Add(-time.Duration(cacheTtlHours)*time.Hour))
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				panic(err.Error())
			}
		}
	}
}

// A Transformer updates the files in the worktree
type Transformer interface {
	Transform(ctx context.Context, state *State, t TransformerContext, transaction *sql.Tx) (commitMsg string, e error)
	GetDBEventType() db.EventType
	SetEslVersion(eslVersion db.TransformerID)
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
		Commits:         nil,
		State:           s,
		Stack:           [][]string{nil},
	}
	if err := runner.Execute(ctx, t, transaction); err != nil {
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
	//Context context.Context
	State *State
	// Stores the current stack of commit messages. Each entry of
	// the outer slice corresponds to a step being executed. Each
	// entry of the inner slices correspond to a message generated
	// by that step.
	Stack           [][]string
	ChangedApps     []AppEnv
	DeletedRootApps []RootApp
	Commits         *CommitIds
}

func (r *transformerRunner) Execute(ctx context.Context, t Transformer, transaction *sql.Tx) error {
	r.Stack = append(r.Stack, nil)
	msg, err := t.Transform(ctx, r.State, r, transaction)
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

type ctxMarkerGenerateUuid struct{}

var (
	ctxMarkerGenerateUuidKey = &ctxMarkerGenerateUuid{}
)

func GetLastReleaseFromFile(fs billy.Filesystem, application string) (uint64, error) {
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
func (s *State) GetLastRelease(ctx context.Context, transaction *sql.Tx, fs billy.Filesystem, application string) (uint64, error) {
	if s.DBHandler.ShouldUseOtherTables() {
		releases, err := s.DBHandler.DBSelectAllReleasesOfApp(ctx, transaction, application)
		if err != nil {
			return 0, fmt.Errorf("could not get releases of app %s: %v", application, err)
		}
		if releases == nil || len(releases.Metadata.Releases) == 0 {
			return 0, nil
		}
		l := len(releases.Metadata.Releases)
		return uint64(releases.Metadata.Releases[l-1]), nil
	} else {
		return GetLastReleaseFromFile(fs, application)
	}
}

func isValidLink(urlToCheck string, allowedDomains []string) bool {
	u, err := url.ParseRequestURI(urlToCheck) //Check if is a valid URL
	if err != nil {
		return false
	}
	return slices.Contains(allowedDomains, u.Hostname())
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
	fs := state.Filesystem
	if !valid.ApplicationName(c.Application) {
		return "", GetCreateReleaseAppNameTooLong(c.Application, valid.AppNameRegExp, uint32(valid.MaxAppNameLen))
	}
	if state.DBHandler.ShouldUseOtherTables() {
		allApps, err := state.DBHandler.DBSelectAllApplications(ctx, transaction)
		if err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
		if allApps == nil {
			now, err := state.DBHandler.DBReadTransactionTimestamp(ctx, transaction)
			if err != nil {
				return "", GetCreateReleaseGeneralFailure(fmt.Errorf("could not get transaction timestamp"))
			}

			allApps = &db.AllApplicationsGo{
				Version: 1,
				AllApplicationsJson: db.AllApplicationsJson{
					Apps: []string{},
				},
				Created: *now,
			}
		}

		if !slices.Contains(allApps.Apps, c.Application) {
			// this app is new
			allApps.Apps = append(allApps.Apps, c.Application)
			err := state.DBHandler.DBWriteAllApplications(ctx, transaction, allApps.Version, allApps.Apps)
			if err != nil {
				return "", GetCreateReleaseGeneralFailure(fmt.Errorf("could not write all apps: %w", err))
			}

			//We need to check that this is not an app that has been previously deleted
			app, err := state.DBHandler.DBSelectApp(ctx, transaction, c.Application)
			if err != nil {
				return "", GetCreateReleaseGeneralFailure(fmt.Errorf("could not read apps: %v", err))
			}
			var ver db.EslVersion
			if app == nil {
				ver = db.InitialEslVersion
			} else {
				if app.StateChange != db.AppStateChangeDelete {
					return "", GetCreateReleaseGeneralFailure(fmt.Errorf("could not write new app, app already exists: %v", err)) //Should never happen
				}
				ver = app.EslVersion + 1
			}

			err = state.DBHandler.InsertAppFun(
				ctx,
				transaction,
				c.Application,
				ver,
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
				err = state.DBHandler.DBInsertApplication(
					ctx,
					transaction,
					c.Application,
					existingApp.EslVersion,
					db.AppStateChangeUpdate,
					newMeta,
				)
				if err != nil {
					return "", GetCreateReleaseGeneralFailure(fmt.Errorf("could not update app: %v", err))
				}
			}
		}
	}

	releaseDir := releasesDirectoryWithVersion(fs, c.Application, version)
	appDir := applicationDirectory(fs, c.Application)
	if err = fs.MkdirAll(releaseDir, 0777); err != nil {
		return "", GetCreateReleaseGeneralFailure(err)
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

	if !state.DBHandler.ShouldUseOtherTables() {
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
	}

	if c.Team != "" && !state.DBHandler.ShouldUseOtherTables() {
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
	} else {
		logger.FromContext(ctx).Sugar().Warnf("skipping team file for team %s and should=%v", c.Team, state.DBHandler.ShouldUseOtherTables())
	}
	if c.CiLink != "" {
		if !state.DBHandler.ShouldUseOtherTables() {
			return "", GetCreateReleaseGeneralFailure(fmt.Errorf("Ci Link is only supported when database is fully enabled."))
		} else if !isValidLink(c.CiLink, c.AllowedDomains) {
			return "", GetCreateReleaseGeneralFailure(fmt.Errorf("Provided CI Link: %s is not valid or does not match any of the allowed domain", c.CiLink))
		}
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

	if state.DBHandler.ShouldUseOtherTables() {
		prevRelease, err := state.DBHandler.DBSelectReleasesByAppOrderedByEslVersion(ctx, transaction, c.Application, false, false)
		if err != nil {
			return "", err
		}
		var v = db.InitialEslVersion - 1
		if len(prevRelease) > 0 {
			v = prevRelease[0].EslVersion
		}
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
			EslVersion:    0,
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
			Deleted:      false,
		}
		err = state.DBHandler.DBInsertRelease(ctx, transaction, release, v)
		if err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
		allReleases, err := state.DBHandler.DBSelectAllReleasesOfApp(ctx, transaction, c.Application)
		if err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
		if allReleases == nil {

			allReleases = &db.DBAllReleasesWithMetaData{
				EslVersion: db.InitialEslVersion - 1,
				Created:    *now,
				App:        c.Application,
				Metadata: db.DBAllReleaseMetaData{
					Releases: []int64{int64(release.ReleaseNumber)},
				},
			}
		} else {
			allReleases.Metadata.Releases = append(allReleases.Metadata.Releases, int64(release.ReleaseNumber))
		}
		if !c.IsPrepublish {
			err = state.DBHandler.DBInsertAllReleases(ctx, transaction, c.Application, allReleases.Metadata.Releases, allReleases.EslVersion)
		}
		if err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
	}

	for i := range sortedKeys {
		env := sortedKeys[i]
		man := c.Manifests[env]
		// Add application to the environment if not exist
		if state.DBHandler.ShouldUseOtherTables() {
			envInfo, err := state.DBHandler.DBSelectEnvironment(ctx, transaction, env)
			if err != nil {
				return "", GetCreateReleaseGeneralFailure(err)
			}
			found := false
			if envInfo != nil && envInfo.Applications != nil {
				for _, app := range envInfo.Applications {
					if app == c.Application {
						found = true
						break
					}
				}
			}
			if envInfo != nil && !found {
				err = state.DBHandler.DBWriteEnvironment(ctx, transaction, env, envInfo.Config, append(envInfo.Applications, c.Application))
				if err != nil {
					return "", GetCreateReleaseGeneralFailure(err)
				}
			}
		}

		err := state.checkUserPermissions(ctx, transaction, env, c.Application, auth.PermissionCreateRelease, c.Team, c.RBACConfig, true)
		if err != nil {
			return "", err
		}
		envDir := fs.Join(releaseDir, "environments", env)

		config, found := configs[env]
		hasUpstream := false
		if found {
			hasUpstream = config.Upstream != nil
		}

		if !state.DBHandler.ShouldUseOtherTables() {
			if err = fs.MkdirAll(envDir, 0777); err != nil {
				return "", GetCreateReleaseGeneralFailure(err)
			}
			if err := util.WriteFile(fs, fs.Join(envDir, "manifests.yaml"), []byte(man), 0666); err != nil {
				return "", GetCreateReleaseGeneralFailure(err)
			}
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
				SkipOverview:          false,
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
	allReleases, err := dbHandler.DBSelectAllReleasesOfApp(ctx, transaction, c.Application)
	if err != nil {
		return false, err
	}
	if allReleases == nil {
		return false, err
	}
	releaseVersions := allReleases.Metadata.Releases
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
		err = dbHandler.DBInsertRelease(ctx, transaction, *nextRelease, nextRelease.EslVersion)
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
	fs := state.Filesystem
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

	ev := &event.NewRelease{
		Environments: envMap,
	}
	var writeError error
	if h.ShouldUseEslTable() {
		gen := getGenerator(ctx)
		eventUuid := gen.Generate()
		writeError = state.DBHandler.DBWriteNewReleaseEvent(ctx, transaction, transformerEslVersion, releaseVersion, eventUuid, sourceCommitId, ev)
	} else {
		writeError = writeEvent(ctx, eventId, sourceCommitId, fs, ev)
	}

	if writeError != nil {
		return fmt.Errorf("error while writing event: %v", writeError)
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

func writeEvent(ctx context.Context, eventId string, sourceCommitId string, filesystem billy.Filesystem, ev event.Event) error {
	span, _ := tracer.StartSpanFromContext(ctx, "writeEvent")
	defer span.Finish()
	eventDir := commitEventDir(filesystem, sourceCommitId, eventId)
	if err := event.Write(filesystem, eventDir, ev); err != nil {
		return fmt.Errorf(
			"could not write an event for commit %s for uuid %s, error: %w",
			sourceCommitId, eventId, err)
	}
	return nil

}

func (c *CreateApplicationVersion) calculateVersion(ctx context.Context, transaction *sql.Tx, state *State) (uint64, error) {
	bfs := state.Filesystem
	if c.Version == 0 {
		if state.DBHandler.ShouldUseOtherTables() {
			return 0, fmt.Errorf("version is required when using the database")
		}
		lastRelease, err := state.GetLastRelease(ctx, transaction, bfs, c.Application)
		if err != nil {
			return 0, err
		}
		return lastRelease + 1, nil
	} else {
		if state.DBHandler.ShouldUseEslTable() {
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
				return 0, c.sameAsExisting(ctx, state, c.Version)
			}
			// TODO: check GC here
			return c.Version, nil
		}
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

func (c *CreateApplicationVersion) sameAsExisting(ctx context.Context, state *State, version uint64) error {
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
			logger.FromContext(ctx).Sugar().Warnf("manifest is different1 %v", err)
			return GetCreateReleaseAlreadyExistsDifferent(api.DifferingField_MANIFESTS, fmt.Sprintf("manifest missing for env %s", env))
		}
		existingManStr := string(existingMan)
		if canonicalizeYaml(existingManStr) != canonicalizeYaml(man) {
			logger.FromContext(ctx).Sugar().Warnf("manifest is different2 %s!=%s", existingManStr, man)
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
	if state.DBHandler.ShouldUseOtherTables() {
		all, err := state.DBHandler.DBSelectAllReleasesOfApp(ctx, transaction, application)
		if err != nil {
			return false, err
		}
		//Convert
		if all == nil {
			rels = make([]uint64, 0)
		} else {
			rels = make([]uint64, len(all.Metadata.Releases))
			for idx, rel := range all.Metadata.Releases {
				rels[idx] = uint64(rel)
			}
		}

	} else {
		rels, err = state.GetAllApplicationReleasesFromManifest(application)

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

func (c *CreateUndeployApplicationVersion) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	fs := state.Filesystem
	lastRelease, err := state.GetLastRelease(ctx, transaction, fs, c.Application)
	if err != nil {
		return "", err
	}
	if lastRelease == 0 {
		return "", fmt.Errorf("cannot undeploy non-existing application '%v'", c.Application)
	}
	releaseDir := releasesDirectoryWithVersion(fs, c.Application, lastRelease+1)
	if state.DBHandler.ShouldUseOtherTables() {
		prevRelease, err := state.DBHandler.DBSelectReleaseByVersion(ctx, transaction, c.Application, lastRelease, true)
		if err != nil {
			return "", err
		}
		var v = db.InitialEslVersion - 1
		if prevRelease != nil {
			v = prevRelease.EslVersion
		}
		now, err := state.DBHandler.DBReadTransactionTimestamp(ctx, transaction)
		if err != nil {
			return "", fmt.Errorf("could not get transaction timestamp")
		}
		release := db.DBReleaseWithMetaData{
			EslVersion:    0,
			ReleaseNumber: lastRelease + 1,
			App:           c.Application,
			Manifests: db.DBReleaseManifests{
				Manifests: map[string]string{ //empty manifest
					"": "",
				},
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
			Environments: []string{},
			Created:      *now,
			Deleted:      false,
		}
		err = state.DBHandler.DBInsertRelease(ctx, transaction, release, v)
		if err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
		allReleases, err := state.DBHandler.DBSelectAllReleasesOfApp(ctx, transaction, c.Application)
		if err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
		if allReleases == nil {
			allReleases = &db.DBAllReleasesWithMetaData{
				EslVersion: db.InitialEslVersion - 1,
				Created:    *now,
				App:        c.Application,
				Metadata: db.DBAllReleaseMetaData{
					Releases: []int64{int64(release.ReleaseNumber)},
				},
			}
		} else {
			allReleases.Metadata.Releases = append(allReleases.Metadata.Releases, int64(release.ReleaseNumber))
		}
		err = state.DBHandler.DBInsertAllReleases(ctx, transaction, c.Application, allReleases.Metadata.Releases, allReleases.EslVersion)
		if err != nil {
			return "", GetCreateReleaseGeneralFailure(err)
		}
	} else {

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
		if !state.DBHandler.ShouldUseOtherTables() {
			envDir := fs.Join(releaseDir, "environments", env)

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
				SkipOverview:          false,
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
	Authentication        `json:"-"`
	Application           string           `json:"app"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (u *UndeployApplication) GetDBEventType() db.EventType {
	return db.EvtUndeployApplication
}

func (c *UndeployApplication) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (u *UndeployApplication) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	fs := state.Filesystem
	lastRelease, err := state.GetLastRelease(ctx, transaction, fs, u.Application)
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
		if state.DBHandler.ShouldUseOtherTables() {
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
				err = state.DBHandler.DBWriteDeployment(ctx, transaction, *deployment, deployment.EslVersion, false)
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
				for _, currentLockID := range locks.AppLocks {
					err := state.DBHandler.DBDeleteApplicationLock(ctx, transaction, env, u.Application, currentLockID)
					if err != nil {
						return "", err
					}
				}
				continue
			}
			return "", fmt.Errorf("UndeployApplication(db): error cannot un-deploy application '%v' the release '%v' is not un-deployed", u.Application, env)
		} else {
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
			if err != nil && errors.Is(err, os.ErrNotExist) {
				return "", fmt.Errorf("UndeployApplication(repo): error cannot un-deploy application '%v' the release '%v' is not un-deployed: '%v'", u.Application, env, undeployFile)
			}
		}
	}
	if state.DBHandler.ShouldUseOtherTables() {
		applications, err := state.DBHandler.DBSelectAllApplications(ctx, transaction)
		if err != nil {
			return "", fmt.Errorf("UndeployApplication: could not select all apps '%v': '%w'", u.Application, err)
		}
		applications.Apps = db.Remove(applications.Apps, u.Application)
		err = state.DBHandler.DBWriteAllApplications(ctx, transaction, applications.Version, applications.Apps)
		if err != nil {
			return "", fmt.Errorf("UndeployApplication: could not write all apps '%v': '%w'", u.Application, err)
		}
		dbApp, err := state.DBHandler.DBSelectApp(ctx, transaction, u.Application)
		if err != nil {
			return "", fmt.Errorf("UndeployApplication: could not select app '%s': %v", u.Application, err)
		}
		err = state.DBHandler.InsertAppFun(ctx, transaction, dbApp.App, dbApp.EslVersion, db.AppStateChangeDelete, db.DBAppMetaData{Team: dbApp.Metadata.Team})
		if err != nil {
			return "", fmt.Errorf("UndeployApplication: could not insert app '%s': %v", u.Application, err)
		}

		err = state.DBHandler.DBClearReleases(ctx, transaction, u.Application)

		if err != nil {
			return "", fmt.Errorf("UndeployApplication: could not clear releases for app '%s': %v", u.Application, err)
		}

		err = state.DBHandler.DBClearAllDeploymentsForApp(ctx, transaction, u.Application)
		if err != nil {
			return "", fmt.Errorf("UndeployApplication: could not clear all deployments for app '%s': %v", u.Application, err)
		}
		allEnvs, err := state.DBHandler.DBSelectAllEnvironments(ctx, transaction)
		if err != nil {
			return "", fmt.Errorf("UndeployApplication: could not get all environments: %v", err)
		}
		for _, envName := range allEnvs.Environments {
			env, err := state.DBHandler.DBSelectEnvironment(ctx, transaction, envName)
			if err != nil {
				return "", fmt.Errorf("UndeployApplication: could not get environment %s: %v", envName, err)
			}
			newEnvApps := make([]string, 0)
			for _, app := range env.Applications {
				if app != u.Application {
					newEnvApps = append(newEnvApps, app)
				}
			}
			err = state.DBHandler.DBWriteEnvironment(ctx, transaction, envName, env.Config, newEnvApps)
			if err != nil {
				return "", fmt.Errorf("UndeployApplication: could not write environment: %v", err)
			}
		}
	} else {
		// remove application
		appDir := applicationDirectory(fs, u.Application)

		releasesDir := fs.Join(appDir, "releases")
		files, err := fs.ReadDir(releasesDir)
		if err != nil {
			return "", fmt.Errorf("could not read the releases directory %s %w", releasesDir, err)
		}
		for _, file := range files {
			//For each release, deletes it
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
			teamOwner, err := state.GetApplicationTeamOwner(ctx, transaction, u.Application)
			if err != nil {
				return "", err
			}
			t.AddAppEnv(u.Application, env, teamOwner)
			// remove environment application
			if err := fs.Remove(appDir); err != nil && !errors.Is(err, os.ErrNotExist) {
				return "", fmt.Errorf("UndeployApplication: unexpected error application '%v' environment '%v': '%w'", u.Application, env, err)
			}
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

func (c *DeleteEnvFromApp) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
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
	if state.DBHandler.ShouldUseOtherTables() {
		releases, err := state.DBHandler.DBSelectReleasesByAppLatestEslVersion(ctx, transaction, u.Application, true)
		if err != nil {
			return "", err
		}

		for _, dbReleaseWithMetadata := range releases {
			newManifests := make(map[string]string)
			for envName, manifest := range dbReleaseWithMetadata.Manifests.Manifests {
				if envName != u.Environment {
					newManifests[envName] = manifest
				}
			}
			now, err := state.DBHandler.DBReadTransactionTimestamp(ctx, transaction)
			if err != nil {
				return "", fmt.Errorf("could not get transaction timestamp")
			}
			newRelease := db.DBReleaseWithMetaData{
				EslVersion:    dbReleaseWithMetadata.EslVersion + 1,
				ReleaseNumber: dbReleaseWithMetadata.ReleaseNumber,
				App:           dbReleaseWithMetadata.App,
				Created:       *now,
				Manifests:     db.DBReleaseManifests{Manifests: newManifests},
				Metadata:      dbReleaseWithMetadata.Metadata,
				Deleted:       dbReleaseWithMetadata.Deleted,
				Environments:  []string{},
			}
			err = state.DBHandler.DBInsertRelease(ctx, transaction, newRelease, dbReleaseWithMetadata.EslVersion)
			if err != nil {
				return "", err
			}
		}

		env, err := state.DBHandler.DBSelectEnvironment(ctx, transaction, u.Environment)
		if err != nil {
			return "", fmt.Errorf("Couldn't read environment: %s from environments table, error: %w", u.Environment, err)
		}
		if env == nil {
			return "", fmt.Errorf("Attempting to delete an environment that doesn't exist in the environments table")
		}
		newApps := make([]string, 0)
		if env.Applications != nil {
			for _, app := range env.Applications {
				if app != u.Application {
					newApps = append(newApps, app)
				}
			}
		}
		err = state.DBInsertEnvironmentWithOverview(ctx, transaction, env.Name, env.Config, newApps)
		if err != nil {
			return "", fmt.Errorf("Couldn't write environment: %s into environments table, error: %w", u.Environment, err)
		}
	} else {

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
	}
	return fmt.Sprintf("Environment '%v' was removed from application '%v' successfully.", u.Environment, u.Application), nil
}

type CleanupOldApplicationVersions struct {
	Application           string
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
}

func (c *CleanupOldApplicationVersions) GetDBEventType() db.EventType {
	panic("CleanupOldApplicationVersions GetDBEventType")
}

func (c *CleanupOldApplicationVersions) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
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
	span, ctx := tracer.StartSpanFromContext(ctx, "CleanupOldApplicationVersions")
	defer span.Finish()
	fs := state.Filesystem
	oldVersions, err := findOldApplicationVersions(ctx, transaction, state, c.Application)
	if err != nil {
		return "", fmt.Errorf("cleanup: could not get application releases for app '%s': %w", c.Application, err)
	}

	msg := ""
	for _, oldRelease := range oldVersions {
		if state.DBHandler.ShouldUseOtherTables() {
			// delete release from all_releases
			if err := state.DBHandler.DBDeleteReleaseFromAllReleases(ctx, transaction, c.Application, oldRelease); err != nil {
				return "", err
			}
			//'Delete' from releases table
			if err := state.DBHandler.DBDeleteFromReleases(ctx, transaction, c.Application, oldRelease); err != nil {
				return "", err
			}
		} else {
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
	Authentication        `json:"-"`
	Environment           string           `json:"env"`
	LockId                string           `json:"lockId"`
	Message               string           `json:"message"`
	CiLink                string           `json:"ciLink"`
	AllowedDomains        []string         `json:"-"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (c *CreateEnvironmentLock) GetDBEventType() db.EventType {
	return db.EvtCreateEnvironmentLock
}

func (c *CreateEnvironmentLock) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (s *State) checkUserPermissions(ctx context.Context, transaction *sql.Tx, env, application, action, team string, RBACConfig auth.RBACConfig, checkTeam bool) error {
	if !RBACConfig.DexEnabled {
		return nil
	}
	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("checkUserPermissions: user not found: %v", err))
	}

	config, err := s.GetEnvironmentConfig(ctx, transaction, env)
	if err != nil {
		return err
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
	transaction *sql.Tx,
) (string, error) {
	err := state.checkUserPermissions(ctx, transaction, c.Environment, "*", auth.PermissionCreateLock, "", c.RBACConfig, false)
	if err != nil {
		return "", err
	}

	if c.CiLink != "" {
		if !state.DBHandler.ShouldUseOtherTables() {
			return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Ci Link is only supported when database is fully enabled."))
		} else if !isValidLink(c.CiLink, c.AllowedDomains) {
			return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Provided CI Link: %s is not valid or does not match any of the allowed domain", c.CiLink))
		}
	}

	if state.DBHandler.ShouldUseOtherTables() {
		user, err := auth.ReadUserFromContext(ctx)
		if err != nil {
			return "", err
		}
		//Write to locks table
		metadata := db.LockMetadata{
			Message:        c.Message,
			CreatedByName:  user.Name,
			CreatedByEmail: user.Email,
			CiLink:         c.CiLink,
			CreatedAt:      time.Time{}, //will not be used
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
		now, err := state.DBHandler.DBReadTransactionTimestamp(ctx, transaction)
		if err != nil {
			return "", fmt.Errorf("could not get transaction timestamp")
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

	} else {
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
	}
	GaugeEnvLockMetric(ctx, state, transaction, c.Environment)
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
	if err := util.WriteFile(fs, fs.Join(newLockDir, fieldCreatedAt), []byte(time2.GetTimeNow(ctx).Format(time.RFC3339)), 0666); err != nil {
		return err
	}
	return nil
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
	fs := state.Filesystem
	s := State{
		Commit:               nil,
		MinorRegexes:         state.MinorRegexes,
		Filesystem:           fs,
		DBHandler:            state.DBHandler,
		ReleaseVersionsLimit: state.ReleaseVersionsLimit,
		CloudRunClient:       state.CloudRunClient,
	}
	if s.DBHandler.ShouldUseOtherTables() {
		err := s.DBHandler.DBDeleteEnvironmentLock(ctx, transaction, c.Environment, c.LockId)
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
	} else {
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
	}
	apps, err := s.GetEnvironmentApplications(ctx, transaction, c.Environment)
	if err != nil {
		return "", fmt.Errorf("environment applications for %q not found: %v", c.Environment, err.Error())
	}

	additionalMessageFromDeployment := ""
	for _, appName := range apps {
		queueMessage, err := s.ProcessQueue(ctx, transaction, fs, c.Environment, appName)
		if err != nil {
			return "", err
		}
		if queueMessage != "" {
			additionalMessageFromDeployment = additionalMessageFromDeployment + "\n" + queueMessage
		}
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
	AllowedDomains        []string         `json:"-"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (c *CreateEnvironmentGroupLock) GetDBEventType() db.EventType {
	return db.EvtCreateEnvironmentGroupLock
}

func (c *CreateEnvironmentGroupLock) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
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
	if c.CiLink != "" {
		if !state.DBHandler.ShouldUseOtherTables() {
			return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Ci Link is only supported when database is fully enabled."))
		} else if !isValidLink(c.CiLink, c.AllowedDomains) {
			return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Provided CI Link: %s is not valid or does not match any of the allowed domain", c.CiLink))
		}
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
	AllowedDomains        []string         `json:"-"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (c *CreateEnvironmentApplicationLock) GetDBEventType() db.EventType {
	return db.EvtCreateEnvironmentApplicationLock
}

func (c *CreateEnvironmentApplicationLock) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
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
	if c.CiLink != "" {
		if !state.DBHandler.ShouldUseOtherTables() {
			return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Ci Link is only supported when database is fully enabled."))
		} else if !isValidLink(c.CiLink, c.AllowedDomains) {
			return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Provided CI Link: %s is not valid or does not match any of the allowed domain", c.CiLink))
		}
	}
	if state.DBHandler.ShouldUseOtherTables() {
		user, err := auth.ReadUserFromContext(ctx)
		if err != nil {
			return "", err
		}
		//Write to locks table
		errW := state.DBHandler.DBWriteApplicationLock(ctx, transaction, c.LockId, c.Environment, c.Application, db.LockMetadata{
			CreatedByName:  user.Name,
			CreatedByEmail: user.Email,
			Message:        c.Message,
			CiLink:         c.CiLink,
			CreatedAt:      time.Time{},
		})
		if errW != nil {
			return "", errW
		}

		//Add it to all locks
		allAppLocks, err := state.DBHandler.DBSelectAllAppLocks(ctx, transaction, c.Environment, c.Application)
		if err != nil {
			return "", err
		}
		now, err := state.DBHandler.DBReadTransactionTimestamp(ctx, transaction)
		if err != nil {
			return "", fmt.Errorf("could not get transaction timestamp")
		}
		if allAppLocks == nil {
			allAppLocks = &db.AllAppLocksGo{
				Version: 1,
				AllAppLocksJson: db.AllAppLocksJson{
					AppLocks: []string{},
				},
				Created:     *now,
				Environment: c.Environment,
				AppName:     c.Application,
			}
		}

		if !slices.Contains(allAppLocks.AppLocks, c.LockId) {
			allAppLocks.AppLocks = append(allAppLocks.AppLocks, c.LockId)
			err := state.DBHandler.DBWriteAllAppLocks(ctx, transaction, allAppLocks.Version, c.Environment, c.Application, allAppLocks.AppLocks)
			if err != nil {
				return "", err
			}
		}
	} else {
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

	}
	GaugeEnvAppLockMetric(ctx, state, transaction, c.Environment, c.Application)
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

func (c *DeleteEnvironmentApplicationLock) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	err := state.checkUserPermissions(ctx, transaction, c.Environment, c.Application, auth.PermissionDeleteLock, "", c.RBACConfig, true)

	if err != nil {
		return "", err
	}
	fs := state.Filesystem
	queueMessage := ""
	if state.DBHandler.ShouldUseOtherTables() {
		err := state.DBHandler.DBDeleteApplicationLock(ctx, transaction, c.Environment, c.Application, c.LockId)
		if err != nil {
			return "", err
		}
		allAppLocks, err := state.DBHandler.DBSelectAllAppLocks(ctx, transaction, c.Environment, c.Application)
		if err != nil {
			return "", fmt.Errorf("DeleteEnvironmentApplicationLock: could not select all env app locks for app '%v' on '%v': '%w'", c.Application, c.Environment, err)
		}
		var locks []string
		var version = int64(db.InitialEslVersion)
		if allAppLocks != nil {
			locks = db.Remove(allAppLocks.AppLocks, c.LockId)
			version = allAppLocks.Version
		}

		err = state.DBHandler.DBWriteAllAppLocks(ctx, transaction, version, c.Environment, c.Application, locks)
		if err != nil {
			return "", fmt.Errorf("DeleteEnvironmentApplicationLock: could not write app locks for app '%v' on '%v': '%w'", c.Application, c.Environment, err)
		}
	} else {
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

		queueMessage, err = state.ProcessQueue(ctx, transaction, fs, c.Environment, c.Application)
		if err != nil {
			return "", err
		}
		if err := state.DeleteAppLockIfEmpty(ctx, c.Environment, c.Application); err != nil {
			return "", err
		}

		GaugeEnvAppLockMetric(ctx, state, transaction, c.Environment, c.Application)
	}

	return fmt.Sprintf("Deleted lock %q on environment %q for application %q%s", c.LockId, c.Environment, c.Application, queueMessage), nil
}

type CreateEnvironmentTeamLock struct {
	Authentication        `json:"-"`
	Environment           string           `json:"env"`
	Team                  string           `json:"team"`
	LockId                string           `json:"lockId"`
	Message               string           `json:"message"`
	CiLink                string           `json:"ciLink"`
	AllowedDomains        []string         `json:"-"`
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (c *CreateEnvironmentTeamLock) GetDBEventType() db.EventType {
	return db.EvtCreateEnvironmentTeamLock
}

func (c *CreateEnvironmentTeamLock) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
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

	if c.CiLink != "" {
		if !state.DBHandler.ShouldUseOtherTables() {
			return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Ci Link is only supported when database is fully enabled."))
		} else if !isValidLink(c.CiLink, c.AllowedDomains) {
			return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Provided CI Link: %s is not valid or does not match any of the allowed domain", c.CiLink))
		}
	}
	if state.DBHandler.ShouldUseOtherTables() {
		user, err := auth.ReadUserFromContext(ctx)
		if err != nil {
			return "", err
		}
		//Write to locks table
		now, err := state.DBHandler.DBReadTransactionTimestamp(ctx, transaction)
		if err != nil {
			return "", fmt.Errorf("could not get transaction timestamp")
		}

		errW := state.DBHandler.DBWriteTeamLock(ctx, transaction, c.LockId, c.Environment, c.Team, db.LockMetadata{
			CreatedByName:  user.Name,
			CreatedByEmail: user.Email,
			Message:        c.Message,
			CiLink:         c.CiLink,
			CreatedAt:      *now,
		})

		if errW != nil {
			return "", errW
		}

		//Add it to all locks
		allTeamLocks, err := state.DBHandler.DBSelectAllTeamLocks(ctx, transaction, c.Environment, c.Team)
		if err != nil {
			return "", err
		}

		if allTeamLocks == nil {
			allTeamLocks = &db.AllTeamLocksGo{
				Version: 1,
				AllTeamLocksJson: db.AllTeamLocksJson{
					TeamLocks: []string{},
				},
				Created:     *now,
				Environment: c.Environment,
				Team:        c.Team,
			}
		}

		if !slices.Contains(allTeamLocks.TeamLocks, c.LockId) {
			allTeamLocks.TeamLocks = append(allTeamLocks.TeamLocks, c.LockId)
			err := state.DBHandler.DBWriteAllTeamLocks(ctx, transaction, allTeamLocks.Version, c.Environment, c.Team, allTeamLocks.TeamLocks)
			if err != nil {
				return "", err
			}
		}
	} else {
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

		if apps, err := state.GetApplications(ctx, transaction); err == nil {
			for _, currentApp := range apps {
				currentTeamName, err := state.GetTeamName(ctx, transaction, currentApp)
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
			return "", &TeamNotFoundErr{err: fmt.Errorf("Team '%s' does not exist.", c.Team)}
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
		if err := createLock(ctx, chroot, c.LockId, c.Message); err != nil {
			return "", fmt.Errorf("error creating lock. ID: %s Lock Message: %s. %w", c.LockId, c.Message, err)
		}
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
	if state.DBHandler.ShouldUseOtherTables() {
		err := state.DBHandler.DBDeleteTeamLock(ctx, transaction, c.Environment, c.Team, c.LockId)
		if err != nil {
			return "", err
		}
		allTeamLocks, err := state.DBHandler.DBSelectAllTeamLocks(ctx, transaction, c.Environment, c.Team)
		if err != nil {
			return "", fmt.Errorf("DeleteEnvironmentTeamLock: could not select all env team locks for team '%v' on '%v': '%w'", c.Team, c.Environment, err)
		}
		var locks []string
		if allTeamLocks != nil {
			locks = db.Remove(allTeamLocks.TeamLocks, c.LockId)
		}

		err = state.DBHandler.DBWriteAllTeamLocks(ctx, transaction, allTeamLocks.Version, c.Environment, c.Team, locks)
		if err != nil {
			return "", fmt.Errorf("DeleteEnvironmentTeamLock: could not write team locks for team '%v' on '%v': '%w'", c.Team, c.Environment, err)
		}
	} else {

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
		_, err = fs.Stat(lockDir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return "", grpc.FailedPrecondition(ctx, fmt.Errorf("directory %s for team lock does not exist", lockDir))
			}
			return "", err
		}
		if err := fs.Remove(lockDir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("failed to delete directory %q: %w", lockDir, err)
		}

		if err := state.DeleteTeamLockIfEmpty(ctx, c.Environment, c.Team); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("Deleted lock %q on environment %q for team %q", c.LockId, c.Environment, c.Team), nil
}

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
	if state.DBHandler.ShouldUseOtherTables() {
		// write to environments table
		environmentApplications := make([]string, 0)
		err = state.DBInsertEnvironmentWithOverview(ctx, transaction, c.Environment, c.Config, environmentApplications)
		if err != nil {
			return "", fmt.Errorf("unable to write to the environment table, error: %w", err)
		}
		// write to all_environments table
		allEnvironments, err := state.DBHandler.DBSelectAllEnvironments(ctx, transaction)

		if err != nil {
			return "", fmt.Errorf("unable to read from all_environments table, error: %w", err)
		}

		if allEnvironments == nil {
			//exhaustruct:ignore
			allEnvironments = &db.DBAllEnvironments{}
		}

		if !slices.Contains(allEnvironments.Environments, c.Environment) {
			// this environment is new
			allEnvironments.Environments = append(allEnvironments.Environments, c.Environment)
			err = state.DBHandler.DBWriteAllEnvironments(ctx, transaction, allEnvironments.Environments)

			if err != nil {
				return "", fmt.Errorf("unable to write to all_environments table, error: %w", err)
			}
		}
		overview, err := state.DBHandler.ReadLatestOverviewCache(ctx, transaction)
		if overview == nil {
			overview = &api.GetOverviewResponse{
				Branch:            "",
				ManifestRepoUrl:   "",
				EnvironmentGroups: []*api.EnvironmentGroup{},
				GitRevision:       "0000000000000000000000000000000000000000",
				LightweightApps:   make([]*api.OverviewApplication, 0),
			}
		}
		if err != nil {
			return "", fmt.Errorf("Unable to read overview cache, error: %w", err)
		}
		err = state.UpdateEnvironmentsInOverview(ctx, transaction, overview)
		if err != nil {
			return "", fmt.Errorf("Unable to udpate overview cache, error: %w", err)
		}
		err = state.DBHandler.WriteOverviewCache(ctx, transaction, overview)
		if err != nil {
			return "", fmt.Errorf("Unable to write overview cache, error: %w", err)
		}

		//Should be empty on new environments
		envApps, err := state.GetEnvironmentApplications(ctx, transaction, c.Environment)
		if err != nil {
			return "", fmt.Errorf("Unable to read environment, error: %w", err)

		}
		for _, app := range envApps {
			t.AddAppEnv(app, c.Environment, "")
		}

	} else {
		fs := state.Filesystem
		envDir := fs.Join("environments", c.Environment)
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
	}
	// we do not need to inform argoCd when creating an environment, as there are no apps yet
	return fmt.Sprintf("create environment %q", c.Environment), nil
}

type QueueApplicationVersion struct {
	Environment  string
	Application  string
	Version      uint64
	SkipOverview bool
}

func (c *QueueApplicationVersion) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	if state.DBHandler.ShouldUseOtherTables() {
		version := int64(c.Version)
		err := state.DBHandler.DBWriteDeploymentAttempt(ctx, transaction, c.Environment, c.Application, &version, c.SkipOverview)
		if err != nil {
			return "", err
		}
	} else {
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
	SkipOverview          bool                            `json:"-"`
}

func (c *DeployApplicationVersion) GetDBEventType() db.EventType {
	return db.EvtDeployApplicationVersion
}

func (c *DeployApplicationVersion) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
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
	span, ctx := tracer.StartSpanFromContext(ctx, "DeployApplicationVersion")
	defer span.Finish()

	err := state.checkUserPermissions(ctx, transaction, c.Environment, c.Application, auth.PermissionDeployRelease, "", c.RBACConfig, true)
	if err != nil {
		return "", err
	}
	fs := state.Filesystem

	var manifestContent []byte
	releaseDir := releasesDirectoryWithVersion(fs, c.Application, c.Version)
	if state.DBHandler.ShouldUseOtherTables() {
		version, err := state.DBHandler.DBSelectReleaseByVersion(ctx, transaction, c.Application, c.Version, true)
		if err != nil {
			return "", err
		}
		manifestContent = []byte(version.Manifests.Manifests[c.Environment])
	} else {
		// Check that the release exist and fetch manifest
		manifest := fs.Join(releaseDir, "environments", c.Environment, "manifests.yaml")
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
	}
	lockPreventedDeployment := false
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

		appDir := applicationDirectory(fs, c.Application)

		team, err := util.ReadFile(fs, fs.Join(appDir, "team"))

		if errors.Is(err, os.ErrNotExist) {
			teamLocks = map[string]Lock{} //If we do not find the team file, there is no team for application, meaning there can't be any team locks
		} else {
			teamLocks, err = state.GetEnvironmentTeamLocks(ctx, transaction, c.Environment, string(team))
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
				ev := createLockPreventedDeploymentEvent(c.Application, c.Environment, lockMsg, lockType)
				if state.DBHandler.ShouldUseOtherTables() {
					newReleaseCommitId, err := getCommitID(ctx, transaction, state, fs, c.Version, releaseDir, c.Application)
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
				} else {
					if err := addEventForRelease(ctx, fs, releaseDir, ev); err != nil {
						return "", err
					}
				}
				lockPreventedDeployment = true
			}
			switch c.LockBehaviour {
			case api.LockBehavior_RECORD:
				q := QueueApplicationVersion{
					Environment:  c.Environment,
					Application:  c.Application,
					Version:      c.Version,
					SkipOverview: c.SkipOverview,
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

	applicationDir := fs.Join("environments", c.Environment, "applications", c.Application)
	firstDeployment := false
	versionFile := fs.Join(applicationDir, "version")
	oldReleaseDir := ""
	var oldVersion *int64

	if state.CloudRunClient != nil {
		err := state.CloudRunClient.DeployApplicationVersion(ctx, manifestContent)
		if err != nil {
			return "", err
		}
	}
	if state.DBHandler.ShouldUseOtherTables() {
		existingDeployment, err := state.DBHandler.DBSelectLatestDeployment(ctx, transaction, c.Application, c.Environment)
		if err != nil {
			return "", err
		}
		if existingDeployment.Version == nil {
			firstDeployment = true
		} else {
			oldVersion = existingDeployment.Version
		}
		if err != nil {
			return "", fmt.Errorf("could not find deployment for app %s and env %s", c.Application, c.Environment)
		}
		var v = int64(c.Version)
		newDeployment := db.Deployment{
			EslVersion:    0,
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
		var previousVersion db.EslVersion
		if existingDeployment == nil {
			previousVersion = 0
		} else {
			previousVersion = existingDeployment.EslVersion
		}
		err = state.DBHandler.DBWriteDeployment(ctx, transaction, newDeployment, previousVersion, c.SkipOverview)
		if err != nil {
			return "", fmt.Errorf("could not write deployment for %v - %v", newDeployment, err)
		}
		err = state.DBHandler.DBUpdateAllDeploymentsForApp(ctx, transaction, c.Application, c.Environment, int64(c.Version))
		if err != nil {
			return "", fmt.Errorf("could not write oldest deployment for %v - %v", newDeployment, err)
		}
	} else {
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

		if err := util.WriteFile(fs, fs.Join(applicationDir, "deployed_by"), []byte(user.Name), 0666); err != nil {
			return "", err
		}
		if err := util.WriteFile(fs, fs.Join(applicationDir, "deployed_by_email"), []byte(user.Email), 0666); err != nil {
			return "", err
		}

		if err := util.WriteFile(fs, fs.Join(applicationDir, "deployed_at_utc"), []byte(time2.GetTimeNow(ctx).UTC().String()), 0666); err != nil {
			return "", err
		}
	}
	teamOwner, err := state.GetApplicationTeamOwner(ctx, transaction, c.Application)
	if err != nil {
		return "", err
	}
	t.AddAppEnv(c.Application, c.Environment, teamOwner)
	s := State{
		Commit:               nil,
		MinorRegexes:         state.MinorRegexes,
		Filesystem:           fs,
		DBHandler:            state.DBHandler,
		ReleaseVersionsLimit: state.ReleaseVersionsLimit,
		CloudRunClient:       state.CloudRunClient,
	}
	err = s.DeleteQueuedVersionIfExists(ctx, transaction, c.Environment, c.Application, c.SkipOverview)
	if err != nil {
		return "", err
	}
	if !c.SkipOverview {
		d := &CleanupOldApplicationVersions{
			Application:           c.Application,
			TransformerEslVersion: c.TransformerEslVersion,
		}

		if err := t.Execute(ctx, d, transaction); err != nil {
			return "", err
		}
	}
	if c.WriteCommitData { // write the corresponding event
		newReleaseCommitId, err := getCommitID(ctx, transaction, state, fs, c.Version, releaseDir, c.Application)
		deploymentEvent := createDeploymentEvent(c.Application, c.Environment, c.SourceTrain)
		if s.DBHandler.ShouldUseOtherTables() {

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
		} else {
			if err := addEventForRelease(ctx, fs, releaseDir, deploymentEvent); err != nil {
				return "", GetCreateReleaseGeneralFailure(err)
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
				if s.DBHandler.ShouldUseOtherTables() {
					if oldVersion == nil {
						logger.FromContext(ctx).Sugar().Errorf("did not find old version of app %s - skipping replaced-by event", c.Application)
					} else {
						gen := getGenerator(ctx)
						eventUuid := gen.Generate()
						v := uint64(*oldVersion)
						oldReleaseCommitId, err := getCommitID(ctx, transaction, state, fs, v, oldReleaseDir, c.Application)
						if err != nil {
							logger.FromContext(ctx).Sugar().Warnf("could not find commit for release %d of app %s - skipping replaced-by event", v, c.Application)
						} else {
							err = state.DBHandler.DBWriteReplacedByEvent(ctx, transaction, c.TransformerEslVersion, eventUuid, oldReleaseCommitId, ev)
							if err != nil {
								return "", err
							}
						}
					}
				} else {
					if err := addEventForRelease(ctx, fs, oldReleaseDir, ev); err != nil {
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

func getCommitID(ctx context.Context, transaction *sql.Tx, state *State, fs billy.Filesystem, release uint64, releaseDir string, app string) (string, error) {
	if state.DBHandler.ShouldUseOtherTables() {
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
	} else {
		return getCommitIDFromReleaseDir(ctx, fs, releaseDir)
	}
}

func getCommitIDFromReleaseDir(ctx context.Context, fs billy.Filesystem, releaseDir string) (string, error) {
	commitIdPath := fs.Join(releaseDir, "source_commit_id")

	commitIDBytes, err := util.ReadFile(fs, commitIdPath)
	if err != nil {
		logger.FromContext(ctx).Sugar().Infof(
			"Error while reading source commit ID file at %s, error %w"+
				". Deployment event not stored.",
			commitIdPath, err)
		return "", err
	}
	commitID := string(commitIDBytes)
	// if the stored source commit ID is invalid then we will not be able to store the event (simply)
	return commitID, nil
}

func addEventForRelease(ctx context.Context, fs billy.Filesystem, releaseDir string, ev event.Event) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "eventsForRelease")
	defer span.Finish()
	if commitID, err := getCommitIDFromReleaseDir(ctx, fs, releaseDir); err == nil {
		gen := getGenerator(ctx)
		eventUuid := gen.Generate()

		if !valid.SHA1CommitID(commitID) {
			logger.FromContext(ctx).Sugar().Infof(
				"The source commit ID %s is not a valid/complete SHA1 hash, event cannot be stored.",
				commitID)
			return nil
		}

		if err := writeEvent(ctx, eventUuid, commitID, fs, ev); err != nil {
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

type Overview struct {
	App     string
	Version uint64
}

func getOverrideVersions(ctx context.Context, transaction *sql.Tx, commitHash, upstreamEnvName string, repo Repository) (resp []Overview, err error) {
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
	envs, err := s.GetAllEnvironmentConfigs(ctx, transaction)
	if err != nil {
		return nil, fmt.Errorf("unable to get EnvironmentConfigs for %s: %w", commitHash, err)
	}
	for envName, config := range envs {
		var groupName = mapper.DeriveGroupName(config, envName)
		if upstreamEnvName != envName && groupName != envName {
			continue
		}
		apps, err := s.GetEnvironmentApplications(ctx, transaction, envName)
		if err != nil {
			return nil, fmt.Errorf("unable to get EnvironmentApplication for env %s: %w", envName, err)
		}
		for _, appName := range apps {
			version, err := s.GetEnvironmentApplicationVersion(ctx, transaction, envName, appName)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("unable to get EnvironmentApplicationVersion for %s: %w", appName, err)
			}
			if version == nil {
				continue
			}

			resp = append(resp, Overview{App: appName, Version: *version})
		}
	}
	return resp, nil
}

func (c *ReleaseTrain) getUpstreamLatestApp(ctx context.Context, transaction *sql.Tx, upstreamLatest bool, state *State, upstreamEnvName, source, commitHash string, targetEnv string) (apps []string, appVersions []Overview, err error) {
	if commitHash != "" {
		appVersions, err := getOverrideVersions(ctx, transaction, c.CommitHash, upstreamEnvName, c.Repo)
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
			Parent:                c,
			Env:                   envName,
			EnvConfigs:            configs,
			EnvGroupConfigs:       envGroupConfigs,
			WriteCommitData:       c.WriteCommitData,
			TrainGroup:            trainGroup,
			TransformerEslVersion: c.TransformerEslVersion,
			CiLink:                c.CiLink,
		}

		envPrognosis := envReleaseTrain.prognosis(ctx, state, transaction)

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
	transaction *sql.Tx,
) (string, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "ReleaseTrain")
	defer span.Finish()
	//Prognosis can be a costly operation. Abort straight away if ci link is not valid
	if c.CiLink != "" {
		if !state.DBHandler.ShouldUseOtherTables() {
			return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Ci Link is only supported when database is fully enabled."))
		} else if !isValidLink(c.CiLink, c.AllowedDomains) {
			return "", grpc.FailedPrecondition(ctx, fmt.Errorf("Provided CI Link: %s is not valid or does not match any of the allowed domain", c.CiLink))
		}
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

	for _, envName := range envNames {
		var trainGroup *string
		if isEnvGroup {
			trainGroup = conversion.FromString(targetGroupName)
		}

		if err := t.Execute(ctx, &envReleaseTrain{
			Parent:                c,
			Env:                   envName,
			EnvConfigs:            configs,
			EnvGroupConfigs:       envGroupConfigs,
			WriteCommitData:       c.WriteCommitData,
			TrainGroup:            trainGroup,
			TransformerEslVersion: c.TransformerEslVersion,
			CiLink:                c.CiLink,
		}, transaction); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf(
		"Release Train to environment/environment group '%s':\n",
		targetGroupName), nil
}

type envReleaseTrain struct {
	Parent                *ReleaseTrain
	Env                   string
	EnvConfigs            map[string]config.EnvironmentConfig
	EnvGroupConfigs       map[string]config.EnvironmentConfig
	WriteCommitData       bool
	TrainGroup            *string
	TransformerEslVersion db.TransformerID
	CiLink                string
}

func (c *envReleaseTrain) GetDBEventType() db.EventType {
	panic("envReleaseTrain GetDBEventType")
}

func (c *envReleaseTrain) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *envReleaseTrain) prognosis(
	ctx context.Context,
	state *State,
	transaction *sql.Tx,
) ReleaseTrainEnvironmentPrognosis {
	span, ctx := tracer.StartSpanFromContext(ctx, "EnvReleaseTrain Prognosis")
	defer span.Finish()
	span.SetTag("env", c.Env)

	envConfig := c.EnvGroupConfigs[c.Env]
	if envConfig.Upstream == nil {
		return ReleaseTrainEnvironmentPrognosis{
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
		return ReleaseTrainEnvironmentPrognosis{
			SkipCause:     nil,
			Error:         err,
			Locks:         nil,
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
			Locks:         nil,
			AppsPrognoses: nil,
		}
	}

	if upstreamLatest && upstreamEnvName != "" {
		return ReleaseTrainEnvironmentPrognosis{
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
			return ReleaseTrainEnvironmentPrognosis{
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
		return ReleaseTrainEnvironmentPrognosis{
			SkipCause:     nil,
			Error:         grpc.InternalError(ctx, fmt.Errorf("could not get lock for environment %q: %w", c.Env, err)),
			Locks:         nil,
			AppsPrognoses: nil,
		}
	}

	source := upstreamEnvName
	if upstreamLatest {
		source = "latest"
	}

	apps, overrideVersions, err := c.Parent.getUpstreamLatestApp(ctx, transaction, upstreamLatest, state, upstreamEnvName, source, c.Parent.CommitHash, c.Env)
	if err != nil {
		return ReleaseTrainEnvironmentPrognosis{
			SkipCause:     nil,
			Error:         err,
			Locks:         nil,
			AppsPrognoses: nil,
		}
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
				LockId: lockId,
			}
			locksList = append(locksList, newLock)
		}

		for _, appName := range apps {
			appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
				SkipCause: nil,
				Locks:     locksList,
				Version:   0,
			}
		}
		return ReleaseTrainEnvironmentPrognosis{
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
		return ReleaseTrainEnvironmentPrognosis{
			SkipCause:     nil,
			Error:         grpc.PublicError(ctx, fmt.Errorf("Could not obtain latest deployments for env %s: %w", c.Env, err)),
			Locks:         nil,
			AppsPrognoses: nil,
		}
	}

	allLatestDeploymentsUpstreamEnv, err := state.GetAllLatestDeployments(ctx, transaction, upstreamEnvName, apps)

	if err != nil {
		return ReleaseTrainEnvironmentPrognosis{
			SkipCause:     nil,
			Error:         grpc.PublicError(ctx, fmt.Errorf("Could not obtain latest deployments for env %s: %w", c.Env, err)),
			Locks:         nil,
			AppsPrognoses: nil,
		}
	}

	allLatestReleases, err := state.GetAllLatestReleases(ctx, transaction, apps)
	if err != nil {
		return ReleaseTrainEnvironmentPrognosis{
			SkipCause:     nil,
			Error:         grpc.PublicError(ctx, fmt.Errorf("Error getting all releases of all apps: %w", err)),
			Locks:         nil,
			AppsPrognoses: nil,
		}
	}
	var allLatestReleaseEnvironments map[string]map[uint64][]string
	if state.DBHandler.ShouldUseOtherTables() {
		allLatestReleaseEnvironments, err = state.DBHandler.DBSelectAllManifestsForAllReleases(ctx, transaction)
	}
	if err != nil {
		return ReleaseTrainEnvironmentPrognosis{
			SkipCause:     nil,
			Error:         grpc.PublicError(ctx, fmt.Errorf("Error getting all releases of all apps: %w", err)),
			Locks:         nil,
			AppsPrognoses: nil,
		}
	}
	span.SetTag("ConsideredApps", len(apps))
	for _, appName := range apps {
		if c.Parent.Team != "" {
			if team, err := state.GetApplicationTeamOwner(ctx, transaction, appName); err != nil {
				return ReleaseTrainEnvironmentPrognosis{
					SkipCause:     nil,
					Error:         err,
					Locks:         nil,
					AppsPrognoses: nil,
				}
			} else if c.Parent.Team != team {
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
			}
			continue
		}

		appLocks, err := state.GetEnvironmentApplicationLocks(ctx, transaction, c.Env, appName)

		if err != nil {
			return ReleaseTrainEnvironmentPrognosis{
				SkipCause:     nil,
				Error:         err,
				Locks:         nil,
				AppsPrognoses: nil,
			}
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
					LockId: lockId,
				}
				locksList = append(locksList, newLock)
			}

			appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
				SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
					SkipCause: api.ReleaseTrainAppSkipCause_APP_IS_LOCKED,
				},
				Locks:   locksList,
				Version: 0,
			}
			continue
		}

		if state.DBHandler.ShouldUseOtherTables() {
			releaseEnvs, exists := allLatestReleaseEnvironments[appName][versionToDeploy]
			if !exists {
				return ReleaseTrainEnvironmentPrognosis{
					SkipCause:     nil,
					Error:         fmt.Errorf("No release found for app %s and versionToDeploy %d", appName, versionToDeploy),
					Locks:         nil,
					AppsPrognoses: nil,
				}
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
				}
				continue
			}
		} else {
			fs := state.Filesystem

			releaseDir := releasesDirectoryWithVersion(fs, appName, versionToDeploy)

			manifest := fs.Join(releaseDir, "environments", c.Env, "manifests.yaml")
			if _, err := fs.Stat(manifest); err != nil {
				appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
					SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
						SkipCause: api.ReleaseTrainAppSkipCause_APP_DOES_NOT_EXIST_IN_ENV,
					},
					Locks:   nil,
					Version: 0,
				}
				continue
			}
		}

		teamName, err := state.GetTeamName(ctx, transaction, appName)

		if err == nil { //IF we find information for team

			err := state.checkUserPermissions(ctx, transaction, c.Env, "*", auth.PermissionDeployReleaseTrain, teamName, c.Parent.RBACConfig, true)

			if err != nil {
				appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
					SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
						SkipCause: api.ReleaseTrainAppSkipCause_NO_TEAM_PERMISSION,
					},
					Locks:   nil,
					Version: 0,
				}
				continue
			}

			teamLocks, err := state.GetEnvironmentTeamLocks(ctx, transaction, c.Env, teamName)

			if err != nil {
				return ReleaseTrainEnvironmentPrognosis{
					SkipCause:     nil,
					Error:         err,
					Locks:         nil,
					AppsPrognoses: nil,
				}
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
						LockId: lockId,
					}
					locksList = append(locksList, newLock)
				}

				appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
					SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
						SkipCause: api.ReleaseTrainAppSkipCause_TEAM_IS_LOCKED,
					},
					Locks:   locksList,
					Version: 0,
				}
				continue
			}
		}
		appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
			SkipCause: nil,
			Locks:     nil,
			Version:   versionToDeploy,
		}
	}
	return ReleaseTrainEnvironmentPrognosis{
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
		currentlyDeployedVersion, _ := state.GetEnvironmentApplicationVersion(ctx, transaction, c.Env, appName)
		teamName, _ := state.GetTeamName(ctx, transaction, appName)
		switch SkipCause.SkipCause {
		case api.ReleaseTrainAppSkipCause_APP_HAS_NO_VERSION_IN_UPSTREAM_ENV:
			return fmt.Sprintf("skipping because there is no version for application %q in env %q \n", appName, upstreamEnvName)
		case api.ReleaseTrainAppSkipCause_APP_ALREADY_IN_UPSTREAM_VERSION:
			return fmt.Sprintf("skipping %q because it is already in the version %d\n", appName, currentlyDeployedVersion)
		case api.ReleaseTrainAppSkipCause_APP_IS_LOCKED:
			return fmt.Sprintf("skipping application %q in environment %q due to application lock", appName, c.Env)
		case api.ReleaseTrainAppSkipCause_APP_DOES_NOT_EXIST_IN_ENV:
			return fmt.Sprintf("skipping application %q in environment %q because it doesn't exist there", appName, c.Env)
		case api.ReleaseTrainAppSkipCause_TEAM_IS_LOCKED:
			return fmt.Sprintf("skipping application %q in environment %q due to team lock on team %q", appName, c.Env, teamName)
		case api.ReleaseTrainAppSkipCause_NO_TEAM_PERMISSION:
			return fmt.Sprintf("skipping application %q in environment %q because the user team %q is not the same as the apllication", appName, c.Env, teamName)
		default:
			return fmt.Sprintf("skipping application %q in environment %q for an unrecognized reason", appName, c.Env)
		}
	}

	prognosis := c.prognosis(ctx, state, transaction)

	if prognosis.Error != nil {
		return "", prognosis.Error
	}
	if prognosis.SkipCause != nil {
		if !c.WriteCommitData {
			return renderEnvironmentSkipCause(prognosis.SkipCause), nil
		}
		for appName := range prognosis.AppsPrognoses {
			release, err := state.GetLastRelease(ctx, transaction, state.Filesystem, appName)
			if err != nil {
				return "", fmt.Errorf("error getting latest release for app '%s' - %v", appName, err)
			}
			releaseDir := releasesDirectoryWithVersion(state.Filesystem, appName, release)
			eventMessage := ""
			if len(prognosis.Locks) > 0 {
				eventMessage = prognosis.Locks[0].Message
			}
			newEvent := createLockPreventedDeploymentEvent(appName, c.Env, eventMessage, "environment")
			if state.DBHandler.ShouldUseOtherTables() {
				commitID, err := getCommitID(ctx, transaction, state, state.Filesystem, release, releaseDir, appName)
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
			} else {
				if err := addEventForRelease(ctx, state.Filesystem, releaseDir, newEvent); err != nil {
					return "", err
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

	var overview *api.GetOverviewResponse

	span.SetTag("ConsideredApps", len(appNames))
	var deployCounter uint = 0
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
			Author:                "",
			TransformerEslVersion: c.TransformerEslVersion,
			CiLink:                c.CiLink,
			SkipOverview:          true,
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

	if state.DBHandler.ShouldUseOtherTables() {
		err := state.DBHandler.WriteOverviewCache(ctx, transaction, overview)
		if err != nil {
			return "", grpc.InternalError(ctx, fmt.Errorf("unexpected error for env=%s while writing overview cache: %w", c.Env, err))
		}
	}

	return fmt.Sprintf("Release Train to '%s' environment:\n\n"+
		"The release train deployed %d services from '%s' to '%s'%s",
		c.Env, deployedApps, source, c.Env, teamInfo,
	), nil
}

// skippedServices is a helper Transformer to generate the "skipped
// services" commit log.
type skippedServices struct {
	Messages              []string
	TransformerEslVersion db.TransformerID `json:"-"` // Tags the transformer with EventSourcingLight eslVersion

}

func (c *skippedServices) GetDBEventType() db.EventType {
	panic("GetDBEventType for skippedServices")
}

func (c *skippedServices) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
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
	panic("GetDBEventType for skippedService")
}

func (c *skippedService) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *skippedService) Transform(_ context.Context, _ *State, _ TransformerContext, _ *sql.Tx) (string, error) {
	return c.Message, nil
}
