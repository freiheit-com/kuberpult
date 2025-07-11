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
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"github.com/freiheit-com/kuberpult/pkg/types"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/event"
)

type CommitStatus map[string]api.CommitDeploymentStatus

type CommitDeploymentServer struct {
	DBHandler *db.DBHandler
}

func (s *CommitDeploymentServer) GetCommitDeploymentInfo(ctx context.Context, in *api.GetCommitDeploymentInfoRequest) (*api.GetCommitDeploymentInfoResponse, error) {
	commitDeploymentStatus := make(map[string]*api.AppCommitDeploymentStatus, 0)
	allEnvironments := make([]types.EnvName, 0)
	applicationReleases := make(map[string]map[types.EnvName]uint64, 0)
	var jsonCommitEventsMetadata []byte
	span, ctx := tracer.StartSpanFromContext(ctx, "GetCommitDeploymentInfo")
	defer span.Finish()
	span.SetTag("commit_id", in.CommitId)

	err := s.DBHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
		// Get the latest new-release event for the commit
		var err error
		jsonCommitEventsMetadata, err = getCommitEventByCommitId(ctx, s.DBHandler, transaction, in.CommitId)
		if err != nil {
			return err
		}

		// Get all environments
		allEnvironments, err = s.DBHandler.DBSelectAllEnvironments(ctx, transaction)
		if err != nil {
			return err
		}

		// Get latest releases for all apps
		err2 := getDeploymentsWithReleaseVersion(ctx, transaction, applicationReleases)
		if err2 != nil {
			return err2
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	releaseNumber, err := getCommitReleaseNumber(jsonCommitEventsMetadata)
	if err != nil {
		return nil, fmt.Errorf("could not get commit release number from commit_events metadata: %v", err)
	}

	for app, releases := range applicationReleases {
		commitDeploymentStatusForApp, err := getCommitDeploymentInfoForApp(ctx, releaseNumber, app, allEnvironments, releases)
		if err != nil {
			return nil, fmt.Errorf("could not get commit deployment info for app %s: %v", app, err)
		}
		commitDeploymentStatus[app] = commitDeploymentStatusForApp
	}

	return &api.GetCommitDeploymentInfoResponse{
		DeploymentStatus: commitDeploymentStatus,
	}, nil
}

func getDeploymentsWithReleaseVersion(ctx context.Context, transaction *sql.Tx, applicationReleases map[string]map[types.EnvName]uint64) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "getDeploymentsWithReleaseVersion")
	defer span.Finish()
	allApplicationReleasesQuery := `
		SELECT appname, envname, releaseVersion
		FROM deployments
		WHERE releaseVersion IS NOT NULL;`

	rows, err := transaction.QueryContext(ctx, allApplicationReleasesQuery)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var appName string
		var envName types.EnvName
		var appRelease uint64
		err = rows.Scan(&appName, &envName, &appRelease)
		if err != nil {
			return err
		}
		if _, ok := applicationReleases[appName]; !ok {
			applicationReleases[appName] = make(map[types.EnvName]uint64, 0)
		}
		applicationReleases[appName][envName] = appRelease
	}
	err = rows.Close()
	if err != nil {
		return err
	}
	return nil
}

func getCommitEventByCommitId(ctx context.Context, db *db.DBHandler, transaction *sql.Tx, commitId string) ([]byte, error) {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "getCommitEventByCommitId")
	defer span.Finish()
	query := db.AdaptQuery("SELECT json FROM commit_events WHERE commithash = ? AND eventtype = ? ORDER BY timestamp DESC LIMIT 1;")
	row := transaction.QueryRowContext(ctx, query, commitId, event.EventTypeNewRelease)
	var jsonCommitEventsMetadata []byte
	err := row.Scan(&jsonCommitEventsMetadata)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, onErr(fmt.Errorf("commit \"%s\" could not be found", commitId))
		}
		return nil, onErr(err)
	}
	return jsonCommitEventsMetadata, nil
}

func (s *CommitDeploymentServer) GetDeploymentCommitInfo(ctx context.Context, in *api.GetDeploymentCommitInfoRequest) (*api.GetDeploymentCommitInfoResponse, error) {
	deploymentCommitInfo := &api.GetDeploymentCommitInfoResponse{
		Author:        "",
		CommitId:      "",
		CommitMessage: "",
	}
	err := s.DBHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
		deployment, err := s.DBHandler.DBSelectLatestDeployment(ctx, transaction, in.Application, types.EnvName(in.Environment))
		if err != nil {
			return err
		}
		if deployment == nil {
			return nil
		}
		release, err := s.DBHandler.DBSelectReleaseByVersion(ctx, transaction, deployment.App, *deployment.ReleaseNumbers.Version, true)
		if err != nil {
			return err
		}
		deploymentCommitInfo.Author = release.Metadata.SourceAuthor
		deploymentCommitInfo.CommitMessage = release.Metadata.SourceMessage
		deploymentCommitInfo.CommitId = release.Metadata.SourceCommitId
		return nil
	})
	if err != nil {
		return nil, err
	}
	return deploymentCommitInfo, nil
}

func getCommitDeploymentInfoForApp(ctx context.Context, commitReleaseNumber uint64, app string, environments []types.EnvName, environmentReleases map[types.EnvName]uint64) (*api.AppCommitDeploymentStatus, error) {
	span, _ := tracer.StartSpanFromContext(ctx, "getCommitDeploymentInfoForApp")
	defer span.Finish()
	span.SetTag("app", app)

	commitStatus := getCommitStatus(commitReleaseNumber, environmentReleases, environments)
	return &api.AppCommitDeploymentStatus{
		DeploymentStatus: commitStatus,
	}, nil
}

func getCommitStatus(commitReleaseNumber uint64, environmentReleases map[types.EnvName]uint64, allEnvironments []types.EnvName) CommitStatus {
	commitStatus := make(CommitStatus)
	if commitReleaseNumber == 0 {
		// Since 0 is the default value for uint64, it might mean that the release version was not known when the commit was created.
		// In this case, the commit status is unkown for all environments.
		for _, env := range allEnvironments {
			commitStatus[string(env)] = api.CommitDeploymentStatus_UNKNOWN
		}
		return commitStatus
	}

	for _, env := range allEnvironments {
		// by default, a commit is pending in all environments
		commitStatus[string(env)] = api.CommitDeploymentStatus_PENDING
	}

	for env, environmentReleaseVersion := range environmentReleases {
		if environmentReleaseVersion >= commitReleaseNumber {
			commitStatus[string(env)] = api.CommitDeploymentStatus_DEPLOYED
		} else {
			commitStatus[string(env)] = api.CommitDeploymentStatus_PENDING
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
