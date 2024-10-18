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
	"time"

	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
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
	if env.TeamLocks == nil {
		env.TeamLocks = make(map[string]*api.Locks)
	}

	if teamLock.Deleted {
		locksToKeep := make([]*api.Lock, max(len(env.TeamLocks[teamLock.Team].Locks)-1, 0))
		for _, lock := range env.TeamLocks[teamLock.Team].Locks {
			if lock.LockId != teamLock.LockID {
				locksToKeep = append(locksToKeep, lock)
			}
		}
		if len(locksToKeep) == 0 {
			delete(env.TeamLocks, teamLock.Team)
		} else {
			env.TeamLocks[teamLock.Team].Locks = locksToKeep
		}
	} else {
		if env.TeamLocks[teamLock.Team] == nil {
			env.TeamLocks[teamLock.Team] = &api.Locks{
				Locks: make([]*api.Lock, 0),
			}
		}
		env.TeamLocks[teamLock.Team].Locks = append(env.TeamLocks[teamLock.Team].Locks, &api.Lock{
			Message:   teamLock.Metadata.Message,
			LockId:    teamLock.LockID,
			CreatedAt: timestamppb.New(teamLock.Created),
			CreatedBy: &api.Actor{
				Name:  teamLock.Metadata.CreatedByName,
				Email: teamLock.Metadata.CreatedByEmail,
			},
		})
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

	app := getApplicationByName(latestOverview.Applications, deployment.App)
	if app != nil {
		app.Warnings = CalculateWarnings(ctx, app.Name, latestOverview.EnvironmentGroups)
		app.UndeploySummary = deriveUndeploySummary(app.Name, latestOverview.EnvironmentGroups)
	}
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
	err = h.WriteOverviewCache(ctx, transaction, latestOverview)
	if err != nil {
		return err
	}
	return nil
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

	if env.AppLocks == nil {
		env.AppLocks = make(map[string]*api.Locks)
	}

	if applicationLock.Deleted {
		locksToKeep := make([]*api.Lock, max(len(env.AppLocks[applicationLock.App].Locks)-1, 0))
		for _, lock := range env.AppLocks[applicationLock.App].Locks {
			if lock.LockId != applicationLock.LockID {
				locksToKeep = append(locksToKeep, lock)
			}
		}
		if len(locksToKeep) == 0 {
			delete(env.AppLocks, applicationLock.App)
		} else {
			env.AppLocks[applicationLock.App].Locks = locksToKeep
		}
	} else {
		if env.AppLocks[applicationLock.App] == nil {
			env.AppLocks[applicationLock.App] = &api.Locks{
				Locks: make([]*api.Lock, 0),
			}
		}
		env.AppLocks[applicationLock.App].Locks = append(env.AppLocks[applicationLock.App].Locks, &api.Lock{
			Message:   applicationLock.Metadata.Message,
			LockId:    applicationLock.LockID,
			CreatedAt: timestamppb.New(applicationLock.Created),
			CreatedBy: &api.Actor{
				Name:  applicationLock.Metadata.CreatedByName,
				Email: applicationLock.Metadata.CreatedByEmail,
			},
		})
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
		if release.Deleted {
			return nil
		}
		selectApp, err := h.DBSelectApp(ctx, transaction, release.App)
		if err != nil {
			return fmt.Errorf("could not find application '%s' in apps table, got an error: %w", release.App, err)
		}
		if selectApp == nil {
			return fmt.Errorf("could not find application '%s' in apps table: got no result", release.App)
		}
		app = &api.Application{
			Name:            release.App,
			Releases:        []*api.Release{},
			SourceRepoUrl:   "", // TODO
			Team:            selectApp.Metadata.Team,
			UndeploySummary: 0,
			Warnings:        []*api.Warning{},
		}
		latestOverview.Applications[release.App] = app
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

	if release.Metadata.UndeployVersion {
		app.UndeploySummary = deriveUndeploySummary(app.Name, latestOverview.EnvironmentGroups)
	}

	err = h.WriteOverviewCache(ctx, transaction, latestOverview)
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

func (h *DBHandler) DBDeleteOldOverviews(ctx context.Context, tx *sql.Tx, numberOfOverviewsToKeep uint64, timeThreshold time.Time) error {
	span, _ := tracer.StartSpanFromContext(ctx, "DBDeleteOldOverviews")
	defer span.Finish()

	if h == nil {
		return nil
	}

	if tx == nil {
		return fmt.Errorf("attempting to delete overview caches without a transaction")
	}

	deleteQuery := h.AdaptQuery(`
DELETE FROM overview_cache
WHERE timestamp < ?
AND eslversion NOT IN (
    SELECT eslversion 
	FROM overview_cache
	ORDER BY eslversion DESC
	LIMIT ?
);
`)
	span.SetTag("query", deleteQuery)
	span.SetTag("numberOfOverviewsToKeep", numberOfOverviewsToKeep)
	span.SetTag("timeThreshold", timeThreshold)
	_, err := tx.Exec(
		deleteQuery,
		timeThreshold.UTC(),
		numberOfOverviewsToKeep,
	)
	if err != nil {
		return fmt.Errorf("DBDeleteOldOverviews error executing query: %w", err)
	}
	return nil
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
