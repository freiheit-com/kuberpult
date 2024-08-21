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
	"errors"
	"fmt"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type CommitStatus map[string]api.CommitDeploymentStatus

type CommitDeploymentServer struct {
	DBHandler *db.DBHandler
}

func (s *CommitDeploymentServer) GetCommitDeploymentInfo(ctx context.Context, in *api.GetCommitDeploymentInfoRequest) (*api.GetCommitDeploymentInfoResponse, error) {
	commitDeploymentStatus := make(map[string]*api.AppCommitDeploymentStatus, 0)
	var jsonAllApplicationsMetadata []byte
	var jsonCommitEventsMetadata []byte
	var jsonAllEnvironmentsMetadata []byte

	span, _ := tracer.StartSpanFromContext(ctx, "GetCommitDeploymentInfo")
	defer span.Finish()
	span.SetTag("commit_id", in.CommitId)

	err := s.DBHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
		// Get all applications
		query := s.DBHandler.AdaptQuery("SELECT json FROM all_apps ORDER BY version DESC LIMIT 1;")
		row := transaction.QueryRow(query)
		err := row.Scan(&jsonAllApplicationsMetadata)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("No applications found")
			}
			return err
		}

		// Get the latest new-release event for the commit
		query = s.DBHandler.AdaptQuery("SELECT json FROM commit_events WHERE commithash = ? AND eventtype = ? ORDER BY timestamp DESC LIMIT 1;")
		row = transaction.QueryRow(query, in.CommitId, "new-release")
		err = row.Scan(&jsonCommitEventsMetadata)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("commit \"%s\" could not be found", in.CommitId)
			}
			return err
		}

		// Get all environments
		query = s.DBHandler.AdaptQuery("SELECT json FROM all_environments ORDER BY version DESC LIMIT 1;")
		row = transaction.QueryRow(query)
		err = row.Scan(&jsonAllEnvironmentsMetadata)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("no environments exist")
			}
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	apps, err := getAllApplications(jsonAllApplicationsMetadata)
	if err != nil {
		return nil, fmt.Errorf("Could not get all applications: %v", err)
	}
	releaseNumber, err := getCommitReleaseNumber(jsonCommitEventsMetadata)
	if err != nil {
		return nil, fmt.Errorf("Could not get commit release number from commit_events metadata: %v", err)
	}
	allEnvironments, err := getAllEnvironments(jsonAllEnvironmentsMetadata)
	if err != nil {
		return nil, fmt.Errorf("Could not get all environments from all_environments metadata: %v", err)
	}

	for _, app := range apps {
		commitDeploymentStatusForApp, err := getCommitDeploymentInfoForApp(ctx, s.DBHandler, releaseNumber, app, allEnvironments)
		if err != nil {
			return nil, fmt.Errorf("Could not get commit deployment info for app %s: %v", app, err)
		}
		commitDeploymentStatus[app] = commitDeploymentStatusForApp
	}

	return &api.GetCommitDeploymentInfoResponse{
		DeploymentStatus: commitDeploymentStatus,
	}, nil
}

func getCommitDeploymentInfoForApp(ctx context.Context, h *db.DBHandler, commitReleaseNumber uint64, app string, environments []string) (*api.AppCommitDeploymentStatus, error) {
	var jsonAllDeploymentsMetadata []byte

	span, _ := tracer.StartSpanFromContext(ctx, "getCommitDeploymentInfoForApp")
	defer span.Finish()
	span.SetTag("app", app)

	err := h.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
		// Get all deployments for the application
		query := h.AdaptQuery("SELECT json FROM all_deployments WHERE appname = ? ORDER BY eslversion DESC LIMIT 1;")
		row := transaction.QueryRow(query, app)
		err := row.Scan(&jsonAllDeploymentsMetadata)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("application \"%s\" does not exist or has no deployments yet.", app)
			}
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	environmentReleases, err := getEnvironmentReleases(jsonAllDeploymentsMetadata)
	if err != nil {
		return nil, fmt.Errorf("Could not get environment releases from all_deployments metadata: %v", err)
	}
	commitStatus := getCommitStatus(commitReleaseNumber, environmentReleases, environments)
	return &api.AppCommitDeploymentStatus{
		DeploymentStatus: commitStatus,
	}, nil
}

func getCommitStatus(commitReleaseNumber uint64, environmentReleases map[string]uint64, allEnvironments []string) CommitStatus {
	commitStatus := make(CommitStatus)
	if commitReleaseNumber == 0 {
		// Since 0 is the default value for uint64, it might mean that the release version was not known when the commit was created.
		// In this case, the commit status is unkown for all environments.
		for _, env := range allEnvironments {
			commitStatus[env] = api.CommitDeploymentStatus_UNKNOWN
		}
		return commitStatus
	}

	for _, env := range allEnvironments {
		// by default, a commit is pending in all environments
		commitStatus[env] = api.CommitDeploymentStatus_PENDING
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

func getAllApplications(jsonInput []byte) ([]string, error) {
	applications := db.AllApplicationsJson{
		Apps: []string{},
	}
	err := json.Unmarshal(jsonInput, &applications)
	if err != nil {
		return nil, err
	}
	return applications.Apps, nil
}
