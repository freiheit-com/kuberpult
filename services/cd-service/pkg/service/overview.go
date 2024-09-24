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
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/mapper"
	"sync"
	"sync/atomic"

	"github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"go.uber.org/zap"

	git "github.com/libgit2/git2go/v34"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/notify"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

type OverviewServiceServer struct {
	Repository       repository.Repository
	RepositoryConfig repository.RepositoryConfig
	Shutdown         <-chan struct{}

	notify   notify.Notify
	Context  context.Context
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

	if s.DBHandler.ShouldUseOtherTables() {
		response, err := db.WithTransactionT[api.GetOverviewResponse](s.DBHandler, ctx, db.DefaultNumRetries, false, func(ctx context.Context, transaction *sql.Tx) (*api.GetOverviewResponse, error) {
			var err2 error
			cached_result, err2 := s.DBHandler.ReadLatestOverviewCache(ctx, transaction)
			if err2 != nil {
				return nil, err2
			}
			if !s.DBHandler.IsOverviewEmpty(cached_result) {
				return cached_result, nil
			}

			response, err2 := o.getOverview(ctx, s, transaction)
			if err2 != nil {
				return nil, err2
			}
			err2 = s.DBHandler.WriteOverviewCache(ctx, transaction, response)
			if err2 != nil {
				return nil, err2
			}
			return response, nil
		})
		if err != nil {
			return nil, err
		}
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
	if s.DBHandler.ShouldUseOtherTables() {
		rev = "0000000000000000000000000000000000000000"
	} else {
		if s.Commit != nil {
			rev = s.Commit.Id().String()
		}
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
	if envs, err := s.GetAllEnvironmentConfigs(ctx, transaction); err != nil {
		return nil, grpc.InternalError(ctx, err)
	} else {
		result.EnvironmentGroups = mapper.MapEnvironmentsToGroups(envs)
		for envName, config := range envs {
			var groupName = mapper.DeriveGroupName(config, envName)
			var envInGroup = getEnvironmentInGroup(result.EnvironmentGroups, groupName, envName)
			//exhaustruct:ignore
			argocd := &api.EnvironmentConfig_ArgoCD{}
			if config.ArgoCd != nil {
				argocd = mapper.TransformArgocd(*config.ArgoCd)
			}
			env := api.Environment{
				DistanceToUpstream: 0,
				Priority:           api.Priority_PROD,
				Name:               envName,
				Config: &api.EnvironmentConfig{
					Upstream:         mapper.TransformUpstream(config.Upstream),
					Argocd:           argocd,
					EnvironmentGroup: &groupName,
				},
				Locks:        map[string]*api.Lock{},
				Applications: map[string]*api.Environment_Application{},
			}
			envInGroup.Config = env.Config
			if locks, err := s.GetEnvironmentLocks(ctx, transaction, envName); err != nil {
				return nil, err
			} else {
				for lockId, lock := range locks {
					env.Locks[lockId] = &api.Lock{
						Message:   lock.Message,
						LockId:    lockId,
						CreatedAt: timestamppb.New(lock.CreatedAt),
						CreatedBy: &api.Actor{
							Name:  lock.CreatedBy.Name,
							Email: lock.CreatedBy.Email,
						},
					}
				}
				envInGroup.Locks = env.Locks
			}

			if apps, err := s.GetEnvironmentApplications(ctx, transaction, envName); err != nil {
				return nil, err
			} else {
				for _, appName := range apps {
					app, err2 := s.UpdateOneAppEnv(ctx, transaction, appName, envName, &config)
					if err2 != nil {
						return nil, err2
					}
					env.Applications[appName] = app
				}
			}
			envInGroup.Applications = env.Applications
		}
	}
	if apps, err := s.GetApplications(ctx, transaction); err != nil {
		return nil, err
	} else {
		for _, appName := range apps {
			err2 := s.UpdateTopLevelApp(ctx, transaction, appName, &result)
			if err2 != nil {
				return nil, err2
			}
		}

	}

	return &result, nil
}

//
//func UpdateTopLevelApp(ctx context.Context, s *repository.State, transaction *sql.Tx, appName string, result *api.GetOverviewResponse) error {
//	app := api.Application{
//		UndeploySummary: 0,
//		Warnings:        nil,
//		Name:            appName,
//		Releases:        []*api.Release{},
//		SourceRepoUrl:   "",
//		Team:            "",
//	}
//	if rels, err := s.GetAllApplicationReleases(ctx, transaction, appName); err != nil {
//		return err
//	} else {
//		for _, id := range rels {
//			if rel, err := s.GetApplicationRelease(ctx, transaction, appName, id); err != nil {
//				return err
//			} else {
//				if rel == nil {
//					// ignore
//				} else {
//					release := rel.ToProto()
//					release.Version = id
//					release.UndeployVersion = rel.UndeployVersion
//					app.Releases = append(app.Releases, release)
//				}
//			}
//		}
//	}
//	if team, err := s.GetApplicationTeamOwner(ctx, transaction, appName); err != nil {
//		return err
//	} else {
//		app.Team = team
//	}
//	app.UndeploySummary = deriveUndeploySummary(appName, result.EnvironmentGroups)
//	app.Warnings = CalculateWarnings(ctx, app.Name, result.EnvironmentGroups)
//	result.Applications[appName] = &app
//	return nil
//}

func getEnvironmentInGroup(groups []*api.EnvironmentGroup, groupNameToReturn string, envNameToReturn string) *api.Environment {
	for _, currentGroup := range groups {
		if currentGroup.EnvironmentGroupName == groupNameToReturn {
			for _, currentEnv := range currentGroup.Environments {
				if currentEnv.Name == envNameToReturn {
					return currentEnv
				}
			}
		}
	}
	return nil
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
	r, err := o.getOverviewDB(o.Context, s)
	if err != nil {
		logger.FromContext(o.Context).Error("error getting overview:", zap.Error(err))
		return
	}
	o.response.Store(r)
	o.notify.Notify()
}
