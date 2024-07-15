package service

import (
	"context"
	"database/sql"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type EslServiceServer struct {
	Repository repository.Repository
}

func (s *EslServiceServer) GetFailedEsls(ctx context.Context, req *api.GetFailedEslsRequest) (*api.GetFailedEslsResponse, error) {
	state := s.Repository.State()
	var response *api.GetFailedEslsResponse = &api.GetFailedEslsResponse{}
	if state.DBHandler.ShouldUseOtherTables() {
		err :=  state.DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
			failedEslRows, err := s.Repository.State().DBHandler.DBReadLastFailedEslEvents(ctx, transaction, 20)
			if err != nil {
				return err
			}
			failedEslItems := make([]*api.EslItem, len(failedEslRows))
			for i, failedEslRow := range failedEslRows {
				failedEslItems[i] = &api.EslItem{
					EslId: int64(failedEslRow.EslId),
					CreatedAt: timestamppb.New(failedEslRow.Created),
					EventType: string(failedEslRow.EventType),
					Json: failedEslRow.EventJson,
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
	}
	return response, nil
}