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
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func (h *DBHandler) UpdateOverviewDeleteEnvironment(ctx context.Context, tx *sql.Tx, environmentName string) error {
	//Overview cache
	overview, err := h.ReadLatestOverviewCache(ctx, tx)
	if overview == nil {
		//If no overview, there is no need to update it
		return nil
	}
	if err != nil {
		return fmt.Errorf("unable to read overview cache, error: %w", err)
	}

	for gIdx, group := range overview.EnvironmentGroups {
		for idx, currentEnv := range group.Environments {
			if currentEnv.Name == environmentName {
				if len(group.Environments) == 1 { //Delete whole group
					overview.EnvironmentGroups = append(overview.EnvironmentGroups[:gIdx], overview.EnvironmentGroups[gIdx+1:]...)
				} else {
					overview.EnvironmentGroups[gIdx].Environments = append(group.Environments[:idx], group.Environments[idx+1:]...)
				}
				break
			}
		}
	}
	err = h.WriteOverviewCache(ctx, tx, overview)
	if err != nil {
		return fmt.Errorf("Unable to write overview cache, error: %w", err)
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
