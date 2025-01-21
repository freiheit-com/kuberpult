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
/*
This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com
*/
package revolution

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/service"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/versions"
	"github.com/google/go-cmp/cmp"
)

func TestRevolution(t *testing.T) {
	type request struct {
		Url    string
		Header http.Header
		Body   string
	}
	type step struct {
		ArgoEvent       *service.ArgoEvent
		KuberpultEvent  *versions.KuberpultEvent
		ExpectedRequest *request
	}
	tcs := []struct {
		Name  string
		Steps []step
		Now   time.Time
	}{
		{
			Name: "send out deployment events with timestamp once",
			Now:  time.Unix(123456789, 0).UTC(),
			Steps: []step{
				{
					ArgoEvent: &service.ArgoEvent{
						Environment:      "foo",
						Application:      "bar",
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusHealthy,
						Version: &versions.VersionInfo{
							Version:        1,
							SourceCommitId: "123456",
							DeployedAt:     time.Unix(123456789, 0).UTC(),
						},
					},
					KuberpultEvent: &versions.KuberpultEvent{
						Environment:  "foo",
						Application:  "bar",
						IsProduction: true,
						Version: &versions.VersionInfo{
							Version:        1,
							SourceCommitId: "123456",
							DeployedAt:     time.Unix(123456789, 0).UTC(),
						},
					},

					ExpectedRequest: &request{
						Url: "/",
						Header: http.Header{
							"Content-Type":        []string{"application/json"},
							"User-Agent":          []string{"kuberpult"},
							"X-Hub-Signature-256": []string{"sha256=c227a4f702ce00368a15bd00c3678dd20c76ed7275d82c5a2d48009beb78b5ee"},
						},
						Body: `{"id":"0ee3e568-0f9d-5be9-b75c-caa9025599c2","commitHash":"123456","eventTime":"1973-11-29T21:33:09Z","serviceName":"bar"}`,
					},
				},
				{
					ArgoEvent: &service.ArgoEvent{
						Environment:      "foo",
						Application:      "bar",
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusDegraded,
						Version: &versions.VersionInfo{
							Version:        1,
							SourceCommitId: "123456",
							DeployedAt:     time.Unix(123456789, 0).UTC(),
						},
					},

					ExpectedRequest: nil,
				},
			},
		},
		{
			Name: "ignore old events",
			Now:  time.Date(2024, 2, 15, 12, 15, 0, 0, time.UTC),
			Steps: []step{
				{
					ArgoEvent: &service.ArgoEvent{
						Environment:      "foo",
						Application:      "bar",
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusDegraded,
						Version: &versions.VersionInfo{
							Version:        1,
							SourceCommitId: "123456",
							DeployedAt:     time.Date(2024, 2, 14, 12, 15, 0, 0, time.UTC),
						},
					},

					KuberpultEvent: &versions.KuberpultEvent{
						Environment:  "foo",
						Application:  "bar",
						IsProduction: true,
						Version: &versions.VersionInfo{
							Version:        1,
							SourceCommitId: "123456",
							DeployedAt:     time.Date(2024, 2, 15, 12, 15, 0, 0, time.UTC),
						},
					},

					ExpectedRequest: nil,
				},
			},
		},
		{
			Name: "don't notify on a lone kuberpult event",
			Now:  time.Unix(123456789, 0).UTC(),
			Steps: []step{
				{
					KuberpultEvent: &versions.KuberpultEvent{
						Environment:      "fakeprod",
						Application:      "foo",
						EnvironmentGroup: "prod",
						IsProduction:     true,
						Team:             "bar",
						Version: &versions.VersionInfo{
							Version:        1,
							SourceCommitId: "123456",
							DeployedAt:     time.Unix(123456789, 0).UTC(),
						},
					},
					ExpectedRequest: nil,
				},
			},
		},
		{
			Name: "don't notify on a lone argo event",
			Now:  time.Unix(123456789, 0).UTC(),
			Steps: []step{
				{
					ArgoEvent: &service.ArgoEvent{
						Environment:      "foo",
						Application:      "bar",
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusDegraded,
						Version: &versions.VersionInfo{
							Version:        1,
							SourceCommitId: "123456",
							DeployedAt:     time.Unix(123456789, 0).UTC(),
						},
					},
					ExpectedRequest: nil,
				},
			},
		},
		{
			Name: "don't notify on non production env",
			Now:  time.Unix(123456789, 0).UTC(),
			Steps: []step{
				{
					ArgoEvent: &service.ArgoEvent{
						Environment:      "foo",
						Application:      "bar",
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusHealthy,
						Version: &versions.VersionInfo{
							Version:        1,
							SourceCommitId: "123456",
							DeployedAt:     time.Unix(123456789, 0).UTC(),
						},
					},
					KuberpultEvent: &versions.KuberpultEvent{
						Environment:  "foo",
						Application:  "bar",
						IsProduction: false,
						Version: &versions.VersionInfo{
							Version:        1,
							SourceCommitId: "123456",
							DeployedAt:     time.Unix(123456789, 0).UTC(),
						},
					},
					ExpectedRequest: nil,
				},
			},
		},
		{
			Name: "don't notify on non production env",
			Now:  time.Unix(123456789, 0).UTC(),
			Steps: []step{
				{
					ArgoEvent: &service.ArgoEvent{
						Environment:      "foo",
						Application:      "bar",
						SyncStatusCode:   v1alpha1.SyncStatusCodeSynced,
						HealthStatusCode: health.HealthStatusHealthy,
						Version: &versions.VersionInfo{
							Version:        1,
							SourceCommitId: "123456",
							DeployedAt:     time.Unix(123456789, 0).UTC(),
						},
					},
					KuberpultEvent: &versions.KuberpultEvent{
						Environment:  "foo",
						Application:  "bar",
						IsProduction: false,
						Version: &versions.VersionInfo{
							Version:        1,
							SourceCommitId: "123456",
							DeployedAt:     time.Unix(123456789, 0).UTC(),
						},
					},
					ExpectedRequest: nil,
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			reqCh := make(chan *request)
			ctx, cancel := context.WithCancel(context.Background())
			revolution := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				hdr := r.Header.Clone()
				hdr.Del("Accept-Encoding")
				hdr.Del("Content-Length")
				select {
				case reqCh <- &request{
					Url:    r.URL.String(),
					Header: hdr,
					Body:   string(body),
				}:
					w.WriteHeader(http.StatusOK)
				case <-ctx.Done():
					t.Errorf("Ended test with requests left to process")
				}
			}))
			readyCh := make(chan struct{}, 1)
			bc := service.New()
			errCh := make(chan error, 1)
			cs := New(Config{
				URL:         revolution.URL,
				Token:       []byte("revolution"),
				Concurrency: 100,
				MaxEventAge: time.Second,
			})
			cs.ready = func() { readyCh <- struct{}{} }
			cs.now = func() time.Time { return tc.Now }
			go func() {
				errCh <- cs.Subscribe(ctx, bc)
			}()
			<-readyCh
			for i, s := range tc.Steps {
				if s.ArgoEvent != nil {
					bc.ProcessArgoEvent(context.Background(), *s.ArgoEvent)
				}
				if s.KuberpultEvent != nil {
					bc.ProcessKuberpultEvent(context.Background(), *s.KuberpultEvent)
				}
				if s.ExpectedRequest != nil {
					select {
					case <-time.After(5 * time.Second):
						t.Fatalf("expected request in step %d, but didn't receive any", i)
					case req := <-reqCh:
						d := cmp.Diff(req, s.ExpectedRequest)
						if d != "" {
							t.Errorf("unexpected requests diff in step %d: %s", i, d)
						}
					}

				} else {
					select {
					case req := <-reqCh:
						t.Errorf("unexpected requests in step %d: %#v", i, req)
					default:
					}
				}
			}
			cancel()
			err := <-errCh
			if err != nil {
				t.Errorf("expected no error but got %q", err)
			}
		})
	}
}
