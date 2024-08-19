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

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/event"
)

type CommitStatus struct {
	Status map[string]api.CommitDeploymentStatus
}

type CommitDeploymentServer struct {
	DBHandler *db.DBHandler
}

func (s *CommitDeploymentServer) GetCommitDeploymentInfo(ctx context.Context, in *api.GetCommitDeploymentInfoRequest) (*api.GetCommitDeploymentInfoResponse, error) {
	return &api.GetCommitDeploymentInfoResponse{}, nil
}

func getCommitDeploymentInfoForApp(ctx context.Context, h *db.DBHandler, commit, app string) (*CommitStatus, error) {
	commitStatus := &CommitStatus{}
	err := h.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {

		return nil
	})
	if err != nil {
		return nil, err
	}
	return commitStatus, nil
}

func getCommitReleaseNumber(jsonInput []byte) (uint64, error) {
	releaseEvent := event.DBEventGo{
		EventData:     &event.NewRelease{},
		EventMetadata: event.Metadata{},
	}
	err := json.Unmarshal(jsonInput, &releaseEvent)
	if err != nil {
		return 0, err
	}
	return releaseEvent.EventMetadata.ReleaseVersion, nil
}

func getCommitEnvironments(jsonInput []byte) ([]string, error) {
	environments := []string{}
	releaseEvent := event.DBEventGo{
		EventData:     &event.NewRelease{},
		EventMetadata: event.Metadata{},
	}
	err := json.Unmarshal(jsonInput, &releaseEvent)
	if err != nil {
		return nil, err
	}
	for k, _ := range releaseEvent.EventData.(*event.NewRelease).Environments {
		environments = append(environments, k)
	}
	return environments, nil
}
