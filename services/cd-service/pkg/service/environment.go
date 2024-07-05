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

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"

	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

type EnvironmentServiceServer struct {
	Repository repository.Repository
}

func (o *EnvironmentServiceServer) GetEnvironmentConfig(
	ctx context.Context,
	in *api.GetEnvironmentConfigRequest) (*api.GetEnvironmentConfigResponse, error) {
	state := o.Repository.State()

	config, err := db.WithTransactionT(state.DBHandler, ctx, true, func(ctx context.Context, transaction *sql.Tx) (*config.EnvironmentConfig, error) {
		return state.GetEnvironmentConfig(ctx, transaction, in.Environment)
	})

	if err != nil {
		return nil, err
	}
	var out api.GetEnvironmentConfigResponse
	out.Config = TransformEnvironmentConfigToApi(*config)
	return &out, nil
}

func TransformEnvironmentConfigToApi(in config.EnvironmentConfig) *api.EnvironmentConfig {
	return &api.EnvironmentConfig{
		Upstream:         transformUpstreamToApi(in.Upstream),
		Argocd:           transformArgoCdToApi(in.ArgoCd),
		EnvironmentGroup: in.EnvironmentGroup,
	}
}

func transformUpstreamToConfig(upstream *api.EnvironmentConfig_Upstream) *config.EnvironmentConfigUpstream {
	if upstream == nil {
		return nil
	}
	if upstream.GetLatest() {
		return &config.EnvironmentConfigUpstream{
			Environment: "",
			Latest:      true,
		}
	}
	if upstream.GetEnvironment() != "" {
		return &config.EnvironmentConfigUpstream{
			Latest:      false,
			Environment: upstream.GetEnvironment(),
		}
	}
	return nil
}

func transformUpstreamToApi(in *config.EnvironmentConfigUpstream) *api.EnvironmentConfig_Upstream {
	if in == nil {
		return nil
	}
	return &api.EnvironmentConfig_Upstream{
		Environment: &in.Environment,
		Latest:      &in.Latest,
	}
}

func transformArgoCdToApi(in *config.EnvironmentConfigArgoCd) *api.EnvironmentConfig_ArgoCD {
	if in == nil {
		return nil
	}
	return &api.EnvironmentConfig_ArgoCD{
		ApplicationAnnotations: nil,
		IgnoreDifferences:      nil,
		SyncOptions:            nil,
		SyncWindows:            transformSyncWindowsToApi(in.SyncWindows),
		Destination:            transformDestinationToApi(&in.Destination),
		AccessList:             transformAccessEntryToApi(in.ClusterResourceWhitelist),
	}
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

func transformSyncWindowsToApi(in []config.ArgoCdSyncWindow) []*api.EnvironmentConfig_ArgoCD_SyncWindows {
	var out []*api.EnvironmentConfig_ArgoCD_SyncWindows
	for _, syncWindow := range in {
		out = append(out, &api.EnvironmentConfig_ArgoCD_SyncWindows{
			Applications: nil,
			Kind:         syncWindow.Kind,
			Schedule:     syncWindow.Schedule,
			Duration:     syncWindow.Duration,
		})
	}
	return out
}

func transformAccessListToConfig(accessList []*api.EnvironmentConfig_ArgoCD_AccessEntry) []config.AccessEntry {
	var transformedAccessList []config.AccessEntry
	for _, accessEntry := range accessList {
		transformedAccessList = append(transformedAccessList, config.AccessEntry{
			Group: accessEntry.Group,
			Kind:  accessEntry.Kind,
		})
	}
	return transformedAccessList
}

func transformAccessEntryToApi(in []config.AccessEntry) []*api.EnvironmentConfig_ArgoCD_AccessEntry {
	var out []*api.EnvironmentConfig_ArgoCD_AccessEntry
	for _, accessEntry := range in {
		out = append(out, &api.EnvironmentConfig_ArgoCD_AccessEntry{
			Group: accessEntry.Group,
			Kind:  accessEntry.Kind,
		})
	}
	return out
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

func transformDestinationToConfig(in *api.EnvironmentConfig_ArgoCD_Destination) config.ArgoCdDestination {
	if in == nil {
		//exhaustruct:ignore
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

func transformDestinationToApi(in *config.ArgoCdDestination) *api.EnvironmentConfig_ArgoCD_Destination {
	if in == nil {
		//exhaustruct:ignore
		return &api.EnvironmentConfig_ArgoCD_Destination{}
	}
	return &api.EnvironmentConfig_ArgoCD_Destination{
		Name:                 in.Name,
		Server:               in.Server,
		Namespace:            in.Namespace,
		AppProjectNamespace:  in.AppProjectNamespace,
		ApplicationNamespace: in.ApplicationNamespace,
	}
}
