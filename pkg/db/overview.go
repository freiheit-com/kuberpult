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

package db

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *DBHandler) UpdateOverviewTeamLock(ctx context.Context, transaction *sql.Tx, teamLock TeamLock) error {
	latestOverview, err := h.ReadLatestOverviewCache(ctx, transaction)
	if err != nil {
		return err
	}
	if h.IsOverviewEmpty(latestOverview) {
		return nil
	}
	env := getEnvironmentByName(latestOverview.EnvironmentGroups, teamLock.Env)
	if env == nil {
		return fmt.Errorf("could not find environment %s in overview", teamLock.Env)
	}
	apps := getEnvironmentApplicationsByTeam(env.Applications, teamLock.Team)
	for _, app := range apps {
		if app.TeamLocks == nil {
			app.TeamLocks = map[string]*api.Lock{}
		}
		app.TeamLocks[teamLock.LockID] = &api.Lock{
			Message:   teamLock.Metadata.Message,
			LockId:    teamLock.LockID,
			CreatedAt: timestamppb.New(teamLock.Created),
			CreatedBy: &api.Actor{
				Name:  teamLock.Metadata.CreatedByName,
				Email: teamLock.Metadata.CreatedByEmail,
			},
		}
		if teamLock.Deleted {
			delete(app.TeamLocks, teamLock.LockID)
		}
	}
	err = h.WriteOverviewCache(ctx, transaction, latestOverview)
	if err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) UpdateOverviewEnvironmentLock(ctx context.Context, transaction *sql.Tx, environmentLock EnvironmentLock) error {
	latestOverview, err := h.ReadLatestOverviewCache(ctx, transaction)
	if err != nil {
		return err
	}
	if h.IsOverviewEmpty(latestOverview) {
		return nil
	}
	env := getEnvironmentByName(latestOverview.EnvironmentGroups, environmentLock.Env)
	if env == nil {
		return fmt.Errorf("could not find environment %s in overview", environmentLock.Env)
	}
	if env.Locks == nil {
		env.Locks = map[string]*api.Lock{}
	}
	env.Locks[environmentLock.LockID] = &api.Lock{
		Message:   environmentLock.Metadata.Message,
		LockId:    environmentLock.LockID,
		CreatedAt: timestamppb.New(environmentLock.Created),
		CreatedBy: &api.Actor{
			Name:  environmentLock.Metadata.CreatedByName,
			Email: environmentLock.Metadata.CreatedByEmail,
		},
	}
	if environmentLock.Deleted {
		delete(env.Locks, environmentLock.LockID)
	}
	err = h.WriteOverviewCache(ctx, transaction, latestOverview)
	if err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) UpdateOverviewDeployment(ctx context.Context, transaction *sql.Tx, deployment Deployment, createdTime time.Time) error {
	latestOverview, err := h.ReadLatestOverviewCache(ctx, transaction)
	if err != nil {
		return err
	}
	if h.IsOverviewEmpty(latestOverview) {
		return nil
	}
	env := getEnvironmentByName(latestOverview.EnvironmentGroups, deployment.Env)
	if env == nil {
		return fmt.Errorf("could not find environment %s in overview", deployment.Env)
	}
	appInEnv := getEnvironmentApplicationByName(env.Applications, deployment.App)
	if appInEnv == nil {
		return fmt.Errorf("could not find application %s in environment %s in overview", deployment.App, deployment.Env)
	}
	if deployment.Version == nil {
		appInEnv.Version = 0
	} else {
		appInEnv.Version = uint64(*deployment.Version)
	}
	appInEnv.DeploymentMetaData.DeployAuthor = deployment.Metadata.DeployedByEmail
	appInEnv.DeploymentMetaData.DeployTime = fmt.Sprintf("%d", createdTime.Unix())

	app := getApplicationByName(latestOverview.Applications, deployment.App)

	if deployment.Version != nil { //Check if not trying to deploy an undeploy version
		//Get the undeploy information from the release
		release, err := h.DBSelectReleaseByVersion(ctx, transaction, appInEnv.Name, appInEnv.Version, true)
		if err != nil {
			return fmt.Errorf("error getting release %d for app %s", appInEnv.Version, appInEnv.Name)
		}
		if release == nil {
			return fmt.Errorf("could not find release %d for app %s", appInEnv.Version, appInEnv.Name)
		}
		appInEnv.UndeployVersion = release.Metadata.UndeployVersion
	}
	app.Warnings = CalculateWarnings(ctx, app.Name, latestOverview.EnvironmentGroups)
	app.UndeploySummary = deriveUndeploySummary(app.Name, latestOverview.EnvironmentGroups)
	err = h.WriteOverviewCache(ctx, transaction, latestOverview)
	if err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) UpdateOverviewDeploymentAttempt(ctx context.Context, transaction *sql.Tx, queuedDeployment *QueuedDeployment) error {
	latestOverview, err := h.ReadLatestOverviewCache(ctx, transaction)
	if err != nil {
		return err
	}
	if h.IsOverviewEmpty(latestOverview) {
		return nil
	}
	if queuedDeployment == nil {
		return nil
	}
	env := getEnvironmentByName(latestOverview.EnvironmentGroups, queuedDeployment.Env)
	if env == nil {
		return fmt.Errorf("could not find environment %s in overview", queuedDeployment.Env)
	}
	app := getEnvironmentApplicationByName(env.Applications, queuedDeployment.App)
	if app == nil {
		return fmt.Errorf("could not find application %s in environment %s in overview", queuedDeployment.App, queuedDeployment.Env)
	}
	if queuedDeployment.Version != nil {
		app.QueuedVersion = uint64(*queuedDeployment.Version)
	}
	err = h.WriteOverviewCache(ctx, transaction, latestOverview)
	if err != nil {
		return err
	}
	return nil
}

func deriveUndeploySummary(appName string, groups []*api.EnvironmentGroup) api.UndeploySummary {
	var allNormal = true
	var allUndeploy = true
	for _, group := range groups {
		for _, environment := range group.Environments {
			var app, exists = environment.Applications[appName]
			if !exists {
				continue
			}
			if app.Version == 0 {
				// if the app exists but nothing is deployed, we ignore this
				continue
			}
			if app.UndeployVersion {
				allNormal = false
			} else {
				allUndeploy = false
			}
		}
	}
	if allUndeploy {
		return api.UndeploySummary_UNDEPLOY
	}
	if allNormal {
		return api.UndeploySummary_NORMAL
	}
	return api.UndeploySummary_MIXED
}

func (h *DBHandler) UpdateOverviewApplicationLock(ctx context.Context, transaction *sql.Tx, applicationLock ApplicationLock, createdTime time.Time) error {
	latestOverview, err := h.ReadLatestOverviewCache(ctx, transaction)
	if err != nil {
		return err
	}
	if h.IsOverviewEmpty(latestOverview) {
		return nil
	}
	env := getEnvironmentByName(latestOverview.EnvironmentGroups, applicationLock.Env)
	if env == nil {
		return fmt.Errorf("could not find environment %s in overview", applicationLock.Env)
	}
	app := getEnvironmentApplicationByName(env.Applications, applicationLock.App)
	if app == nil {
		return fmt.Errorf("could not find application %s in environment %s in overview", applicationLock.App, applicationLock.Env)
	}
	if app.Locks == nil {
		app.Locks = map[string]*api.Lock{}
	}
	app.Locks[applicationLock.LockID] = &api.Lock{
		Message:   applicationLock.Metadata.Message,
		LockId:    applicationLock.LockID,
		CreatedAt: timestamppb.New(createdTime),
		CreatedBy: &api.Actor{
			Name:  applicationLock.Metadata.CreatedByName,
			Email: applicationLock.Metadata.CreatedByEmail,
		},
	}
	if applicationLock.Deleted {
		delete(app.Locks, applicationLock.LockID)
	}
	err = h.WriteOverviewCache(ctx, transaction, latestOverview)
	if err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) UpdateOverviewRelease(ctx context.Context, transaction *sql.Tx, release DBReleaseWithMetaData) error {
	latestOverview, err := h.ReadLatestOverviewCache(ctx, transaction)
	if err != nil {
		return err
	}
	if h.IsOverviewEmpty(latestOverview) {
		return nil
	}
	app := getApplicationByName(latestOverview.Applications, release.App)
	if app == nil {
		return fmt.Errorf("could not find application '%s' in overview", release.App)
	}
	apiRelease := &api.Release{
		PrNumber:        extractPrNumber(release.Metadata.SourceMessage),
		Version:         release.ReleaseNumber,
		UndeployVersion: release.Metadata.UndeployVersion,
		SourceAuthor:    release.Metadata.SourceAuthor,
		SourceCommitId:  release.Metadata.SourceCommitId,
		SourceMessage:   release.Metadata.SourceMessage,
		CreatedAt:       timestamppb.New(release.Created),
		DisplayVersion:  release.Metadata.DisplayVersion,
		IsMinor:         release.Metadata.IsMinor,
		IsPrepublish:    release.Metadata.IsPrepublish,
	}
	foundRelease := false
	for relIndex, currentRelease := range app.Releases {
		if currentRelease.Version == release.ReleaseNumber {
			if release.Deleted {
				app.Releases = append(app.Releases[:relIndex], app.Releases[relIndex+1:]...)
			} else {
				app.Releases[relIndex] = apiRelease
			}
			foundRelease = true
		}
	}
	if !foundRelease && !release.Deleted {
		app.Releases = append(app.Releases, apiRelease)
	}

	if release.Metadata.UndeployVersion { //We need to force recalculation as we need to determine undeploySummary
		err = h.ForceOverviewRecalculation(ctx, transaction)
	} else {
		err = h.WriteOverviewCache(ctx, transaction, latestOverview)
	}

	if err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) ForceOverviewRecalculation(ctx context.Context, transaction *sql.Tx) error {
	latestOverview, err := h.ReadLatestOverviewCache(ctx, transaction)
	if err != nil {
		return err
	}
	if h.IsOverviewEmpty(latestOverview) {
		return nil
	}
	emptyOverview := &api.GetOverviewResponse{
		Applications:      map[string]*api.Application{},
		EnvironmentGroups: []*api.EnvironmentGroup{},
		GitRevision:       "",
		Branch:            "",
		ManifestRepoUrl:   "",
	}
	err = h.WriteOverviewCache(ctx, transaction, emptyOverview)
	if err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) IsOverviewEmpty(overviewResp *api.GetOverviewResponse) bool {
	if overviewResp == nil {
		return true
	}
	if len(overviewResp.Applications) == 0 && len(overviewResp.EnvironmentGroups) == 0 && overviewResp.GitRevision == "" {
		return true
	}
	return false
}

func getEnvironmentByName(groups []*api.EnvironmentGroup, envNameToReturn string) *api.Environment {
	for _, currentGroup := range groups {
		for _, currentEnv := range currentGroup.Environments {
			if currentEnv.Name == envNameToReturn {
				return currentEnv
			}
		}
	}
	return nil
}

func getEnvironmentApplicationsByTeam(apps map[string]*api.Environment_Application, team string) []*api.Environment_Application {
	foundApps := []*api.Environment_Application{}
	for _, app := range apps {
		if app.Team == team {
			foundApps = append(foundApps, app)
		}
	}
	return foundApps
}

func getEnvironmentApplicationByName(apps map[string]*api.Environment_Application, appNameToReturn string) *api.Environment_Application {
	for _, app := range apps {
		if app.Name == appNameToReturn {
			return app
		}
	}
	return nil
}

func getApplicationByName(apps map[string]*api.Application, appNameToReturn string) *api.Application {
	for _, app := range apps {
		if app.Name == appNameToReturn {
			return app
		}
	}
	return nil
}

func extractPrNumber(sourceMessage string) string {
	re := regexp.MustCompile(`\(#(\d+)\)`)
	res := re.FindAllStringSubmatch(sourceMessage, -1)

	if len(res) == 0 {
		return ""
	} else {
		return res[len(res)-1][1]
	}
}

func CalculateWarnings(ctx context.Context, appName string, groups []*api.EnvironmentGroup) []*api.Warning {
	result := make([]*api.Warning, 0)
	for e := 0; e < len(groups); e++ {
		group := groups[e]
		for i := 0; i < len(groups[e].Environments); i++ {
			env := group.Environments[i]
			if env.Config.Upstream == nil || env.Config.Upstream.Environment == nil {
				// if the env has no upstream, there's nothing to warn about
				continue
			}
			upstreamEnvName := env.Config.GetUpstream().Environment
			upstreamEnv := getEnvironmentByName(groups, *upstreamEnvName)
			if upstreamEnv == nil {
				// this is already checked on startup and therefore shouldn't happen here
				continue
			}

			appInEnv := env.Applications[appName]
			if appInEnv == nil {
				// appName is not deployed here, ignore it
				continue
			}
			versionInEnv := appInEnv.Version
			appInUpstreamEnv := upstreamEnv.Applications[appName]
			if appInUpstreamEnv == nil {
				// appName is not deployed upstream... that's unusual!
				var warning = api.Warning{
					WarningType: &api.Warning_UpstreamNotDeployed{
						UpstreamNotDeployed: &api.UpstreamNotDeployed{
							UpstreamEnvironment: *upstreamEnvName,
							ThisVersion:         versionInEnv,
							ThisEnvironment:     env.Name,
						},
					},
				}
				result = append(result, &warning)
				continue
			}
			versionInUpstreamEnv := appInUpstreamEnv.Version

			if versionInEnv > versionInUpstreamEnv && len(appInEnv.Locks) == 0 {
				var warning = api.Warning{
					WarningType: &api.Warning_UnusualDeploymentOrder{
						UnusualDeploymentOrder: &api.UnusualDeploymentOrder{
							UpstreamVersion:     versionInUpstreamEnv,
							UpstreamEnvironment: *upstreamEnvName,
							ThisVersion:         versionInEnv,
							ThisEnvironment:     env.Name,
						},
					},
				}
				result = append(result, &warning)
			}
		}
	}
	return result
}
