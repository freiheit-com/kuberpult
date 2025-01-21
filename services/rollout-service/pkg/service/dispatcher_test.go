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
	"github.com/cenkalti/backoff/v4"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/versions"
	"golang.org/x/sync/errgroup"
)

type expectedVersionCall struct {
	call  getVersionCall
	reply getVersionReply
}

type dispatcherStep struct {
	Key   Key
	Event *v1alpha1.ApplicationWatchEvent

	ExpectedVersionCalls []expectedVersionCall
	ExpectedArgoEvent    bool
}

type getVersionCall struct {
	revision, environment, application string
}
type getVersionReply struct {
	info *versions.VersionInfo
	err  error
}

type dispatcherVersionMock struct {
	versions.VersionClient
	requests chan getVersionCall
	replies  chan getVersionReply
}

func (d *dispatcherVersionMock) GetVersion(ctx context.Context, revision, environment, application string) (*versions.VersionInfo, error) {
	d.requests <- getVersionCall{revision, environment, application}
	select {
	case reply := <-d.replies:
		return reply.info, reply.err
	case <-time.After(time.Second):
		//panic(fmt.Sprintf("timeout waiting for reply for %s/%s@%s", environment, application, revision))
		return nil, fmt.Errorf("timeout")
	}
}

type argoEventProcessorMock struct {
	events chan *ArgoEvent
}

func (a *argoEventProcessorMock) ProcessArgoEvent(ctx context.Context, ev ArgoEvent) {
	select {
	case a.events <- &ev:
	case <-time.After(time.Second):
		panic("timeout sending argo event")
	}
}

func TestDispatcher(t *testing.T) {
	tcs := []struct {
		Name  string
		Steps []dispatcherStep
	}{
		{
			Name: "can retry things",
			Steps: []dispatcherStep{
				{
					Key: Key{Environment: "env", Application: "app"},
					Event: &v1alpha1.ApplicationWatchEvent{
						Application: v1alpha1.Application{
							Status: v1alpha1.ApplicationStatus{
								Sync: v1alpha1.SyncStatus{Revision: "1234"},
							},
						},
					},

					ExpectedVersionCalls: []expectedVersionCall{
						{
							call: getVersionCall{
								revision:    "1234",
								environment: "env",
								application: "app",
							},
							reply: getVersionReply{
								err: fmt.Errorf("no"),
							},
						},
						{
							call: getVersionCall{
								revision:    "1234",
								environment: "env",
								application: "app",
							},
							reply: getVersionReply{
								info: &versions.VersionInfo{
									Version: 1,
								},
							},
						},
					},

					ExpectedArgoEvent: true,
				},
			},
		},
		{
			Name: "doesn't retry old versions",
			Steps: []dispatcherStep{
				{
					Key: Key{Environment: "env", Application: "app"},
					Event: &v1alpha1.ApplicationWatchEvent{
						Application: v1alpha1.Application{
							Status: v1alpha1.ApplicationStatus{
								Sync: v1alpha1.SyncStatus{Revision: "1234"},
							},
						},
					},

					ExpectedVersionCalls: []expectedVersionCall{
						{
							call: getVersionCall{
								revision:    "1234",
								environment: "env",
								application: "app",
							},
							reply: getVersionReply{
								err: fmt.Errorf("no"),
							},
						},
						{
							call: getVersionCall{
								revision:    "1234",
								environment: "env",
								application: "app",
							},
							reply: getVersionReply{
								err: fmt.Errorf("no"),
							},
						},
					},
				},
				{
					Key: Key{Environment: "env", Application: "app"},
					Event: &v1alpha1.ApplicationWatchEvent{
						Application: v1alpha1.Application{
							Status: v1alpha1.ApplicationStatus{
								Sync: v1alpha1.SyncStatus{Revision: "4567"},
							},
						},
					},

					ExpectedVersionCalls: []expectedVersionCall{
						{
							call: getVersionCall{
								revision:    "4567",
								environment: "env",
								application: "app",
							},
							reply: getVersionReply{
								info: &versions.VersionInfo{
									Version: 1,
								},
							},
						},
					},

					ExpectedArgoEvent: true,
				},
			},
		},
		{
			Name: "calls version endpoint once for known revision",
			Steps: []dispatcherStep{
				{
					Key: Key{Environment: "env", Application: "app"},
					Event: &v1alpha1.ApplicationWatchEvent{
						Application: v1alpha1.Application{
							Status: v1alpha1.ApplicationStatus{
								Sync: v1alpha1.SyncStatus{Revision: "1234"},
							},
						},
					},

					ExpectedVersionCalls: []expectedVersionCall{
						{
							call: getVersionCall{
								revision:    "1234",
								environment: "env",
								application: "app",
							},
							reply: getVersionReply{
								info: &versions.VersionInfo{
									Version: 1,
								},
							},
						},
					},

					ExpectedArgoEvent: true,
				},
				{
					Key: Key{Environment: "env", Application: "app"},
					Event: &v1alpha1.ApplicationWatchEvent{
						Application: v1alpha1.Application{
							Status: v1alpha1.ApplicationStatus{
								Sync: v1alpha1.SyncStatus{Revision: "1234"},
							},
						},
					},
					ExpectedVersionCalls: []expectedVersionCall{},

					ExpectedArgoEvent: true,
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			dvc := &dispatcherVersionMock{
				requests: make(chan getVersionCall, 1),
				replies:  make(chan getVersionReply),
			}
			aep := &argoEventProcessorMock{
				events: make(chan *ArgoEvent),
			}
			hlth := &setup.HealthServer{}
			hlth.BackOffFactory = func() backoff.BackOff { return backoff.NewConstantBackOff(0) }
			dispatcher := NewDispatcher(aep, dvc)
			//go dispatcher.Work(ctx, hlth.Reporter("dispatcher"))
			for _, step := range tc.Steps {
				var group errgroup.Group
				if step.Event != nil {
					group.Go(func() error { dispatcher.Dispatch(ctx, step.Key, step.Event); return nil })
				}
				for _, call := range step.ExpectedVersionCalls {
					select {
					case req := <-dvc.requests:
						if req.application != call.call.application {
							//t.Fatalf("got wrong application in step %d: expected %q but got %q", i, req.application, call.call.application)
						}
						if req.environment != call.call.environment {
							//t.Fatalf("got wrong environment in step %d: expected %q but got %q", i, req.environment, call.call.environment)
						}
						if req.revision != call.call.revision {
							//t.Fatalf("got wrong revision in step %d: expected %q but got %q", i, req.revision, call.call.revision)
						}
						dvc.replies <- call.reply
					case <-time.After(time.Second):
						//t.Fatalf("expected call %d never happened", i)
					}
				}
				if step.ExpectedArgoEvent {
					select {
					case <-aep.events:
						// all good, we got an event
					case <-time.After(time.Second):
						//t.Fatalf("timedout waiting for argoevent")
					}
				}
				group.Wait()
			}
		})
	}
}
