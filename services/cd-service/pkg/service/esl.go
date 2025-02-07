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
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/tracing"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type EslServiceServer struct {
	Repository repository.Repository
}

func (s *EslServiceServer) GetFailedEsls(ctx context.Context, req *api.GetFailedEslsRequest) (*api.GetFailedEslsResponse, error) {
	state := s.Repository.State()
	var response *api.GetFailedEslsResponse = &api.GetFailedEslsResponse{
		FailedEsls: make([]*api.EslFailedItem, 0),
	}
	if state.DBHandler.ShouldUseOtherTables() {
		err := state.DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
			failedEslRows, err := s.Repository.State().DBHandler.DBReadLastFailedEslEvents(ctx, transaction, 20)
			if err != nil {
				return err
			}
			failedEslItems := make([]*api.EslFailedItem, len(failedEslRows))
			for i, failedEslRow := range failedEslRows {
				failedEslItems[i] = &api.EslFailedItem{
					EslVersion:            int64(failedEslRow.EslVersion),
					CreatedAt:             timestamppb.New(failedEslRow.Created),
					EventType:             string(failedEslRow.EventType),
					Json:                  failedEslRow.EventJson,
					Reason:                failedEslRow.Reason,
					TransformerEslVersion: int64(failedEslRow.TransformerEslVersion),
				}
			}
			response = &api.GetFailedEslsResponse{
				FailedEsls: failedEslItems,
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("GetFailedEsls is only implemented for the database")
	}
	return response, nil
}

func (s *EslServiceServer) SkipEslEvent(ctx context.Context, in *api.SkipEslEventRequest) (*api.SkipEslEventResponse, error) {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "SkipEslEvent")
	defer span.Finish()

	//Query for UNSYNCED
	dbHandler := s.Repository.State().DBHandler

	err := dbHandler.WithTransactionR(ctx, 2, false, func(ctx context.Context, transaction *sql.Tx) error {
		return dbHandler.DBSkipFailedEslEvent(ctx, transaction, db.TransformerID(in.EventEslVersion))
	})
	return &api.SkipEslEventResponse{}, onErr(err)
}
