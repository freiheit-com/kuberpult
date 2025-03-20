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
	"fmt"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/versions"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

type testSrv struct {
	ch  chan *api.StreamStatusResponse
	ctx context.Context
	grpc.ServerStream
}

func (t *testSrv) Send(resp *api.StreamStatusResponse) error {
	t.ch <- resp
	return nil
}

func (t *testSrv) Context() context.Context {
	return t.ctx
}

type errSrv struct {
	err error
	ctx context.Context
	grpc.ServerStream
}

func (t *errSrv) Send(_ *api.StreamStatusResponse) error {
	return t.err
}

func (t *errSrv) Context() context.Context {
	return t.ctx
}

func TestBroadcast(t *testing.T) {
	t.Parallel()
	var (
		RolloutStatusSuccesful   = api.RolloutStatus_ROLLOUT_STATUS_SUCCESFUL
		RolloutStatusProgressing = api.RolloutStatus_ROLLOUT_STATUS_PROGRESSING
		RolloutStatusError       = api.RolloutStatus_ROLLOUT_STATUS_ERROR
		RolloutStatusUnknown     = api.RolloutStatus_ROLLOUT_STATUS_UNKNOWN
		RolloutStatusUnhealthy   = api.RolloutStatus_ROLLOUT_STATUS_UNHEALTHY
		RolloutStatusPending     = api.RolloutStatus_ROLLOUT_STATUS_PENDING
	)
	type step struct {
		ArgoEvent    *ArgoEvent
		VersionEvent *versions.KuberpultEvent

		ExpectStatus *api.RolloutStatus
	}

	application := func(s step) string {
		if s.ArgoEvent != nil {
			return s.ArgoEvent.Application
		}
		return s.VersionEvent.Application
	}
	environment := func(s step) string {
		if s.ArgoEvent != nil {
			return s.ArgoEvent.Environment
		}
		return s.VersionEvent.Environment
	}

	tcs := []struct {
		Name  string
		Steps []step
	}{
		{
			Name: "simple case",
			Steps: []step{
				{
					ArgoEvent: &ArgoEvent{
						Application:      "foo",
						Environment:      "bar",
						Version:          &versions.VersionInfo{Version: 1},
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusHealthy,
					},
				},
				{
					VersionEvent: &versions.KuberpultEvent{
						Application: "foo",
						Environment: "bar",
						Version:     &versions.VersionInfo{Version: 1},
					},

					ExpectStatus: &RolloutStatusSuccesful,
				},
			},
		},
		{
			Name: "missing argo app",
			Steps: []step{
				{
					VersionEvent: &versions.KuberpultEvent{
						Application: "foo",
						Environment: "bar",
						Version:     &versions.VersionInfo{Version: 2},
					},

					ExpectStatus: &RolloutStatusUnknown,
				},
			},
		},
		{
			Name: "missing version in argo event",
			Steps: []step{
				{
					ArgoEvent: &ArgoEvent{
						Application:      "foo",
						Environment:      "bar",
						Version:          &versions.VersionInfo{Version: 1},
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusHealthy,
					},

					ExpectStatus: &RolloutStatusUnknown,
				},
				{
					ArgoEvent: &ArgoEvent{
						Application:      "foo",
						Environment:      "bar",
						Version:          nil,
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusHealthy,
					},

					ExpectStatus: &RolloutStatusUnknown,
				},
			},
		},
		{
			Name: "app syncing and becomming healthy",
			Steps: []step{
				{
					VersionEvent: &versions.KuberpultEvent{
						Application: "foo",
						Environment: "bar",
						Version:     &versions.VersionInfo{Version: 1},
					},

					ExpectStatus: &RolloutStatusUnknown,
				},
				{
					ArgoEvent: &ArgoEvent{
						Application:      "foo",
						Environment:      "bar",
						Version:          &versions.VersionInfo{Version: 1},
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusHealthy,
					},

					ExpectStatus: &RolloutStatusSuccesful,
				},
				{
					VersionEvent: &versions.KuberpultEvent{
						Application: "foo",
						Environment: "bar",
						Version:     &versions.VersionInfo{Version: 2},
					},

					ExpectStatus: &RolloutStatusPending,
				},
				{
					ArgoEvent: &ArgoEvent{
						Application:      "foo",
						Environment:      "bar",
						Version:          &versions.VersionInfo{Version: 2},
						SyncStatusCode:   v1alpha1.SyncStatusCodeOutOfSync,
						HealthStatusCode: health.HealthStatusHealthy,
					},

					ExpectStatus: &RolloutStatusProgressing,
				},
				{
					ArgoEvent: &ArgoEvent{
						Application:      "foo",
						Environment:      "bar",
						Version:          &versions.VersionInfo{Version: 2},
						SyncStatusCode:   v1alpha1.SyncStatusCodeOutOfSync,
						HealthStatusCode: health.HealthStatusProgressing,
					},

					ExpectStatus: nil,
				},
				{
					ArgoEvent: &ArgoEvent{
						Application:      "foo",
						Environment:      "bar",
						Version:          &versions.VersionInfo{Version: 2},
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusHealthy,
					},

					ExpectStatus: &RolloutStatusSuccesful,
				},
			},
		},
		{
			Name: "app becomming unhealthy and recovers",
			Steps: []step{
				{
					VersionEvent: &versions.KuberpultEvent{
						Application: "foo",
						Environment: "bar",
						Version:     &versions.VersionInfo{Version: 1},
					},

					ExpectStatus: &RolloutStatusUnknown,
				},
				{
					ArgoEvent: &ArgoEvent{
						Application:      "foo",
						Environment:      "bar",
						Version:          &versions.VersionInfo{Version: 1},
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusHealthy,
					},

					ExpectStatus: &RolloutStatusSuccesful,
				},
				{
					ArgoEvent: &ArgoEvent{
						Application:      "foo",
						Environment:      "bar",
						Version:          &versions.VersionInfo{Version: 1},
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusDegraded,
					},

					ExpectStatus: &RolloutStatusUnhealthy,
				},
				{
					ArgoEvent: &ArgoEvent{
						Application:      "foo",
						Environment:      "bar",
						Version:          &versions.VersionInfo{Version: 1},
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusHealthy,
					},

					ExpectStatus: &RolloutStatusSuccesful,
				},
			},
		},
		{
			Name: "rollout fails",
			Steps: []step{
				{
					ArgoEvent: &ArgoEvent{
						Application:      "foo",
						Environment:      "bar",
						Version:          &versions.VersionInfo{Version: 1},
						SyncStatusCode:   v1alpha1.SyncStatusCodeOutOfSync,
						HealthStatusCode: health.HealthStatusHealthy,
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationFailed,
						},
					},

					ExpectStatus: &RolloutStatusError,
				},
			},
		},
		{
			Name: "rollout errors",
			Steps: []step{
				{
					ArgoEvent: &ArgoEvent{
						Application:      "foo",
						Environment:      "bar",
						Version:          &versions.VersionInfo{Version: 1},
						SyncStatusCode:   v1alpha1.SyncStatusCodeOutOfSync,
						HealthStatusCode: health.HealthStatusHealthy,
						OperationState: &v1alpha1.OperationState{
							Phase: common.OperationError,
						},
					},

					ExpectStatus: &RolloutStatusError,
				},
			},
		},
		{
			Name: "healthy app switches to pending when a new version in kuberpult is deployed",
			Steps: []step{
				{
					VersionEvent: &versions.KuberpultEvent{
						Application: "foo",
						Environment: "bar",
						Version:     &versions.VersionInfo{Version: 1},
					},

					ExpectStatus: &RolloutStatusUnknown,
				},
				{
					ArgoEvent: &ArgoEvent{
						Application:      "foo",
						Environment:      "bar",
						Version:          &versions.VersionInfo{Version: 1},
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusHealthy,
					},

					ExpectStatus: &RolloutStatusSuccesful,
				},
				{
					VersionEvent: &versions.KuberpultEvent{
						Application: "foo",
						Environment: "bar",
						Version:     &versions.VersionInfo{Version: 2},
					},

					ExpectStatus: &RolloutStatusPending,
				},
				{
					ArgoEvent: &ArgoEvent{
						Application:      "foo",
						Environment:      "bar",
						Version:          &versions.VersionInfo{Version: 2},
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusHealthy,
					},

					ExpectStatus: &RolloutStatusSuccesful,
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name+" (streaming)", func(t *testing.T) {
			bc := New()
			ctx, cancel := context.WithCancel(context.Background())
			ch := make(chan *api.StreamStatusResponse)
			srv := testSrv{ctx: ctx, ch: ch}
			go bc.StreamStatus(&api.StreamStatusRequest{}, &srv)
			for i, s := range tc.Steps {
				if s.ArgoEvent != nil {
					bc.ProcessArgoEvent(context.Background(), *s.ArgoEvent)
				} else if s.VersionEvent != nil {
					bc.ProcessKuberpultEvent(context.Background(), *s.VersionEvent)
				}
				if s.ExpectStatus != nil {
					resp := <-ch
					if resp.Application != application(s) {
						t.Errorf("wrong application received in step %d: expected %q, got %q", i, application(s), resp.Application)
					}
					if resp.Environment != environment(s) {
						t.Errorf("wrong environment received in step %d: expected %q, got %q", i, environment(s), resp.Environment)
					}
					if resp.RolloutStatus != *s.ExpectStatus {
						t.Errorf("wrong status received in step %d: expected %q, got %q", i, s.ExpectStatus, resp.RolloutStatus)
					}
				} else {
					select {
					case resp := <-ch:
						t.Errorf("didn't expect status update but got %#v", resp)
					default:
					}
				}
			}
			cancel()
		})
		t.Run(tc.Name+" (once)", func(t *testing.T) {
			bc := New()
			for i, s := range tc.Steps {
				if s.ArgoEvent != nil {
					bc.ProcessArgoEvent(context.Background(), *s.ArgoEvent)
				} else if s.VersionEvent != nil {
					bc.ProcessKuberpultEvent(context.Background(), *s.VersionEvent)
				}
				if s.ExpectStatus != nil {
					ctx, cancel := context.WithCancel(context.Background())
					ch := make(chan *api.StreamStatusResponse, 1)
					srv := testSrv{ctx: ctx, ch: ch}
					go bc.StreamStatus(&api.StreamStatusRequest{}, &srv)
					resp := <-ch
					cancel()
					if resp.Application != application(s) {
						t.Errorf("wrong application received in step %d: expected %q, got %q", i, application(s), resp.Application)
					}
					if resp.Environment != environment(s) {
						t.Errorf("wrong environment received in step %d: expected %q, got %q", i, environment(s), resp.Environment)
					}
					if resp.RolloutStatus != *s.ExpectStatus {
						t.Errorf("wrong status received in step %d: expected %q, got %q", i, s.ExpectStatus, resp.RolloutStatus)
					}
				}
			}
		})
		t.Run(tc.Name+" (get)", func(t *testing.T) {
			bc := New()
			lastStatus := RolloutStatusUnknown
			for i, s := range tc.Steps {
				if s.ArgoEvent != nil {
					bc.ProcessArgoEvent(context.Background(), *s.ArgoEvent)
				} else if s.VersionEvent != nil {
					bc.ProcessKuberpultEvent(context.Background(), *s.VersionEvent)
				}

				ctx, cancel := context.WithCancel(context.Background())
				resp, err := bc.GetStatus(ctx, &api.GetStatusRequest{})
				cancel()
				if err != nil {
					t.Errorf("didn't expect an error but got %q", err)
				}

				if s.ExpectStatus != nil {
					lastStatus = *s.ExpectStatus
				}
				if resp.Status != lastStatus {
					t.Errorf("wrong status received in step %d: expected %q, got %q", i, lastStatus, resp.Status)
				}

				if lastStatus == RolloutStatusSuccesful {
					// Apps with successful state are excluded
					if len(resp.Applications) != 0 {
						t.Errorf("expected no applications but got %d", len(resp.Applications))
					}
					continue
				}
				app := resp.Applications[0]
				if app.Application != application(s) {
					t.Errorf("wrong application received in step %d: expected %q, got %q", i, application(s), app.Application)
				}
				if app.Environment != environment(s) {
					t.Errorf("wrong environment received in step %d: expected %q, got %q", i, environment(s), app.Environment)
				}
				if app.RolloutStatus != lastStatus {
					t.Errorf("wrong status received in step %d: expected %q, got %q", i, lastStatus, app.RolloutStatus)
				}
			}
		})

	}
}

func TestBroadcastDoesntGetStuck(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		Name   string
		Events uint
	}{
		{
			Name:   "200 events",
			Events: 200,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			bc := New()
			// srv1 will just be blocked
			ctx1, cancel1 := context.WithCancel(context.Background())
			ch1 := make(chan *api.StreamStatusResponse, 200)
			ech1 := make(chan error, 1)
			srv1 := testSrv{ctx: ctx1, ch: ch1}
			go func() {
				ech1 <- bc.StreamStatus(&api.StreamStatusRequest{}, &srv1)
			}()
			defer cancel1()
			// srv2 will actually get consumed
			ctx2, cancel2 := context.WithCancel(context.Background())
			ch2 := make(chan *api.StreamStatusResponse)
			ech2 := make(chan error, 1)
			srv2 := testSrv{ctx: ctx2, ch: ch2}
			go func() {
				ech2 <- bc.StreamStatus(&api.StreamStatusRequest{}, &srv2)
			}()
			defer cancel2()
			// srv3 will just return an error
			ctx3, cancel3 := context.WithCancel(context.Background())
			ech3 := make(chan error, 1)
			testErr := fmt.Errorf("some error")
			srv3 := errSrv{ctx: ctx3, err: testErr}
			go func() {
				ech3 <- bc.StreamStatus(&api.StreamStatusRequest{}, &srv3)
			}()
			defer cancel3()

			for i := uint(0); i < tc.Events; i += 1 {
				app := fmt.Sprintf("app-%d", i)
				bc.ProcessArgoEvent(context.Background(), ArgoEvent{
					Application:      app,
					Environment:      "doesntmatter",
					HealthStatusCode: health.HealthStatusHealthy,
					SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
					Version:          &versions.VersionInfo{Version: 1},
				})
				select {
				case resp := <-ch2:
					if resp.Application != app {
						t.Errorf("didn't receive correct application in for event %d, expected %q, got %q", i, app, resp.Application)
					}
				case <-time.After(1 * time.Second):
					t.Fatalf("didn't receive event %d", i)
				}
			}
			// Shutdown all consumers
			cancel1()
			cancel2()
			cancel3()
			// Unblock ch1
			go func() {
				for range ch1 {
				}
			}()
			e1 := <-ech1
			if e1 != nil {
				t.Errorf("first subscription failed with unexpected error: %q", e1)
			}
			e2 := <-ech2
			if e2 != nil {
				t.Errorf("second subscription failed with unexpected error: %q", e2)
			}
			e3 := <-ech3
			if e3 != testErr {
				t.Errorf("third subscription failed with unexpected error: %q, exepcted: %q", e3, testErr)
			}
		})

	}
}

func TestGetStatus(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		Name              string
		ArgoEvents        []ArgoEvent
		KuberpultEvents   []versions.KuberpultEvent
		Request           *api.GetStatusRequest
		DelayedArgoEvents []ArgoEvent

		ExpectedResponse *api.GetStatusResponse
	}{
		{
			Name:    "simple case",
			Request: &api.GetStatusRequest{},
			ExpectedResponse: &api.GetStatusResponse{
				Status: api.RolloutStatus_ROLLOUT_STATUS_SUCCESFUL,
			},
		},
		{
			Name: "filters for environmentGroup",
			ArgoEvents: []ArgoEvent{
				{
					Application:      "foo",
					Environment:      "dev",
					Version:          &versions.VersionInfo{Version: 2},
					SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
					HealthStatusCode: health.HealthStatusHealthy,
				},
				{
					Application:      "foo",
					Environment:      "prd",
					Version:          &versions.VersionInfo{Version: 1},
					SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
					HealthStatusCode: health.HealthStatusHealthy,
				},
			},
			KuberpultEvents: []versions.KuberpultEvent{
				{
					Application:      "foo",
					Environment:      "dev",
					Version:          &versions.VersionInfo{Version: 3},
					EnvironmentGroup: "dev-group",
				},
				{
					Application:      "foo",
					Environment:      "prd",
					Version:          &versions.VersionInfo{Version: 3},
					EnvironmentGroup: "prd-group",
				},
			},
			Request: &api.GetStatusRequest{
				EnvironmentGroup: "dev-group",
			},
			ExpectedResponse: &api.GetStatusResponse{
				Status: api.RolloutStatus_ROLLOUT_STATUS_PENDING,
				Applications: []*api.GetStatusResponse_ApplicationStatus{
					{
						Environment:   "dev",
						Application:   "foo",
						RolloutStatus: api.RolloutStatus_ROLLOUT_STATUS_PENDING,
					},
				},
			},
		},
		{
			Name: "processes late health events",
			ArgoEvents: []ArgoEvent{
				{
					Application:      "foo",
					Environment:      "dev",
					Version:          &versions.VersionInfo{Version: 2},
					SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
					HealthStatusCode: health.HealthStatusHealthy,
				},
			},
			KuberpultEvents: []versions.KuberpultEvent{
				{
					Application:      "foo",
					Environment:      "dev",
					Version:          &versions.VersionInfo{Version: 3},
					EnvironmentGroup: "dev-group",
				},
			},
			// This signals that the application is now healthy
			DelayedArgoEvents: []ArgoEvent{
				{
					Application:      "foo",
					Environment:      "dev",
					Version:          &versions.VersionInfo{Version: 3},
					SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
					HealthStatusCode: health.HealthStatusHealthy,
				},
			},
			Request: &api.GetStatusRequest{
				EnvironmentGroup: "dev-group",
				WaitSeconds:      1,
			},
			ExpectedResponse: &api.GetStatusResponse{
				Status:       api.RolloutStatus_ROLLOUT_STATUS_SUCCESFUL,
				Applications: []*api.GetStatusResponse_ApplicationStatus{},
			},
		},
		{
			Name: "processes late error events",
			ArgoEvents: []ArgoEvent{
				{
					Application:      "foo",
					Environment:      "dev",
					Version:          &versions.VersionInfo{Version: 2},
					SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
					HealthStatusCode: health.HealthStatusHealthy,
				},
			},
			KuberpultEvents: []versions.KuberpultEvent{
				{
					Application:      "foo",
					Environment:      "dev",
					Version:          &versions.VersionInfo{Version: 3},
					EnvironmentGroup: "dev-group",
				},
			},
			// This signals that the application is now broken
			DelayedArgoEvents: []ArgoEvent{
				{
					Application:      "foo",
					Environment:      "dev",
					Version:          &versions.VersionInfo{Version: 3},
					SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
					HealthStatusCode: health.HealthStatusDegraded,
					OperationState: &v1alpha1.OperationState{
						Phase: common.OperationFailed,
					},
				},
			},
			Request: &api.GetStatusRequest{
				EnvironmentGroup: "dev-group",
				WaitSeconds:      1,
			},
			ExpectedResponse: &api.GetStatusResponse{
				Status: api.RolloutStatus_ROLLOUT_STATUS_ERROR,
				Applications: []*api.GetStatusResponse_ApplicationStatus{
					{
						Environment:   "dev",
						Application:   "foo",
						RolloutStatus: api.RolloutStatus_ROLLOUT_STATUS_ERROR,
					},
				},
			},
		},
		{
			Name: "excludes succesful applications",
			ArgoEvents: []ArgoEvent{
				{
					Application:      "foo",
					Environment:      "dev",
					Version:          &versions.VersionInfo{Version: 2},
					SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
					HealthStatusCode: health.HealthStatusHealthy,
				},
				{
					Application:      "bar",
					Environment:      "dev",
					Version:          &versions.VersionInfo{Version: 1},
					SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
					HealthStatusCode: health.HealthStatusHealthy,
				},
			},
			KuberpultEvents: []versions.KuberpultEvent{
				{
					Application:      "foo",
					Environment:      "dev",
					Version:          &versions.VersionInfo{Version: 3},
					EnvironmentGroup: "dev-group",
				},
				{
					Application:      "bar",
					Environment:      "dev",
					Version:          &versions.VersionInfo{Version: 1},
					EnvironmentGroup: "dev-group",
				},
			},
			Request: &api.GetStatusRequest{
				EnvironmentGroup: "dev-group",
			},
			ExpectedResponse: &api.GetStatusResponse{
				Status: api.RolloutStatus_ROLLOUT_STATUS_PENDING,
				Applications: []*api.GetStatusResponse_ApplicationStatus{
					{
						Environment:   "dev",
						Application:   "foo",
						RolloutStatus: api.RolloutStatus_ROLLOUT_STATUS_PENDING,
					},
				},
			},
		},
		{
			Name: "filters for environmentGroup and team",
			ArgoEvents: []ArgoEvent{
				{
					Application:      "foo",
					Environment:      "dev",
					Version:          &versions.VersionInfo{Version: 2},
					SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
					HealthStatusCode: health.HealthStatusHealthy,
				},
				{
					Application:      "foo",
					Environment:      "prd",
					Version:          &versions.VersionInfo{Version: 1},
					SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
					HealthStatusCode: health.HealthStatusHealthy,
				},
			},
			KuberpultEvents: []versions.KuberpultEvent{
				{
					Application:      "foo",
					Environment:      "dev",
					Version:          &versions.VersionInfo{Version: 3},
					EnvironmentGroup: "dev-group",
					Team:             "ArgoAppProcessor",
				},
				{
					Application:      "foo",
					Environment:      "bar",
					Version:          &versions.VersionInfo{Version: 3},
					EnvironmentGroup: "dev-group",
					Team:             "b",
				},
				{
					Application:      "foo",
					Environment:      "prd",
					Version:          &versions.VersionInfo{Version: 3},
					EnvironmentGroup: "prd-group",
				},
			},
			Request: &api.GetStatusRequest{
				EnvironmentGroup: "dev-group",
				Team:             "b",
			},
			ExpectedResponse: &api.GetStatusResponse{
				Status: api.RolloutStatus_ROLLOUT_STATUS_UNKNOWN,
				Applications: []*api.GetStatusResponse_ApplicationStatus{
					{
						Environment:   "bar",
						Application:   "foo",
						RolloutStatus: api.RolloutStatus_ROLLOUT_STATUS_UNKNOWN,
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			bc := New()
			for _, s := range tc.ArgoEvents {
				bc.ProcessArgoEvent(context.Background(), s)
			}
			for _, s := range tc.KuberpultEvents {
				bc.ProcessKuberpultEvent(context.Background(), s)
			}

			bc.waiting = func() {
				for _, s := range tc.DelayedArgoEvents {
					bc.ProcessArgoEvent(context.Background(), s)
				}
			}

			resp, err := bc.GetStatus(context.Background(), tc.Request)
			if err != nil {
				t.Errorf("didn't expect an error but got %q", err)
			}
			if d := cmp.Diff(tc.ExpectedResponse, resp, protocmp.Transform()); d != "" {
				t.Errorf("response mismatch:\ndiff:%s", d)
			}
		})

		// This runs all test-cases again but delays all argoevents.
		// The effect is that all apps will start as "unknown" and then will eventually converge.
		t.Run(tc.Name+" (delay all argo events)", func(t *testing.T) {
			bc := New()
			for _, s := range tc.KuberpultEvents {
				bc.ProcessKuberpultEvent(context.Background(), s)
			}
			bc.waiting = func() {
				for _, s := range tc.ArgoEvents {
					bc.ProcessArgoEvent(context.Background(), s)
				}
				for _, s := range tc.DelayedArgoEvents {
					bc.ProcessArgoEvent(context.Background(), s)
				}
				bc.DisconnectAll()
			}
			var req api.GetStatusRequest
			proto.Merge(&req, tc.Request)
			req.WaitSeconds = 1

			resp, err := bc.GetStatus(context.Background(), &req)
			if err != nil {
				t.Errorf("didn't expect an error but got %q", err)
			}
			if d := cmp.Diff(tc.ExpectedResponse, resp, protocmp.Transform()); d != "" {
				t.Errorf("response mismatch:\ndiff:%s", d)
			}
		})
	}
}
