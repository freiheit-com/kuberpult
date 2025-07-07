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
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/conversion"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/sorting"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"math"
	"sort"
	"time"
)

func createLockPreventedDeploymentEvent(application string, environment types.EnvName, lockMsg, lockType string) *event.LockPreventedDeployment {
	ev := event.LockPreventedDeployment{
		Application: application,
		Environment: string(environment),
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

func (c *ReleaseTrain) getUpstreamLatestApp(ctx context.Context, transaction *sql.Tx, upstreamLatest bool, state *State, upstreamEnvName types.EnvName, source types.EnvName, commitHash string, targetEnv types.EnvName) (apps []string, appVersions []Overview, err error) {
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
		apps, err = state.GetEnvironmentApplications(ctx, transaction, types.EnvName(targetEnv))
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

type ReleaseTrainApplicationPrognosis struct {
	SkipCause *api.ReleaseTrainAppPrognosis_SkipCause
	EnvLocks  map[string]*api.Lock
	TeamLocks map[string]*api.Lock
	AppLocks  map[string]*api.Lock

	Version            uint64
	Team               string
	NewReleaseCommitId string
	ExistingDeployment *db.Deployment
	OldReleaseCommitId string
}

type ReleaseTrainEnvironmentPrognosis struct {
	SkipCause *api.ReleaseTrainEnvPrognosis_SkipCause
	Error     error
	EnvLocks  map[string]*api.Lock

	AppsPrognoses        map[string]ReleaseTrainApplicationPrognosis // map key is app name
	AllLatestDeployments map[string]*int64                           // map key is app name
}

type ReleaseTrainPrognosisOutcome = uint64

type ReleaseTrainPrognosis struct {
	Error                error
	EnvironmentPrognoses map[types.EnvName]ReleaseTrainEnvironmentPrognosis
}

func failedPrognosis(err error) *ReleaseTrainEnvironmentPrognosis {
	return &ReleaseTrainEnvironmentPrognosis{
		SkipCause:            nil,
		Error:                err,
		EnvLocks:             nil,
		AppsPrognoses:        nil,
		AllLatestDeployments: nil,
	}
}

func (c *ReleaseTrain) Prognosis(
	ctx context.Context,
	state *State,
	transaction *sql.Tx,
	configs map[types.EnvName]config.EnvironmentConfig,
) ReleaseTrainPrognosis {
	span, ctx := tracer.StartSpanFromContext(ctx, "ReleaseTrain Prognosis")
	defer span.Finish()
	span.SetTag("targetEnv", c.Target)
	span.SetTag("targetType", c.TargetType)
	span.SetTag("team", c.Team)

	var targetGroupName = c.Target
	var envGroupConfigs, isEnvGroup = GetEnvironmentGroupsEnvironmentsOrEnvironment(configs, targetGroupName, c.TargetType)
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
	envGroups := make([]types.EnvName, 0, len(envGroupConfigs))
	for env := range envGroupConfigs {
		envGroups = append(envGroups, env)
	}
	types.Sort(envGroups)

	envPrognoses := make(map[types.EnvName]ReleaseTrainEnvironmentPrognosis)
	for _, envName := range envGroups {
		var trainGroup *string
		if isEnvGroup {
			trainGroup = conversion.FromString(targetGroupName)
		}

		envReleaseTrain := &envReleaseTrain{
			Parent:                       c,
			Env:                          envName,
			AllEnvConfigs:                configs,
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
	var envGroupConfigs, _ = GetEnvironmentGroupsEnvironmentsOrEnvironment(configs, targetGroupName, c.TargetType)
	if len(envGroupConfigs) == 0 {
		if c.TargetType == api.ReleaseTrainRequest_ENVIRONMENT.String() || c.TargetType == api.ReleaseTrainRequest_ENVIRONMENTGROUP.String() {
			return "", grpc.PublicError(ctx, fmt.Errorf("could not find target of type %v and name '%v'", c.TargetType, targetGroupName))
		}
		return "", grpc.PublicError(ctx, fmt.Errorf("could not find environment group or environment configs for '%v'", targetGroupName))
	}

	// sorting for determinism
	envNames := make([]types.EnvName, 0, len(envGroupConfigs))
	for env := range envGroupConfigs {
		envNames = append(envNames, env)
	}
	types.Sort(envNames)
	span.SetTag("environments", len(envNames))

	allLatestReleases, err := state.GetAllLatestReleases(ctx, transaction, nil)
	if err != nil {
		return "", grpc.PublicError(ctx, fmt.Errorf("could not get all releases of all apps %w", err))
	}

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
				return c.runEnvReleaseTrainBackground(ctx, state, transformerContext, envNameLocal, trainGroup, configs, allLatestReleases)
			})
		}
	}
	err = releaseTrainErrGroup.Wait()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"Release Train to environment/environment group '%s':\n",
		targetGroupName), nil
}

func (c *ReleaseTrain) runWithNewGoRoutines(
	envNames []types.EnvName,
	targetGroupName string,
	maxThreads int,
	parentCtx context.Context,
	configs map[types.EnvName]config.EnvironmentConfig,
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
	var allReleasesEnvironments db.AppVersionEnvironments
	var err error
	allReleasesEnvironments, err = state.DBHandler.DBSelectAllEnvironmentsForAllReleases(ctxManifests, transaction)
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
			envNameLocal := types.EnvName(envName)
			// we want to schedule all go routines,
			// but still have the limit set, so "group.Go" would block
			group.Go(func() error {
				span, ctx, onErr := tracing.StartSpanFromContext(ctxRoutines, "EnvReleaseTrain Transform")
				span.SetTag("kuberpultEnvironment", envName)
				defer span.Finish()

				train := &envReleaseTrain{
					Parent:                c,
					Env:                   envName,
					AllEnvConfigs:         configs,
					WriteCommitData:       c.WriteCommitData,
					TrainGroup:            trainGroup,
					TransformerEslVersion: c.TransformerEslVersion,
					CiLink:                c.CiLink,

					AllLatestReleasesCache:       allLatestReleases,
					AllLatestReleaseEnvironments: allReleasesEnvironments,
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
			return "", onErrOne(onErrAll(fmt.Errorf("prognosis could not be applied for env '%s': %w", result.train.Env, err)))
		}
		i++
		spanOne.Finish()
	}

	return "", nil
}

func (c *ReleaseTrain) runEnvReleaseTrainBackground(ctx context.Context, state *State, t TransformerContext, envName types.EnvName, trainGroup *string, configs map[types.EnvName]config.EnvironmentConfig, releases map[string][]int64) error {
	spanOne, ctx, onErr := tracing.StartSpanFromContext(ctx, "runEnvReleaseTrainBackground")
	spanOne.SetTag("kuberpultEnvironment", envName)
	defer spanOne.Finish()

	err := state.DBHandler.WithTransactionR(ctx, 2, false, func(ctx context.Context, transaction2 *sql.Tx) error {
		err := t.Execute(ctx, &envReleaseTrain{
			Parent:                c,
			Env:                   envName,
			AllEnvConfigs:         configs,
			WriteCommitData:       c.WriteCommitData,
			TrainGroup:            trainGroup,
			TransformerEslVersion: c.TransformerEslVersion,
			CiLink:                c.CiLink,

			AllLatestReleasesCache:       releases,
			AllLatestReleaseEnvironments: nil,
		}, transaction2)
		return err
	})
	return onErr(err)
}

func (c *envReleaseTrain) runEnvPrognosisBackground(
	ctx context.Context,
	state *State,
	envName types.EnvName,
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
	Env                   types.EnvName
	AllEnvConfigs         map[types.EnvName]config.EnvironmentConfig // all environments that exist
	WriteCommitData       bool
	TrainGroup            *string
	TransformerEslVersion db.TransformerID
	CiLink                string

	AllLatestReleasesCache       AllLatestReleasesCache
	AllLatestReleaseEnvironments db.AppVersionEnvironments // can be prefilled with all manifests, so that each envReleaseTrain does not need to fetch them again
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

	envName := c.Env

	envConfig := c.AllEnvConfigs[envName]
	if envConfig.Upstream == nil {
		return &ReleaseTrainEnvironmentPrognosis{
			SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
				SkipCause: api.ReleaseTrainEnvSkipCause_ENV_HAS_NO_UPSTREAM,
			},
			Error:                nil,
			EnvLocks:             nil,
			AppsPrognoses:        nil,
			AllLatestDeployments: map[string]*int64{},
		}
	}

	err := state.checkUserPermissionsFromConfig(
		ctx,
		transaction,
		envName,
		"*",
		auth.PermissionDeployReleaseTrain,
		c.Parent.Team,
		c.Parent.RBACConfig,
		false,
		&envConfig,
	)

	if err != nil {
		return failedPrognosis(err)
	}

	upstreamLatest := envConfig.Upstream.Latest
	upstreamEnvName := types.EnvName(envConfig.Upstream.Environment)
	if !upstreamLatest && upstreamEnvName == "" {
		return &ReleaseTrainEnvironmentPrognosis{
			SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
				SkipCause: api.ReleaseTrainEnvSkipCause_ENV_HAS_NO_UPSTREAM_LATEST_OR_UPSTREAM_ENV,
			},
			Error:                nil,
			EnvLocks:             nil,
			AppsPrognoses:        nil,
			AllLatestDeployments: map[string]*int64{},
		}
	}

	if upstreamLatest && upstreamEnvName != "" {
		return &ReleaseTrainEnvironmentPrognosis{
			SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
				SkipCause: api.ReleaseTrainEnvSkipCause_ENV_HAS_BOTH_UPSTREAM_LATEST_AND_UPSTREAM_ENV,
			},
			Error:                nil,
			EnvLocks:             nil,
			AppsPrognoses:        nil,
			AllLatestDeployments: map[string]*int64{},
		}
	}

	if !upstreamLatest {
		_, ok := c.AllEnvConfigs[upstreamEnvName]
		if !ok {
			return &ReleaseTrainEnvironmentPrognosis{
				SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
					SkipCause: api.ReleaseTrainEnvSkipCause_UPSTREAM_ENV_CONFIG_NOT_FOUND,
				},
				Error:                nil,
				EnvLocks:             nil,
				AppsPrognoses:        nil,
				AllLatestDeployments: map[string]*int64{},
			}
		}
	}
	envLocks, err := state.GetEnvironmentLocksFromDB(ctx, transaction, envName)
	if err != nil {
		return failedPrognosis(grpc.InternalError(ctx, fmt.Errorf("could not get lock for environment %q: %w", envName, err)))
	}

	var source types.EnvName = upstreamEnvName
	if upstreamLatest {
		source = "latest"
	}

	apps, overrideVersions, err := c.Parent.getUpstreamLatestApp(ctx, transaction, upstreamLatest, state, upstreamEnvName, source, c.Parent.CommitHash, envName)
	if err != nil {
		return failedPrognosis(err)
	}
	sort.Strings(apps)

	appsPrognoses := make(map[string]ReleaseTrainApplicationPrognosis)

	allLatestDeploymentsTargetEnv, err := state.DBHandler.DBSelectAllLatestDeploymentsOnEnvironment(ctx, transaction, envName)
	if err != nil {
		return failedPrognosis(grpc.PublicError(ctx, fmt.Errorf("Could not obtain latest deployments for env %s: %w", envName, err)))
	}

	allLatestDeploymentsUpstreamEnv, err := state.GetAllLatestDeployments(ctx, transaction, upstreamEnvName, apps)
	if err != nil {
		return failedPrognosis(grpc.PublicError(ctx, fmt.Errorf("Could not obtain latest deployments for env %s: %w", envName, err)))
	}

	if len(envLocks) > 0 {
		envLocksMap := map[string]*api.Lock{}
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
			envLocksMap[lockId] = newLock
		}

		for _, appName := range apps {
			releases := c.AllLatestReleasesCache[appName]
			var release uint64
			if releases == nil {
				release = 0
			} else {
				release = uint64(releases[len(releases)-1])
			}
			commitID, err := getCommitID(ctx, transaction, state, release, appName)
			if err != nil {
				logger.FromContext(ctx).Sugar().Warnf("could not write event data - continuing. %v", fmt.Errorf("getCommitIDFromReleaseDir %v", err))
			}
			appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
				SkipCause:          nil,
				EnvLocks:           envLocksMap,
				TeamLocks:          nil,
				AppLocks:           nil,
				Version:            0,
				Team:               "",
				NewReleaseCommitId: commitID,
				ExistingDeployment: nil,
				OldReleaseCommitId: "",
			}
		}
		return &ReleaseTrainEnvironmentPrognosis{
			SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
				SkipCause: api.ReleaseTrainEnvSkipCause_ENV_IS_LOCKED,
			},
			Error:                nil,
			EnvLocks:             envLocksMap,
			AppsPrognoses:        appsPrognoses,
			AllLatestDeployments: allLatestDeploymentsTargetEnv,
		}
	}

	var allLatestReleaseEnvironments db.AppVersionEnvironments
	if c.AllLatestReleaseEnvironments == nil {
		allLatestReleaseEnvironments, err = state.DBHandler.DBSelectAllEnvironmentsForAllReleases(ctx, transaction)
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
		releases := c.AllLatestReleasesCache[appName]
		var release uint64
		if releases == nil {
			release = 0
		} else {
			release = uint64(releases[len(releases)-1])
		}
		commitID, err := getCommitID(ctx, transaction, state, release, appName)
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("could not write event data - continuing. %v", fmt.Errorf("getCommitIDFromReleaseDir %v", err))
		}

		if c.Parent.Team != "" {
			team, ok := allTeams[appName]
			if !ok {
				// If we cannot find the app in all teams, we cannot determine the team of the app.
				// This indicates an incorrect db state, and we just skip it:
				appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
					SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
						SkipCause: api.ReleaseTrainAppSkipCause_APP_WITHOUT_TEAM,
					},
					EnvLocks:           nil,
					TeamLocks:          nil,
					AppLocks:           nil,
					Version:            0,
					Team:               team,
					NewReleaseCommitId: commitID,
					ExistingDeployment: nil,
					OldReleaseCommitId: "",
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
				logger.FromContext(ctx).Sugar().Warnf("app %s appears to have no releases on env=%s, so it is skipped", appName, envName)
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
					EnvLocks:           nil,
					TeamLocks:          nil,
					AppLocks:           nil,
					Version:            0,
					Team:               "",
					NewReleaseCommitId: commitID,
					ExistingDeployment: nil,
					OldReleaseCommitId: "",
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
				EnvLocks:           nil,
				TeamLocks:          nil,
				AppLocks:           nil,
				Version:            0,
				Team:               "",
				NewReleaseCommitId: commitID,
				ExistingDeployment: nil,
				OldReleaseCommitId: "",
			}
			continue
		}

		appLocks, err := state.GetEnvironmentApplicationLocks(ctx, transaction, envName, appName)

		if err != nil {
			return failedPrognosis(err)
		}

		if len(appLocks) > 0 {
			appLocksMap := map[string]*api.Lock{}
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
				appLocksMap[lockId] = newLock
			}

			appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
				SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
					SkipCause: api.ReleaseTrainAppSkipCause_APP_IS_LOCKED,
				},
				EnvLocks:           nil,
				TeamLocks:          nil,
				AppLocks:           appLocksMap,
				Version:            0,
				Team:               "",
				NewReleaseCommitId: commitID,
				ExistingDeployment: nil,
				OldReleaseCommitId: "",
			}
			continue
		}

		releaseEnvs, exists := allLatestReleaseEnvironments[appName][versionToDeploy]
		if !exists {
			return failedPrognosis(fmt.Errorf("No release found for app %s and versionToDeploy %d", appName, versionToDeploy))
		}

		found := false
		for _, env := range releaseEnvs {
			if env == envName {
				found = true
				break
			}
		}
		if !found {
			appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
				SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
					SkipCause: api.ReleaseTrainAppSkipCause_APP_DOES_NOT_EXIST_IN_ENV,
				},
				EnvLocks:           nil,
				TeamLocks:          nil,
				AppLocks:           nil,
				Version:            0,
				Team:               "",
				NewReleaseCommitId: commitID,
				ExistingDeployment: nil,
				OldReleaseCommitId: "",
			}
			continue
		}

		teamName, ok := allTeams[appName]

		if ok { //IF we find information for team
			envConfig := c.AllEnvConfigs[c.Env]
			err = state.checkUserPermissionsFromConfig(ctx, transaction, envName, "*", auth.PermissionDeployReleaseTrain, teamName, c.Parent.RBACConfig, true, &envConfig)
			if err != nil {
				appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
					SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
						SkipCause: api.ReleaseTrainAppSkipCause_NO_TEAM_PERMISSION,
					},
					EnvLocks:           nil,
					TeamLocks:          nil,
					AppLocks:           nil,
					Version:            0,
					Team:               teamName,
					NewReleaseCommitId: commitID,
					ExistingDeployment: nil,
					OldReleaseCommitId: "",
				}
				continue
			}

			teamLocks, err := state.GetEnvironmentTeamLocks(ctx, transaction, c.Env, teamName)

			if err != nil {
				return failedPrognosis(err)
			}

			if len(teamLocks) > 0 {
				teamLocksMap := map[string]*api.Lock{}
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
					teamLocksMap[lockId] = newLock
				}

				appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
					SkipCause: &api.ReleaseTrainAppPrognosis_SkipCause{
						SkipCause: api.ReleaseTrainAppSkipCause_TEAM_IS_LOCKED,
					},
					EnvLocks:           nil,
					TeamLocks:          teamLocksMap,
					AppLocks:           nil,
					Version:            0,
					Team:               teamName,
					NewReleaseCommitId: commitID,
					ExistingDeployment: nil,
					OldReleaseCommitId: "",
				}
				continue
			}
		}

		existingDeployment, err := state.DBHandler.DBSelectLatestDeployment(ctx, transaction, appName, c.Env)
		if err != nil {
			return failedPrognosis(err)
		}

		var existingVersion *uint64 = nil
		if existingDeployment != nil && existingDeployment.ReleaseNumbers.Version != nil {
			var tmp2 = (uint64)(*existingDeployment.ReleaseNumbers.Version)
			existingVersion = &tmp2
		}
		var oldReleaseCommitId = ""
		if existingVersion != nil {
			oldReleaseCommitId, _ = getCommitID(ctx, transaction, state, *existingVersion, appName)
			// continue anyway, this is only for events
		}

		appsPrognoses[appName] = ReleaseTrainApplicationPrognosis{
			SkipCause: nil,

			EnvLocks:  nil,
			TeamLocks: nil,
			AppLocks:  nil,

			Version:            versionToDeploy,
			Team:               teamName,
			NewReleaseCommitId: commitID,
			ExistingDeployment: existingDeployment,
			OldReleaseCommitId: oldReleaseCommitId,
		}
	}
	return &ReleaseTrainEnvironmentPrognosis{
		SkipCause:            nil,
		Error:                nil,
		EnvLocks:             nil,
		AppsPrognoses:        appsPrognoses,
		AllLatestDeployments: allLatestDeploymentsTargetEnv,
	}
}

func (c *envReleaseTrain) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "EnvReleaseTrain Transform")
	span.SetTag("env", c.Env)
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
	allLatestDeployments := prognosis.AllLatestDeployments

	renderApplicationSkipCause := c.renderApplicationSkipCause(ctx, allLatestDeployments)
	renderEnvironmentSkipCause := c.renderEnvironmentSkipCause()

	if prognosis.Error != nil {
		return "", prognosis.Error
	}
	if prognosis.SkipCause != nil {
		if !c.WriteCommitData {
			return renderEnvironmentSkipCause(prognosis.SkipCause), nil
		}
		for appName, appPrognosis := range prognosis.AppsPrognoses {
			eventMessage := ""
			if len(prognosis.EnvLocks) > 0 {
				for e := range prognosis.EnvLocks {
					eventMessage = prognosis.EnvLocks[e].Message
					break
				}
			}
			newEvent := createLockPreventedDeploymentEvent(appName, c.Env, eventMessage, "environment")
			commitID := appPrognosis.NewReleaseCommitId
			if commitID == "" {
				// continue anyway
			} else {
				gen := getGenerator(ctx)
				eventUuid := gen.Generate()
				err := state.DBHandler.DBWriteLockPreventedDeploymentEvent(ctx, transaction, c.TransformerEslVersion, eventUuid, commitID, newEvent)
				if err != nil {
					return "", GetCreateReleaseGeneralFailure(err)
				}
			}
		}

		return renderEnvironmentSkipCause(prognosis.SkipCause), nil
	}

	envConfig := c.AllEnvConfigs[c.Env]
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
		prognosisData := DeployPrognosis{
			TeamName:           appPrognosis.Team,
			EnvironmentConfig:  &envConfig,
			ManifestContent:    nil,
			EnvLocks:           convertLockMap(appPrognosis.EnvLocks),
			AppLocks:           convertLockMap(appPrognosis.AppLocks),
			TeamLocks:          convertLockMap(appPrognosis.TeamLocks),
			NewReleaseCommitId: appPrognosis.NewReleaseCommitId,
			ExistingDeployment: appPrognosis.ExistingDeployment,
			OldReleaseCommitId: appPrognosis.OldReleaseCommitId,
		}
		_, err := d.ApplyPrognosis(
			ctx,
			state,
			t,
			transaction,
			&prognosisData,
		)
		if err != nil {
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
		envConfig := c.AllEnvConfigs[c.Env]
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
		envConfig := c.AllEnvConfigs[c.Env]
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

type EnvMap map[types.EnvName]config.EnvironmentConfig

func GetEnvironmentGroupsEnvironmentsOrEnvironment(configs EnvMap, targetName string, targetType string) (EnvMap, bool) {
	envGroupConfigs := make(EnvMap)
	isEnvGroup := false

	if targetType != api.ReleaseTrainRequest_ENVIRONMENT.String() {
		// if it's an envGroup, then we filter for envs of that group:
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

func convertLockMap(locks map[string]*api.Lock) map[string]Lock {
	result := make(map[string]Lock)
	for key, lock := range locks {
		result[key] = *convertLock(lock)
	}
	return result
}

func ConvertLockMapToLockList(locks map[string]*api.Lock) []*api.Lock {
	result := make([]*api.Lock, 0, len(locks))
	for _, lock := range locks {
		result = append(result, lock)
	}
	return result
}

func convertLock(lock *api.Lock) *Lock {
	var createdBy = Actor{
		Name:  "",
		Email: "",
	}
	if lock.CreatedBy != nil {
		createdBy.Name = lock.CreatedBy.Name
		createdBy.Email = lock.CreatedBy.Email
	}
	var createdAt = time.Time{}
	if lock.CreatedAt != nil {
		createdAt = lock.CreatedAt.AsTime()
	}
	l := Lock{
		Message:           lock.Message,
		LockId:            lock.LockId,
		CreatedBy:         createdBy,
		CreatedAt:         createdAt,
		CiLink:            lock.CiLink,
		SuggestedLifetime: lock.SuggestedLifetime,
	}
	return &l
}
