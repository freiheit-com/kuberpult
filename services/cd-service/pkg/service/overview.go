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
	"sync"
	"sync/atomic"

	"github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"go.uber.org/zap"

	git "github.com/libgit2/git2go/v34"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/notify"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

type OverviewServiceServer struct {
	Repository       repository.Repository
	RepositoryConfig repository.RepositoryConfig
	Shutdown         <-chan struct{}

	notify notify.Notify

	init     sync.Once
	response atomic.Value
}

func (o *OverviewServiceServer) GetOverview(
	ctx context.Context,
	in *api.GetOverviewRequest) (*api.GetOverviewResponse, error) {
	if in.GitRevision != "" {
		oid, err := git.NewOid(in.GitRevision)
		if err != nil {
			return nil, grpc.PublicError(ctx, fmt.Errorf("getOverview: could not find revision %v: %v", in.GitRevision, err))
		}
		state, err := o.Repository.StateAt(oid)
		if err != nil {
			var gerr *git.GitError
			if errors.As(err, &gerr) {
				if gerr.Code == git.ErrorCodeNotFound {
					return nil, status.Error(codes.NotFound, "not found")
				}
			}
			return nil, err
		}
		return o.getOverviewDB(ctx, state)
	}
	return o.getOverviewDB(ctx, o.Repository.State())
}

func (o *OverviewServiceServer) getOverviewDB(
	ctx context.Context,
	s *repository.State) (*api.GetOverviewResponse, error) {

	var response *api.GetOverviewResponse
	if s.DBHandler.ShouldUseOtherTables() {
		err := s.DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
			var err2 error
			cached_result, err2 := s.DBHandler.ReadLatestOverviewCache(ctx, transaction)
			if err2 != nil {
				return err2
			}
			if cached_result != nil {
				return json.Unmarshal([]byte(cached_result.Blob), &response)
			}

			response, err2 = o.getOverview(ctx, s, transaction)
			if err2 != nil {
				return err2
			}
			resultJson, err2 := json.Marshal(response)
			if err2 != nil {
				return err2
			}
			return s.DBHandler.WriteOverviewCache(ctx, transaction, string(resultJson))
		})
		if err != nil {
			return nil, err
		}
		response.GitRevision = s.Commit.Id().String()
		return response, nil
	}
	return o.getOverview(ctx, s, nil)
}

func (o *OverviewServiceServer) getOverview(
	ctx context.Context,
	s *repository.State,
	transaction *sql.Tx,
) (*api.GetOverviewResponse, error) {
	var rev string
	if s.Commit != nil {
		rev = s.Commit.Id().String()
	}
	result := api.GetOverviewResponse{
		Branch:            "",
		ManifestRepoUrl:   "",
		Applications:      map[string]*api.Application{},
		EnvironmentGroups: []*api.EnvironmentGroup{},
		GitRevision:       rev,
	}
	result.ManifestRepoUrl = o.RepositoryConfig.URL
	result.Branch = o.RepositoryConfig.Branch
	resultWithEnvGroups, err := repository.UpdateOverviewEnvironmentGroups(ctx, s, transaction, &result)
	if err != nil {
		return nil, err
	}
	resultWithApplications, err := repository.UpdateOverviewApplications(ctx, s, transaction, resultWithEnvGroups)
	if err != nil {
		return nil, err
	}

	return resultWithApplications, nil
}

func (o *OverviewServiceServer) StreamOverview(in *api.GetOverviewRequest,
	stream api.OverviewService_StreamOverviewServer) error {
	ch, unsubscribe := o.subscribe()
	defer unsubscribe()
	done := stream.Context().Done()
	for {
		select {
		case <-o.Shutdown:
			return nil
		case <-ch:
			ov := o.response.Load().(*api.GetOverviewResponse)
			if err := stream.Send(ov); err != nil {
				// if we don't log this here, the details will be lost - so this is an exception to the rule "either return an error or log it".
				// for example if there's an invalid encoding, grpc will just give a generic error like
				// "error while marshaling: string field contains invalid UTF-8"
				// but it won't tell us which field has the issue. This is then very hard to debug further.
				logger.FromContext(stream.Context()).Error("error sending overview response:", zap.Error(err), zap.String("overview", fmt.Sprintf("%+v", ov)))
				return err
			}

		case <-done:
			return nil
		}
	}
}

func (o *OverviewServiceServer) subscribe() (<-chan struct{}, notify.Unsubscribe) {
	o.init.Do(func() {
		ch, unsub := o.Repository.Notify().Subscribe()
		// Channels obtained from subscribe are by default triggered
		//
		// This means, we have to wait here until the first overview is loaded.
		<-ch
		o.update(o.Repository.State())
		go func() {
			defer unsub()
			for {
				select {
				case <-o.Shutdown:
					return
				case <-ch:
					o.update(o.Repository.State())
				}
			}
		}()
	})
	return o.notify.Subscribe()
}

func (o *OverviewServiceServer) update(s *repository.State) {
	r, err := o.getOverviewDB(context.Background(), s)
	if err != nil {
		panic(err)
	}
	o.response.Store(r)
	o.notify.Notify()
}
