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
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
)

func transformUpstreamToConfig(upstream *api.EnvironmentConfig_Upstream) *config.EnvironmentConfigUpstream {
	if upstream == nil {
		return nil
	}
	if upstream.GetLatest() {
		return &config.EnvironmentConfigUpstream{
			Latest: true,
		}
	}
	if upstream.GetEnvironment() != "" {
		return &config.EnvironmentConfigUpstream{
			Environment: upstream.GetEnvironment(),
		}
	}
	return nil
}

func transformSyncWindowsToConfig(syncWindows []*api.EnvironmentConfig_ArgoCD_SyncWindows) []config.ArgoCdSyncWindow {
	var transformedSyncWindows []config.ArgoCdSyncWindow
	for _, syncWindow := range syncWindows {
		transformedSyncWindows = append(transformedSyncWindows, config.ArgoCdSyncWindow{
			Schedule: syncWindow.Schedule,
			Duration: syncWindow.Duration,
			Kind:     syncWindow.Kind,
			Apps:     syncWindow.Applications,
		})
	}
	return transformedSyncWindows
}

func transformClusterResourceWhitelistToConfig(accessList []*api.EnvironmentConfig_ArgoCD_AccessEntry) []config.AccessEntry {
	var transformedAccessList []config.AccessEntry
	for _, accessEntry := range accessList {
		transformedAccessList = append(transformedAccessList, config.AccessEntry{
			Group: accessEntry.Group,
			Kind:  accessEntry.Kind,
		})
	}
	return transformedAccessList
}

func transformIgnoreDifferencesToConfig(ignoreDifferences []*api.EnvironmentConfig_ArgoCD_IgnoreDifferences) []config.ArgoCdIgnoreDifference {
	var transformedIgnoreDifferences []config.ArgoCdIgnoreDifference
	for _, ignoreDifference := range ignoreDifferences {
		transformedIgnoreDifferences = append(transformedIgnoreDifferences, config.ArgoCdIgnoreDifference{
			Group:                 ignoreDifference.Group,
			Kind:                  ignoreDifference.Kind,
			Name:                  ignoreDifference.Name,
			Namespace:             ignoreDifference.Namespace,
			JSONPointers:          ignoreDifference.JsonPointers,
			JqPathExpressions:     ignoreDifference.JqPathExpressions,
			ManagedFieldsManagers: ignoreDifference.ManagedFieldsManagers,
		})
	}
	return transformedIgnoreDifferences
}

func transformDestination(in *api.EnvironmentConfig_ArgoCD_Destination) config.ArgoCdDestination {
	if in == nil {
		return config.ArgoCdDestination{}
	}
	return config.ArgoCdDestination{
		Name:                 in.Name,
		Server:               in.Server,
		Namespace:            in.Namespace,
		AppProjectNamespace:  in.AppProjectNamespace,
		ApplicationNamespace: in.ApplicationNamespace,
	}
}
