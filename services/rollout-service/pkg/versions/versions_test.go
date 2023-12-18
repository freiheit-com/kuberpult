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

	"github.com/cenkalti/backoff/v4"
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/google/go-cmp/cmp"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type step struct {
	Overview      *api.GetOverviewResponse
	ConnectErr    error
	RecvErr       error
	CancelContext bool

	ExpectReady    bool
	ExpectedEvents []KuberpultEvent
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
	StartStep    chan struct{}
	Steps        chan step
	savedStep    *step
	current      int
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

func (m *mockOverviewClient) Recv() (*api.GetOverviewResponse, error) {
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
	return reply.Overview, reply.RecvErr
}

var _ api.OverviewServiceClient = (*mockOverviewClient)(nil)

type mockVersionResponse struct {
	response *api.GetVersionResponse
	err      error
}
type mockVersionClient struct {
	responses map[string]mockVersionResponse
}

func (m *mockVersionClient) GetVersion(ctx context.Context, in *api.GetVersionRequest, opts ...grpc.CallOption) (*api.GetVersionResponse, error) {
	key := fmt.Sprintf("%s/%s@%s", in.Environment, in.Application, in.GitRevision)
	res, ok := m.responses[key]
	if !ok {
		return nil, status.Error(codes.Unknown, "no")
	}
	return res.response, res.err
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
		VersionResponses map[string]mockVersionResponse
		ExpectedVersions []expectedVersion
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
					Overview: testOverview,

					ExpectReady: true,
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
						Version:        1,
						SourceCommitId: "00001",
						DeployedAt:     timestamppb.New(time.Unix(123456789, 0).UTC()),
					},
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
		},
		{
			Name: "Don't notify twice for the same version",
			Steps: []step{
				{
					Overview: testOverview,

					ExpectReady: true,
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
					Overview: testOverview,

					ExpectReady: true,
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
					Overview: testOverview,

					ExpectReady: true,
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
					Overview: emptyTestOverview,

					ExpectReady: true,
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
					Overview: testOverview,

					ExpectReady: true,
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
					RecvErr: fmt.Errorf("no"),

					ExpectReady: false,
				},
				{
					Overview: emptyTestOverview,

					ExpectReady: true,
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
			Name: "Updates environment groups",
			Steps: []step{
				{
					Overview: testOverview,

					ExpectReady: true,
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
					Overview: testOverviewWithDifferentEnvgroup,

					ExpectReady: true,
					ExpectedEvents: []KuberpultEvent{
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
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
		},
		{
			Name: "Reports production environments",
			Steps: []step{
				{
					Overview: testOverviewWithProdEnvs,

					ExpectReady: true,
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
			vc := New(moc, mvc)
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
			assertExpectedVersions(t, tc.ExpectedVersions, vc, moc)

		})
	}
}

func assertStep(t *testing.T, i int, s step, vp *mockVersionEventProcessor, hs *setup.HealthServer) {
	if hs.IsReady("versions") != s.ExpectReady {
		t.Errorf("wrong readyness in step %d, expected %t but got %t", i, s.ExpectReady, hs.IsReady("versions"))
	}
	if !cmp.Equal(s.ExpectedEvents, vp.events) {
		t.Errorf("version events differ: %s", cmp.Diff(s.ExpectedEvents, vp.events))
	}
	vp.events = nil
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
