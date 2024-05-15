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

package metrics

import (
	"context"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	pkgmetrics "github.com/freiheit-com/kuberpult/pkg/metrics"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/service"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/versions"
	"github.com/google/go-cmp/cmp"
)

type step struct {
	VersionsEvent *versions.KuberpultEvent
	ArgoEvent     *service.ArgoEvent
	Disconnect    bool

	ExpectedBody string
}

func TestMetric(t *testing.T) {
	tcs := []struct {
		Name  string
		Steps []step
	}{
		{
			Name: "doesnt write metrics when argocd data is missing",
			Steps: []step{
				{
					VersionsEvent: &versions.KuberpultEvent{
						Application:      "foo",
						Environment:      "bar",
						EnvironmentGroup: "buz",
						Version: &versions.VersionInfo{
							Version:    1,
							DeployedAt: time.Unix(1000, 0),
						},
					},
					ExpectedBody: ``,
				},
			},
		},
		{
			Name: "doesn't write metrics when kuberpult data is missing",
			Steps: []step{
				{
					ArgoEvent: &service.ArgoEvent{
						Application: "foo",
						Environment: "bar",
						Version: &versions.VersionInfo{
							Version: 1,
						},
						HealthStatusCode: health.HealthStatusHealthy,
						SyncStatusCode:   v1alpha1.SyncStatusCodeUnknown,
					},
					ExpectedBody: ``,
				},
			},
		},
		{
			Name: "writes out 0 for successful deployments",
			Steps: []step{
				{
					VersionsEvent: &versions.KuberpultEvent{
						Application:      "foo",
						Environment:      "bar",
						EnvironmentGroup: "buz",
						Version: &versions.VersionInfo{
							Version:    1,
							DeployedAt: time.Unix(1000, 0),
						},
					},
					ArgoEvent: &service.ArgoEvent{
						Application: "foo",
						Environment: "bar",
						Version: &versions.VersionInfo{
							Version: 1,
						},
						HealthStatusCode: health.HealthStatusHealthy,
						SyncStatusCode:   v1alpha1.SyncStatusCodeUnknown,
					},
					ExpectedBody: `# HELP rollout_lag_seconds 
# TYPE rollout_lag_seconds gauge
rollout_lag_seconds{kuberpult_application="foo",kuberpult_environment="bar",kuberpult_environment_group="buz"} 0
`,
				},
			},
		},
		{
			Name: "writes out time diff for missing deployments",
			Steps: []step{
				{
					VersionsEvent: &versions.KuberpultEvent{
						Application:      "foo",
						Environment:      "bar",
						EnvironmentGroup: "buz",
						Version: &versions.VersionInfo{
							Version:    2,
							DeployedAt: time.Unix(1000, 0),
						},
					},
					ArgoEvent: &service.ArgoEvent{
						Application: "foo",
						Environment: "bar",
						Version: &versions.VersionInfo{
							Version: 1,
						},
						HealthStatusCode: health.HealthStatusHealthy,
						SyncStatusCode:   v1alpha1.SyncStatusCodeUnknown,
					},
					ExpectedBody: `# HELP rollout_lag_seconds 
# TYPE rollout_lag_seconds gauge
rollout_lag_seconds{kuberpult_application="foo",kuberpult_environment="bar",kuberpult_environment_group="buz"} 1000
`,
				},
			},
		},
		{
			Name: "reports no lag if the application is just unhealthy",
			Steps: []step{
				{
					VersionsEvent: &versions.KuberpultEvent{
						Application:      "foo",
						Environment:      "bar",
						EnvironmentGroup: "buz",
						Version: &versions.VersionInfo{
							Version:    2,
							DeployedAt: time.Unix(1000, 0),
						},
					},
					ArgoEvent: &service.ArgoEvent{
						Application: "foo",
						Environment: "bar",
						Version: &versions.VersionInfo{
							Version: 2,
						},
						HealthStatusCode: health.HealthStatusDegraded,
						SyncStatusCode:   v1alpha1.SyncStatusCodeUnknown,
					},
					ExpectedBody: `# HELP rollout_lag_seconds 
# TYPE rollout_lag_seconds gauge
rollout_lag_seconds{kuberpult_application="foo",kuberpult_environment="bar",kuberpult_environment_group="buz"} 0
`,
				},
			},
		},
		{
			Name: "removes metrics when app is removed",
			Steps: []step{
				{
					VersionsEvent: &versions.KuberpultEvent{
						Application:      "foo",
						Environment:      "bar",
						EnvironmentGroup: "buz",
						Version: &versions.VersionInfo{
							Version:    2,
							DeployedAt: time.Unix(1000, 0),
						},
					},
					ArgoEvent: &service.ArgoEvent{
						Application: "foo",
						Environment: "bar",
						Version: &versions.VersionInfo{
							Version: 1,
						},
						HealthStatusCode: health.HealthStatusHealthy,
						SyncStatusCode:   v1alpha1.SyncStatusCodeUnknown,
					},
					ExpectedBody: `# HELP rollout_lag_seconds 
# TYPE rollout_lag_seconds gauge
rollout_lag_seconds{kuberpult_application="foo",kuberpult_environment="bar",kuberpult_environment_group="buz"} 1000
`,
				},
				{
					VersionsEvent: &versions.KuberpultEvent{
						Application:      "foo",
						Environment:      "bar",
						EnvironmentGroup: "buz",
						Version: &versions.VersionInfo{
							Version: 0,
						},
					},
					ExpectedBody: ``,
				},
			},
		},
		{
			Name: "updates environment group when it changes",
			Steps: []step{
				{
					VersionsEvent: &versions.KuberpultEvent{
						Application:      "foo",
						Environment:      "bar",
						EnvironmentGroup: "buz",
						Version: &versions.VersionInfo{
							Version:    2,
							DeployedAt: time.Unix(1000, 0),
						},
					},
					ArgoEvent: &service.ArgoEvent{
						Application: "foo",
						Environment: "bar",
						Version: &versions.VersionInfo{
							Version: 1,
						},
						HealthStatusCode: health.HealthStatusHealthy,
						SyncStatusCode:   v1alpha1.SyncStatusCodeUnknown,
					},
					ExpectedBody: `# HELP rollout_lag_seconds 
# TYPE rollout_lag_seconds gauge
rollout_lag_seconds{kuberpult_application="foo",kuberpult_environment="bar",kuberpult_environment_group="buz"} 1000
`,
				},
				{
					VersionsEvent: &versions.KuberpultEvent{
						Application:      "foo",
						Environment:      "bar",
						EnvironmentGroup: "not-buz",
						Version: &versions.VersionInfo{
							Version:    3,
							DeployedAt: time.Unix(1500, 0),
						},
					},
					ExpectedBody: `# HELP rollout_lag_seconds 
# TYPE rollout_lag_seconds gauge
rollout_lag_seconds{kuberpult_application="foo",kuberpult_environment="bar",kuberpult_environment_group="not-buz"} 500
`,
				},
			},
		},
		{
			Name: "handles reconnects",
			Steps: []step{
				{
					VersionsEvent: &versions.KuberpultEvent{
						Application:      "foo",
						Environment:      "bar",
						EnvironmentGroup: "buz",
						Version: &versions.VersionInfo{
							Version:    2,
							DeployedAt: time.Unix(1000, 0),
						},
					},
					ArgoEvent: &service.ArgoEvent{
						Application: "foo",
						Environment: "bar",
						Version: &versions.VersionInfo{
							Version: 1,
						},
						HealthStatusCode: health.HealthStatusHealthy,
						SyncStatusCode:   v1alpha1.SyncStatusCodeUnknown,
					},
					ExpectedBody: `# HELP rollout_lag_seconds 
# TYPE rollout_lag_seconds gauge
rollout_lag_seconds{kuberpult_application="foo",kuberpult_environment="bar",kuberpult_environment_group="buz"} 1000
`,
				},
				{
					Disconnect: true,
					ExpectedBody: `# HELP rollout_lag_seconds 
# TYPE rollout_lag_seconds gauge
rollout_lag_seconds{kuberpult_application="foo",kuberpult_environment="bar",kuberpult_environment_group="buz"} 1000
`,
				},
				{
					VersionsEvent: &versions.KuberpultEvent{
						Application:      "foo",
						Environment:      "bar",
						EnvironmentGroup: "buz",
						Version: &versions.VersionInfo{
							Version:    3,
							DeployedAt: time.Unix(1500, 0),
						},
					},
					ExpectedBody: `# HELP rollout_lag_seconds 
# TYPE rollout_lag_seconds gauge
rollout_lag_seconds{kuberpult_application="foo",kuberpult_environment="bar",kuberpult_environment_group="buz"} 500
`,
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			mpv, handler, _ := pkgmetrics.Init()
			srv := httptest.NewServer(handler)
			defer srv.Close()
			bc := service.New()
			eCh := make(chan error)
			doneCh := make(chan struct{})
			go func() {
				eCh <- Metrics(ctx, bc, mpv, func() time.Time { return time.Unix(2000, 0).UTC() }, func() { doneCh <- struct{}{} })
			}()
			for i, s := range tc.Steps {
				if s.Disconnect {
					bc.DisconnectAll()
				}
				if s.VersionsEvent != nil {
					bc.ProcessKuberpultEvent(ctx, *s.VersionsEvent)
				}
				if s.ArgoEvent != nil {
					bc.ProcessArgoEvent(ctx, *s.ArgoEvent)
				}
				<-doneCh
				resp, err := srv.Client().Get(srv.URL + "/metrics")
				if err != nil {
					t.Errorf("error in step %d: %q", i, err)
				}
				if resp.StatusCode != 200 {
					t.Errorf("invalid status in step %d: %d", i, resp.StatusCode)
				}
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Errorf("error reading body in step %d: %q", i, err)
				}
				if string(body) != s.ExpectedBody {
					t.Errorf("wrong metrics received, diff: %s", cmp.Diff(s.ExpectedBody, string(body)))
				}
			}
			cancel()
			err := <-eCh
			if err != nil {
				t.Errorf("expected no error but got %q", err)
			}
		})

	}
}
