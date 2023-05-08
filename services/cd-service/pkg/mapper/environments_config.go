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

package mapper

import (
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"path/filepath"
	"sort"
)

type EnvSortOrder = map[string]int

func MapEnvironmentsToGroups(envs map[string]config.EnvironmentConfig) []*api.EnvironmentGroup {
	var result = []*api.EnvironmentGroup{}
	var buckets = map[string]*api.EnvironmentGroup{}
	// first, group all envs into buckets by groupName
	for envName, env := range envs {
		var groupName = DeriveGroupName(env, envName)
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
				Upstream:         TransformUpstream(env.Upstream),
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
func DeriveGroupName(env config.EnvironmentConfig, envName string) string {
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

func TransformUpstream(upstream *config.EnvironmentConfigUpstream) *api.EnvironmentConfig_Upstream {
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

func TransformSyncWindows(syncWindows []config.ArgoCdSyncWindow, appName string) ([]*api.Environment_Application_ArgoCD_SyncWindow, error) {
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
