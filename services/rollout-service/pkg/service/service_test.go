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
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type mockRecvReply struct {
	Event *v1alpha1.ApplicationWatchEvent
	Err   error
}

type mockApplicationWatch struct {
	Replies []mockRecvReply
	current int
	cancel  context.CancelFunc
}

// CloseSend implements application.ApplicationService_WatchClient
func (*mockApplicationWatch) CloseSend() error {
	panic("unimplemented")
}

// Context implements application.ApplicationService_WatchClient
func (*mockApplicationWatch) Context() context.Context {
	panic("unimplemented")
}

// Header implements application.ApplicationService_WatchClient
func (*mockApplicationWatch) Header() (metadata.MD, error) {
	panic("unimplemented")
}

// RecvMsg implements application.ApplicationService_WatchClient
func (*mockApplicationWatch) RecvMsg(m interface{}) error {
	panic("unimplemented")
}

// SendMsg implements application.ApplicationService_WatchClient
func (*mockApplicationWatch) SendMsg(m interface{}) error {
	panic("unimplemented")
}

// Trailer implements application.ApplicationService_WatchClient
func (*mockApplicationWatch) Trailer() metadata.MD {
	panic("unimplemented")
}

func (m *mockApplicationWatch) Recv() (*v1alpha1.ApplicationWatchEvent, error) {
	if m.current >= len(m.Replies) {
		return nil, fmt.Errorf("exhausted: %w", io.EOF)
	}
	reply := m.Replies[m.current]
	m.current = m.current + 1
	if m.current >= len(m.Replies) && m.cancel != nil {
		m.cancel()
	}
	return reply.Event, reply.Err
}

func (m *mockApplicationWatch) Start(cancel context.CancelFunc) {
	m.cancel = cancel
}

var _ application.ApplicationService_WatchClient = (*mockApplicationWatch)(nil)

type mockApplicationReply struct {
	Watch *mockApplicationWatch
	Err   error
}

type mockApplicationServiceClient struct {
	Replies []mockApplicationReply
	current int
	cancel  context.CancelFunc
}

func (m *mockApplicationServiceClient) Watch(ctx context.Context, qry *application.ApplicationQuery, opts ...grpc.CallOption) (application.ApplicationService_WatchClient, error) {
	if m.current >= len(m.Replies) {
		return nil, fmt.Errorf("exhausted: %w", io.EOF)
	}
	reply := m.Replies[m.current]
	m.current = m.current + 1
	if m.current >= len(m.Replies) {
		if reply.Err != nil {
			m.cancel()
		}
		if reply.Watch != nil {
			reply.Watch.Start(m.cancel)
		}
	}
	return reply.Watch, reply.Err
}

func (m *mockApplicationServiceClient) Start(cancel context.CancelFunc) {
	m.cancel = cancel
}
func (m *mockApplicationServiceClient) testAllConsumed(t *testing.T) {
	if m.current < len(m.Replies) {
           t.Errorf("expected to consume all %d replies, only consumed %d", len(m.Replies), m.current)
	}
}

func TestArgoConection(t *testing.T) {
	tcs := []struct {
		Name               string
		ApplicationService *mockApplicationServiceClient

		ExpectedError string
	}{
		{
			Name: "stops when ctx is closed on Recv call",
			ApplicationService: &mockApplicationServiceClient{
				Replies: []mockApplicationReply{
					{
						Err: status.Error(codes.Canceled, "context cancelled"),
					},
				},
			},
		},
		{
			Name: "stops when ctx closes in the watch call",
			ApplicationService: &mockApplicationServiceClient{
				Replies: []mockApplicationReply{
					{
						Watch: &mockApplicationWatch{
							Replies: []mockRecvReply{{
								Err: status.Error(codes.Canceled, "context cancelled"),
							}},
						},
					},
				},
			},
		},
		{
			Name: "retries when Recv fails",
			ApplicationService: &mockApplicationServiceClient{
				Replies: []mockApplicationReply{
					{
						Watch: &mockApplicationWatch{
							Replies: []mockRecvReply{{
								Err: fmt.Errorf("no"),
							}},
						},
					},
					{
						Watch: &mockApplicationWatch{
							Replies: []mockRecvReply{{
								Err: status.Error(codes.Canceled, "context cancelled"),
							}},
						},
					},
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			tc.ApplicationService.Start(cancel)
			err := ConsumeEvents(ctx, tc.ApplicationService, nil)
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
                        tc.ApplicationService.testAllConsumed(t)
		})
	}
}
