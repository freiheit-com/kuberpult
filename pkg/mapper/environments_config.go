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

package mapper

import (
	"sort"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/config"
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
				DistanceToUpstream:   0,
				Priority:             api.Priority_PROD,
				EnvironmentGroupName: groupNameCopy,
				Environments:         []*api.Environment{},
			}
			buckets[groupNameCopy] = bucket
		}
		var newEnv = &api.Environment{
			DistanceToUpstream: 0,
			Priority:           api.Priority_PROD,
			Name:               envName,
			Config: &api.EnvironmentConfig{
				Argocd:           nil,
				Upstream:         TransformUpstream(env.Upstream),
				EnvironmentGroup: &groupNameCopy,
			},
			Locks:     map[string]*api.Lock{},
			TeamLocks: map[string]*api.Locks{},
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

	{
		var downstreamDepth uint32 = 0
		for _, group := range result {
			downstreamDepth = max(downstreamDepth, group.DistanceToUpstream)
		}

		for _, group := range result {
			group.Priority = calculateGroupPriority(group.DistanceToUpstream, downstreamDepth)
		}
	}

	return result
}

func calculateGroupPriority(distanceToUpstream, downstreamDepth uint32) api.Priority {
	lookup := [][]api.Priority{
		[]api.Priority{api.Priority_YOLO},
		[]api.Priority{api.Priority_UPSTREAM, api.Priority_PROD},
		[]api.Priority{api.Priority_UPSTREAM, api.Priority_PRE_PROD, api.Priority_PROD},
		[]api.Priority{api.Priority_UPSTREAM, api.Priority_PRE_PROD, api.Priority_CANARY, api.Priority_PROD},
		[]api.Priority{api.Priority_UPSTREAM, api.Priority_OTHER, api.Priority_PRE_PROD, api.Priority_CANARY, api.Priority_PROD},
	}
	if downstreamDepth > uint32(len(lookup)-1) {
		if distanceToUpstream == 0 {
			return api.Priority_UPSTREAM
		}
		if distanceToUpstream == downstreamDepth {
			return api.Priority_PROD
		}
		if distanceToUpstream == downstreamDepth-1 {
			return api.Priority_CANARY
		}
		if distanceToUpstream == downstreamDepth-2 {
			return api.Priority_PRE_PROD
		}
		return api.Priority_OTHER
	}
	return lookup[downstreamDepth][distanceToUpstream]
}

// either the groupName is set in the config, or we use the envName as a default
func DeriveGroupName(env config.EnvironmentConfig, envName string) string {
	var groupName = env.EnvironmentGroup
	if groupName == nil {
		groupName = &envName
	}
	return *groupName
}

type EnvsByName map[string]*api.Environment

func getUpstreamEnvironment(env *api.Environment, envsByName EnvsByName) *api.Environment {
	if env == nil || env.Config == nil || env.Config.Upstream == nil || env.Config.Upstream.Environment == nil {
		return nil
	}
	return envsByName[*env.Config.Upstream.Environment]
}

func calculateEnvironmentPriorities(environments []*api.Environment) {
	type Childs []string
	type ChildsByName map[string]Childs
	var envsByName = make(EnvsByName)
	var childsByName = make(ChildsByName)
	// latest is UPSTREAM, so mark them as such, and the rest as OTHER for now
	// oherwise append us to the list of the childs of the upstream env
	for i := 0; i < len(environments); i++ {
		var env = environments[i]
		envsByName[env.Name] = env
		if env.Config.Upstream.GetLatest() {
			env.Priority = api.Priority_UPSTREAM
		} else {
			env.Priority = api.Priority_OTHER
			if env.Config != nil && env.Config.Upstream != nil {
				var upstream = env.Config.Upstream.Environment
				if upstream != nil {
					var upstreamChildsBefore = childsByName[*upstream]
					childsByName[*upstream] = append(upstreamChildsBefore, env.Name)
				}
			}
		}
	}
	// remaining childless envs can now be identified as PROD
	for i := 0; i < len(environments); i++ {
		var env = environments[i]
		if len(childsByName[env.Name]) > 0 {
			continue
		}
		// even if an env is UPSTREAM, if it is a leaf too, it is a Priority_YOLO
		if env.Priority == api.Priority_UPSTREAM {
			env.Priority = api.Priority_YOLO
		} else {
			env.Priority = api.Priority_PROD
		}

		// find the two environments before PROD, if available
		var upstream = getUpstreamEnvironment(env, envsByName)
		var upstreamsUpstream = getUpstreamEnvironment(upstream, envsByName)

		if upstreamsUpstream == nil || upstreamsUpstream.Priority == api.Priority_UPSTREAM {
			// we only have at most one environment to mark, so its PRE_PROD
			if upstream != nil && upstream.Priority != api.Priority_UPSTREAM {
				upstream.Priority = api.Priority_PRE_PROD
			}
		} else {
			// we have two non-UPSTREAM environments to mark.
			upstream.Priority = api.Priority_CANARY
			upstreamsUpstream.Priority = api.Priority_PRE_PROD
		}
	}
}

func max(a uint32, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

// EnvironmentByDistance is there to sort by distance first and by name second
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
			Environment: nil,
			Latest:      &upstream.Latest,
		}
	}
	if upstream.Environment != "" {
		return &api.EnvironmentConfig_Upstream{
			Latest:      nil,
			Environment: &upstream.Environment,
		}
	}
	return nil
}

func TransformArgocd(config config.EnvironmentConfigArgoCd) *api.EnvironmentConfig_ArgoCD {
	var syncWindows []*api.EnvironmentConfig_ArgoCD_SyncWindows
	var accessList []*api.EnvironmentConfig_ArgoCD_AccessEntry
	var ignoreDifferences []*api.EnvironmentConfig_ArgoCD_IgnoreDifferences

	for _, i := range config.SyncWindows {
		syncWindow := &api.EnvironmentConfig_ArgoCD_SyncWindows{
			Kind:         i.Kind,
			Duration:     i.Duration,
			Schedule:     i.Schedule,
			Applications: i.Apps,
		}
		syncWindows = append(syncWindows, syncWindow)
	}

	for _, i := range config.ClusterResourceWhitelist {
		access := &api.EnvironmentConfig_ArgoCD_AccessEntry{
			Group: i.Group,
			Kind:  i.Kind,
		}
		accessList = append(accessList, access)
	}

	for _, i := range config.IgnoreDifferences {
		ignoreDiff := &api.EnvironmentConfig_ArgoCD_IgnoreDifferences{
			Group:                 i.Group,
			Kind:                  i.Kind,
			Name:                  i.Name,
			Namespace:             i.Namespace,
			JsonPointers:          i.JSONPointers,
			JqPathExpressions:     i.JqPathExpressions,
			ManagedFieldsManagers: i.ManagedFieldsManagers,
		}
		ignoreDifferences = append(ignoreDifferences, ignoreDiff)
	}

	return &api.EnvironmentConfig_ArgoCD{
		Destination: &api.EnvironmentConfig_ArgoCD_Destination{
			Name:                 config.Destination.Name,
			Server:               config.Destination.Server,
			Namespace:            config.Destination.Namespace,
			AppProjectNamespace:  config.Destination.AppProjectNamespace,
			ApplicationNamespace: config.Destination.ApplicationNamespace,
		},
		SyncWindows:            syncWindows,
		AccessList:             accessList,
		IgnoreDifferences:      ignoreDifferences,
		ApplicationAnnotations: config.ApplicationAnnotations,
		SyncOptions:            config.SyncOptions,
	}
}
