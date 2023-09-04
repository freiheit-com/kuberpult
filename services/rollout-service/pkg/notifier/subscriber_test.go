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

package notifier

import (
	"context"
	"testing"

	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/service"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/versions"
	"github.com/google/go-cmp/cmp"
)

type expectedNotification struct {
	Application string
	Environment string
}

type mockArgocdNotifier struct {
	ch chan<- expectedNotification
}

func (m *mockArgocdNotifier) NotifyArgoCd(ctx context.Context, environment, application string) {
	m.ch <- expectedNotification{application, environment}
}

func TestSubscribe(t *testing.T) {
	type step struct {
		ArgoEvent    *service.ArgoEvent
		VersionEvent *versions.KuberpultEvent

		ExpectedNotification *expectedNotification
	}
	tcs := []struct {
		Name  string
		Steps []step
	}{
		{
			Name: "shuts down correctly",
		},
		{
			Name: "notifies when the kuberpult version differs",
			Steps: []step{
				{
					ArgoEvent: &service.ArgoEvent{
						Application: "foo",
						Environment: "bar",
						Version:     1,
					},
				},
				{
					VersionEvent: &versions.KuberpultEvent{
						Application: "foo",
						Environment: "bar",
						Version:     2,
					},
					ExpectedNotification: &expectedNotification{
						Application: "foo",
						Environment: "bar",
					},
				},
			},
		},
		{
			Name: "doesnt notify for the same version again",
			Steps: []step{
				{
					ArgoEvent: &service.ArgoEvent{
						Application: "foo",
						Environment: "bar",
						Version:     1,
					},
				},
				{
					VersionEvent: &versions.KuberpultEvent{
						Application: "foo",
						Environment: "bar",
						Version:     2,
					},
					ExpectedNotification: &expectedNotification{
						Application: "foo",
						Environment: "bar",
					},
				},
				{
					VersionEvent: &versions.KuberpultEvent{
						Application: "foo",
						Environment: "bar",
						Version:     2,
					},
				},
			},
		},
		{
			Name: "does notify for each version",
			Steps: []step{
				{
					ArgoEvent: &service.ArgoEvent{
						Application: "foo",
						Environment: "bar",
						Version:     1,
					},
				},
				{
					VersionEvent: &versions.KuberpultEvent{
						Application: "foo",
						Environment: "bar",
						Version:     2,
					},
					ExpectedNotification: &expectedNotification{
						Application: "foo",
						Environment: "bar",
					},
				},
				{
					VersionEvent: &versions.KuberpultEvent{
						Application: "foo",
						Environment: "bar",
						Version:     3,
					},
					ExpectedNotification: &expectedNotification{
						Application: "foo",
						Environment: "bar",
					},
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			notifications := make(chan expectedNotification, len(tc.Steps))
			mn := &mockArgocdNotifier{notifications}
			bc := service.New()
			ctx, cancel := context.WithCancel(ctx)
			eCh := make(chan error, 1)
			go func() {
				eCh <- Subscribe(ctx, mn, bc)
			}()

			for _, s := range tc.Steps {
				if s.ArgoEvent != nil {
					bc.ProcessArgoEvent(ctx, *s.ArgoEvent)
				} else {
					bc.ProcessKuberpultEvent(ctx, *s.VersionEvent)
				}
				if s.ExpectedNotification != nil {
					notification := <-notifications
					if !cmp.Equal(notification, *s.ExpectedNotification) {
						t.Errorf("expected notification %v, but got %v", s.ExpectedNotification, notification)
					}
				} else {
					select {
					case notification := <-notifications:
						t.Errorf("exptected no notification, but got %v", notification)
					default:
					}
				}
			}
			cancel()
			err := <-eCh
			if err != nil {
				t.Errorf("expected no error, but got %q", err)
			}
		})
	}
}
