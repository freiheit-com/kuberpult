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

package notifier

import (
	"context"
	"testing"

	argoapplication "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"google.golang.org/grpc"
)

type mockApplicationClient struct {
	requests chan *argoapplication.ApplicationQuery
}

func (m *mockApplicationClient) Get(ctx context.Context, in *argoapplication.ApplicationQuery, opts ...grpc.CallOption) (*argoappv1.Application, error) {
	m.requests <- in
	return nil, nil
}

func TestNotifier(t *testing.T) {
	tcs := []struct {
		Name             string
		ConcurrencyLimit int
	}{
		{
			Name:             "sends requests in parallel",
			ConcurrencyLimit: 10,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			// chan without capacity will block all requests
			ch := make(chan *argoapplication.ApplicationQuery)
			ma := &mockApplicationClient{ch}
			nf := New(ma, tc.ConcurrencyLimit, 60)
			for i := 0; i < tc.ConcurrencyLimit; i = i + 1 {
				nf.NotifyArgoCd(ctx, "foo", "bar")
			}

			for i := 0; i < tc.ConcurrencyLimit; i = i + 1 {
				in := <-ch
				if *in.Name != "foo-bar" {
					t.Errorf("expected application %q, but got %q", "foo-bar", *in.Name)
				}
				if *in.Refresh != "normal" {
					t.Errorf("expected referesh type %q, but got %q", "normal", *in.Refresh)
				}
			}
		})

	}
}
