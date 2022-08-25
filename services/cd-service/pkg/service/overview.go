/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/

package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/notify"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	git "github.com/libgit2/git2go/v33"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OverviewServiceServer struct {
	Repository repository.Repository
	Shutdown   <-chan struct{}

	notify notify.Notify

	init     sync.Once
	response atomic.Value
}

func (o *OverviewServiceServer) GetDeployedOverview(
	ctx context.Context,
	in *api.GetDeployedOverviewRequest) (*api.GetDeployedOverviewResponse, error) {
	return o.getDeployedOverview(ctx, o.Repository.State())
}

func (o *OverviewServiceServer) getDeployedOverview(
	ctx context.Context,
	s *repository.State) (*api.GetDeployedOverviewResponse, error) {
	result := api.GetDeployedOverviewResponse{
		Environments: map[string]*api.Environment{},
		Applications: map[string]*api.Application{},
	}
	if envs, err := s.GetEnvironmentConfigs(); err != nil {
		return nil, internalError(ctx, err)
	} else {
		for envName, config := range envs {
			env := api.Environment{
				Name: envName,
				Config: &api.Environment_Config{
					Upstream: transformUpstream(config.Upstream),
				},
				Locks:        map[string]*api.Lock{},
				Applications: map[string]*api.Environment_Application{},
			}
			if locks, err := s.GetEnvironmentLocks(envName); err != nil {
				return nil, err
			} else {
				for lockId, lock := range locks {
					var lockCommit *api.Commit = nil
					if commit, err := s.GetEnvironmentLocksCommit(envName, lockId); err != nil {
						return nil, err
					} else {
						lockCommit = transformCommit(commit)
					}
					env.Locks[lockId] = &api.Lock{
						Message: lock.Message,
						Commit:  lockCommit,
						LockId:  lockId,
					}
				}
			}
			if apps, err := s.GetEnvironmentApplications(envName); err != nil {
				return nil, err
			} else {
				for _, appName := range apps {
					app := api.Environment_Application{
						Name:  appName,
						Locks: map[string]*api.Lock{},
					}
					var version *uint64
					if version, err = s.GetEnvironmentApplicationVersion(envName, appName); err != nil && !errors.Is(err, os.ErrNotExist) {
						return nil, err
					} else {
						if version == nil {
							app.Version = 0
						} else {
							app.Version = *version
						}
					}
					if app.Version != 0 {
						if commit, err := s.GetEnvironmentApplicationVersionCommit(envName, appName); err != nil {
							return nil, err
						} else {
							app.VersionCommit = transformCommit(commit)
						}
					}
					if queuedVersion, err := s.GetQueuedVersion(envName, appName); err != nil && !errors.Is(err, os.ErrNotExist) {
						return nil, err
					} else {
						if queuedVersion == nil {
							app.QueuedVersion = 0
						} else {
							app.QueuedVersion = *queuedVersion
						}
					}
					app.UndeployVersion = false
					if version != nil {
						if release, err := s.GetApplicationRelease(appName, *version); err != nil && !errors.Is(err, os.ErrNotExist) {
							return nil, err
						} else if release != nil {
							app.UndeployVersion = release.UndeployVersion
						}
					}
					if appLocks, err := s.GetEnvironmentApplicationLocks(envName, appName); err != nil {
						return nil, err
					} else {
						for lockId, lock := range appLocks {
							var lockCommit *api.Commit = nil
							if commit, err := s.GetEnvironmentApplicationLocksCommit(envName, appName, lockId); err != nil {
								return nil, err
							} else {
								lockCommit = transformCommit(commit)
							}
							app.Locks[lockId] = &api.Lock{
								Message: lock.Message,
								Commit:  lockCommit,
								LockId:  lockId,
							}
						}
					}
					if config.ArgoCd != nil {
						if syncWindows, err := transformSyncWindows(config.ArgoCd.SyncWindows, appName); err != nil {
							return nil, err
						} else {
							app.ArgoCD = &api.Environment_Application_ArgoCD{
								SyncWindows: syncWindows,
							}
						}
					}

					env.Applications[appName] = &app
				}
			}
			result.Environments[envName] = &env
		}
	}
	if apps, err := s.GetApplications(); err != nil {
		return nil, err
	} else {
		for _, appName := range apps {
			app := api.Application{
				Name:     appName,
				Releases: []*api.Release{},
				Team:     "",
			}
			if rels, err := s.GetApplicationReleases(appName); err != nil {
				return nil, err
			} else {
				for _, id := range rels {
					if rel, err := s.GetApplicationRelease(appName, id); err != nil {
						return nil, err
					} else {
						release := &api.Release{
							Version:         id,
							SourceAuthor:    rel.SourceAuthor,
							SourceCommitId:  rel.SourceCommitId,
							SourceMessage:   rel.SourceMessage,
							UndeployVersion: rel.UndeployVersion,
						}
						if commit, err := s.GetApplicationReleaseCommit(appName, id); err != nil {
							return nil, err
						} else {
							release.Commit = transformCommit(commit)
						}
						for _, env := range result.Environments {
							if env.Applications[appName] != nil {
								version := env.Applications[appName].Version
								if version == release.Version {
									app.Releases = append(app.Releases, release)
									break
								}
							}
						}
					}
				}
			}
			if team, err := s.GetApplicationTeamOwner(appName); err != nil {
				return nil, err
			} else {
				app.Team = team
			}
			result.Applications[appName] = &app
		}
	}
	return &result, nil
}

func (o *OverviewServiceServer) GetOverview(
	ctx context.Context,
	in *api.GetOverviewRequest) (*api.GetOverviewResponse, error) {
	return o.getOverview(ctx, o.Repository.State())
}

func (o *OverviewServiceServer) getOverview(
	ctx context.Context,
	s *repository.State) (*api.GetOverviewResponse, error) {
	result := api.GetOverviewResponse{
		Environments: map[string]*api.Environment{},
		Applications: map[string]*api.Application{},
	}
	if envs, err := s.GetEnvironmentConfigs(); err != nil {
		return nil, internalError(ctx, err)
	} else {
		for envName, config := range envs {
			env := api.Environment{
				Name: envName,
				Config: &api.Environment_Config{
					Upstream: transformUpstream(config.Upstream),
				},
				Locks:        map[string]*api.Lock{},
				Applications: map[string]*api.Environment_Application{},
			}
			if locks, err := s.GetEnvironmentLocks(envName); err != nil {
				return nil, err
			} else {
				for lockId, lock := range locks {
					var lockCommit *api.Commit = nil
					if commit, err := s.GetEnvironmentLocksCommit(envName, lockId); err != nil {
						return nil, err
					} else {
						lockCommit = transformCommit(commit)
					}
					env.Locks[lockId] = &api.Lock{
						Message: lock.Message,
						Commit:  lockCommit,
						LockId:  lockId,
					}
				}
			}
			if apps, err := s.GetEnvironmentApplications(envName); err != nil {
				return nil, err
			} else {
				for _, appName := range apps {
					app := api.Environment_Application{
						Name:  appName,
						Locks: map[string]*api.Lock{},
					}
					var version *uint64
					if version, err = s.GetEnvironmentApplicationVersion(envName, appName); err != nil && !errors.Is(err, os.ErrNotExist) {
						return nil, err
					} else {
						if version == nil {
							app.Version = 0
						} else {
							app.Version = *version
						}
					}
					if app.Version != 0 {
						if commit, err := s.GetEnvironmentApplicationVersionCommit(envName, appName); err != nil {
							return nil, err
						} else {
							app.VersionCommit = transformCommit(commit)
						}
					}
					if queuedVersion, err := s.GetQueuedVersion(envName, appName); err != nil && !errors.Is(err, os.ErrNotExist) {
						return nil, err
					} else {
						if queuedVersion == nil {
							app.QueuedVersion = 0
						} else {
							app.QueuedVersion = *queuedVersion
						}
					}
					app.UndeployVersion = false
					if version != nil {
						if release, err := s.GetApplicationRelease(appName, *version); err != nil && !errors.Is(err, os.ErrNotExist) {
							return nil, err
						} else if release != nil {
							app.UndeployVersion = release.UndeployVersion
						}
					}
					if appLocks, err := s.GetEnvironmentApplicationLocks(envName, appName); err != nil {
						return nil, err
					} else {
						for lockId, lock := range appLocks {
							var lockCommit *api.Commit = nil
							if commit, err := s.GetEnvironmentApplicationLocksCommit(envName, appName, lockId); err != nil {
								return nil, err
							} else {
								lockCommit = transformCommit(commit)
							}
							app.Locks[lockId] = &api.Lock{
								Message: lock.Message,
								Commit:  lockCommit,
								LockId:  lockId,
							}
						}
					}
					if config.ArgoCd != nil {
						if syncWindows, err := transformSyncWindows(config.ArgoCd.SyncWindows, appName); err != nil {
							return nil, err
						} else {
							app.ArgoCD = &api.Environment_Application_ArgoCD{
								SyncWindows: syncWindows,
							}
						}
					}

					env.Applications[appName] = &app
				}
			}
			result.Environments[envName] = &env
		}
	}
	if apps, err := s.GetApplications(); err != nil {
		return nil, err
	} else {
		for _, appName := range apps {
			app := api.Application{
				Name:     appName,
				Releases: []*api.Release{},
				Team:     "",
			}
			if rels, err := s.GetApplicationReleases(appName); err != nil {
				return nil, err
			} else {
				for _, id := range rels {
					if rel, err := s.GetApplicationRelease(appName, id); err != nil {
						return nil, err
					} else {
						release := &api.Release{
							Version:         id,
							SourceAuthor:    rel.SourceAuthor,
							SourceCommitId:  rel.SourceCommitId,
							SourceMessage:   rel.SourceMessage,
							UndeployVersion: rel.UndeployVersion,
						}
						if commit, err := s.GetApplicationReleaseCommit(appName, id); err != nil {
							return nil, err
						} else {
							release.Commit = transformCommit(commit)
						}
						app.Releases = append(app.Releases, release)
					}
				}
			}
			if team, err := s.GetApplicationTeamOwner(appName); err != nil {
				return nil, err
			} else {
				app.Team = team
			}
			result.Applications[appName] = &app
		}
	}
	return &result, nil
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
				return err
			}
		case <-done:
			return nil
		}
	}
}

func (o *OverviewServiceServer) StreamDeployedOverview(in *api.GetDeployedOverviewRequest,
	stream api.OverviewService_StreamDeployedOverviewServer) error {
	ch, unsubscribe := o.subscribeDeployed()
	defer unsubscribe()
	done := stream.Context().Done()
	for {
		select {
		case <-o.Shutdown:
			return nil
		case <-ch:
			ov := o.response.Load().(*api.GetDeployedOverviewResponse)
			if err := stream.Send(ov); err != nil {
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
		select {
		case <-ch:
			o.update(o.Repository.State())
		}
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

func (o *OverviewServiceServer) subscribeDeployed() (<-chan struct{}, notify.Unsubscribe) {
	o.init.Do(func() {
		ch, unsub := o.Repository.Notify().Subscribe()
		// Channels obtained from subscribe are by default triggered
		//
		// This means, we have to wait here until the first overview is loaded.
		select {
		case <-ch:
			o.updateDeployed(o.Repository.State())
		}
		go func() {
			defer unsub()
			for {
				select {
				case <-o.Shutdown:
					return
				case <-ch:
					o.updateDeployed(o.Repository.State())
				}
			}
		}()
	})
	return o.notify.Subscribe()
}

func (o *OverviewServiceServer) update(s *repository.State) {
	r, err := o.getOverview(context.Background(), s)
	if err != nil {
		panic(err)
	}
	o.response.Store(r)
	o.notify.Notify()
}

func (o *OverviewServiceServer) updateDeployed(s *repository.State) {
	r, err := o.getDeployedOverview(context.Background(), s)
	if err != nil {
		panic(err)
	}
	o.response.Store(r)
	o.notify.Notify()
}

func transformUpstream(upstream *config.EnvironmentConfigUpstream) *api.Environment_Config_Upstream {
	if upstream == nil {
		return nil
	}
	if upstream.Latest {
		return &api.Environment_Config_Upstream{
			Upstream: &api.Environment_Config_Upstream_Latest{
				Latest: upstream.Latest,
			},
		}
	}
	if upstream.Environment != "" {
		return &api.Environment_Config_Upstream{
			Upstream: &api.Environment_Config_Upstream_Environment{
				Environment: upstream.Environment,
			},
		}
	}
	return nil
}

func transformCommit(commit *git.Commit) *api.Commit {
	if commit == nil {
		return nil
	}
	author := commit.Author()
	return &api.Commit{
		AuthorName:  author.Name,
		AuthorEmail: author.Email,
		AuthorTime:  timestamppb.New(author.When),
	}
}

func transformSyncWindows(syncWindows []config.ArgoCdSyncWindow, appName string) ([]*api.Environment_Application_ArgoCD_SyncWindow, error) {
	var envAppSyncWindows []*api.Environment_Application_ArgoCD_SyncWindow
	for _, syncWindow := range syncWindows {
		for _, pattern := range syncWindow.Apps {
			if match, err := filepath.Match(pattern, appName); err != nil {
				return nil, fmt.Errorf("failed to match app pattern %s of sync window to %s at %s with duration %s: %w", pattern, syncWindow.Kind, syncWindow.Schedule, syncWindow.Duration, err)
			} else if match {
				envAppSyncWindows = append(envAppSyncWindows, &api.Environment_Application_ArgoCD_SyncWindow{
					Kind:     syncWindow.Kind,
					Schedule: syncWindow.Schedule,
					Duration: syncWindow.Duration,
				})
			}
		}
	}
	return envAppSyncWindows, nil
}
