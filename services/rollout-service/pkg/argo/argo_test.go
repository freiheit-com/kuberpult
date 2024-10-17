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

package argo

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"io"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/watch"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/cenkalti/backoff/v4"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/conversion"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Used to compare two error message strings, needed because errors.Is(fmt.Errorf(text),fmt.Errorf(text)) == false
type errMatcher struct {
	msg string
}

func (e errMatcher) Error() string {
	return e.msg
}

func (e errMatcher) Is(err error) bool {
	return e.Error() == err.Error()
}

type step struct {
	Event         *v1alpha1.ApplicationWatchEvent
	WatchErr      error
	RecvErr       error
	CancelContext bool
}

func (m *mockApplicationServiceClient) Recv() (*v1alpha1.ApplicationWatchEvent, error) {
	if m.current >= len(m.Steps) {
		return nil, fmt.Errorf("exhausted: %w", io.EOF)
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
	Apps      []*ArgoApp
	current   int
	t         *testing.T
	lastEvent chan *ArgoEvent
	cancel    context.CancelFunc
	grpc.ClientStream
}

type ArgoApp struct {
	App       *v1alpha1.Application
	LastEvent string
}

type mockArgoProcessor struct {
	trigger               chan *ArgoOverview
	lastOverview          *ArgoOverview
	argoApps              chan *v1alpha1.ApplicationWatchEvent
	ApplicationClient     *mockApplicationServiceClient
	HealthReporter        *setup.HealthReporter
	ManageArgoAppsEnabled bool
	ManageArgoAppsFilter  []string
}

func (a *mockArgoProcessor) Push(ctx context.Context, last *ArgoOverview) {
	a.lastOverview = last
	select {
	case a.trigger <- a.lastOverview:
	default:
	}
}

func (a *mockArgoProcessor) checkEvent(ev *v1alpha1.ApplicationWatchEvent) bool {
	for _, argoApp := range a.ApplicationClient.Apps {
		if argoApp.App.Name == ev.Application.Name && string(ev.Type) == argoApp.LastEvent {
			return true
		}
	}
	return false
}

func (a *mockArgoProcessor) Consume(ctx context.Context, hlth *setup.HealthReporter) error {
	return nil
}

func (a *mockArgoProcessor) TestConsume(t *testing.T, ctx context.Context, expectedTypes []string, existingArgoApps bool, syncDisable bool) error {
	appsKnownToArgo := map[string]map[string]*v1alpha1.Application{}
	envAppsKnownToArgo := make(map[string]*v1alpha1.Application)

	for {
		select {
		case argoOverview := <-a.trigger:
			overview := argoOverview.Overview
			for currentApp, currentAppDetails := range argoOverview.AppDetails {
				for _, envGroup := range overview.EnvironmentGroups {
					for _, env := range envGroup.Environments {
						if ok := appsKnownToArgo[env.Name]; ok != nil {
							envAppsKnownToArgo = appsKnownToArgo[env.Name]
							a.DeleteArgoApps(ctx, envAppsKnownToArgo, currentApp, currentAppDetails.Deployments[env.Name])
						}
						if existingArgoApps {
							argoApp := CreateArgoApplication(overview, currentApp, currentAppDetails.Application.Team, env)
							if syncDisable {
								argoApp.Spec.SyncPolicy = &v1alpha1.SyncPolicy{
									Automated: &v1alpha1.SyncPolicyAutomated{
										Prune:      false,
										SelfHeal:   false,
										AllowEmpty: false,
									},
								}
							}
							envAppsKnownToArgo[currentApp] = argoApp

							appsKnownToArgo[env.Name] = envAppsKnownToArgo
						}
						a.CreateOrUpdateApp(ctx, overview, currentApp, currentAppDetails.Application.Team, env, envAppsKnownToArgo)
					}
				}

			}
		case ev := <-a.argoApps:
			switch ev.Type {
			case "ADDED", "MODIFIED", "DELETED":
				if ev.Type != watch.EventType(expectedTypes[0]) {
					t.Fatalf("expected type to be %s, but got %s", expectedTypes[0], ev.Type)
				}
				if len(expectedTypes) > 1 {
					expectedTypes = expectedTypes[1:]
				}
			}

		case <-ctx.Done():
			return nil
		}
	}
}

func (a *mockArgoProcessor) ConsumeArgo(ctx context.Context, hlth *setup.HealthReporter) error {
	for {
		watch, err := a.ApplicationClient.Watch(ctx, &application.ApplicationQuery{})
		if err != nil {
			if status.Code(err) == codes.Canceled {
				// context is cancelled -> we are shutting down
				return setup.Permanent(nil)
			}
			return fmt.Errorf("watching applications: %w", err)
		}
		hlth.ReportReady("consuming events")
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

func (m *mockApplicationServiceClient) Delete(ctx context.Context, req *application.ApplicationDeleteRequest) {
	for _, app := range m.Apps {
		if app.App.Name == *req.Name {
			deleteApp := &ArgoApp{App: app.App, LastEvent: "DELETED"}
			m.Apps = append(m.Apps, deleteApp)
			return
		}
	}
}

func (m *mockApplicationServiceClient) Update(ctx context.Context, req *application.ApplicationUpdateRequest) {
	for _, a := range m.Apps {
		if a.App.Name == req.Application.Name {
			updateApp := &ArgoApp{App: a.App, LastEvent: "MODIFIED"}
			m.Apps = append(m.Apps, updateApp)
			break
		}
	}
	// If reached here, no application in the request is known to argo
}

func (m *mockApplicationServiceClient) Create(ctx context.Context, req *application.ApplicationCreateRequest) error {
	newApp := &ArgoApp{
		App:       req.Application,
		LastEvent: "ADDED",
	}
	for _, existingArgoApp := range m.Apps {
		if existingArgoApp.App.Name == req.Application.Name {
			// App alrady exists
			return nil
		}
	}
	m.Apps = append(m.Apps, newApp)

	return nil
}

func (m *mockApplicationServiceClient) testAllConsumed(t *testing.T, expectedConsumed int) {
	for _, app := range m.Apps {
		if !app.App.Spec.SyncPolicy.Automated.SelfHeal {
			t.Errorf("expected app %s to have selfHeal enabled", app.App.Name)
		}
		if !app.App.Spec.SyncPolicy.Automated.Prune {
			t.Errorf("expected app %s to have prune enabled", app.App.Name)
		}
	}
	if expectedConsumed != m.current && m.current < len(m.Steps) {
		t.Errorf("expected to consume all %d replies, only consumed %d", len(m.Steps), m.current)
	}
}

func TestArgoConsume(t *testing.T) {
	tcs := []struct {
		Name                  string
		Steps                 []step
		ExpectedError         error
		ExpectedConsumed      int
		ExpectedConsumedTypes []string
		ExistingArgoApps      bool
		SyncDisable           bool
		ArgoOverview          *ArgoOverview
	}{
		{
			Name: "when ctx in cancelled no app is processed",
			Steps: []step{
				{
					WatchErr:      status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			ArgoOverview: &ArgoOverview{
				AppDetails: map[string]*api.GetAppDetailsResponse{
					"foo": {
						Application: &api.Application{
							Team: "footeam",
							Releases: []*api.Release{
								{

									Version:        1,
									SourceCommitId: "00001",
								},
							},
						},
						Deployments: map[string]*api.Deployment{
							"staging": {
								Version: 1,
								DeploymentMetaData: &api.Deployment_DeploymentMetaData{
									DeployTime: "123456789",
								},
							},
						},
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
									Name:         "staging",
									Applications: map[string]*api.Environment_Application{},
									Priority:     api.Priority_UPSTREAM,
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
			},
		},
		{
			Name: "an error is detected",
			Steps: []step{
				{
					WatchErr: fmt.Errorf("no"),
				},
				{
					WatchErr:      status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			ArgoOverview: &ArgoOverview{
				AppDetails: map[string]*api.GetAppDetailsResponse{
					"foo": {
						Application: &api.Application{
							Team: "footeam",
							Releases: []*api.Release{
								{

									Version:        1,
									SourceCommitId: "00001",
								},
							},
						},
						Deployments: map[string]*api.Deployment{
							"staging": {
								Version: 1,
								DeploymentMetaData: &api.Deployment_DeploymentMetaData{
									DeployTime: "123456789",
								},
							},
						},
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
									Name:         "staging",
									Applications: map[string]*api.Environment_Application{},
									Priority:     api.Priority_UPSTREAM,
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
			},
			ExpectedError:    errMatcher{"watching applications: no"},
			ExpectedConsumed: 1,
		},
		{
			Name: "create an app",
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
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			ArgoOverview: &ArgoOverview{
				AppDetails: map[string]*api.GetAppDetailsResponse{
					"foo": {
						Application: &api.Application{
							Team: "footeam",
							Releases: []*api.Release{
								{

									Version:        1,
									SourceCommitId: "00001",
								},
							},
						},
						Deployments: map[string]*api.Deployment{
							"staging": {
								Version: 1,
								DeploymentMetaData: &api.Deployment_DeploymentMetaData{
									DeployTime: "123456789",
								},
							},
						},
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
									Name:         "staging",
									Applications: map[string]*api.Environment_Application{},
									Priority:     api.Priority_UPSTREAM,
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
			},

			ExpectedConsumedTypes: []string{"ADDED"},
			ExpectedConsumed:      2,
		},
		{
			Name: "updates an already existing app",
			Steps: []step{
				{
					Event: &v1alpha1.ApplicationWatchEvent{
						Type: "MODIFIED",
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
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			ArgoOverview: &ArgoOverview{
				AppDetails: map[string]*api.GetAppDetailsResponse{
					"foo": {
						Application: &api.Application{
							Team: "footeam",
							Releases: []*api.Release{
								{

									Version:        1,
									SourceCommitId: "00001",
								},
							},
						},
						Deployments: map[string]*api.Deployment{
							"staging": {
								Version: 1,
								DeploymentMetaData: &api.Deployment_DeploymentMetaData{
									DeployTime: "123456789",
								},
							},
						},
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
									Name:         "staging",
									Applications: map[string]*api.Environment_Application{},
									Priority:     api.Priority_UPSTREAM,
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
			},
			ExpectedConsumed:      1,
			ExpectedConsumedTypes: []string{"MODIFIED"},
			ExistingArgoApps:      true,
		},
		{
			Name: "two applications in the overview but only one is updated",
			Steps: []step{
				{
					Event: &v1alpha1.ApplicationWatchEvent{
						Type: "MODIFIED",
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
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			ArgoOverview: &ArgoOverview{
				AppDetails: map[string]*api.GetAppDetailsResponse{
					"foo": {
						Application: &api.Application{
							Team: "footeam",
							Releases: []*api.Release{
								{

									Version:        1,
									SourceCommitId: "00001",
								},
							},
						},
						Deployments: map[string]*api.Deployment{
							"staging": {
								Version: 1,
								DeploymentMetaData: &api.Deployment_DeploymentMetaData{
									DeployTime: "123456789",
								},
							},
						},
					},
					"foo2": {
						Application: &api.Application{
							Team: "footeam",
							Releases: []*api.Release{
								{
									Version:        1,
									SourceCommitId: "00012",
								},
							},
						},
						Deployments: map[string]*api.Deployment{
							"staging": {
								Version: 1,
								DeploymentMetaData: &api.Deployment_DeploymentMetaData{
									DeployTime: "1234567892",
								},
							},
						},
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
						"foo2": {
							Releases: []*api.Release{
								{
									Version:        1,
									SourceCommitId: "00012",
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
									Name:         "staging",
									Applications: map[string]*api.Environment_Application{},
									Priority:     api.Priority_UPSTREAM,
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
			},
			ExpectedConsumed:      1,
			ExpectedConsumedTypes: []string{"MODIFIED"},
			ExistingArgoApps:      true,
		},
		{
			Name: "two applications in the overview but none is updated",
			Steps: []step{
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			ArgoOverview: &ArgoOverview{
				AppDetails: map[string]*api.GetAppDetailsResponse{
					"foo": {
						Application: &api.Application{
							Team: "footeam",
							Releases: []*api.Release{
								{

									Version:        1,
									SourceCommitId: "00001",
								},
							},
						},
						Deployments: map[string]*api.Deployment{
							"staging": {
								Version: 1,
								DeploymentMetaData: &api.Deployment_DeploymentMetaData{
									DeployTime: "123456789",
								},
							},
						},
					},
					"foo2": {
						Application: &api.Application{
							Team: "footeam",
							Releases: []*api.Release{
								{
									Version:        1,
									SourceCommitId: "00012",
								},
							},
						},
						Deployments: map[string]*api.Deployment{
							"staging": {
								Version: 1,
								DeploymentMetaData: &api.Deployment_DeploymentMetaData{
									DeployTime: "1234567892",
								},
							},
						},
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
						"foo2": {
							Releases: []*api.Release{
								{
									Version:        1,
									SourceCommitId: "00012",
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
									Name:         "staging",
									Applications: map[string]*api.Environment_Application{},
									Priority:     api.Priority_UPSTREAM,
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
			},
			ExpectedConsumed:      0,
			ExpectedConsumedTypes: []string{},
			ExistingArgoApps:      true,
		},
		{
			Name: "one application in the overview but no event is consumed",
			Steps: []step{
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			ArgoOverview: &ArgoOverview{
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
									Name:         "staging",
									Applications: map[string]*api.Environment_Application{},
									Priority:     api.Priority_UPSTREAM,
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
				AppDetails: map[string]*api.GetAppDetailsResponse{
					"foo": {
						Application: &api.Application{
							Team: "footeam",
							Releases: []*api.Release{
								{

									Version:        1,
									SourceCommitId: "00001",
								},
							},
						},
						Deployments: map[string]*api.Deployment{
							"staging": {
								Version: 1,
								DeploymentMetaData: &api.Deployment_DeploymentMetaData{
									DeployTime: "123456789",
								},
							},
						},
					},
				},
			},
			ExpectedConsumed:      0,
			ExpectedConsumedTypes: []string{},
			ExistingArgoApps:      true,
			SyncDisable:           true,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			as := &mockApplicationServiceClient{
				Steps:     tc.Steps,
				cancel:    cancel,
				t:         t,
				lastEvent: make(chan *ArgoEvent, 10),
			}
			hlth := &setup.HealthServer{}
			argoProcessor := &mockArgoProcessor{
				lastOverview:          tc.ArgoOverview,
				ApplicationClient:     as,
				trigger:               make(chan *ArgoOverview, 10),
				argoApps:              make(chan *v1alpha1.ApplicationWatchEvent, 10),
				ManageArgoAppsEnabled: true,
				ManageArgoAppsFilter:  []string{"non", "*"},
			}
			hlth.BackOffFactory = func() backoff.BackOff { return backoff.NewConstantBackOff(0) }
			errCh := make(chan error)
			go func() {
				errCh <- argoProcessor.TestConsume(t, ctx, tc.ExpectedConsumedTypes, tc.ExistingArgoApps, tc.SyncDisable)
			}()

			go func() {
				errCh <- argoProcessor.ConsumeArgo(ctx, hlth.Reporter("consume-argo"))
			}()

			argoProcessor.Push(ctx, tc.ArgoOverview)
			//We add a delay so that all the events are reported by the application client
			time.Sleep(10 * time.Second)
			err := <-errCh

			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			as.testAllConsumed(t, tc.ExpectedConsumed)
		})
	}
}

func TestCreateOrUpdateArgoApp(t *testing.T) {
	tcs := []struct {
		Name              string
		Overview          *ArgoOverview
		Application       *api.Environment_Application
		Environment       *api.Environment
		AppsKnownToArgo   map[string]*v1alpha1.Application
		ArgoManageEnabled bool
		ArgoManageFilter  []string
		ExpectedOutput    bool
		ExpectedError     string
	}{
		{
			Name: "when filter has `*` and a team name",
			Overview: &ArgoOverview{
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
			},
			Application: &api.Environment_Application{
				Name:    "foo",
				Version: 1,
				DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
					DeployTime: "123456789",
				},
			},
			Environment: &api.Environment{
				Name: "development",
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
							Name:   "development",
							Server: "test-server",
						},
					},
				},
			},
			ArgoManageEnabled: true,
			ArgoManageFilter:  []string{"*", "sreteam"},
			ExpectedOutput:    false,
			ExpectedError:     "filter can only have length of 1 when `*` is active",
		},
		{
			Name: "when filter has `*`",
			Overview: &ArgoOverview{
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
									Name:         "staging",
									Applications: map[string]*api.Environment_Application{},
									Priority:     api.Priority_UPSTREAM,
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
			},
			Application: &api.Environment_Application{
				Name:    "foo",
				Version: 1,
				DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
					DeployTime: "123456789",
				},
			},
			Environment: &api.Environment{
				Name: "development",
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
							Name:   "development",
							Server: "test-server",
						},
					},
				},
			},
			ArgoManageEnabled: true,
			ArgoManageFilter:  []string{"*"},
			ExpectedOutput:    true,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			_, cancel := context.WithCancel(context.Background())
			as := &mockApplicationServiceClient{
				cancel:    cancel,
				t:         t,
				lastEvent: make(chan *ArgoEvent, 10),
			}
			hlth := &setup.HealthServer{}
			argoProcessor := &mockArgoProcessor{
				lastOverview:          tc.Overview,
				ApplicationClient:     as,
				trigger:               make(chan *ArgoOverview, 10),
				argoApps:              make(chan *v1alpha1.ApplicationWatchEvent, 10),
				ManageArgoAppsEnabled: tc.ArgoManageEnabled,
				ManageArgoAppsFilter:  tc.ArgoManageFilter,
			}
			hlth.BackOffFactory = func() backoff.BackOff { return backoff.NewConstantBackOff(0) }

			isActive, err := IsSelfManagedFilterActive(team(tc.Overview.Overview, tc.Application.Name), argoProcessor)
			if tc.ExpectedError != "" {
				if err.Error() != tc.ExpectedError {
					t.Fatalf("expected error to be %s but got %s", tc.ExpectedError, err.Error())
				}
			}
			if isActive != tc.ExpectedOutput {
				t.Fatalf("expected processor to have done %v operations but it did %v", tc.ExpectedOutput, len(argoProcessor.argoApps))
			}
		})
	}
}

func (a *mockArgoProcessor) DeleteArgoApps(ctx context.Context, argoApps map[string]*v1alpha1.Application, appName string, deployment *api.Deployment) {
	toDelete := make([]*v1alpha1.Application, 0)
	for _, argoApp := range argoApps {
		if argoApp.Name == fmt.Sprintf("%s-%s", appName, appName) {
			break

		}
		toDelete = append(toDelete, argoApp)
	}

	for i := range toDelete {
		a.ApplicationClient.Delete(ctx, &application.ApplicationDeleteRequest{
			Name: conversion.FromString(toDelete[i].Name),
		})

	}
}

func (a *mockArgoProcessor) GetManageArgoAppsFilter() []string {
	return a.ManageArgoAppsFilter
}

func (a *mockArgoProcessor) GetManageArgoAppsEnabled() bool {
	return a.ManageArgoAppsEnabled
}

func (a *mockArgoProcessor) CreateOrUpdateApp(ctx context.Context, overview *api.GetOverviewResponse, appName, team string, env *api.Environment, appsKnownToArgo map[string]*v1alpha1.Application) {
	var existingApp *v1alpha1.Application
	selfManaged, err := IsSelfManagedFilterActive(team, a)
	if err != nil {
		logger.FromContext(ctx).Error("detecting self manage:", zap.Error(err))
	}
	if selfManaged {
		for _, argoApp := range appsKnownToArgo {
			if argoApp.Name == fmt.Sprintf("%s-%s", env.Name, appName) && argoApp.Annotations["com.freiheit.kuberpult/application"] != "" {
				existingApp = argoApp
				break
			}
		}

		if existingApp == nil {
			appToCreate := CreateArgoApplication(overview, appName, team, env)
			appToCreate.ResourceVersion = ""
			upsert := false
			validate := false
			appCreateRequest := &application.ApplicationCreateRequest{
				Application: appToCreate,
				Upsert:      &upsert,
				Validate:    &validate,
			}
			err := a.ApplicationClient.Create(ctx, appCreateRequest)
			if err != nil {
				// We check if the application was created in the meantime
				if status.Code(err) != codes.InvalidArgument {
					logger.FromContext(ctx).Error("creating application: %w")
				}
			}
		} else {
			appToUpdate := CreateArgoApplication(overview, appName, team, env)
			appUpdateRequest := &application.ApplicationUpdateRequest{
				XXX_NoUnkeyedLiteral: struct{}{},
				XXX_unrecognized:     nil,
				XXX_sizecache:        0,
				Validate:             conversion.Bool(false),
				Application:          appToUpdate,
				Project:              conversion.FromString(appToUpdate.Spec.Project),
			}
			//We have to exclude the unexported type destination and the syncPolicy
			//exhaustruct:ignore
			diff := cmp.Diff(appUpdateRequest.Application.Spec, existingApp.Spec,
				cmp.AllowUnexported(v1alpha1.ApplicationDestination{}),
				cmpopts.IgnoreTypes(v1alpha1.SyncPolicy{}))

			if diff != "" {
				a.ApplicationClient.Update(ctx, appUpdateRequest)
			}
		}
	}
}

type ArgoEvent struct {
	Environment      string
	Application      string
	SyncStatusCode   v1alpha1.SyncStatusCode
	HealthStatusCode health.HealthStatusCode
	OperationState   *v1alpha1.OperationState
}

func team(overview *api.GetOverviewResponse, app string) string {
	a := overview.Applications[app]
	if a == nil {
		return ""
	}
	return a.Team
}
