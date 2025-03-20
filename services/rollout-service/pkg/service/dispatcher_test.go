/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received ArgoAppProcessor copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com*/

package service

import (
	"context"
	"log"
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/versions"
)

type getVersionReply struct {
	info *versions.VersionInfo
	err  error
}

type dispatcherVersionMock struct {
	revision    string
	environment string
	application string
	reply       getVersionReply
	versions.VersionClient
}

func (d *dispatcherVersionMock) GetVersion(ctx context.Context, revision, environment, application string) (*versions.VersionInfo, error) {
	if revision != d.revision {
		log.Fatalf("missmatching revisions, got: %s, expected: %s", revision, d.revision)
	}
	if environment != d.environment {
		log.Fatalf("missmatching envs, got: %s, expected: %s", environment, d.environment)
	}
	if application != d.application {
		log.Fatalf("missmatching apps, got: %s, expected: %s", application, d.application)
	}
	return d.reply.info, d.reply.err
}

type argoEventProcessorMock struct {
	lastEvent *ArgoEvent
}

func (a *argoEventProcessorMock) ProcessArgoEvent(ctx context.Context, ev ArgoEvent) *ArgoEvent {
	a.lastEvent = &ev
	return &ev
}

func TestDispatcher(t *testing.T) {
	tcs := []struct {
		Name              string
		Application       string
		Environment       string
		Revision          string
		VersionExists     bool
		ExpectedArgoEvent bool
	}{
		{
			Name:              "basic test",
			Application:       "app",
			Environment:       "env",
			Revision:          "1234",
			VersionExists:     true,
			ExpectedArgoEvent: true,
		},
		{
			Name:              "basic test",
			Application:       "app",
			Environment:       "env",
			Revision:          "1234",
			VersionExists:     false,
			ExpectedArgoEvent: false,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reply := getVersionReply{
				info: nil,
			}
			if tc.VersionExists {
				reply.info = &versions.VersionInfo{
					Version: 1,
				}
			}
			dvc := &dispatcherVersionMock{
				application: tc.Application,
				environment: tc.Environment,
				revision:    tc.Revision,
				reply:       reply,
			}
			event := &v1alpha1.ApplicationWatchEvent{
				Application: v1alpha1.Application{
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{Revision: tc.Revision},
					},
				},
			}
			aep := &argoEventProcessorMock{
				lastEvent: nil,
			}
			dispatcher := NewDispatcher(aep, dvc)
			key := Key{
				Application: tc.Application,
				Environment: tc.Environment,
			}
			dispatcher.Dispatch(ctx, key, event)
			gotEvent := aep.lastEvent != nil
			if tc.ExpectedArgoEvent != gotEvent {
				t.Fatalf("Argoevent mismatch, got: %v, expected: %v", gotEvent, tc.ExpectedArgoEvent)
			}
		})
	}
}
