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

Copyright 2023 freiheit.com*/

package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/httperrors"
	git "github.com/libgit2/git2go/v34"
	"google.golang.org/protobuf/types/known/timestamppb"

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
		return nil, httperrors.InternalError(ctx, err)
	} else {
		for envName, config := range envs {
			env := api.Environment{
				Name: envName,
				Config: &api.EnvironmentConfig{
					Upstream:         transformUpstream(config.Upstream),
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
	if in.GitRevision != "" {
		oid, err := git.NewOid(in.GitRevision)
		if err != nil {
			return nil, err
		}
		state, err := o.Repository.StateAt(oid)
		if err != nil {
			return nil, err
		}
		return o.getOverview(ctx, state)
	}
	return o.getOverview(ctx, o.Repository.State())
}

func (o *OverviewServiceServer) getOverview(
	ctx context.Context,
	s *repository.State) (*api.GetOverviewResponse, error) {
	var rev string
	if s.Commit != nil {
		rev = s.Commit.Id().String()
	}
	result := api.GetOverviewResponse{
		Environments:      map[string]*api.Environment{},
		Applications:      map[string]*api.Application{},
		EnvironmentGroups: []*api.EnvironmentGroup{},
		GitRevision:       rev,
	}
	if envs, err := s.GetEnvironmentConfigs(); err != nil {
		return nil, httperrors.InternalError(ctx, err)
	} else {
		result.EnvironmentGroups = mapEnvironmentsToGroups(envs)
		for envName, config := range envs {
			var groupName = deriveGroupName(config, envName)
			var envInGroup = getEnvironmentInGroup(result.EnvironmentGroups, groupName, envName)
			env := api.Environment{
				Name: envName,
				Config: &api.EnvironmentConfig{
					Upstream:         transformUpstream(config.Upstream),
					EnvironmentGroup: &groupName,
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
				envInGroup.Locks = env.Locks
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
			envInGroup.Applications = env.Applications

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
			app.UndeploySummary = deriveUndeploySummary(appName, result.EnvironmentGroups)
			result.Applications[appName] = &app
		}
	}
	return &result, nil
}

func deriveUndeploySummary(appName string, groups []*api.EnvironmentGroup) api.UndeploySummary {
	var allNormal = true
	var allUndeploy = true
	for _, group := range groups {
		for _, environment := range group.Environments {
			var app, exists = environment.Applications[appName]
			if !exists {
				continue
			}
			if app.Version == 0 {
				// if the app exists but nothing is deployed, we ignore this
				continue
			}
			if app.UndeployVersion {
				allNormal = false
			} else {
				allUndeploy = false
			}
		}
	}
	if allUndeploy {
		return api.UndeploySummary_Undeploy
	}
	if allNormal {
		return api.UndeploySummary_Normal
	}
	return api.UndeploySummary_Mixed

}

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

type EnvSortOrder = map[string]int

func mapEnvironmentsToGroups(envs map[string]config.EnvironmentConfig) []*api.EnvironmentGroup {
	var result = []*api.EnvironmentGroup{}
	var buckets = map[string]*api.EnvironmentGroup{}
	// first, group all envs into buckets by groupName
	for envName, env := range envs {
		var groupName = deriveGroupName(env, envName)
		var groupNameCopy = groupName + "" // without this copy, unexpected pointer things happen :/
		var bucket, ok = buckets[groupName]
		if !ok {
			bucket = &api.EnvironmentGroup{
				EnvironmentGroupName: groupNameCopy,
				Environments:         []*api.Environment{},
			}
			buckets[groupNameCopy] = bucket
		}
		var newEnv = &api.Environment{
			Name: envName,
			Config: &api.EnvironmentConfig{
				Upstream:         transformUpstream(env.Upstream),
				EnvironmentGroup: &groupNameCopy,
			},
			Locks:        map[string]*api.Lock{},
			Applications: map[string]*api.Environment_Application{},
		}
		bucket.Environments = append(bucket.Environments, newEnv)
	}
	// now we have all environments grouped correctly.
	// next step, sort envs by distance to prod.
	// to do that, we first need to calculate the distance to upstream.
	//
	tmpDistancesToUpstreamByEnv := map[string]uint32{}
	rest := []*api.Environment{}

	// we need to sort the buckets here because:
	// A) `range` of a map is not sorted in golang
	// B) the result depends on the sort order, even though this happens just in some special cases
	keys := make([]string, 0)
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		var bucket = buckets[k]
		// first, find all envs with distance 0
		for i := 0; i < len(bucket.Environments); i++ {
			var environment = bucket.Environments[i]
			if environment.Config.Upstream.GetLatest() {
				environment.DistanceToUpstream = 0
				tmpDistancesToUpstreamByEnv[environment.Name] = 0
			} else if environment.Config.Upstream == nil {
				// the environment has neither an upstream, nor latest configured. We can't determine where it belongs
				environment.DistanceToUpstream = 100 // we can just pick an arbitrary number
				tmpDistancesToUpstreamByEnv[environment.Name] = 100
			} else {
				upstreamEnv := environment.Config.Upstream.GetEnvironment()
				if _, exists := envs[upstreamEnv]; !exists { // upstreamEnv is not exists!
					tmpDistancesToUpstreamByEnv[upstreamEnv] = 666
				}
				// and remember the rest:
				rest = append(rest, environment)
			}
		}
	}
	// now we have all envs remaining that have upstream.latest == false
	for len(rest) > 0 {
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
			for i := 0; i < len(rest); i++ {
				env := rest[i]
				tmpDistancesToUpstreamByEnv[env.Config.Upstream.GetEnvironment()] = 666
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
	// now, everything is sorted, so we can calculate the env priorities. For that we convert the data to an array:
	var tmpEnvs []*api.Environment
	for i := 0; i < len(result); i++ {
		var group = result[i]
		for j := 0; j < len(group.Environments); j++ {
			tmpEnvs = append(tmpEnvs, group.Environments[j])
		}
	}
	calculateEnvironmentPriorities(tmpEnvs) // note that `tmpEnvs` were copied by reference - otherwise this function would have no effect on `result`
	return result
}

// either the groupName is set in the config, or we use the envName as a default
func deriveGroupName(env config.EnvironmentConfig, envName string) string {
	var groupName = env.EnvironmentGroup
	if groupName == nil {
		groupName = &envName
	}
	return *groupName
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
		} else if env.DistanceToUpstream == maxDistance-1 {
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

func transformUpstream(upstream *config.EnvironmentConfigUpstream) *api.EnvironmentConfig_Upstream {
	if upstream == nil {
		return nil
	}
	if upstream.Latest {
		return &api.EnvironmentConfig_Upstream{
			Latest: &upstream.Latest,
		}
	}
	if upstream.Environment != "" {
		return &api.EnvironmentConfig_Upstream{
			Environment: &upstream.Environment,
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
