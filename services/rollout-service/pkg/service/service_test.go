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
	"google.golang.org/grpc/status"
)

type mockReply struct {
	Event *v1alpha1.ApplicationWatchEvent
	WatchErr   error
        RecvErr    error
        
        ExpectedEvent *Event
}


func (m *mockApplicationServiceClient) Recv() (*v1alpha1.ApplicationWatchEvent, error) {
	if m.current >= len(m.Replies) {
		return nil, fmt.Errorf("exhausted: %w", io.EOF)
	}
	reply := m.Replies[m.current]
	m.current = m.current + 1
	if m.current >= len(m.Replies) && m.cancel != nil {
		m.cancel()
	}
	return reply.Event, reply.RecvErr
}

type mockApplicationServiceClient struct {
	Replies []mockReply
	current int
	cancel  context.CancelFunc
	grpc.ClientStream
}

func (m *mockApplicationServiceClient) Watch(ctx context.Context, qry *application.ApplicationQuery, opts ...grpc.CallOption) (application.ApplicationService_WatchClient, error) {
	if m.current >= len(m.Replies) {
		return nil, fmt.Errorf("exhausted: %w", io.EOF)
	}
	reply := m.Replies[m.current]
        if reply.WatchErr != nil {
	  m.current = m.current + 1
          return nil, reply.WatchErr
        }
	return m, nil
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
				Replies: []mockReply{
					{
						WatchErr: status.Error(codes.Canceled, "context cancelled"),
					},
				},
			},
		},
		{
			Name: "stops when ctx closes in the watch call",
			ApplicationService: &mockApplicationServiceClient{
				Replies: []mockReply{
					{
						RecvErr: status.Error(codes.Canceled, "context cancelled"),
					},
				},
			},
		},
		{
			Name: "retries when Recv fails",
			ApplicationService: &mockApplicationServiceClient{
				Replies: []mockReply{
					{
						RecvErr: fmt.Errorf("no"),
					},
					{
						RecvErr: status.Error(codes.Canceled, "context cancelled"),
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
			err := ConsumeEvents(ctx, tc.ApplicationService, nil, nil)
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
