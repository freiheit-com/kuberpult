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
	"google.golang.org/protobuf/types/known/timestamppb"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/notify"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
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
					EnvironmentGroup: config.EnvironmentGroup,
				},
				Locks:        map[string]*api.Lock{},
				Applications: map[string]*api.Environment_Application{},
			}
			if locks, err := s.GetEnvironmentLocks(envName); err != nil {
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
							app.Locks[lockId] = &api.Lock{
								Message:   lock.Message,
								LockId:    lockId,
								CreatedAt: timestamppb.New(lock.CreatedAt),
								CreatedBy: &api.Actor{
									Name:  lock.CreatedBy.Name,
									Email: lock.CreatedBy.Email,
								},
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
							CreatedAt:       timestamppb.New(rel.CreatedAt),
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
		EnvironmentGroups: []*api.EnvironmentGroup{},
	}
	if envs, err := s.GetEnvironmentConfigs(); err != nil {
		return nil, internalError(ctx, err)
	} else {
		result.EnvironmentGroups = mapEnvironmentsToGroups(envs)
		todo("now we just need to fill the groups data additionally to the iteration here over envs / maybe we can combine both")
		for envName, config := range envs {
			env := api.Environment{
				Name: envName,
				Config: &api.Environment_Config{
					Upstream: transformUpstream(config.Upstream),
					EnvironmentGroup: config.EnvironmentGroup,
				},
				Locks:        map[string]*api.Lock{},
				Applications: map[string]*api.Environment_Application{},
			}
			if locks, err := s.GetEnvironmentLocks(envName); err != nil {
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
							app.Locks[lockId] = &api.Lock{
								Message:   lock.Message,
								LockId:    lockId,
								CreatedAt: timestamppb.New(lock.CreatedAt),
								CreatedBy: &api.Actor{
									Name:  lock.CreatedBy.Name,
									Email: lock.CreatedBy.Email,
								},
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
				Name:          appName,
				Releases:      []*api.Release{},
				SourceRepoUrl: "",
				Team:          "",
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
							CreatedAt:       timestamppb.New(rel.CreatedAt),
						}

						release.PrNumber = extractPrNumber(release.SourceMessage)

						app.Releases = append(app.Releases, release)
					}
				}
			}
			if team, err := s.GetApplicationTeamOwner(appName); err != nil {
				return nil, err
			} else {
				app.Team = team
			}
			if url, err := s.GetApplicationSourceRepoUrl(appName); err != nil {
				return nil, err
			} else {
				app.SourceRepoUrl = url
			}
			result.Applications[appName] = &app
		}
	}
	return &result, nil
}

type EnvSortOrder = map[string]int;

func mapEnvironmentsToGroups(envs map[string]config.EnvironmentConfig) []*api.EnvironmentGroup {
	var result = []*api.EnvironmentGroup{}
	var buckets = map[string]*api.EnvironmentGroup{}
	// first, group all envs into buckets by groupName
	for envName, env := range envs {
		var groupName = env.EnvironmentGroup
		if groupName == nil {
			groupName = &envName
		}
		var groupNameCopy = *groupName + "" // without this copy, unexpected pointer things happen :/
		var bucket, ok = buckets[*groupName]
		if !ok {
			bucket = &api.EnvironmentGroup{
				EnvironmentGroupName: groupNameCopy,
				Environments: []*api.Environment{},
			}
			buckets[groupNameCopy] = bucket
		}
		 var newEnv = &api.Environment{
			Name: envName,
			Config: &api.Environment_Config{
				Upstream: transformUpstream(env.Upstream),
				EnvironmentGroup: &groupNameCopy,
			},
			Locks:        map[string]*api.Lock{},
			Applications: map[string]*api.Environment_Application{},
		}
		bucket.Environments= append(bucket.Environments, newEnv)
	}
	// now we have all environments grouped correctly.
	// next step, sort envs by distance to prod.
	// to do that, we first need to calculate the distance to upstream.
	//
	tmpDistancesToUpstreamByEnv := map[string]uint32{}
	rest := []*api.Environment{}
	for _, bucket := range buckets {
		// first, find all envs with distance 0
		for i := 0; i < len(bucket.Environments); i++ {
			var environment = bucket.Environments[i]
			if environment.Config.Upstream.GetLatest() {
				environment.DistanceToUpstream = 0
				tmpDistancesToUpstreamByEnv[environment.Name] = 0
			} else {
				// and remember the rest:
				rest = append(rest, environment)
			}
		}
	}
		// now we have all envs remaining that have upstream.latest == false
		for ; len(rest) > 0; {
			nextRest := []*api.Environment{}
			for i := 0; i < len(rest); i++ {
				env := rest[i]
				upstreamEnv := env.Config.Upstream.GetEnvironment()
				_, ok := tmpDistancesToUpstreamByEnv[upstreamEnv]
				if ok {
					tmpDistancesToUpstreamByEnv[env.Name] = tmpDistancesToUpstreamByEnv[upstreamEnv] + 1
					env.DistanceToUpstream = tmpDistancesToUpstreamByEnv[env.Name]
				} else {
					nextRest = append(nextRest, env)
				}
			}
			if len(rest) == len(nextRest) {
				// if nothing changed in the previous for-loop, we have an undefined distance.
				// to avoid an infinite loop, we fill it with an arbitrary number:
				for i := 0; i < len(rest); i++{
					env := rest[i]
					tmpDistancesToUpstreamByEnv[env.Name] = uint32(len(envs) + 1)
				}
			}
			rest = nextRest
		}

	// now each environment has a distanceToUpstream.
	// we set the distanceToUpstream also to each group:
	for _, bucket := range buckets {
		bucket.DistanceToUpstream = bucket.Environments[0].DistanceToUpstream
	}

	// now we can actually sort the environments:
	for _, bucket := range buckets {
		sort.Sort(EnvironmentByDistance(bucket.Environments))
	}
	// environments are sorted, now sort the groups:
	// to do that we first need to convert the map into an array:
	for _, bucket := range buckets {
		result = append(result, bucket)
	}
	sort.Sort(EnvironmentGroupsByDistance(result))
	// now, everything is sorted, so we can add more data on top that depends on the sorting.
	// colllect all envs:
	tmpEnvs :=  []*api.Environment{}
	for i := 0; i < len(result); i++ {
		var group = result[i]
		//calculateEnvironmentPriorities(group.Environments)
		for j := 0; j < len(group.Environments); j++ {
			tmpEnvs = append(tmpEnvs , group.Environments[j])
		}
	}
	calculateEnvironmentPriorities(tmpEnvs)
	return result
}

func calculateEnvironmentPriorities(environments []*api.Environment) {
	// first find the maximum:
	var maxDistance uint32 = 0
	for i := 0; i < len(environments); i++ {
		maxDistance = max(maxDistance, environments[i].DistanceToUpstream)
	}
	// now we can assign each environment a priority
	for i := 0; i < len(environments); i++ {
		var env = environments[i]
		if env.DistanceToUpstream == maxDistance {
			env.Priority = api.Priority_PROD
		} else if env.DistanceToUpstream == maxDistance - 1 {
			env.Priority = api.Priority_PRE_PROD
		} else if env.DistanceToUpstream == 0 {
			env.Priority = api.Priority_UPSTREAM
		} else {
			env.Priority = api.Priority_OTHER
		}
	}
}

func max(a uint32, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}


type EnvironmentByDistance []*api.Environment

func (s EnvironmentByDistance) Len() int {
	return len(s)
}
func (s EnvironmentByDistance) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s EnvironmentByDistance) Less(i, j int) bool {
	// first sort by distance, then by name
	var di = s[i].DistanceToUpstream
	var dj = s[j].DistanceToUpstream
	if di != dj {
		return di < dj
	}
	return s[i].Name < s[j].Name
}

type EnvironmentGroupsByDistance []*api.EnvironmentGroup

func (s EnvironmentGroupsByDistance) Len() int {
	return len(s)
}
func (s EnvironmentGroupsByDistance) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s EnvironmentGroupsByDistance) Less(i, j int) bool {
	// first sort by distance, then by name
	var di = s[i].Environments[0].DistanceToUpstream
	var dj = s[j].Environments[0].DistanceToUpstream
	if dj != di {
		return di < dj
	}
	return s[i].Environments[0].Name < s[j].Environments[0].Name
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

func extractPrNumber(sourceMessage string) string {
	re := regexp.MustCompile("\\(#(\\d+)\\)")
	res := re.FindAllStringSubmatch(sourceMessage, -1)

	if len(res) == 0 {
		return ""
	} else {
		return res[len(res)-1][1]
	}
}
