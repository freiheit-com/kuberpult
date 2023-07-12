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

package versions

import (
	"context"
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/google/go-cmp/cmp"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type expectedVersion struct {
	Revision        string
	Environment     string
	Application     string
	DeployedVersion uint64
	Metadata        metadata.MD
}

type mockOverviewClient struct {
	Responses    map[string]*api.GetOverviewResponse
	LastMetadata metadata.MD
}

// GetOverview implements api.OverviewServiceClient
func (m *mockOverviewClient) GetOverview(ctx context.Context, in *api.GetOverviewRequest, opts ...grpc.CallOption) (*api.GetOverviewResponse, error) {
	m.LastMetadata, _ = metadata.FromOutgoingContext(ctx)
	if resp := m.Responses[in.GitRevision]; resp != nil {
		return resp, nil
	}
	return nil, status.Error(codes.Unknown, "no")
}

// StreamOverview implements api.OverviewServiceClient
func (*mockOverviewClient) StreamOverview(ctx context.Context, in *api.GetOverviewRequest, opts ...grpc.CallOption) (api.OverviewService_StreamOverviewClient, error) {
	panic("unimplemented")
}

var _ api.OverviewServiceClient = (*mockOverviewClient)(nil)

func TestVersionClient(t *testing.T) {
	t.Parallel()
	defaultResponses := map[string]*api.GetOverviewResponse{
		"1234": {
			EnvironmentGroups: []*api.EnvironmentGroup{
				{
					Environments: []*api.Environment{
						{
							Name: "staging",
							Applications: map[string]*api.Environment_Application{
								"foo": {
									Version: 1,
								},
							},
						},
					},
				},
			},
			GitRevision: "1234",
		},
		"5678": {
			EnvironmentGroups: []*api.EnvironmentGroup{
				{},
				{},
				{
					Environments: []*api.Environment{
						{
							Name: "staging",
							Applications: map[string]*api.Environment_Application{
								"bar": {
									Version: 2,
								},
							},
						},
					},
				},
			},
			GitRevision: "5678",
		},
	}
	defaultMetadata := metadata.MD{
		// Base64 of "kuberpult-rollout-service@local"
		"author-email": {"a3ViZXJwdWx0LXJvbGxvdXQtc2VydmljZUBsb2NhbA=="},
		// Base64 of "kuberpult-rollout-service"
		"author-name": {"a3ViZXJwdWx0LXJvbGxvdXQtc2VydmljZQ=="},
	}

	tcs := []struct {
		Name             string
		Responses        map[string]*api.GetOverviewResponse
		ExpectedVersions []expectedVersion
	}{
		{
			Name:      "Returns the deployed version",
			Responses: defaultResponses,
			ExpectedVersions: []expectedVersion{
				{
					Revision:        "1234",
					Environment:     "staging",
					Application:     "foo",
					DeployedVersion: 1,
					Metadata:        defaultMetadata,
				},
				{
					Revision:        "5678",
					Environment:     "staging",
					Application:     "bar",
					DeployedVersion: 2,
					Metadata:        defaultMetadata,
				},
			},
		},
		{
			Name:      "Returns an empty reply when a service is not deployed",
			Responses: defaultResponses,
			ExpectedVersions: []expectedVersion{
				{
					Revision:        "1234",
					Environment:     "staging",
					Application:     "bar",
					DeployedVersion: 0, // bar is deployed in rev 5678 but not in 1234
					Metadata:        defaultMetadata,
				},
				{
					Revision:        "1234",
					Environment:     "production",
					Application:     "foo",
					DeployedVersion: 0, // foo is deployed in rev 1234 but not in env production
					Metadata:        defaultMetadata,
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			mc := &mockOverviewClient{Responses: tc.Responses}
			vc := New(mc)
			for _, ev := range tc.ExpectedVersions {
				version, err := vc.GetVersion(context.Background(), ev.Revision, ev.Environment, ev.Application)
				if err != nil {
					t.Errorf("expected no error for %s/%s@%s, but got %q", ev.Environment, ev.Application, ev.Revision, err)
					continue
				}
				if version != ev.DeployedVersion {
					t.Errorf("expected version %d to be deployed for %s/%s@%s but got %d", ev.DeployedVersion, ev.Environment, ev.Application, ev.Revision, version)
				}
				if !cmp.Equal(mc.LastMetadata, ev.Metadata) {
					t.Errorf("mismachted metadata %s", cmp.Diff(mc.LastMetadata, ev.Metadata))
				}
			}
		})
	}

}
