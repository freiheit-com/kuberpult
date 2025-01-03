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
	env := GetEnvironmentByName(latestOverview.EnvironmentGroups, teamLock.Env)
	if env == nil {
		return fmt.Errorf("could not find environment %s in overview", teamLock.Env)
	}

	if env.TeamLocks == nil {
		env.TeamLocks = make(map[string]*api.Locks)
	}

	if teamLock.Deleted {
		locksToKeep := make([]*api.Lock, 0)
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
	env := GetEnvironmentByName(latestOverview.EnvironmentGroups, environmentLock.Env)
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

func (h *DBHandler) IsOverviewEmpty(overviewResp *api.GetOverviewResponse) bool {
	if overviewResp == nil {
		return true
	}
	if len(overviewResp.EnvironmentGroups) == 0 && overviewResp.GitRevision == "" {
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

func GetEnvironmentByName(groups []*api.EnvironmentGroup, envNameToReturn string) *api.Environment {
	for _, currentGroup := range groups {
		for _, currentEnv := range currentGroup.Environments {
			if currentEnv.Name == envNameToReturn {
				return currentEnv
			}
		}
	}
	return nil
}
