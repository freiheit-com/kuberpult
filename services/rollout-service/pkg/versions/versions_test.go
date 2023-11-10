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
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/google/go-cmp/cmp"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type step struct {
	Overview      *api.GetOverviewResponse
	ConnectErr    error
	RecvErr       error
	CancelContext bool
}

type expectedVersion struct {
	Revision        string
	Environment     string
	Application     string
	DeployedVersion uint64
	DeployTime      time.Time
	SourceCommitId  string
	Metadata        metadata.MD
	IsProduction    bool
}

type mockOverviewStreamMessage struct {
	Overview     *api.GetOverviewResponse
	Error        error
	ConnectError error
}

type mockOverviewClient struct {
	grpc.ClientStream
	Responses    map[string]*api.GetOverviewResponse
	LastMetadata metadata.MD
	Steps        chan step
	savedStep    *step
	current      int
	cancel       context.CancelFunc
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
func (m *mockOverviewClient) StreamOverview(ctx context.Context, in *api.GetOverviewRequest, opts ...grpc.CallOption) (api.OverviewService_StreamOverviewClient, error) {
	reply, ok := <-m.Steps
	if !ok {
		return nil, fmt.Errorf("exhausted: %w", io.EOF)
	}
	if reply.ConnectErr != nil {
		if reply.CancelContext {
			m.cancel()
		}
		return nil, reply.ConnectErr
	}
	m.savedStep = &reply
	return m, nil
}

func (m *mockOverviewClient) Recv() (*api.GetOverviewResponse, error) {
	var reply step
	var ok bool
	if m.savedStep != nil {
		reply = *m.savedStep
		m.savedStep = nil
		ok = true
	} else {
		reply, ok = <-m.Steps
	}
	if !ok {
		return nil, fmt.Errorf("exhausted: %w", io.EOF)
	}
	if reply.CancelContext {
		m.cancel()
	}
	return reply.Overview, reply.RecvErr
}

var _ api.OverviewServiceClient = (*mockOverviewClient)(nil)

type mockVersionEventProcessor struct {
	events []KuberpultEvent
}

func (m *mockVersionEventProcessor) ProcessKuberpultEvent(ctx context.Context, ev KuberpultEvent) {
	m.events = append(m.events, ev)
}

func TestVersionClient(t *testing.T) {
	t.Parallel()
	defaultResponses := map[string]*api.GetOverviewResponse{
		"1234": {
			Applications: map[string]*api.Application{
				"foo": {
					Releases: []*api.Release{
						{
							Version:        1,
							SourceCommitId: "00001",
						},
					},
				},
			},
			EnvironmentGroups: []*api.EnvironmentGroup{
				{
					Environments: []*api.Environment{
						{
							Name: "staging",
							Applications: map[string]*api.Environment_Application{
								"foo": {
									Name:    "foo",
									Version: 1,
								},
							},
							Priority: api.Priority_UPSTREAM,
						},
					},
				},
			},
			GitRevision: "1234",
		},
		"5678": {
			Applications: map[string]*api.Application{
				"bar": {
					Releases: []*api.Release{
						{
							Version:        2,
							SourceCommitId: "00002",
						},
					},
				},
			},
			EnvironmentGroups: []*api.EnvironmentGroup{
				{},
				{},
				{
					Environments: []*api.Environment{
						{
							Name: "staging",
							Applications: map[string]*api.Environment_Application{
								"bar": {
									Name:    "bar",
									Version: 2,
									DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
										DeployTime: "123456789",
									},
								},
							},
							Priority: api.Priority_UPSTREAM,
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
					SourceCommitId:  "00001",
					Metadata:        defaultMetadata,
				},
				{
					Revision:        "5678",
					Environment:     "staging",
					Application:     "bar",
					DeployedVersion: 2,
					DeployTime:      time.Unix(123456789, 0).UTC(),
					SourceCommitId:  "00002",
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
			assertExpectedVersions(t, tc.ExpectedVersions, vc, mc)
		})
	}

}

func TestVersionClientStream(t *testing.T) {
	t.Parallel()
	testOverview := &api.GetOverviewResponse{
		Applications: map[string]*api.Application{
			"foo": {
				Releases: []*api.Release{
					{
						Version:        1,
						SourceCommitId: "00001",
					},
				},
				Team: "footeam",
			},
		},
		EnvironmentGroups: []*api.EnvironmentGroup{
			{

				EnvironmentGroupName: "staging-group",
				Environments: []*api.Environment{
					{
						Name: "staging",
						Applications: map[string]*api.Environment_Application{
							"foo": {
								Name:    "foo",
								Version: 1,
								DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
									DeployTime: "123456789",
								},
							},
						},
						Priority: api.Priority_UPSTREAM,
					},
				},
			},
		},
		GitRevision: "1234",
	}
	testOverviewWithDifferentEnvgroup := &api.GetOverviewResponse{
		Applications: map[string]*api.Application{
			"foo": {
				Releases: []*api.Release{
					{
						Version:        2,
						SourceCommitId: "00002",
					},
				},
			},
		},
		EnvironmentGroups: []*api.EnvironmentGroup{
			{

				EnvironmentGroupName: "not-staging-group",
				Environments: []*api.Environment{
					{
						Name: "staging",
						Applications: map[string]*api.Environment_Application{
							"foo": {
								Name:    "foo",
								Version: 2,
								DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
									DeployTime: "123456789",
								},
							},
						},
						Priority: api.Priority_UPSTREAM,
					},
				},
			},
		},
		GitRevision: "1234",
	}
	testOverviewWithProdEnvs := &api.GetOverviewResponse{
		Applications: map[string]*api.Application{
			"foo": {
				Releases: []*api.Release{
					{
						Version:        2,
						SourceCommitId: "00002",
					},
				},
			},
		},
		EnvironmentGroups: []*api.EnvironmentGroup{
			{

				EnvironmentGroupName: "production",
				Environments: []*api.Environment{
					{
						Name: "production",
						Applications: map[string]*api.Environment_Application{
							"foo": {
								Name:    "foo",
								Version: 2,
								DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
									DeployTime: "123456789",
								},
							},
						},
						Priority: api.Priority_PROD,
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
		Name             string
		Steps            []step
		ExpectedVersions []expectedVersion
		ExpectedEvents   []KuberpultEvent
	}{
		{
			Name: "Retries connections and finishes",
			Steps: []step{
				{
					ConnectErr: fmt.Errorf("no"),
				},
				{
					RecvErr: fmt.Errorf("no"),
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
					Overview: testOverview,
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
					DeployedVersion: 1,
					SourceCommitId:  "00001",
					DeployTime:      time.Unix(123456789, 0).UTC(),
				},
			},
			ExpectedEvents: []KuberpultEvent{
				{
					Environment:      "staging",
					Application:      "foo",
					EnvironmentGroup: "staging-group",
					Team:             "footeam",
					Version: &VersionInfo{
						Version:        1,
						SourceCommitId: "00001",
						DeployedAt:     time.Unix(123456789, 0).UTC(),
					},
				},
			},
		},
		{
			Name: "Don't notify twice for the same version",
			Steps: []step{
				{
					Overview: testOverview,
				},
				{
					Overview: testOverview,
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			ExpectedEvents: []KuberpultEvent{
				{
					Environment:      "staging",
					Application:      "foo",
					EnvironmentGroup: "staging-group",
					Team:             "footeam",
					Version: &VersionInfo{
						Version:        1,
						SourceCommitId: "00001",
						DeployedAt:     time.Unix(123456789, 0).UTC(),
					},
				},
			},
		},
		{
			Name: "Notify for apps that are deleted",
			Steps: []step{
				{
					Overview: testOverview,
				},
				{
					Overview: emptyTestOverview,
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			ExpectedEvents: []KuberpultEvent{
				{
					Environment:      "staging",
					Application:      "foo",
					EnvironmentGroup: "staging-group",
					Team:             "footeam",
					Version: &VersionInfo{
						Version:        1,
						SourceCommitId: "00001",
						DeployedAt:     time.Unix(123456789, 0).UTC(),
					},
				},
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
			Name: "Notify for apps that are deleted across reconnects",
			Steps: []step{
				{
					Overview: testOverview,
				},
				{
					RecvErr: fmt.Errorf("no"),
				},
				{
					Overview: emptyTestOverview,
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			ExpectedEvents: []KuberpultEvent{
				{
					Environment:      "staging",
					Application:      "foo",
					EnvironmentGroup: "staging-group",
					Team:             "footeam",
					Version: &VersionInfo{
						Version:        1,
						SourceCommitId: "00001",
						DeployedAt:     time.Unix(123456789, 0).UTC(),
					},
				},
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
			Name: "Updates environment groups",
			Steps: []step{
				{
					Overview: testOverview,
				},
				{
					Overview: testOverviewWithDifferentEnvgroup,
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			ExpectedEvents: []KuberpultEvent{
				{
					Environment:      "staging",
					Application:      "foo",
					EnvironmentGroup: "staging-group",
					Team:             "footeam",
					Version: &VersionInfo{
						Version:        1,
						SourceCommitId: "00001",
						DeployedAt:     time.Unix(123456789, 0).UTC(),
					},
				},
				{
					Environment:      "staging",
					Application:      "foo",
					EnvironmentGroup: "not-staging-group",
					Team:             "",
					Version: &VersionInfo{
						Version:        2,
						SourceCommitId: "00002",
						DeployedAt:     time.Unix(123456789, 0).UTC(),
					},
				},
			},
		},
		{
			Name: "Reports production environments",
			Steps: []step{
				{
					Overview: testOverviewWithProdEnvs,
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			ExpectedEvents: []KuberpultEvent{
				{
					Environment:      "production",
					Application:      "foo",
					EnvironmentGroup: "production",
					IsProduction:     true,
					Version: &VersionInfo{
						Version:        2,
						SourceCommitId: "00002",
						DeployedAt:     time.Unix(123456789, 0).UTC(),
					},
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			vp := &mockVersionEventProcessor{}
			steps := make(chan step)
			mc := &mockOverviewClient{Steps: steps, cancel: cancel}
			vc := New(mc)
			hs := &setup.HealthServer{}
			errCh := make(chan error)
			go func() {
				errCh <- vc.ConsumeEvents(ctx, vp, hs.Reporter("versions"))
			}()
			for _, s := range tc.Steps {
				select {
				case steps <- s:
				case err := <-errCh:
					t.Fatalf("expected no error but received %q", err)
				case <-time.After(10 * time.Second):
					t.Fatal("test got stuck after 10 seconds")
				}
			}
			err := <-errCh
			if err != nil {
				t.Errorf("expected no error, but received %q", err)
			}
			if len(steps) != 0 {
				t.Errorf("expected all events to be consumed, but got %d left", len(steps))
			}
			assertExpectedVersions(t, tc.ExpectedVersions, vc, mc)
			if !cmp.Equal(tc.ExpectedEvents, vp.events) {
				t.Errorf("version events differ: %s", cmp.Diff(tc.ExpectedEvents, vp.events))
			}
		})
	}
}

func assertExpectedVersions(t *testing.T, expectedVersions []expectedVersion, vc VersionClient, mc *mockOverviewClient) {
	for _, ev := range expectedVersions {
		version, err := vc.GetVersion(context.Background(), ev.Revision, ev.Environment, ev.Application)
		if err != nil {
			t.Errorf("expected no error for %s/%s@%s, but got %q", ev.Environment, ev.Application, ev.Revision, err)
			continue
		}
		if version.Version != ev.DeployedVersion {
			t.Errorf("expected version %d to be deployed for %s/%s@%s but got %d", ev.DeployedVersion, ev.Environment, ev.Application, ev.Revision, version.Version)
		}
		if version.DeployedAt != ev.DeployTime {
			t.Errorf("expected deploy time to be %q for %s/%s@%s but got %q", ev.DeployTime, ev.Environment, ev.Application, ev.Revision, version.DeployedAt)
		}
		if version.SourceCommitId != ev.SourceCommitId {
			t.Errorf("expected source commit id to be %q for %s/%s@%s but got %q", ev.SourceCommitId, ev.Environment, ev.Application, ev.Revision, version.SourceCommitId)
		}
		if !cmp.Equal(mc.LastMetadata, ev.Metadata) {
			t.Errorf("mismachted metadata %s", cmp.Diff(mc.LastMetadata, ev.Metadata))
		}
	}
}
