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
	"fmt"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/types"

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

	config, err := db.WithTransactionT(state.DBHandler, ctx, db.DefaultNumRetries, true, func(ctx context.Context, transaction *sql.Tx) (*config.EnvironmentConfig, error) {
		return state.GetEnvironmentConfigFromDB(ctx, transaction, types.EnvName(in.Environment))
	})
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, fmt.Errorf("could not find environment configuration for env: %q", in.Environment)
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
		ArgoConfigs:      transformArgoCdConfigsToApi(in.ArgoCdConfigs),
	}
}

func transformArgoCdToConfig(conf *api.ArgoCDEnvironmentConfiguration) *config.EnvironmentConfigArgoCd {
	syncWindows := transformSyncWindowsToConfig(conf.SyncWindows)
	clusterResourceWhitelist := transformAccessListToConfig(conf.AccessList)
	ignoreDifferences := transformIgnoreDifferencesToConfig(conf.IgnoreDifferences)
	argocd := &config.EnvironmentConfigArgoCd{
		Destination:              transformDestinationToConfig(conf.Destination),
		SyncWindows:              syncWindows,
		ClusterResourceWhitelist: clusterResourceWhitelist,
		ApplicationAnnotations:   conf.ApplicationAnnotations,
		IgnoreDifferences:        ignoreDifferences,
		SyncOptions:              conf.SyncOptions,
		ConcreteEnvName:          conf.ConcreteEnvName,
	}
	return argocd
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
			Environment: types.EnvName(upstream.GetEnvironment()),
		}
	}
	return nil
}

func transformUpstreamToApi(in *config.EnvironmentConfigUpstream) *api.EnvironmentConfig_Upstream {
	if in == nil {
		return nil
	}

	return &api.EnvironmentConfig_Upstream{
		Environment: types.StringPtr(in.Environment),
		Latest:      &in.Latest,
	}
}

func transformArgoCdToApi(in *config.EnvironmentConfigArgoCd) *api.ArgoCDEnvironmentConfiguration {
	if in == nil {
		return nil
	}
	return &api.ArgoCDEnvironmentConfiguration{
		ApplicationAnnotations: in.ApplicationAnnotations,
		IgnoreDifferences:      transformIgnoreDifferencesToApi(in.IgnoreDifferences),
		SyncOptions:            in.SyncOptions,
		SyncWindows:            transformSyncWindowsToApi(in.SyncWindows),
		Destination:            transformDestinationToApi(&in.Destination),
		AccessList:             transformAccessEntryToApi(in.ClusterResourceWhitelist),
		ConcreteEnvName:        in.ConcreteEnvName,
	}
}

func transformArgoCdConfigsToApi(in *config.ArgoCDConfigs) *api.EnvironmentConfig_ArgoConfigs {
	if in == nil {
		return nil
	}

	toReturn := &api.EnvironmentConfig_ArgoConfigs{
		CommonEnvPrefix: *in.CommonEnvPrefix,
		Configs:         make([]*api.ArgoCDEnvironmentConfiguration, 0),
	}

	for _, cfg := range in.ArgoCdConfigurations {
		toReturn.Configs = append(toReturn.Configs, transformArgoCdToApi(cfg))
	}
	return toReturn
}

func transformArgoCdConfigsToConfig(in *api.EnvironmentConfig_ArgoConfigs) *config.ArgoCDConfigs {
	if in == nil {
		return nil
	}

	toReturn := &config.ArgoCDConfigs{
		CommonEnvPrefix:      &in.CommonEnvPrefix,
		ArgoCdConfigurations: make([]*config.EnvironmentConfigArgoCd, 0),
	}

	for _, cfg := range in.Configs {
		toReturn.ArgoCdConfigurations = append(toReturn.ArgoCdConfigurations, transformArgoCdToConfig(cfg))
	}
	return toReturn
}

func transformSyncWindowsToConfig(syncWindows []*api.ArgoCDEnvironmentConfiguration_SyncWindows) []config.ArgoCdSyncWindow {
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

func transformSyncWindowsToApi(in []config.ArgoCdSyncWindow) []*api.ArgoCDEnvironmentConfiguration_SyncWindows {
	var out []*api.ArgoCDEnvironmentConfiguration_SyncWindows
	for _, syncWindow := range in {
		out = append(out, &api.ArgoCDEnvironmentConfiguration_SyncWindows{
			Applications: nil,
			Kind:         syncWindow.Kind,
			Schedule:     syncWindow.Schedule,
			Duration:     syncWindow.Duration,
		})
	}
	return out
}

func transformIgnoreDifferencesToApi(in []config.ArgoCdIgnoreDifference) []*api.ArgoCDEnvironmentConfiguration_IgnoreDifferences {
	var out []*api.ArgoCDEnvironmentConfiguration_IgnoreDifferences
	for _, currentIgnoreDifferences := range in {
		out = append(out, &api.ArgoCDEnvironmentConfiguration_IgnoreDifferences{
			Kind:                  currentIgnoreDifferences.Kind,
			Group:                 currentIgnoreDifferences.Group,
			Name:                  currentIgnoreDifferences.Name,
			Namespace:             currentIgnoreDifferences.Namespace,
			JsonPointers:          currentIgnoreDifferences.JSONPointers,
			JqPathExpressions:     currentIgnoreDifferences.JqPathExpressions,
			ManagedFieldsManagers: currentIgnoreDifferences.ManagedFieldsManagers,
		})
	}
	return out
}

func transformAccessListToConfig(accessList []*api.ArgoCDEnvironmentConfiguration_AccessEntry) []config.AccessEntry {
	var transformedAccessList []config.AccessEntry
	for _, accessEntry := range accessList {
		transformedAccessList = append(transformedAccessList, config.AccessEntry{
			Group: accessEntry.Group,
			Kind:  accessEntry.Kind,
		})
	}
	return transformedAccessList
}

func transformAccessEntryToApi(in []config.AccessEntry) []*api.ArgoCDEnvironmentConfiguration_AccessEntry {
	var out []*api.ArgoCDEnvironmentConfiguration_AccessEntry
	for _, accessEntry := range in {
		out = append(out, &api.ArgoCDEnvironmentConfiguration_AccessEntry{
			Group: accessEntry.Group,
			Kind:  accessEntry.Kind,
		})
	}
	return out
}

func transformIgnoreDifferencesToConfig(ignoreDifferences []*api.ArgoCDEnvironmentConfiguration_IgnoreDifferences) []config.ArgoCdIgnoreDifference {
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

func transformDestinationToConfig(in *api.ArgoCDEnvironmentConfiguration_Destination) config.ArgoCdDestination {
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

func transformDestinationToApi(in *config.ArgoCdDestination) *api.ArgoCDEnvironmentConfiguration_Destination {
	if in == nil {
		//exhaustruct:ignore
		return &api.ArgoCDEnvironmentConfiguration_Destination{}
	}
	return &api.ArgoCDEnvironmentConfiguration_Destination{
		Name:                 in.Name,
		Server:               in.Server,
		Namespace:            in.Namespace,
		AppProjectNamespace:  in.AppProjectNamespace,
		ApplicationNamespace: in.ApplicationNamespace,
	}
}
