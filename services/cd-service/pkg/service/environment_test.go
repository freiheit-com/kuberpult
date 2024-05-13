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
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/ptr"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestEnnvironmentConfigToApi(t *testing.T) {
	tcs := []struct {
		Name              string
		configConfig      config.EnvironmentConfig
		expectedApiConfig api.EnvironmentConfig
	}{
		{
			Name: "basic tranformation to api",
			configConfig: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Environment: "upstream",
					Latest:      true,
				},
				ArgoCd: &config.EnvironmentConfigArgoCd{
					Destination: config.ArgoCdDestination{
						Name:                 "destination",
						Server:               "destinationServer",
						Namespace:            ptr.FromString("destinationNamespace"),
						AppProjectNamespace:  ptr.FromString("destinationAppProjectNamespace"),
						ApplicationNamespace: ptr.FromString("destinationApplicationNamespace"),
					},
					SyncWindows: []config.ArgoCdSyncWindow{
						{
							Schedule: "syncWindowSchedule",
							Duration: "syncWindowDuration",
							Kind:     "syncWindowKind",
							Apps: []string{
								"app1",
								"app2",
								"app3",
							},
						},
					},
					ClusterResourceWhitelist: []config.AccessEntry{
						{
							Group: "accessListGroup",
							Kind:  "accessListKind",
						},
					},
					ApplicationAnnotations: map[string]string{
						"app1": "foo",
					},
					IgnoreDifferences: []config.ArgoCdIgnoreDifference{
						{
							Group:     "diffGroup",
							Kind:      "diffKind",
							Name:      "diffName",
							Namespace: "diffNamespace",
							JSONPointers: []string{
								"diffJSONPointer",
							},
							JqPathExpressions: []string{
								"diffJqPathExpression",
							},
							ManagedFieldsManagers: []string{
								"managedFieldsManager",
							},
						},
					},
					SyncOptions: []string{
						"syncOption",
					},
				},
			},
			expectedApiConfig: api.EnvironmentConfig{
				Upstream: &api.EnvironmentConfig_Upstream{
					Environment: ptr.FromString("upstream"),
					Latest:      ptr.Bool(true),
				},
				Argocd: &api.EnvironmentConfig_ArgoCD{
					SyncWindows: []*api.EnvironmentConfig_ArgoCD_SyncWindows{
						{
							Kind:     "syncWindowKind",
							Schedule: "syncWindowSchedule",
							Duration: "syncWindowDuration",
						},
					},
					Destination: &api.EnvironmentConfig_ArgoCD_Destination{
						Name:                 "destination",
						Server:               "destinationServer",
						Namespace:            ptr.FromString("destinationNamespace"),
						AppProjectNamespace:  ptr.FromString("destinationAppProjectNamespace"),
						ApplicationNamespace: ptr.FromString("destinationApplicationNamespace"),
					},
					AccessList: []*api.EnvironmentConfig_ArgoCD_AccessEntry{
						{
							Group: "accessListGroup",
							Kind:  "accessListKind",
						},
					},
				},
				EnvironmentGroup: nil,
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			actualApiConfig := TransformEnvironmentConfigToApi(tc.configConfig)
			// first diff individual parts as they ensure shorter readable diffs ...
			if !cmp.Equal(tc.expectedApiConfig.Upstream, actualApiConfig.Upstream, cmpopts.IgnoreUnexported(api.EnvironmentConfig_Upstream{})) {
				t.Fatalf("transformed api config upstream does not match expectation: %s", cmp.Diff(tc.expectedApiConfig.Upstream, actualApiConfig.Upstream, cmpopts.IgnoreUnexported(api.EnvironmentConfig_Upstream{})))
			}
			if !cmp.Equal(tc.expectedApiConfig.Argocd, actualApiConfig.Argocd, cmpopts.IgnoreUnexported(
				api.EnvironmentConfig_ArgoCD{},
				api.EnvironmentConfig_ArgoCD_AccessEntry{},
				api.EnvironmentConfig_ArgoCD_Destination{},
				api.EnvironmentConfig_ArgoCD_IgnoreDifferences{},
				api.EnvironmentConfig_ArgoCD_SyncWindows{},
			)) {
				t.Fatalf("transformed api config argo cd does not match expectation: %s", cmp.Diff(tc.expectedApiConfig.Argocd, actualApiConfig.Argocd, cmpopts.IgnoreUnexported(
					api.EnvironmentConfig_ArgoCD{},
					api.EnvironmentConfig_ArgoCD_AccessEntry{},
					api.EnvironmentConfig_ArgoCD_Destination{},
					api.EnvironmentConfig_ArgoCD_IgnoreDifferences{},
					api.EnvironmentConfig_ArgoCD_SyncWindows{},
				)))
			}
			if !cmp.Equal(tc.expectedApiConfig.EnvironmentGroup, actualApiConfig.EnvironmentGroup, cmpopts.IgnoreUnexported(api.EnvironmentGroup{})) {
				t.Fatalf("transformed api config env group does not match expectation: %s", cmp.Diff(tc.expectedApiConfig.EnvironmentGroup, actualApiConfig.EnvironmentGroup, cmpopts.IgnoreUnexported(api.EnvironmentGroup{})))
			}
			// ... then compare the full struct.
			if !cmp.Equal(&tc.expectedApiConfig, actualApiConfig, cmpopts.IgnoreUnexported(
				api.EnvironmentConfig{},
				api.EnvironmentConfig_Upstream{},
				api.EnvironmentConfig_ArgoCD{},
				api.EnvironmentConfig_ArgoCD_AccessEntry{},
				api.EnvironmentConfig_ArgoCD_Destination{},
				api.EnvironmentConfig_ArgoCD_IgnoreDifferences{},
				api.EnvironmentConfig_ArgoCD_SyncWindows{},
			)) {
				t.Fatalf("transformed api config does not match expectation: %s", cmp.Diff(&tc.expectedApiConfig, actualApiConfig, cmpopts.IgnoreUnexported(
					api.EnvironmentConfig{},
					api.EnvironmentConfig_Upstream{},
					api.EnvironmentConfig_ArgoCD{},
					api.EnvironmentConfig_ArgoCD_AccessEntry{},
					api.EnvironmentConfig_ArgoCD_Destination{},
					api.EnvironmentConfig_ArgoCD_IgnoreDifferences{},
					api.EnvironmentConfig_ArgoCD_SyncWindows{},
				)))
			}
		})
	}
}
