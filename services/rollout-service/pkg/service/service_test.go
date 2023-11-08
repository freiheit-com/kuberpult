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
	"io"
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/versions"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type step struct {
	Event    *v1alpha1.ApplicationWatchEvent
	WatchErr error
	RecvErr  error

	ExpectedEvent *ArgoEvent
}

func (m *mockApplicationServiceClient) Recv() (*v1alpha1.ApplicationWatchEvent, error) {
	if m.current >= len(m.Steps) {
		return nil, fmt.Errorf("exhausted: %w", io.EOF)
	}
	if m.current != 0 {
		lastReply := m.Steps[m.current-1]
		if !cmp.Equal(lastReply.ExpectedEvent, m.lastEvent) {
			m.t.Errorf("step %d did not generate the expected event, diff: %s", m.current-1, cmp.Diff(lastReply.ExpectedEvent, m.lastEvent))
		}
	}
	m.lastEvent = nil
	reply := m.Steps[m.current]
	m.current = m.current + 1
	return reply.Event, reply.RecvErr
}

type mockApplicationServiceClient struct {
	Steps     []step
	current   int
	t         *testing.T
	lastEvent *ArgoEvent
	grpc.ClientStream
}

func (m *mockApplicationServiceClient) Watch(ctx context.Context, qry *application.ApplicationQuery, opts ...grpc.CallOption) (application.ApplicationService_WatchClient, error) {
	if m.current >= len(m.Steps) {
		return nil, fmt.Errorf("exhausted: %w", io.EOF)
	}
	reply := m.Steps[m.current]
	if reply.WatchErr != nil {
		m.current = m.current + 1
		return nil, reply.WatchErr
	}
	return m, nil
}

func (m *mockApplicationServiceClient) testAllConsumed(t *testing.T) {
	if m.current < len(m.Steps) {
		t.Errorf("expected to consume all %d replies, only consumed %d", len(m.Steps), m.current)
	}
}

// Process implements service.EventProcessor
func (m *mockApplicationServiceClient) ProcessArgoEvent(ctx context.Context, ev ArgoEvent) {
	m.lastEvent = &ev
}

type version struct {
	Revision        string
	Environment     string
	Application     string
	DeployedVersion uint64
	Error           error
}

type mockVersionClient struct {
	versions []version
}

// GetVersion implements versions.VersionClient
func (m *mockVersionClient) GetVersion(ctx context.Context, revision string, environment string, application string) (*versions.VersionInfo, error) {
	for _, v := range m.versions {
		if v.Revision == revision && v.Environment == environment && v.Application == application {
			return &versions.VersionInfo{Version: v.DeployedVersion}, v.Error
		}
	}
	return nil, nil
}

func (m *mockVersionClient) ConsumeEvents(ctx context.Context, pc versions.VersionEventProcessor) error {
	panic("not implemented")
}

var _ versions.VersionClient = (*mockVersionClient)(nil)

func TestArgoConection(t *testing.T) {
	tcs := []struct {
		Name     string
		Versions []version
		Steps    []step

		ExpectedError string
		ExpectedReady bool
	}{
		{
			Name: "stops without error when ctx is closed on Recv call",
			Steps: []step{
				{
					WatchErr: status.Error(codes.Canceled, "context cancelled"),
				},
			},
			ExpectedReady: false,
		},
		{
			Name: "stops with error on other watch errors",
			Steps: []step{
				{
					WatchErr: fmt.Errorf("no"),
				},
			},

			ExpectedError: "watching applications: no",
			ExpectedReady: false,
		},

		{
			Name: "stops when ctx closes in the watch call",
			Steps: []step{
				{
					RecvErr: status.Error(codes.Canceled, "context cancelled"),
				},
			},
			ExpectedReady: true,
		},
		{
			Name: "retries when Recv fails",
			Steps: []step{
				{
					RecvErr: fmt.Errorf("no"),
				},
				{
					RecvErr: status.Error(codes.Canceled, "context cancelled"),
				},
			},
			ExpectedReady: true,
		},
		{
			Name: "ignore events for applications that were not generated by kuberpult",
			Steps: []step{
				{
					Event: &v1alpha1.ApplicationWatchEvent{
						Type: "ADDED",
						Application: v1alpha1.Application{
							ObjectMeta: metav1.ObjectMeta{
								Name:        "foo",
								Annotations: map[string]string{},
							},
							Spec: v1alpha1.ApplicationSpec{
								Project: "",
							},
							Status: v1alpha1.ApplicationStatus{
								Sync:   v1alpha1.SyncStatus{Revision: "1234"},
								Health: v1alpha1.HealthStatus{},
							},
						},
					},
					// Applications generated by kuberpult have name = "<env>-<name>" and project = "<env>".
					// This application doesn't follow this scheme and must not create an event.
					ExpectedEvent: nil,
				},
				{
					RecvErr: status.Error(codes.Canceled, "context cancelled"),
				},
			},
			ExpectedReady: true,
		},
		{
			Name: "generates events for applications that were generated by kuberpult",
			Versions: []version{
				{
					Revision:        "1234",
					Environment:     "foo",
					Application:     "bar",
					DeployedVersion: 42,
				},
			},
			Steps: []step{
				{
					Event: &v1alpha1.ApplicationWatchEvent{
						Type: "ADDED",
						Application: v1alpha1.Application{
							ObjectMeta: metav1.ObjectMeta{
								Name: "doesntmatter",
								Annotations: map[string]string{
									"com.freiheit.kuberpult/environment": "foo",
									"com.freiheit.kuberpult/application": "bar",
								},
							},
							Spec: v1alpha1.ApplicationSpec{
								Project: "foo",
							},
							Status: v1alpha1.ApplicationStatus{
								Sync:   v1alpha1.SyncStatus{Revision: "1234"},
								Health: v1alpha1.HealthStatus{},
							},
						},
					},
					ExpectedEvent: &ArgoEvent{
						Application: "bar",
						Environment: "foo",
						Version:     &versions.VersionInfo{Version: 42},
					},
				},
				{
					RecvErr: status.Error(codes.Canceled, "context cancelled"),
				},
			},
			ExpectedReady: true,
		},
		{
			Name: "doesnt generate events for deleted",
			Versions: []version{
				{
					Revision:        "1234",
					Environment:     "foo",
					Application:     "bar",
					DeployedVersion: 42,
				},
			},
			Steps: []step{
				{
					Event: &v1alpha1.ApplicationWatchEvent{
						Type: "DELETED",
						Application: v1alpha1.Application{
							ObjectMeta: metav1.ObjectMeta{
								Name: "doesntmatter",
								Annotations: map[string]string{
									"com.freiheit.kuberpult/environment": "foo",
									"com.freiheit.kuberpult/application": "bar",
								},
							},
							Spec: v1alpha1.ApplicationSpec{
								Project: "foo",
							},
							Status: v1alpha1.ApplicationStatus{
								Sync:   v1alpha1.SyncStatus{Revision: "1234"},
								Health: v1alpha1.HealthStatus{},
							},
						},
					},
					ExpectedEvent: &ArgoEvent{
						Application: "bar",
						Environment: "foo",
						Version:     &versions.VersionInfo{Version: 0},
					},
				},
				{
					RecvErr: status.Error(codes.Canceled, "context cancelled"),
				},
			},
			ExpectedReady: true,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			as := mockApplicationServiceClient{
				Steps: tc.Steps,
				t:     t,
			}
			hlth := &setup.HealthServer{}
			err := ConsumeEvents(ctx, &as, &mockVersionClient{versions: tc.Versions}, &as, hlth.Reporter("consume"))
			if tc.ExpectedError == "" {
				if err != nil {
					t.Errorf("expected no error, but got %q", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error %q, but got <nil>", tc.ExpectedError)
				} else if err.Error() != tc.ExpectedError {
					t.Errorf("expected error %q, but got %q", tc.ExpectedError, err)
				}
			}
			ready := hlth.IsReady("consume")
			if tc.ExpectedReady != ready {
				t.Errorf("expected ready to be %t but got %t", tc.ExpectedReady, ready)
			}
			as.testAllConsumed(t)
		})
	}
}
