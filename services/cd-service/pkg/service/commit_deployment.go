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
	var jsonCommitEventsMetadata []byte
	var jsonAllEnvironmentsMetadata []byte
	applicationReleases := make(map[string]map[string]uint64, 0)
	allApplicationReleasesQuery := `
SELECT appname, envname, releaseVersion
FROM deployments
WHERE releaseVersion IS NOT NULL;
`
	span, ctx := tracer.StartSpanFromContext(ctx, "GetCommitDeploymentInfo")
	defer span.Finish()
	span.SetTag("commit_id", in.CommitId)

	err := s.DBHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
		// Get the latest new-release event for the commit
		query := s.DBHandler.AdaptQuery("SELECT json FROM commit_events WHERE commithash = ? AND eventtype = ? ORDER BY timestamp DESC LIMIT 1;")
		row := transaction.QueryRow(query, in.CommitId, "new-release")
		err := row.Scan(&jsonCommitEventsMetadata)
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

		// Get latest releases for all apps
		rows, err := transaction.QueryContext(ctx, allApplicationReleasesQuery)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var appName string
			var envName string
			var appRelease uint64
			err = rows.Scan(&appName, &appRelease, &envName)
			if err != nil {
				return err
			}
			if _, ok := applicationReleases[appName]; !ok {
				applicationReleases[appName] = make(map[string]uint64, 0)
			}
			applicationReleases[appName][envName] = appRelease
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

	for app, releases := range applicationReleases {
		commitDeploymentStatusForApp, err := getCommitDeploymentInfoForApp(ctx, s.DBHandler, releaseNumber, app, allEnvironments, releases)
		if err != nil {
			return nil, fmt.Errorf("Could not get commit deployment info for app %s: %v", app, err)
		}
		commitDeploymentStatus[app] = commitDeploymentStatusForApp
	}

	return &api.GetCommitDeploymentInfoResponse{
		DeploymentStatus: commitDeploymentStatus,
	}, nil
}

func getCommitDeploymentInfoForApp(ctx context.Context, h *db.DBHandler, commitReleaseNumber uint64, app string, environments []string, environmentReleases map[string]uint64) (*api.AppCommitDeploymentStatus, error) {
	span, _ := tracer.StartSpanFromContext(ctx, "getCommitDeploymentInfoForApp")
	defer span.Finish()
	span.SetTag("app", app)

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
