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

package versions

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"sort"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/go-cmp/cmp"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/pkg/types"
)

type step struct {
	ChangedApps         *api.GetChangedAppsResponse
	ConnectErr          error
	RecvErr             error
	CancelContext       bool
	OverviewResponse    *api.GetOverviewResponse
	AppDetailsResponses map[string]*api.GetAppDetailsResponse
	ExpectReady         bool
	ExpectedEvents      []KuberpultEvent
}

type expectedVersion struct {
	Revision         string
	Environment      string
	Application      string
	DeployedVersion  uint64
	DeployTime       time.Time
	SourceCommitId   string
	OverviewMetadata metadata.MD
	VersionMetadata  metadata.MD
	IsProduction     bool
}

type mockOverviewClient struct {
	grpc.ClientStream
	OverviewResponse           *api.GetOverviewResponse
	GetAllAppLocksResponse     *api.GetAllAppLocksResponse
	GetAllEnvTeamLocksResponse *api.GetAllEnvTeamLocksResponse
	AppDetailsResponses        map[string]*api.GetAppDetailsResponse
	LastMetadata               metadata.MD
	StartStep                  chan struct{}
	Steps                      chan step
	savedStep                  *step
}

// GetOverview implements api.OverviewServiceClient
func (m *mockOverviewClient) GetOverview(ctx context.Context, in *api.GetOverviewRequest, opts ...grpc.CallOption) (*api.GetOverviewResponse, error) {
	return m.OverviewResponse, nil
}

// GetOverview implements api.GetAllAppLocks
func (m *mockOverviewClient) GetAllAppLocks(ctx context.Context, in *api.GetAllAppLocksRequest, opts ...grpc.CallOption) (*api.GetAllAppLocksResponse, error) {
	return m.GetAllAppLocksResponse, nil
}

// GetOverview implements api.GetAllEnvLocks
func (m *mockOverviewClient) GetAllEnvTeamLocks(ctx context.Context, in *api.GetAllEnvTeamLocksRequest, opts ...grpc.CallOption) (*api.GetAllEnvTeamLocksResponse, error) {
	return m.GetAllEnvTeamLocksResponse, nil
}

// GetOverview implements api.GetAppDetails
func (m *mockOverviewClient) GetAppDetails(ctx context.Context, in *api.GetAppDetailsRequest, opts ...grpc.CallOption) (*api.GetAppDetailsResponse, error) {
	if resp := m.AppDetailsResponses[in.AppName]; resp != nil {
		return resp, nil
	}
	return nil, status.Error(codes.Unknown, "no")
}

// StreamOverview implements api.OverviewServiceClient
func (m *mockOverviewClient) StreamOverview(ctx context.Context, in *api.GetOverviewRequest, opts ...grpc.CallOption) (api.OverviewService_StreamOverviewClient, error) {
	return nil, nil
}

// StreamOverview implements api.OverviewServiceClient
func (m *mockOverviewClient) StreamChangedApps(ctx context.Context, in *api.GetChangedAppsRequest, opts ...grpc.CallOption) (api.OverviewService_StreamChangedAppsClient, error) {
	m.StartStep <- struct{}{}
	reply, ok := <-m.Steps
	if !ok {
		return nil, fmt.Errorf("exhausted: %w", io.EOF)
	}
	if reply.ConnectErr != nil {
		return nil, reply.ConnectErr
	}
	m.savedStep = &reply
	return m, nil
}

// StreamOverview implements api.OverviewServiceClient
func (m *mockOverviewClient) StreamDeploymentHistory(ctx context.Context, in *api.DeploymentHistoryRequest, opts ...grpc.CallOption) (api.OverviewService_StreamDeploymentHistoryClient, error) {
	return nil, nil
}

func (m *mockOverviewClient) Recv() (*api.GetChangedAppsResponse, error) {
	var reply step
	var ok bool
	if m.savedStep != nil {
		reply = *m.savedStep
		m.savedStep = nil
		ok = true
	} else {
		m.StartStep <- struct{}{}
		reply, ok = <-m.Steps

	}
	if !ok {
		return nil, fmt.Errorf("exhausted: %w", io.EOF)
	}
	m.OverviewResponse = reply.OverviewResponse
	m.AppDetailsResponses = reply.AppDetailsResponses //Endpoint responses at different steps
	return reply.ChangedApps, reply.RecvErr
}

var _ api.OverviewServiceClient = (*mockOverviewClient)(nil)

type mockVersionResponse struct {
	response *api.GetVersionResponse
	err      error
}
type mockVersionClient struct {
	responses    map[string]mockVersionResponse
	LastMetadata metadata.MD
}

func (m *mockVersionClient) GetVersion(ctx context.Context, in *api.GetVersionRequest, opts ...grpc.CallOption) (*api.GetVersionResponse, error) {
	m.LastMetadata, _ = metadata.FromOutgoingContext(ctx)
	key := fmt.Sprintf("%s/%s@%s", in.Environment, in.Application, in.GitRevision)
	res, ok := m.responses[key]
	if !ok {
		return nil, status.Error(codes.Unknown, "no")
	}
	return res.response, res.err
}

func (m *mockVersionClient) GetManifests(ctx context.Context, in *api.GetManifestsRequest, opts ...grpc.CallOption) (*api.GetManifestsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

type mockVersionEventProcessor struct {
	events []KuberpultEvent
}

func (m *mockVersionEventProcessor) ProcessKuberpultEvent(ctx context.Context, ev KuberpultEvent) {
	m.events = append(m.events, ev)
}

func TestVersionClientStream(t *testing.T) {
	t.Parallel()
	testOverview := &api.GetOverviewResponse{
		EnvironmentGroups: []*api.EnvironmentGroup{
			{

				EnvironmentGroupName: "staging-group",
				Priority:             api.Priority_UPSTREAM,
				Environments: []*api.Environment{
					{
						Name: "staging",
						Config: &api.EnvironmentConfig{
							ArgoConfigs: &api.EnvironmentConfig_ArgoConfigs{
								CommonEnvPrefix: "",
								Configs:         []*api.ArgoCDEnvironmentConfiguration{},
							},
							Upstream: &api.EnvironmentConfig_Upstream{},
							Argocd:   &api.ArgoCDEnvironmentConfiguration{},
						},
					},
				},
			},
		},
		GitRevision: "1234",
	}

	testOverviewMultiCluster := &api.GetOverviewResponse{
		EnvironmentGroups: []*api.EnvironmentGroup{
			{

				EnvironmentGroupName: "staging-group",
				Priority:             api.Priority_UPSTREAM,
				Environments: []*api.Environment{
					{
						Name: "staging",
						Config: &api.EnvironmentConfig{
							ArgoConfigs: &api.EnvironmentConfig_ArgoConfigs{
								CommonEnvPrefix: "prefix",
								Configs: []*api.ArgoCDEnvironmentConfiguration{
									{
										SyncWindows:            []*api.ArgoCDEnvironmentConfiguration_SyncWindows{},
										Destination:            &api.ArgoCDEnvironmentConfiguration_Destination{},
										AccessList:             []*api.ArgoCDEnvironmentConfiguration_AccessEntry{},
										ApplicationAnnotations: map[string]string{},
										IgnoreDifferences:      []*api.ArgoCDEnvironmentConfiguration_IgnoreDifferences{},
										SyncOptions:            []string{},
										ConcreteEnvName:        "primary",
									},
									{
										SyncWindows:            []*api.ArgoCDEnvironmentConfiguration_SyncWindows{},
										Destination:            &api.ArgoCDEnvironmentConfiguration_Destination{},
										AccessList:             []*api.ArgoCDEnvironmentConfiguration_AccessEntry{},
										ApplicationAnnotations: map[string]string{},
										IgnoreDifferences:      []*api.ArgoCDEnvironmentConfiguration_IgnoreDifferences{},
										SyncOptions:            []string{},
										ConcreteEnvName:        "secondary",
									},
								},
							},
						},
					},
				},
			},
		},
		GitRevision: "1234",
	}

	testOverviewWithDifferentEnvgroup := &api.GetOverviewResponse{
		EnvironmentGroups: []*api.EnvironmentGroup{
			{

				EnvironmentGroupName: "not-staging-group",
				Priority:             api.Priority_UPSTREAM,
				Environments: []*api.Environment{
					{
						Name: "staging",
					},
				},
			},
		},
		GitRevision: "1234",
	}
	testOverviewWithProdEnvs := &api.GetOverviewResponse{
		EnvironmentGroups: []*api.EnvironmentGroup{
			{
				EnvironmentGroupName: "production",
				Priority:             api.Priority_PROD,
				Environments: []*api.Environment{
					{
						Name: "production",
					},
				},
			},
			{
				EnvironmentGroupName: "canary",
				Priority:             api.Priority_CANARY,
				Environments: []*api.Environment{
					{
						Name: "canary",
					},
				},
			},
		},
		GitRevision: "1234",
	}
	emptyTestOverview := &api.GetOverviewResponse{
		EnvironmentGroups: []*api.EnvironmentGroup{},
		GitRevision:       "000",
	}

	tcs := []struct {
		Name                 string
		Steps                []step
		VersionResponses     map[string]mockVersionResponse
		GetOverviewResponses map[string]*api.GetOverviewResponse
		ExpectedVersions     []expectedVersion
		BracketClusters      []string
	}{
		{
			Name: "Retries connections and finishes",
			Steps: []step{
				{
					ConnectErr: fmt.Errorf("no"),

					ExpectReady: false,
				},
				{
					RecvErr: fmt.Errorf("no"),

					ExpectReady: false,
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
		},
		{
			Name: "Puts received overviews in the cache",
			Steps: []step{
				{
					ChangedApps: &api.GetChangedAppsResponse{
						ChangedApps: []*api.GetAppDetailsResponse{
							{
								Application: &api.Application{
									Team: "footeam",
									Name: "foo",
									Releases: []*api.Release{
										{
											Version:        1234,
											SourceCommitId: "",
										},
									},
								},
								Deployments: map[string]*api.Deployment{
									"staging": {
										Version: 1234,
										DeploymentMetaData: &api.Deployment_DeploymentMetaData{
											DeployTime: timestamppb.New(time.Unix(123456789, 0)),
										},
									},
								},
							},
						},
					},
					OverviewResponse: testOverview,
					ExpectReady:      true,
					ExpectedEvents: []KuberpultEvent{
						{
							Environment:       "staging",
							ParentEnvironment: "staging",
							Application:       "foo",
							EnvironmentGroup:  "staging-group",
							Team:              "footeam",
							Version: &VersionInfo{
								Version:        types.RolloutAppBracketVersionFromUint64(1234),
								SourceCommitId: "",
								DeployedAt:     time.Unix(123456789, 0).UTC(),
							},
						},
					},
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			ExpectedVersions: []expectedVersion{
				{
					Revision:        "1234",
					Environment:     "staging",
					Application:     "foo",
					DeployedVersion: 1234,
					SourceCommitId:  "",
					DeployTime:      time.Unix(123456789, 0).UTC(),
				},
			},
		},
		{
			Name: "Can resolve versions from the versions client",
			Steps: []step{
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			VersionResponses: map[string]mockVersionResponse{
				"staging/foo@1234": {
					response: &api.GetVersionResponse{
						Version:        1234,
						SourceCommitId: "",
						DeployedAt:     timestamppb.New(time.Unix(123456789, 0).UTC()),
					},
				},
			},
			ExpectedVersions: []expectedVersion{
				{
					Revision:        "1234",
					Environment:     "staging",
					Application:     "foo",
					DeployedVersion: 1234,
					SourceCommitId:  "",
					DeployTime:      time.Unix(123456789, 0).UTC(),
					VersionMetadata: nil,
				},
			},
		},
		{
			Name: "Don't notify twice for the same version",
			Steps: []step{
				{
					ChangedApps: &api.GetChangedAppsResponse{
						ChangedApps: []*api.GetAppDetailsResponse{
							{
								Application: &api.Application{
									Team: "footeam",
									Name: "foo",
									Releases: []*api.Release{
										{
											Version:        1,
											SourceCommitId: "00001",
										},
									},
								},
								Deployments: map[string]*api.Deployment{
									"staging": {
										Version: 1,
										DeploymentMetaData: &api.Deployment_DeploymentMetaData{
											DeployTime: timestamppb.New(time.Unix(123456789, 0)),
										},
									},
								},
							},
						},
					},
					OverviewResponse: testOverview,
					ExpectReady:      true,
					ExpectedEvents: []KuberpultEvent{
						{
							Environment:       "staging",
							ParentEnvironment: "staging",
							Application:       "foo",
							EnvironmentGroup:  "staging-group",
							Team:              "footeam",
							Version: &VersionInfo{
								Version:        types.RolloutAppBracketVersionFromUint64(1),
								SourceCommitId: "00001",
								DeployedAt:     time.Unix(123456789, 0).UTC(),
							},
						},
					},
				},
				{
					ChangedApps: &api.GetChangedAppsResponse{
						ChangedApps: []*api.GetAppDetailsResponse{
							{
								Application: &api.Application{
									Team: "footeam",
									Name: "foo",
									Releases: []*api.Release{
										{
											Version:        1,
											SourceCommitId: "00001",
										},
									},
								},
								Deployments: map[string]*api.Deployment{
									"staging": {
										Version: 1,
										DeploymentMetaData: &api.Deployment_DeploymentMetaData{
											DeployTime: timestamppb.New(time.Unix(123456789, 0)),
										},
									},
								},
							},
						},
					},
					OverviewResponse: testOverview,
					ExpectReady:      true,
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
		},
		{
			Name: "Notifies for each cluster",
			Steps: []step{
				{
					ChangedApps: &api.GetChangedAppsResponse{
						ChangedApps: []*api.GetAppDetailsResponse{
							{
								Application: &api.Application{
									Team: "footeam",
									Name: "foo",
									Releases: []*api.Release{
										{
											Version:        1,
											SourceCommitId: "00001",
										},
									},
								},
								Deployments: map[string]*api.Deployment{
									"staging": {
										Version: 1,
										DeploymentMetaData: &api.Deployment_DeploymentMetaData{
											DeployTime: timestamppb.New(time.Unix(123456789, 0)),
										},
									},
								},
							},
						},
					},
					OverviewResponse: testOverviewMultiCluster,
					ExpectReady:      true,
					ExpectedEvents: []KuberpultEvent{
						{
							Environment:       "prefix-staging-primary",
							ParentEnvironment: "staging",
							Application:       "foo",
							EnvironmentGroup:  "staging-group",
							Team:              "footeam",
							Version: &VersionInfo{
								Version:        types.RolloutAppBracketVersionFromUint64(1),
								SourceCommitId: "00001",
								DeployedAt:     time.Unix(123456789, 0).UTC(),
							},
						},
						{
							Environment:       "prefix-staging-secondary",
							ParentEnvironment: "staging",
							Application:       "foo",
							EnvironmentGroup:  "staging-group",
							Team:              "footeam",
							Version: &VersionInfo{
								Version:        types.RolloutAppBracketVersionFromUint64(1),
								SourceCommitId: "00001",
								DeployedAt:     time.Unix(123456789, 0).UTC(),
							},
						},
					},
				},
				{
					ChangedApps: &api.GetChangedAppsResponse{
						ChangedApps: []*api.GetAppDetailsResponse{
							{
								Application: &api.Application{
									Team: "footeam",
									Name: "foo",
									Releases: []*api.Release{
										{
											Version:        1,
											SourceCommitId: "00001",
										},
									},
								},
								Deployments: map[string]*api.Deployment{
									"staging": {
										Version: 1,
										DeploymentMetaData: &api.Deployment_DeploymentMetaData{
											DeployTime: timestamppb.New(time.Unix(123456789, 0)),
										},
									},
								},
							},
						},
					},
					OverviewResponse: testOverview,
					ExpectReady:      true,
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
		},
		{
			Name: "Only alter deployments in changed apps",
			Steps: []step{
				{
					ChangedApps: &api.GetChangedAppsResponse{
						ChangedApps: []*api.GetAppDetailsResponse{
							{
								Application: &api.Application{
									Team: "footeam",
									Name: "foo",
									Releases: []*api.Release{
										{
											Version:        1,
											SourceCommitId: "00001",
										},
									},
								},
								Deployments: map[string]*api.Deployment{
									"staging": {
										Version: 1,
										DeploymentMetaData: &api.Deployment_DeploymentMetaData{
											DeployTime: timestamppb.New(time.Unix(123456789, 0)),
										},
									},
								},
							},
						},
					},
					OverviewResponse: testOverview,
					ExpectReady:      true,
					ExpectedEvents: []KuberpultEvent{
						{
							Environment:       "staging",
							ParentEnvironment: "staging",
							Application:       "foo",
							EnvironmentGroup:  "staging-group",
							Team:              "footeam",
							Version: &VersionInfo{
								Version:        types.RolloutAppBracketVersionFromUint64(1),
								SourceCommitId: "00001",
								DeployedAt:     time.Unix(123456789, 0).UTC(),
							},
						},
					},
				},
				{
					ChangedApps: &api.GetChangedAppsResponse{
						ChangedApps: []*api.GetAppDetailsResponse{
							{
								Application: &api.Application{
									Team: "footeam",
									Name: "bar",
									Releases: []*api.Release{
										{
											Version:        1,
											SourceCommitId: "00001",
										},
									},
								},
								Deployments: map[string]*api.Deployment{
									"staging": {
										Version: 1,
										DeploymentMetaData: &api.Deployment_DeploymentMetaData{
											DeployTime: timestamppb.New(time.Unix(123456789, 0)),
										},
									},
								},
							},
						},
					},
					OverviewResponse: testOverview,
					ExpectReady:      true,
					ExpectedEvents: []KuberpultEvent{
						{
							Environment:       "staging",
							ParentEnvironment: "staging",
							Application:       "bar",
							EnvironmentGroup:  "staging-group",
							Team:              "footeam",
							Version: &VersionInfo{
								Version:        types.RolloutAppBracketVersionFromUint64(1),
								SourceCommitId: "00001",
								DeployedAt:     time.Unix(123456789, 0).UTC(),
							},
						},
					},
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
		},

		{
			Name: "Notify for apps that are deleted",
			Steps: []step{
				{
					ChangedApps: &api.GetChangedAppsResponse{
						ChangedApps: []*api.GetAppDetailsResponse{
							{
								Application: &api.Application{
									Team: "footeam",
									Name: "foo",
									Releases: []*api.Release{
										{
											Version:        1,
											SourceCommitId: "00001",
										},
									},
								},
								Deployments: map[string]*api.Deployment{
									"staging": {
										Version: 1,
										DeploymentMetaData: &api.Deployment_DeploymentMetaData{
											DeployTime: timestamppb.New(time.Unix(123456789, 0)),
										},
									},
								},
							},
						},
					},
					OverviewResponse: testOverview,
					ExpectReady:      true,
					ExpectedEvents: []KuberpultEvent{
						{
							Environment:       "staging",
							ParentEnvironment: "staging",
							Application:       "foo",
							EnvironmentGroup:  "staging-group",
							Team:              "footeam",
							Version: &VersionInfo{
								Version:        types.RolloutAppBracketVersionFromUint64(1),
								SourceCommitId: "00001",
								DeployedAt:     time.Unix(123456789, 0).UTC(),
							},
						},
					},
				},
				{
					ChangedApps: &api.GetChangedAppsResponse{
						ChangedApps: []*api.GetAppDetailsResponse{
							{
								Application: &api.Application{
									Name: "foo",
									Team: "footeam",
									Releases: []*api.Release{
										{
											Version:        1,
											SourceCommitId: "00001",
										},
									},
								},
								Deployments: map[string]*api.Deployment{},
							},
						},
					},
					OverviewResponse: emptyTestOverview,
					ExpectReady:      true,
					ExpectedEvents: []KuberpultEvent{
						{
							Environment:      "staging",
							Application:      "foo",
							EnvironmentGroup: "staging-group",
							Team:             "footeam",
							Version:          &VersionInfo{},
						},
					},
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
		},
		{
			Name: "Notify for apps that are deleted across reconnects",
			Steps: []step{
				{
					ChangedApps: &api.GetChangedAppsResponse{
						ChangedApps: []*api.GetAppDetailsResponse{
							{
								Application: &api.Application{
									Name: "foo",
									Team: "footeam",
									Releases: []*api.Release{
										{
											Version:        1,
											SourceCommitId: "00001",
										},
									},
								},
								Deployments: map[string]*api.Deployment{
									"staging": {
										Version: 1,
										DeploymentMetaData: &api.Deployment_DeploymentMetaData{
											DeployTime: timestamppb.New(time.Unix(123456789, 0)),
										},
									},
								},
							},
						},
					},
					OverviewResponse: testOverview,
					ExpectReady:      true,
					ExpectedEvents: []KuberpultEvent{
						{
							Environment:       "staging",
							ParentEnvironment: "staging",
							Application:       "foo",
							EnvironmentGroup:  "staging-group",
							Team:              "footeam",
							Version: &VersionInfo{
								Version:        types.RolloutAppBracketVersionFromUint64(1),
								SourceCommitId: "00001",
								DeployedAt:     time.Unix(123456789, 0).UTC(),
							},
						},
					},
				},
				{
					RecvErr: fmt.Errorf("no"),

					ExpectReady: false,
				},
				{
					ChangedApps: &api.GetChangedAppsResponse{
						ChangedApps: []*api.GetAppDetailsResponse{
							{
								Application: &api.Application{
									Name: "foo",
									Team: "footeam",
									Releases: []*api.Release{
										{
											Version:        1,
											SourceCommitId: "00001",
										},
									},
								},
								Deployments: map[string]*api.Deployment{},
							},
						},
					},
					OverviewResponse: emptyTestOverview,
					ExpectReady:      true,
					ExpectedEvents: []KuberpultEvent{
						{
							Environment: "staging",

							Application:      "foo",
							EnvironmentGroup: "staging-group",
							Team:             "footeam",
							Version:          &VersionInfo{},
						},
					},
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
		},
		{
			Name: "Updates environment groups",
			Steps: []step{
				{
					ChangedApps: &api.GetChangedAppsResponse{
						ChangedApps: []*api.GetAppDetailsResponse{
							{
								Application: &api.Application{
									Name: "foo",
									Team: "footeam",
									Releases: []*api.Release{
										{
											Version:        1,
											SourceCommitId: "00001",
										},
									},
								},
								Deployments: map[string]*api.Deployment{
									"staging": {
										Version: 1,
										DeploymentMetaData: &api.Deployment_DeploymentMetaData{
											DeployTime: timestamppb.New(time.Unix(123456789, 0)),
										},
									},
								},
							},
						},
					},
					OverviewResponse: testOverview,
					ExpectReady:      true,
					ExpectedEvents: []KuberpultEvent{
						{
							Environment:       "staging",
							ParentEnvironment: "staging",
							Application:       "foo",
							EnvironmentGroup:  "staging-group",
							Team:              "footeam",
							Version: &VersionInfo{
								Version:        types.RolloutAppBracketVersionFromUint64(1),
								SourceCommitId: "00001",
								DeployedAt:     time.Unix(123456789, 0).UTC(),
							},
						},
					},
				},
				{
					ChangedApps: &api.GetChangedAppsResponse{
						ChangedApps: []*api.GetAppDetailsResponse{
							{
								Application: &api.Application{
									Name: "foo",
									Team: "footeam",
									Releases: []*api.Release{
										{
											Version:        1,
											SourceCommitId: "00001",
										},
									},
								},
								Deployments: map[string]*api.Deployment{
									"staging": {
										Version: 2,
										DeploymentMetaData: &api.Deployment_DeploymentMetaData{
											DeployTime: timestamppb.New(time.Unix(123456789, 0)),
										},
									},
								},
							},
						},
					},
					OverviewResponse: testOverviewWithDifferentEnvgroup,
					ExpectReady:      true,
					ExpectedEvents: []KuberpultEvent{
						{
							Environment:       "staging",
							ParentEnvironment: "staging",
							Application:       "foo",
							EnvironmentGroup:  "not-staging-group",
							Team:              "footeam",
							Version: &VersionInfo{
								Version:        types.RolloutAppBracketVersionFromUint64(2),
								SourceCommitId: "00002",
								DeployedAt:     time.Unix(123456789, 0).UTC(),
							},
						},
					},
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
		},
		{
			Name: "Reports production environments",
			Steps: []step{
				{
					ChangedApps: &api.GetChangedAppsResponse{
						ChangedApps: []*api.GetAppDetailsResponse{
							{
								Application: &api.Application{
									Name: "foo",
									Team: "footeam",
									Releases: []*api.Release{
										{
											Version:        1234,
											SourceCommitId: "00002",
										},
									},
								},
								Deployments: map[string]*api.Deployment{
									"production": {
										Version: 1234,
										DeploymentMetaData: &api.Deployment_DeploymentMetaData{
											DeployTime: timestamppb.New(time.Unix(123456789, 0)),
										},
									},
									"canary": {
										Version: 1234,
										DeploymentMetaData: &api.Deployment_DeploymentMetaData{
											DeployTime: timestamppb.New(time.Unix(123456789, 0)),
										},
									},
								},
							},
						},
					},
					OverviewResponse: testOverviewWithProdEnvs,
					ExpectReady:      true,
					ExpectedEvents: []KuberpultEvent{
						{
							Environment:       "production",
							ParentEnvironment: "production",
							Application:       "foo",
							EnvironmentGroup:  "production",
							IsProduction:      true,
							Team:              "footeam",
							Version: &VersionInfo{
								Version:        types.RolloutAppBracketVersionFromUint64(1234),
								SourceCommitId: "00002",
								DeployedAt:     time.Unix(123456789, 0).UTC(),
							},
						},
						{
							Environment:       "canary",
							ParentEnvironment: "canary",
							Application:       "foo",
							EnvironmentGroup:  "canary",
							IsProduction:      true,
							Team:              "footeam",
							Version: &VersionInfo{
								Version:        types.RolloutAppBracketVersionFromUint64(1234),
								SourceCommitId: "00002",
								DeployedAt:     time.Unix(123456789, 0).UTC(),
							},
						},
					},
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
		},
		{
			Name:            "Emits bracket events for bracket clusters",
			BracketClusters: []string{"bracket-env"},
			Steps: []step{
				{
					ChangedApps: &api.GetChangedAppsResponse{
						ChangedBrackets: []*api.GetBracketDetailsResponse{
							{
								BracketName: "my-bracket",
								Deployments: map[string]*api.BracketDeployment{
									"bracket-env": {
										Version:        "5:3",
										SourceCommitId: "abc123",
										DeployedAt:     timestamppb.New(time.Unix(123456789, 0)),
									},
								},
							},
						},
					},
					OverviewResponse: &api.GetOverviewResponse{
						EnvironmentGroups: []*api.EnvironmentGroup{
							{
								EnvironmentGroupName: "bracket-group",
								Priority:             api.Priority_UPSTREAM,
								Environments: []*api.Environment{
									{Name: "bracket-env"},
								},
							},
						},
						GitRevision: "1234",
					},
					ExpectReady: true,
					ExpectedEvents: []KuberpultEvent{
						{
							Environment:       "bracket-env",
							ParentEnvironment: "bracket-env",
							Application:       "my-bracket",
							EnvironmentGroup:  "bracket-group",
							IsProduction:      false,
							Team:              "",
							Version: &VersionInfo{
								Version:        types.RolloutAppBracketVersion("5:3"),
								SourceCommitId: "abc123",
								DeployedAt:     time.Unix(123456789, 0).UTC(),
							},
						},
					},
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
		},
		{
			Name:            "Ignores bracket events for non-bracket clusters",
			BracketClusters: []string{},
			Steps: []step{
				{
					ChangedApps: &api.GetChangedAppsResponse{
						ChangedBrackets: []*api.GetBracketDetailsResponse{
							{
								BracketName: "my-bracket",
								Deployments: map[string]*api.BracketDeployment{
									"non-bracket-env": {
										Version: "5:3",
									},
								},
							},
						},
					},
					OverviewResponse: &api.GetOverviewResponse{
						EnvironmentGroups: []*api.EnvironmentGroup{
							{
								EnvironmentGroupName: "other-group",
								Priority:             api.Priority_UPSTREAM,
								Environments: []*api.Environment{
									{Name: "non-bracket-env"},
								},
							},
						},
						GitRevision: "1234",
					},
					ExpectReady:    true,
					ExpectedEvents: nil,
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
		},
		{
			Name: "Notify deletion for env removed from app while app still deployed elsewhere",
			Steps: []step{
				{
					ChangedApps: &api.GetChangedAppsResponse{
						ChangedApps: []*api.GetAppDetailsResponse{
							{
								Application: &api.Application{
									Team: "footeam",
									Name: "foo",
								},
								Deployments: map[string]*api.Deployment{
									"staging": {
										Version: 1,
										DeploymentMetaData: &api.Deployment_DeploymentMetaData{
											DeployTime: timestamppb.New(time.Unix(123456789, 0)),
										},
									},
									"production": {
										Version: 1,
										DeploymentMetaData: &api.Deployment_DeploymentMetaData{
											DeployTime: timestamppb.New(time.Unix(123456789, 0)),
										},
									},
								},
							},
						},
					},
					OverviewResponse: &api.GetOverviewResponse{
						EnvironmentGroups: []*api.EnvironmentGroup{
							{
								EnvironmentGroupName: "staging-group",
								Priority:             api.Priority_UPSTREAM,
								Environments:         []*api.Environment{{Name: "staging"}},
							},
							{
								EnvironmentGroupName: "prod-group",
								Priority:             api.Priority_PROD,
								Environments:         []*api.Environment{{Name: "production"}},
							},
						},
						GitRevision: "1234",
					},
					ExpectReady: true,
					ExpectedEvents: []KuberpultEvent{
						{
							Environment:       "production",
							ParentEnvironment: "production",
							Application:       "foo",
							EnvironmentGroup:  "prod-group",
							IsProduction:      true,
							Team:              "footeam",
							Version: &VersionInfo{
								Version:    types.RolloutAppBracketVersionFromUint64(1),
								DeployedAt: time.Unix(123456789, 0).UTC(),
							},
						},
						{
							Environment:       "staging",
							ParentEnvironment: "staging",
							Application:       "foo",
							EnvironmentGroup:  "staging-group",
							Team:              "footeam",
							Version: &VersionInfo{
								Version:    types.RolloutAppBracketVersionFromUint64(1),
								DeployedAt: time.Unix(123456789, 0).UTC(),
							},
						},
					},
				},
				{
					// production deployment removed; staging unchanged
					ChangedApps: &api.GetChangedAppsResponse{
						ChangedApps: []*api.GetAppDetailsResponse{
							{
								Application: &api.Application{
									Team: "footeam",
									Name: "foo",
								},
								Deployments: map[string]*api.Deployment{
									"staging": {
										Version: 1,
										DeploymentMetaData: &api.Deployment_DeploymentMetaData{
											DeployTime: timestamppb.New(time.Unix(123456789, 0)),
										},
									},
								},
							},
						},
					},
					OverviewResponse: &api.GetOverviewResponse{
						EnvironmentGroups: []*api.EnvironmentGroup{
							{
								EnvironmentGroupName: "staging-group",
								Priority:             api.Priority_UPSTREAM,
								Environments:         []*api.Environment{{Name: "staging"}},
							},
							{
								EnvironmentGroupName: "prod-group",
								Priority:             api.Priority_PROD,
								Environments:         []*api.Environment{{Name: "production"}},
							},
						},
						GitRevision: "1234",
					},
					ExpectReady: true,
					ExpectedEvents: []KuberpultEvent{
						{
							Environment:      "production",
							Application:      "foo",
							EnvironmentGroup: "prod-group",
							Team:             "footeam",
							Version:          &VersionInfo{},
						},
					},
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			vp := &mockVersionEventProcessor{}
			startSteps := make(chan struct{})
			steps := make(chan step)
			moc := &mockOverviewClient{StartStep: startSteps, Steps: steps}
			if tc.VersionResponses == nil {
				tc.VersionResponses = map[string]mockVersionResponse{}
			}
			mvc := &mockVersionClient{responses: tc.VersionResponses}
			bracketClusters := tc.BracketClusters
			if bracketClusters == nil {
				bracketClusters = []string{}
			}
			vc := New(moc, mvc, nil, false, false, false, []string{}, *setupDB(t), 50, 50, nil, bracketClusters)
			hs := &setup.HealthServer{}
			hs.BackOffFactory = func() backoff.BackOff {
				return backoff.NewConstantBackOff(time.Millisecond)
			}
			errCh := make(chan error)

			go func() {
				errCh <- vc.ConsumeEvents(ctx, vp, hs.Reporter("versions"))
			}()
			for i, s := range tc.Steps {
				<-startSteps
				if i > 0 {
					assertStep(t, i-1, tc.Steps[i-1], vp, hs)
				}
				if s.CancelContext {
					cancel()
				}
				select {
				case steps <- s:
				case err := <-errCh:
					t.Fatalf("expected no error but received %q", err)
				case <-time.After(10 * time.Second):
					t.Fatal("test got stuck after 10 seconds")
				}
			}
			cancel()
			err := <-errCh
			if err != nil {
				t.Errorf("expected no error, but received %q", err)
			}
			if len(steps) != 0 {
				t.Errorf("expected all events to be consumed, but got %d left", len(steps))
			}
			assertExpectedVersions(t, tc.ExpectedVersions, vc, moc, mvc)

		})
	}
}

func assertStep(t *testing.T, i int, s step, vp *mockVersionEventProcessor, hs *setup.HealthServer) {
	if hs.IsReady("versions") != s.ExpectReady {
		t.Errorf("wrong readyness in step %d, expected %t but got %t", i, s.ExpectReady, hs.IsReady("versions"))
	}
	//Sort this to avoid flakeyness based on order
	sort.Slice(vp.events, func(i, j int) bool {
		return vp.events[i].Environment < vp.events[j].Environment
	})
	//Sort this to avoid flakeyness based on order
	sort.Slice(s.ExpectedEvents, func(i, j int) bool {
		return s.ExpectedEvents[i].Environment < s.ExpectedEvents[j].Environment
	})
	if !cmp.Equal(s.ExpectedEvents, vp.events) {
		t.Errorf("version events differ: %s", cmp.Diff(s.ExpectedEvents, vp.events))
	}
	vp.events = nil
}

func assertExpectedVersions(t *testing.T, expectedVersions []expectedVersion, vc VersionClient, mc *mockOverviewClient, mvc *mockVersionClient) {
	for _, ev := range expectedVersions {
		version, err := vc.GetVersion(context.Background(), ev.Revision, ev.Environment, ev.Application)
		if err != nil {
			t.Errorf("expected no error for %s/%s@%s, but got %q", ev.Environment, ev.Application, ev.Revision, err)
			continue
		}
		//We ignore the timestamp as it is based on test execution. Everything else we check

		if version.Version != types.RolloutAppBracketVersionFromUint64(ev.DeployedVersion) {
			t.Errorf("expected version %d to be deployed for %s/%s@%s but got %s", ev.DeployedVersion, ev.Environment, ev.Application, ev.Revision, version.Version)
		}

		if version.SourceCommitId != ev.SourceCommitId {
			t.Errorf("expected source commit id to be %q for %s/%s@%s but got %q", ev.SourceCommitId, ev.Environment, ev.Application, ev.Revision, version.SourceCommitId)
		}
		if !cmp.Equal(mc.LastMetadata, ev.OverviewMetadata) {
			t.Errorf("mismachted version metadata %s", cmp.Diff(mc.LastMetadata, ev.OverviewMetadata))
		}
		if !cmp.Equal(mvc.LastMetadata, ev.VersionMetadata) {
			t.Errorf("mismachted version metadata %s", cmp.Diff(mvc.LastMetadata, ev.VersionMetadata))
		}

	}
}

// setupDB returns a new DBHandler with a tmp directory every time, so tests can are completely independent
func setupDB(t *testing.T) *db.DBHandler {
	ctx := context.Background()
	migrationsPath, err := db.CreateMigrationsPath(4)
	if err != nil {
		t.Fatal(err)
	}
	dbConfig, err := db.ConnectToPostgresContainer(ctx, t, migrationsPath, t.Name())
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	t.Logf("directory for DB migrations: %s", migrationsPath)
	t.Logf("tmp dir for DB data: %s", tmpDir)

	migErr := db.RunDBMigrations(ctx, *dbConfig)
	if migErr != nil {
		t.Fatal(migErr)
	}

	dbHandler, err := db.Connect(ctx, *dbConfig)
	if err != nil {
		t.Fatal(err)
	}
	setupErr := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
		if err != nil {
			return err
		}
		err = dbHandler.DBWriteEnvironment(ctx, transaction, "staging", config.EnvironmentConfig{})
		if err != nil {
			return err
		}
		err = dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "foo", db.AppStateChangeCreate, db.DBAppMetaData{}, "foo")
		if err != nil {
			return err
		}
		var version uint64 = 1234
		err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
			ReleaseNumbers: types.ReleaseNumbers{
				Revision: 0,
				Version:  &version,
			},
			Created: time.Unix(123456789, 0).UTC(),
			App:     "foo",
			Manifests: db.DBReleaseManifests{
				Manifests: map[types.EnvName]string{"staging": ""},
			},
			Metadata: db.DBReleaseMetaData{},
		})
		if err != nil {
			return err
		}

		err = dbHandler.DBUpdateOrCreateDeployment(ctx, transaction, db.Deployment{
			Created: time.Unix(123456789, 0).UTC(),
			App:     "foo",
			Env:     "staging",
			ReleaseNumbers: types.ReleaseNumbers{
				Revision: 0,
				Version:  &version,
			},
			TransformerID: 0,
		})

		return err
	})

	if setupErr != nil {
		t.Fatal(setupErr)
	}
	return dbHandler
}

func TestGetVersion_Bracket(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	migrationsPath, err := db.CreateMigrationsPath(4)
	if err != nil {
		t.Fatal(err)
	}
	dbConfig, err := db.ConnectToPostgresContainer(ctx, t, migrationsPath, t.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("directory for DB migrations: %s", migrationsPath)
	if err := db.RunDBMigrations(ctx, *dbConfig); err != nil {
		t.Fatal(err)
	}
	dbHandler, err := db.Connect(ctx, *dbConfig)
	if err != nil {
		t.Fatal(err)
	}

	const bracketEnv = "bracket-env"
	const bracketName types.ArgoBracketName = "my-bracket"
	var versionA uint64 = 5
	var versionB uint64 = 3

	// Use two separate transactions so deployments get distinct transaction timestamps.
	setupErr := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, tx *sql.Tx) error {
		if err := dbHandler.DBWriteMigrationsTransformer(ctx, tx); err != nil {
			return err
		}
		if err := dbHandler.DBWriteEnvironment(ctx, tx, bracketEnv, config.EnvironmentConfig{}); err != nil {
			return err
		}
		for _, appName := range []types.AppName{"app-a", "app-b"} {
			if err := dbHandler.DBInsertOrUpdateApplication(ctx, tx, appName, db.AppStateChangeCreate, db.DBAppMetaData{}, bracketName); err != nil {
				return err
			}
		}
		// releases
		for _, r := range []db.DBReleaseWithMetaData{
			{
				ReleaseNumbers: types.ReleaseNumbers{Version: &versionA, Revision: 0},
				App:            "app-a",
				Manifests:      db.DBReleaseManifests{Manifests: map[types.EnvName]string{bracketEnv: ""}},
				Metadata:       db.DBReleaseMetaData{SourceCommitId: "commit-a"},
			},
			{
				ReleaseNumbers: types.ReleaseNumbers{Version: &versionB, Revision: 0},
				App:            "app-b",
				Manifests:      db.DBReleaseManifests{Manifests: map[types.EnvName]string{bracketEnv: ""}},
				Metadata:       db.DBReleaseMetaData{SourceCommitId: "commit-b"},
			},
		} {
			if err := dbHandler.DBUpdateOrCreateRelease(ctx, tx, r); err != nil {
				return err
			}
		}
		// deploy app-a in this transaction
		if err := dbHandler.DBUpdateOrCreateDeployment(ctx, tx, db.Deployment{
			App: "app-a", Env: bracketEnv,
			ReleaseNumbers: types.ReleaseNumbers{Version: &versionA, Revision: 0},
		}); err != nil {
			return err
		}
		// bracket history: my-bracket → [app-a, app-b]
		return db.DBInsertBracketHistory(ctx, dbHandler, tx, db.BracketRow{
			CreatedAt: time.Now(),
			AllBracketsJsonBlob: db.BracketJsonBlob{
				BracketMap: map[types.ArgoBracketName]db.AppNames{
					bracketName: {"app-a", "app-b"},
				},
			},
		}, 0)
	})
	if setupErr != nil {
		t.Fatal(setupErr)
	}

	// Deploy app-b in a separate transaction so it gets a later timestamp.
	if err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, tx *sql.Tx) error {
		return dbHandler.DBUpdateOrCreateDeployment(ctx, tx, db.Deployment{
			App: "app-b", Env: bracketEnv,
			ReleaseNumbers: types.ReleaseNumbers{Version: &versionB, Revision: 0},
		})
	}); err != nil {
		t.Fatal(err)
	}

	vc := New(nil, nil, nil, false, false, false, []string{}, *dbHandler, 50, 50, nil, []string{bracketEnv})

	version, err := vc.GetVersion(ctx, "5:3", bracketEnv, string(bracketName))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version == nil {
		t.Fatal("expected non-nil VersionInfo")
	}
	// The revision string is returned as-is as the Version field.
	if version.Version != types.RolloutAppBracketVersion("5:3") {
		t.Errorf("expected Version=%q, got %q", "5:3", version.Version)
	}
	// DeployedAt must be non-zero since both apps have deployments.
	if version.DeployedAt.IsZero() {
		t.Errorf("expected non-zero DeployedAt")
	}
	// app-b is deployed later (separate transaction), so its SourceCommitId should be returned.
	if version.SourceCommitId != "commit-b" {
		t.Errorf("expected SourceCommitId=%q, got %q", "commit-b", version.SourceCommitId)
	}
}
