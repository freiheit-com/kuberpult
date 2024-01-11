package argo

import (
	"context"
	"fmt"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/cenkalti/backoff/v4"
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"testing"
	"time"
)

type step struct {
	Event         *v1alpha1.ApplicationWatchEvent
	WatchErr      error
	RecvErr       error
	CancelContext bool

	ExpectedEvent *ArgoEvent
}

type mockApplicationRequest struct {
	Type string
	Name string
}

func (m *mockApplicationServiceClient) Recv() (*v1alpha1.ApplicationWatchEvent, error) {
	if m.current >= len(m.Steps) {
		return nil, fmt.Errorf("exhausted: %w", io.EOF)
	}
	if m.current != 0 {
		lastReply := m.Steps[m.current-1]
		if lastReply.ExpectedEvent == nil {

		} else {
			select {
			case lastEvent := <-m.lastEvent:
				if !cmp.Equal(lastReply.ExpectedEvent, lastEvent) {
					m.t.Errorf("step %d did not generate the expected event, diff: %s", m.current-1, cmp.Diff(lastReply.ExpectedEvent, lastEvent))
				}
			case <-time.After(time.Second):
				m.t.Errorf("step %d timed out waiting for event", m.current-1)
			}
		}
	}
	reply := m.Steps[m.current]
	if reply.CancelContext {
		m.cancel()
	}
	m.current = m.current + 1
	return reply.Event, reply.RecvErr
}

type mockApplicationServiceClient struct {
	Steps     []step
	current   int
	t         *testing.T
	lastEvent chan *ArgoEvent
	cancel    context.CancelFunc
	grpc.ClientStream
}

type mockArgoProcessor struct {
	trigger           chan *v1alpha1.Application
	lastOverview      *api.GetOverviewResponse
	argoApps          chan *v1alpha1.ApplicationWatchEvent
	ApplicationClient *mockApplicationServiceClient
	HealthReporter    *setup.HealthReporter
}

func (a *mockArgoProcessor) Push(last *api.GetOverviewResponse, appToPush *v1alpha1.Application) {
	a.lastOverview = last
	select {
	case a.trigger <- appToPush:
	default:
	}
}

func (a *mockArgoProcessor) Consume(ctx context.Context) error {
	seen := map[Key]*api.Environment_Application{}

	for {
		select {
		case <-a.trigger:
			err := a.ConsumeArgo(ctx, a.ApplicationClient, a.HealthReporter)
			if err != nil {
				return err
			}
		case ev := <-a.argoApps:
			appName, envName := getEnvironmentAndName(ev.Application.Annotations)
			key := Key{Application: appName, Environment: envName}

			if ok := seen[key]; ok == nil {

				switch ev.Type {
				case "ADDED":
					appCreateRequest := &mockApplicationRequest{
						Type: "Create",
						Name: ev.Application.Name,
					}
					fmt.Println(appCreateRequest.Name)
				case "MODIFIED":
					appUpdateRequest := &mockApplicationRequest{
						Type: "Update",
						Name: ev.Application.Name,
					}
					fmt.Println(appUpdateRequest.Name)
				case "DELETED":
					appDeleteRequest := &mockApplicationRequest{
						Type: "Delete",
						Name: ev.Application.Name,
					}
					fmt.Println(appDeleteRequest.Name)
				}
			}

		case <-ctx.Done():
			return nil
		}

		overview := a.lastOverview
		for _, envGroup := range overview.EnvironmentGroups {
			for _, env := range envGroup.Environments {
				for _, app := range env.Applications {
					k := Key{Application: app.Name, Environment: env.Name}
					if ok := seen[k]; ok != nil {
						seen[k] = app
					}
				}
			}
		}
	}
}

func (a *mockArgoProcessor) ConsumeArgo(ctx context.Context, argo SimplifiedApplicationServiceClient, hlth *setup.HealthReporter) error {
	watch, err := argo.Watch(ctx, &application.ApplicationQuery{})
	if err != nil {
		if status.Code(err) == codes.Canceled {
			// context is cancelled -> we are shutting down
			return setup.Permanent(nil)
		}
		return fmt.Errorf("watching applications: %w", err)
	}
	hlth.ReportReady("consuming events")
	for {
		ev, err := watch.Recv()
		if err != nil {
			if status.Code(err) == codes.Canceled {
				// context is cancelled -> we are shutting down
				return setup.Permanent(nil)
			}
			return err
		}

		if ev.Type == "ADDED" || ev.Type == "MODIFIED" || ev.Type == "DELETED" {
			a.argoApps <- ev
		}
	}
}

func (m *mockApplicationServiceClient) Watch(ctx context.Context, qry *application.ApplicationQuery, opts ...grpc.CallOption) (application.ApplicationService_WatchClient, error) {
	if m.current >= len(m.Steps) {
		return nil, setup.Permanent(fmt.Errorf("exhausted: %w", io.EOF))
	}
	reply := m.Steps[m.current]
	if reply.WatchErr != nil {
		if reply.CancelContext {
			m.cancel()
		}
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
	m.lastEvent <- &ev
}

func TestArgoConection(t *testing.T) {
	tcs := []struct {
		Name          string
		Steps         []step
		Overview      *api.GetOverviewResponse
		ExpectedError string
		ExpectedReady bool
	}{
		{
			Name: "when ctx in cancelled, no app is processed",
			Steps: []step{
				{
					WatchErr:      status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			Overview: &api.GetOverviewResponse{
				Applications: map[string]*api.Application{
					"foo": {
						Releases: []*api.Release{
							{
								Version:        1,
								SourceCommitId: "00001",
							},
						},
						Team: "footeam",
					},
				},
				EnvironmentGroups: []*api.EnvironmentGroup{
					{

						EnvironmentGroupName: "staging-group",
						Environments: []*api.Environment{
							{
								Name: "staging",
								Applications: map[string]*api.Environment_Application{
									"foo": {
										Name:    "foo",
										Version: 1,
										DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
											DeployTime: "123456789",
										},
									},
								},
								Priority: api.Priority_UPSTREAM,
								Config: &api.EnvironmentConfig{
									Argocd: &api.EnvironmentConfig_ArgoCD{
										Destination: &api.EnvironmentConfig_ArgoCD_Destination{
											Name:   "staging",
											Server: "test-server",
										},
									},
								},
							},
						},
					},
				},
				GitRevision: "1234",
			},
			ExpectedReady: false,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			as := &mockApplicationServiceClient{
				Steps:     tc.Steps,
				cancel:    cancel,
				t:         t,
				lastEvent: make(chan *ArgoEvent, 10),
			}
			hlth := &setup.HealthServer{}
			argoProcessor := &mockArgoProcessor{
				lastOverview:      tc.Overview,
				ApplicationClient: as,
				HealthReporter:    hlth.Reporter("argo-testing"),
			}
			hlth.BackOffFactory = func() backoff.BackOff { return backoff.NewConstantBackOff(0) }
			go argoProcessor.Consume(ctx)
			for _, envGroup := range tc.Overview.EnvironmentGroups {
				for _, env := range envGroup.Environments {
					for _, app := range env.Applications {
						annotations := map[string]string{}
						labels := map[string]string{}

						annotations["com.freiheit.kuberpult/team"] = "testing-team"
						annotations["com.freiheit.kuberpult/application"] = app.Name
						annotations["com.freiheit.kuberpult/environment"] = env.Name
						// This annotation is so that argoCd does not invalidate *everything* in the whole repo when receiving a git webhook.
						// It has to start with a "/" to be absolute to the git repo.
						// See https://argo-cd.readthedocs.io/en/stable/operator-manual/high_availability/#webhook-and-manifest-paths-annotation
						manifestPath := "/test"
						annotations["argocd.argoproj.io/manifest-generate-paths"] = manifestPath
						labels["com.freiheit.kuberpult/team"] = "testing-team"

						deployApp := CreateDeployApplication(tc.Overview, app, env, annotations, labels, manifestPath)

						argoProcessor.Push(tc.Overview, deployApp)

					}
				}
			}
			err := argoProcessor.ConsumeArgo(ctx, as, hlth.Reporter("argo-consume"))
			if err != nil {
				t.Fatal()
			}
		})
	}
}

type ArgoEvent struct {
	Environment      string
	Application      string
	SyncStatusCode   v1alpha1.SyncStatusCode
	HealthStatusCode health.HealthStatusCode
	OperationState   *v1alpha1.OperationState
}
