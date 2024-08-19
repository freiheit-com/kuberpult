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

package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/event"
)

type CommitStatus map[string]api.CommitDeploymentStatus

type CommitDeploymentServer struct {
	DBHandler *db.DBHandler
}

func (s *CommitDeploymentServer) GetCommitDeploymentInfo(ctx context.Context, in *api.GetCommitDeploymentInfoRequest) (*api.GetCommitDeploymentInfoResponse, error) {
	status, err := getCommitDeploymentInfoForApp(ctx, s.DBHandler, in.CommitId, in.Application)
	if err != nil {
		return nil, fmt.Errorf("Could not get commit deployment info: %v", err)
	}
	return &api.GetCommitDeploymentInfoResponse{
		DeploymentStatus: status,
	}, nil
}

func getCommitDeploymentInfoForApp(ctx context.Context, h *db.DBHandler, commit, app string) (CommitStatus, error) {
	var jsonCommitEventsMetadata []byte
	var jsonAllDeploymentsMetadata []byte
	var jsonAllEnvironmentsMetadata []byte

	err := h.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
		// Get the latest new-release event for the commit
		query := h.AdaptQuery("SELECT json FROM commit_events WHERE commithash = ? AND eventtype = ? ORDER BY timestamp DESC LIMIT 1;")
		row := transaction.QueryRow(query, commit, "new-release")
		err := row.Scan(&jsonCommitEventsMetadata)
		if err != nil {
			return err
		}
		// Get all deployments for the commit
		query = h.AdaptQuery("SELECT json FROM all_deployments WHERE appname = ? ORDER BY timestamp DESC LIMIT 1;")
		row = transaction.QueryRow(query, app)
		err = row.Scan(&jsonAllDeploymentsMetadata)
		if err != nil {
			return err
		}
		// Get all environments
		query = h.AdaptQuery("SELECT json FROM all_environments ORDER BY version DESC LIMIT 1;")
		row = transaction.QueryRow(query)
		err = row.Scan(&jsonAllEnvironmentsMetadata)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	releaseNumber, err := getCommitReleaseNumber(jsonCommitEventsMetadata)
	if err != nil {
		return nil, fmt.Errorf("Could not get commit release number from commit_events metadata: %v", err)
	}
	allEnvironments, err := getAllEnvironments(jsonAllEnvironmentsMetadata)
	if err != nil {
		return nil, fmt.Errorf("Could not get all environments from all_environments metadata: %v", err)
	}
	environmentReleases, err := getEnvironmentReleases(jsonAllDeploymentsMetadata)
	if err != nil {
		return nil, fmt.Errorf("Could not get environment releases from all_deployments metadata: %v", err)
	}

	commitStatus := getCommitStatus(releaseNumber, environmentReleases, allEnvironments)
	return commitStatus, nil
}

func getCommitStatus(commitReleaseNumber uint64, environmentReleases map[string]uint64, allEnvironments []string) CommitStatus {
	commitStatus := make(CommitStatus)
	for _, env := range allEnvironments {
		commitStatus[env] = api.CommitDeploymentStatus_UNKNOWN
	}
	for env, environmentReleaseVersion := range environmentReleases {
		if environmentReleaseVersion >= commitReleaseNumber {
			commitStatus[env] = api.CommitDeploymentStatus_DEPLOYED
		} else {
			commitStatus[env] = api.CommitDeploymentStatus_PENDING
		}
	}
	return commitStatus
}

func getCommitReleaseNumber(jsonInput []byte) (uint64, error) {
	releaseEvent := event.DBEventGo{
		EventData: &event.NewRelease{
			Environments: map[string]struct{}{},
		},
		EventMetadata: event.Metadata{
			Uuid:           "",
			EventType:      "",
			ReleaseVersion: 0,
		},
	}

	err := json.Unmarshal(jsonInput, &releaseEvent)
	if err != nil {
		return 0, err
	}
	return releaseEvent.EventMetadata.ReleaseVersion, nil
}

func getAllEnvironments(jsonInput []byte) ([]string, error) {
	environments := []string{}
	err := json.Unmarshal(jsonInput, &environments)
	if err != nil {
		return nil, err
	}
	return environments, nil
}

func getEnvironmentReleases(jsonInput []byte) (map[string]uint64, error) {
	releases := map[string]uint64{}
	err := json.Unmarshal(jsonInput, &releases)
	if err != nil {
		return nil, err
	}
	return releases, nil
}
