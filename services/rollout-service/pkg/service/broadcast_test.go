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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/freiheit-com/kuberpult/pkg/api"
	"google.golang.org/grpc"
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
		RolloutStatusSuccesful   = api.RolloutStatus_RolloutStatusSuccesful
		RolloutStatusProgressing = api.RolloutStatus_RolloutStatusProgressing
		RolloutStatusError       = api.RolloutStatus_RolloutStatusError
	)
	type step struct {
		Event Event

		ExpectStatus *api.RolloutStatus
	}
	tcs := []struct {
		Name  string
		Steps []step
	}{
		{
			Name: "simple case",
			Steps: []step{
				{
					Event: Event{
						Application:      "foo",
						Environment:      "bar",
						Version:          1,
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusHealthy,
					},

					ExpectStatus: &RolloutStatusSuccesful,
				},
			},
		},

		{
			Name: "app syncing and becomming healthy",
			Steps: []step{
				{
					Event: Event{
						Application:      "foo",
						Environment:      "bar",
						Version:          1,
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusHealthy,
					},

					ExpectStatus: &RolloutStatusSuccesful,
				},
				{
					Event: Event{
						Application:      "foo",
						Environment:      "bar",
						Version:          1,
						SyncStatusCode:   v1alpha1.SyncStatusCodeOutOfSync,
						HealthStatusCode: health.HealthStatusHealthy,
					},

					ExpectStatus: &RolloutStatusProgressing,
				},
				{
					Event: Event{
						Application:      "foo",
						Environment:      "bar",
						Version:          1,
						SyncStatusCode:   v1alpha1.SyncStatusCodeOutOfSync,
						HealthStatusCode: health.HealthStatusProgressing,
					},

					ExpectStatus: &RolloutStatusProgressing,
				},
				{
					Event: Event{
						Application:      "foo",
						Environment:      "bar",
						Version:          2,
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
					Event: Event{
						Application:      "foo",
						Environment:      "bar",
						Version:          1,
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusHealthy,
					},

					ExpectStatus: &RolloutStatusSuccesful,
				},
				{
					Event: Event{
						Application:      "foo",
						Environment:      "bar",
						Version:          1,
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusDegraded,
					},

					ExpectStatus: &RolloutStatusError,
				},
				{
					Event: Event{
						Application:      "foo",
						Environment:      "bar",
						Version:          1,
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
				bc.Process(context.Background(), s.Event)
				if s.ExpectStatus != nil {
					resp := <-ch
					if resp.Application != s.Event.Application {
						t.Errorf("wrong application received in step %d: expected %q, got %q", i, s.Event.Application, resp.Application)
					}
					if resp.Environment != s.Event.Environment {
						t.Errorf("wrong environment received in step %d: expected %q, got %q", i, s.Event.Environment, resp.Environment)
					}
					if resp.RolloutStatus != *s.ExpectStatus {
						t.Errorf("wrong status received in step %d: expected %q, got %q", i, s.ExpectStatus, resp.RolloutStatus)
					}
				}
			}
			cancel()
		})
		t.Run(tc.Name+" (once)", func(t *testing.T) {
			bc := New()
			for i, s := range tc.Steps {
				bc.Process(context.Background(), s.Event)
				if s.ExpectStatus != nil {
					ctx, cancel := context.WithCancel(context.Background())
					ch := make(chan *api.StreamStatusResponse, 1)
					srv := testSrv{ctx: ctx, ch: ch}
					go bc.StreamStatus(&api.StreamStatusRequest{}, &srv)
					resp := <-ch
					cancel()
					if resp.Application != s.Event.Application {
						t.Errorf("wrong application received in step %d: expected %q, got %q", i, s.Event.Application, resp.Application)
					}
					if resp.Environment != s.Event.Environment {
						t.Errorf("wrong environment received in step %d: expected %q, got %q", i, s.Event.Environment, resp.Environment)
					}
					if resp.RolloutStatus != *s.ExpectStatus {
						t.Errorf("wrong status received in step %d: expected %q, got %q", i, s.ExpectStatus, resp.RolloutStatus)
					}
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
			ch1 := make(chan *api.StreamStatusResponse)
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
				bc.Process(context.Background(), Event{
					Application:      app,
					Environment:      "doesntmatter",
					HealthStatusCode: health.HealthStatusHealthy,
					SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
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
