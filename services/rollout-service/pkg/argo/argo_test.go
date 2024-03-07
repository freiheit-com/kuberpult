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

package argo

import (
	"context"
	"fmt"
	"io"
	"k8s.io/apimachinery/pkg/watch"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/cenkalti/backoff/v4"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/ptr"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
	trigger           chan *api.GetOverviewResponse
	lastOverview      *api.GetOverviewResponse
	argoApps          chan *v1alpha1.ApplicationWatchEvent
	ApplicationClient *mockApplicationServiceClient
	HealthReporter    *setup.HealthReporter
}

func (a *mockArgoProcessor) Push(ctx context.Context, last *api.GetOverviewResponse) {
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

func (a *mockArgoProcessor) Consume(t *testing.T, ctx context.Context, expectedTypes []string, triggerError bool) error {
	appsKnownToArgo := map[string]map[string]*v1alpha1.Application{}
	for {
		select {
		case overview := <-a.trigger:
			for _, envGroup := range overview.EnvironmentGroups {
				for _, env := range envGroup.Environments {
					envAppsKnownToArgo := appsKnownToArgo[env.Name]
					err := a.DeleteArgoApps(ctx, envAppsKnownToArgo, env.Applications)

					if err != nil {
						continue
					}

					for _, app := range env.Applications {
						a.CreateOrUpdateApp(ctx, overview, app, env, envAppsKnownToArgo, triggerError)
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

func (m *mockApplicationServiceClient) UpdateSpec(ctx context.Context, req *application.ApplicationUpdateSpecRequest) {
	for _, a := range m.Apps {
		if a.App.Name == *req.Name {
			updateApp := &ArgoApp{App: a.App, LastEvent: "MODIFIED"}
			m.Apps = append(m.Apps, updateApp)
			break
		}
	}
}

func (m *mockApplicationServiceClient) Create(ctx context.Context, req *application.ApplicationCreateRequest, triggerError bool) error {
	newApp := &ArgoApp{
		App:       req.Application,
		LastEvent: "ADDED",
	}
	for _, existingArgoApp := range m.Apps {
		if existingArgoApp.App.Name == req.Application.Name {
			if !triggerError {
				return status.Error(codes.InvalidArgument, "application already exists")
			} else {
				return nil
			}
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
		Overview              *api.GetOverviewResponse
		ExpectedError         string
		ExpectedReady         bool
		ExpectedConsumed      int
		ExpectedConsumedTypes []string
		TriggerError          bool
	}{
		{
			Name: "when ctx in cancelled no app is processed",
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
			ExpectedReady:    false,
			ExpectedError:    "watching applications: no",
			ExpectedConsumed: 1,
		},
		{
			Name: "create an app and update it",
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
			ExpectedReady:         true,
			ExpectedConsumedTypes: []string{"ADDED", "MODIFIED"},
			TriggerError:          false,
			ExpectedConsumed:      2,
		},
		{
			Name: "create an app and try to create it again",
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
			ExpectedReady:         true,
			ExpectedConsumed:      1,
			ExpectedConsumedTypes: []string{"ADDED"},
			TriggerError:          true,
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
				lastOverview:      tc.Overview,
				ApplicationClient: as,
				trigger:           make(chan *api.GetOverviewResponse, 10),
				argoApps:          make(chan *v1alpha1.ApplicationWatchEvent, 10),
			}
			hlth.BackOffFactory = func() backoff.BackOff { return backoff.NewConstantBackOff(0) }
			errCh := make(chan error)
			go func() {
				errCh <- argoProcessor.Consume(t, ctx, tc.ExpectedConsumedTypes, tc.TriggerError)
			}()

			go func() {
				errCh <- argoProcessor.ConsumeArgo(ctx, hlth.Reporter("consume-argo"))
			}()

			argoProcessor.Push(ctx, tc.Overview)
			//We add a delay so that all the events are reported by the application client
			time.Sleep(10 * time.Second)
			err := <-errCh

			if err != nil {
				if tc.ExpectedError == "" || tc.ExpectedError != err.Error() {
					t.Errorf("expected no error, but got %q", err)
				}
			}
			as.testAllConsumed(t, tc.ExpectedConsumed)
		})
	}
}

func (a mockArgoProcessor) DeleteArgoApps(ctx context.Context, appsKnownToArgo map[string]*v1alpha1.Application, apps map[string]*api.Environment_Application) error {
	toDelete := make([]*v1alpha1.Application, 0)
	for _, argoApp := range appsKnownToArgo {
		for i, app := range apps {
			if argoApp.Name == fmt.Sprintf("%s-%s", i, app.Name) {
				break
			}
		}
		toDelete = append(toDelete, argoApp)
	}

	for i := range toDelete {
		a.ApplicationClient.Delete(ctx, &application.ApplicationDeleteRequest{
			Name: ptr.FromString(toDelete[i].Name),
		})

	}

	return nil
}

func (a mockArgoProcessor) CreateOrUpdateApp(ctx context.Context, overview *api.GetOverviewResponse, app *api.Environment_Application, env *api.Environment, appsKnownToArgo map[string]*v1alpha1.Application, triggerError bool) {
	k := Key{AppName: app.Name, EnvName: env.Name, Application: app, Environment: env}

	appExists := false

	for _, argoApp := range appsKnownToArgo {
		if argoApp.Name == env.Name && argoApp.Annotations["com.freiheit.kuberpult/application"] != "" {
			appExists = true
			break
		}
	}

	if !appExists {
		appToCreate := CreateArgoApplication(overview, app, k.Environment)
		appToCreate.ResourceVersion = ""
		upsert := false
		validate := false
		appCreateRequest := &application.ApplicationCreateRequest{
			Application: appToCreate,
			Upsert:      &upsert,
			Validate:    &validate,
		}
		err := a.ApplicationClient.Create(ctx, appCreateRequest, triggerError)
		if err != nil {
			// We check if the application was created in the meantime
			if status.Code(err) != codes.InvalidArgument {
				logger.FromContext(ctx).Error("creating application: %w")
			}
		}
	} else {
		appToUpdate := CreateArgoApplication(overview, app, k.Environment)
		appUpdateRequest := &application.ApplicationUpdateSpecRequest{
			Name:         &appToUpdate.Name,
			Spec:         &appToUpdate.Spec,
			AppNamespace: &appToUpdate.Namespace,
		}
		a.ApplicationClient.UpdateSpec(ctx, appUpdateRequest)
	}
}

type ArgoEvent struct {
	Environment      string
	Application      string
	SyncStatusCode   v1alpha1.SyncStatusCode
	HealthStatusCode health.HealthStatusCode
	OperationState   *v1alpha1.OperationState
}
