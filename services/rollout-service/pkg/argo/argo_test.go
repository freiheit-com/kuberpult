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
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argorepo "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/cenkalti/backoff/v4"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
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

// These mock methods are not needed, but we still need to define them for us to be able to use this as an argo application
// client inside the ArgoAppProcessor. The interesting methods are Create, Update and Delete
func (m *mockApplicationServiceClient) List(ctx context.Context, in *application.ApplicationQuery, opts ...grpc.CallOption) (*v1alpha1.ApplicationList, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) ListResourceEvents(ctx context.Context, in *application.ApplicationResourceEventsQuery, opts ...grpc.CallOption) (*core.EventList, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) Get(ctx context.Context, in *application.ApplicationQuery, opts ...grpc.CallOption) (*v1alpha1.Application, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) GetApplicationSyncWindows(ctx context.Context, in *application.ApplicationSyncWindowsQuery, opts ...grpc.CallOption) (*application.ApplicationSyncWindowsResponse, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) RevisionMetadata(ctx context.Context, in *application.RevisionMetadataQuery, opts ...grpc.CallOption) (*v1alpha1.RevisionMetadata, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) RevisionChartDetails(ctx context.Context, in *application.RevisionMetadataQuery, opts ...grpc.CallOption) (*v1alpha1.ChartDetails, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) GetManifests(ctx context.Context, in *application.ApplicationManifestQuery, opts ...grpc.CallOption) (*argorepo.ManifestResponse, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) GetManifestsWithFiles(ctx context.Context, opts ...grpc.CallOption) (application.ApplicationService_GetManifestsWithFilesClient, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) UpdateSpec(ctx context.Context, in *application.ApplicationUpdateSpecRequest, opts ...grpc.CallOption) (*v1alpha1.ApplicationSpec, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) Patch(ctx context.Context, in *application.ApplicationPatchRequest, opts ...grpc.CallOption) (*v1alpha1.Application, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) Sync(ctx context.Context, in *application.ApplicationSyncRequest, opts ...grpc.CallOption) (*v1alpha1.Application, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) ManagedResources(ctx context.Context, in *application.ResourcesQuery, opts ...grpc.CallOption) (*application.ManagedResourcesResponse, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) ResourceTree(ctx context.Context, in *application.ResourcesQuery, opts ...grpc.CallOption) (*v1alpha1.ApplicationTree, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) WatchResourceTree(ctx context.Context, in *application.ResourcesQuery, opts ...grpc.CallOption) (application.ApplicationService_WatchResourceTreeClient, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) Rollback(ctx context.Context, in *application.ApplicationRollbackRequest, opts ...grpc.CallOption) (*v1alpha1.Application, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) TerminateOperation(ctx context.Context, in *application.OperationTerminateRequest, opts ...grpc.CallOption) (*application.OperationTerminateResponse, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) GetResource(ctx context.Context, in *application.ApplicationResourceRequest, opts ...grpc.CallOption) (*application.ApplicationResourceResponse, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) PatchResource(ctx context.Context, in *application.ApplicationResourcePatchRequest, opts ...grpc.CallOption) (*application.ApplicationResourceResponse, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) ListResourceActions(ctx context.Context, in *application.ApplicationResourceRequest, opts ...grpc.CallOption) (*application.ResourceActionsListResponse, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) RunResourceAction(ctx context.Context, in *application.ResourceActionRunRequest, opts ...grpc.CallOption) (*application.ApplicationResponse, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) DeleteResource(ctx context.Context, in *application.ApplicationResourceDeleteRequest, opts ...grpc.CallOption) (*application.ApplicationResponse, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) PodLogs(ctx context.Context, in *application.ApplicationPodLogsQuery, opts ...grpc.CallOption) (application.ApplicationService_PodLogsClient, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) ListLinks(ctx context.Context, in *application.ListAppLinksRequest, opts ...grpc.CallOption) (*application.LinksResponse, error) {
	panic("implement me")
}

func (m *mockApplicationServiceClient) ListResourceLinks(ctx context.Context, in *application.ApplicationResourceRequest, opts ...grpc.CallOption) (*application.LinksResponse, error) {
	panic("implement me")
}

type ArgoApp struct {
	App       *v1alpha1.Application
	LastEvent string
}

// Simulates receiving events from ARGO. Sends those to the  argoAppsChannel from the argo app processor.
func ConsumeArgo(ctx context.Context, hlth *setup.HealthReporter, appClient application.ApplicationServiceClient, argoAppsChannel chan *v1alpha1.ApplicationWatchEvent) error {
	for {
		watch, err := appClient.Watch(ctx, &application.ApplicationQuery{})
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
			argoAppsChannel <- ev
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

func (m *mockApplicationServiceClient) Delete(ctx context.Context, req *application.ApplicationDeleteRequest, opts ...grpc.CallOption) (*application.ApplicationResponse, error) {
	for _, app := range m.Apps {
		if app.App.Name == *req.Name {
			deleteApp := &ArgoApp{App: app.App, LastEvent: "DELETED"}
			m.Apps = append(m.Apps, deleteApp)
			return nil, nil
		}
	}
	return nil, nil
}

func (m *mockApplicationServiceClient) Update(ctx context.Context, req *application.ApplicationUpdateRequest, opts ...grpc.CallOption) (*v1alpha1.Application, error) {
	for _, a := range m.Apps {
		if a.App.Name == req.Application.Name {
			updateApp := &ArgoApp{App: a.App, LastEvent: "MODIFIED"}
			m.Apps = append(m.Apps, updateApp)
			break
		}
	}

	return nil, nil
	// If reached here, no application in the request is known to argo
}

func (m *mockApplicationServiceClient) Create(ctx context.Context, req *application.ApplicationCreateRequest, opts ...grpc.CallOption) (*v1alpha1.Application, error) {
	newApp := &ArgoApp{
		App:       req.Application,
		LastEvent: "ADDED",
	}
	for _, existingArgoApp := range m.Apps {
		if existingArgoApp.App.Name == req.Application.Name {
			// App alrady exists
			return nil, nil
		}
	}
	m.Apps = append(m.Apps, newApp)
	return nil, nil
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

func (m *mockApplicationServiceClient) checkApplications(t *testing.T, expectedApps []ArgoAppMetadata) {
	if len(m.Apps) != len(expectedApps) {
		t.Errorf("mismatch on number of applications, want %d got %d", len(expectedApps), len(m.Apps))
	}
	for idx, app := range m.Apps {
		currAppMetadata := ArgoAppToMetaData(app)
		if diff := cmp.Diff(expectedApps[idx], currAppMetadata); diff != "" {
			t.Errorf("argo app mismatch (-want, +got):\n%s", diff)
		}
	}
}

func ArgoAppToMetaData(app *ArgoApp) ArgoAppMetadata {
	return ArgoAppMetadata{
		Name:              app.App.Annotations["com.freiheit.kuberpult/application"],
		Environment:       app.App.Annotations["com.freiheit.kuberpult/environment"],
		ParentEnvironment: app.App.Annotations["com.freiheit.kuberpult/aa-parent-environment"],
		Event:             app.LastEvent,
		ManifestPath:      app.App.Annotations["argocd.argoproj.io/manifest-generate-paths"],
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
					EnvironmentGroups: []*api.EnvironmentGroup{
						{

							EnvironmentGroupName: "staging-group",
							Environments: []*api.Environment{
								{
									Name:     "staging",
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
					EnvironmentGroups: []*api.EnvironmentGroup{
						{

							EnvironmentGroupName: "staging-group",
							Environments: []*api.Environment{
								{
									Name:     "staging",
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
					EnvironmentGroups: []*api.EnvironmentGroup{
						{

							EnvironmentGroupName: "staging-group",
							Environments: []*api.Environment{
								{
									Name:     "staging",
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
					EnvironmentGroups: []*api.EnvironmentGroup{
						{

							EnvironmentGroupName: "staging-group",
							Environments: []*api.Environment{
								{
									Name:     "staging",
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
					EnvironmentGroups: []*api.EnvironmentGroup{
						{

							EnvironmentGroupName: "staging-group",
							Environments: []*api.Environment{
								{
									Name:     "staging",
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
					EnvironmentGroups: []*api.EnvironmentGroup{
						{

							EnvironmentGroupName: "staging-group",
							Environments: []*api.Environment{
								{
									Name:     "staging",
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
					EnvironmentGroups: []*api.EnvironmentGroup{
						{

							EnvironmentGroupName: "staging-group",
							Environments: []*api.Environment{
								{
									Name:     "staging",
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
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			as := &mockApplicationServiceClient{
				Steps:     tc.Steps,
				cancel:    cancel,
				t:         t,
				lastEvent: make(chan *ArgoEvent, 10),
			}
			hlth := &setup.HealthServer{}
			argoProcessor := &ArgoAppProcessor{
				lastOverview:          tc.ArgoOverview,
				ApplicationClient:     as,
				trigger:               make(chan *ArgoOverview, 10),
				ArgoApps:              make(chan *v1alpha1.ApplicationWatchEvent, 10),
				ManageArgoAppsEnabled: true,
				ManageArgoAppsFilter:  []string{"*"},
				KnownApps:             map[string]map[string]*v1alpha1.Application{},
			}

			hlth.BackOffFactory = func() backoff.BackOff { return backoff.NewConstantBackOff(0) }
			errCh := make(chan error)
			go func() {
				errCh <- argoProcessor.Consume(ctx, hlth.Reporter("consume"))
			}()

			go func() {
				errCh <- ConsumeArgo(ctx, hlth.Reporter("consume-argo"), as, argoProcessor.ArgoApps)
			}()

			err := argoProcessor.Push(ctx, tc.ArgoOverview)
			if err != nil {
				t.Fatal(err)
			}

			err = <-errCh

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
		Environment       *api.Environment
		AppsKnownToArgo   map[string]*v1alpha1.Application
		ArgoManageEnabled bool
		ArgoManageFilter  []string
		ExpectedOutput    bool
		ExpectedError     string
		Application       *api.OverviewApplication
	}{
		{
			Name: "when filter has `*` and a team name",
			Application: &api.OverviewApplication{
				Name: "foo",
				Team: "footeam",
			},
			Overview: &ArgoOverview{
				Overview: &api.GetOverviewResponse{
					LightweightApps: []*api.OverviewApplication{
						{
							Name: "foo",
							Team: "footeam",
						},
					},
					EnvironmentGroups: []*api.EnvironmentGroup{
						{
							EnvironmentGroupName: "staging-group",
							Environments: []*api.Environment{
								{
									Name:     "staging",
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
			Environment: &api.Environment{
				Name:     "development",
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
					EnvironmentGroups: []*api.EnvironmentGroup{
						{

							EnvironmentGroupName: "staging-group",
							Environments: []*api.Environment{
								{
									Name: "staging",
									//Applications: map[string]*api.Environment_Application{},
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
			Environment: &api.Environment{
				Name:     "development",
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
		t.Run(tc.Name, func(t *testing.T) {
			_, cancel := context.WithCancel(context.Background())
			as := &mockApplicationServiceClient{
				cancel:    cancel,
				t:         t,
				lastEvent: make(chan *ArgoEvent, 10),
			}
			hlth := &setup.HealthServer{}
			argoProcessor := &ArgoAppProcessor{
				lastOverview:          tc.Overview,
				ApplicationClient:     as,
				trigger:               make(chan *ArgoOverview, 10),
				ArgoApps:              make(chan *v1alpha1.ApplicationWatchEvent, 10),
				ManageArgoAppsEnabled: tc.ArgoManageEnabled,
				ManageArgoAppsFilter:  tc.ArgoManageFilter,
				KnownApps:             map[string]map[string]*v1alpha1.Application{},
			}
			hlth.BackOffFactory = func() backoff.BackOff { return backoff.NewConstantBackOff(0) }

			isActive, err := IsSelfManagedFilterActive(tc.Application.Name, argoProcessor)
			if tc.ExpectedError != "" {
				if err.Error() != tc.ExpectedError {
					t.Fatalf("expected error to be %s but got %s", tc.ExpectedError, err.Error())
				}
			}
			if isActive != tc.ExpectedOutput {
				t.Fatalf("expected processor to have done %v operations but it did %v", tc.ExpectedOutput, len(argoProcessor.ArgoApps))
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

type ArgoAppMetadata struct {
	Name              string
	Environment       string
	ParentEnvironment string
	Event             string
	ManifestPath      string
}

// Receiving information kuberpult applications triggers changes in
func TestReactToKuberpultEvents(t *testing.T) {
	tcs := []struct {
		Name             string
		KnowArgoApps     []ArgoAppMetadata
		ExpectedArgoApps []ArgoAppMetadata
		ArgoOverview     []*ArgoOverview
	}{
		{
			Name:         "create an app",
			KnowArgoApps: []ArgoAppMetadata{},
			ArgoOverview: []*ArgoOverview{
				{
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
						EnvironmentGroups: []*api.EnvironmentGroup{
							{

								EnvironmentGroupName: "staging-group",
								Environments: []*api.Environment{
									{
										Name:     "staging",
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
			},
			ExpectedArgoApps: []ArgoAppMetadata{
				{
					Name:              "foo",
					Environment:       "staging",
					ParentEnvironment: "staging",
					Event:             "ADDED",
					ManifestPath:      "/environments/staging/applications/foo/manifests",
				},
			},
		},
		{
			Name: "create an app for an AA environment",
			ArgoOverview: []*ArgoOverview{
				{
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
						EnvironmentGroups: []*api.EnvironmentGroup{
							{

								EnvironmentGroupName: "staging-group",
								Environments: []*api.Environment{
									{
										Name:     "staging",
										Priority: api.Priority_UPSTREAM,
										Config: &api.EnvironmentConfig{
											ArgoConfigs: &api.EnvironmentConfig_ArgoConfigs{
												CommonEnvPrefix: "test",
												Configs: []*api.EnvironmentConfig_ArgoCD{
													{
														ConcreteEnvName: "de-1",
														Destination: &api.EnvironmentConfig_ArgoCD_Destination{
															Name:   "staging",
															Server: "test-server",
														},
													},
													{
														ConcreteEnvName: "de-2",
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
							},
						},
						GitRevision: "1234",
					},
				},
			},
			//Expected annotation information of created apps
			ExpectedArgoApps: []ArgoAppMetadata{
				{
					Name:              "foo",
					Environment:       "test-staging-de-1",
					ParentEnvironment: "staging",
					Event:             "ADDED",
					ManifestPath:      "/environments/staging/applications/foo/manifests",
				},
				{
					Name:              "foo",
					Environment:       "test-staging-de-2",
					ParentEnvironment: "staging",
					Event:             "ADDED",
					ManifestPath:      "/environments/staging/applications/foo/manifests",
				},
			},
		},
		{
			Name: "updates an already existing app on AA env",
			KnowArgoApps: []ArgoAppMetadata{
				{
					Name:              "foo",
					Environment:       "test-staging-de-1",
					ParentEnvironment: "staging",
					ManifestPath:      "/environments/staging/applications/foo/manifests",
				},
				{
					Name:              "foo",
					Environment:       "test-staging-de-2",
					ParentEnvironment: "staging",
					ManifestPath:      "/environments/staging/applications/foo/manifests",
				},
			},
			ArgoOverview: []*ArgoOverview{
				{
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
						EnvironmentGroups: []*api.EnvironmentGroup{
							{
								EnvironmentGroupName: "staging-group",
								Environments: []*api.Environment{
									{
										Name:     "staging",
										Priority: api.Priority_UPSTREAM,
										Config: &api.EnvironmentConfig{
											ArgoConfigs: &api.EnvironmentConfig_ArgoConfigs{
												CommonEnvPrefix: "test",
												Configs: []*api.EnvironmentConfig_ArgoCD{
													{
														ConcreteEnvName: "de-1",
														Destination: &api.EnvironmentConfig_ArgoCD_Destination{
															Name:   "staging",
															Server: "test-server",
														},
													},
													{
														ConcreteEnvName: "de-2",
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
							},
						},
						GitRevision: "1234",
					},
				},
			},
			ExpectedArgoApps: []ArgoAppMetadata{
				{
					Name:              "foo",
					Environment:       "test-staging-de-1",
					ParentEnvironment: "staging",
					Event:             "ADDED",
					ManifestPath:      "/environments/staging/applications/foo/manifests",
				},
				{
					Name:              "foo",
					Environment:       "test-staging-de-2",
					ParentEnvironment: "staging",
					Event:             "ADDED",
					ManifestPath:      "/environments/staging/applications/foo/manifests",
				},
				{
					Name:              "foo",
					Environment:       "test-staging-de-1",
					ParentEnvironment: "staging",
					Event:             "MODIFIED",
					ManifestPath:      "/environments/staging/applications/foo/manifests",
				},
				{
					Name:              "foo",
					Environment:       "test-staging-de-2",
					ParentEnvironment: "staging",
					Event:             "MODIFIED",
					ManifestPath:      "/environments/staging/applications/foo/manifests",
				},
			},
		},
		{
			Name: "delete gets trigger for all apps on AA env",
			KnowArgoApps: []ArgoAppMetadata{
				{
					Name:              "foo",
					Environment:       "test-staging-de-1",
					ParentEnvironment: "staging",
					ManifestPath:      "/environments/staging/applications/foo/manifests",
				},
				{
					Name:              "foo",
					Environment:       "test-staging-de-2",
					ParentEnvironment: "staging",
					ManifestPath:      "/environments/staging/applications/foo/manifests",
				},
				{
					Name:              "foo2",
					Environment:       "test-staging-de-1",
					ParentEnvironment: "staging",
					ManifestPath:      "/environments/staging/applications/foo2/manifests",
				},
				{
					Name:              "foo2",
					Environment:       "test-staging-de-2",
					ParentEnvironment: "staging",
					ManifestPath:      "/environments/staging/applications/foo2/manifests",
				},
			},
			ArgoOverview: []*ArgoOverview{
				{
					AppDetails: map[string]*api.GetAppDetailsResponse{
						"foo": {
							Application: &api.Application{
								Team: "footeam",
								Releases: []*api.Release{
									{
										Version:        1,
										SourceCommitId: "00012",
									},
								},
							},
							Deployments: map[string]*api.Deployment{},
						},
					},
					Overview: &api.GetOverviewResponse{
						EnvironmentGroups: []*api.EnvironmentGroup{
							{
								EnvironmentGroupName: "staging-group",
								Environments: []*api.Environment{
									{
										Name:     "staging",
										Priority: api.Priority_UPSTREAM,
										Config: &api.EnvironmentConfig{
											ArgoConfigs: &api.EnvironmentConfig_ArgoConfigs{
												CommonEnvPrefix: "test",
												Configs: []*api.EnvironmentConfig_ArgoCD{
													{
														ConcreteEnvName: "de-1",
														Destination: &api.EnvironmentConfig_ArgoCD_Destination{
															Name:   "staging",
															Server: "test-server",
														},
													},
													{
														ConcreteEnvName: "de-2",
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
							},
						},
						GitRevision: "1234",
					},
				},
			},
			ExpectedArgoApps: []ArgoAppMetadata{
				{
					Name:              "foo",
					Environment:       "test-staging-de-1",
					ParentEnvironment: "staging",
					Event:             "ADDED",
					ManifestPath:      "/environments/staging/applications/foo/manifests",
				},
				{
					Name:              "foo",
					Environment:       "test-staging-de-2",
					ParentEnvironment: "staging",
					Event:             "ADDED",
					ManifestPath:      "/environments/staging/applications/foo/manifests",
				},
				{
					Name:              "foo2",
					Environment:       "test-staging-de-1",
					ParentEnvironment: "staging",
					Event:             "ADDED",
					ManifestPath:      "/environments/staging/applications/foo2/manifests",
				},
				{
					Name:              "foo2",
					Environment:       "test-staging-de-2",
					ParentEnvironment: "staging",
					Event:             "ADDED",
					ManifestPath:      "/environments/staging/applications/foo2/manifests",
				},
				{
					Name:              "foo",
					Environment:       "test-staging-de-1",
					ParentEnvironment: "staging",
					Event:             "DELETED",
					ManifestPath:      "/environments/staging/applications/foo/manifests",
				},
				{
					Name:              "foo",
					Environment:       "test-staging-de-2",
					ParentEnvironment: "staging",
					Event:             "DELETED",
					ManifestPath:      "/environments/staging/applications/foo/manifests",
				},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			as := &mockApplicationServiceClient{
				Steps: []step{
					{
						RecvErr:       status.Error(codes.Canceled, "context cancelled"),
						CancelContext: true,
					},
				},
				cancel:    cancel,
				t:         t,
				lastEvent: make(chan *ArgoEvent, 10),
			}
			hlth := &setup.HealthServer{}
			argoProcessor := &ArgoAppProcessor{
				lastOverview:          tc.ArgoOverview[0],
				ApplicationClient:     as,
				trigger:               make(chan *ArgoOverview, 10),
				ArgoApps:              make(chan *v1alpha1.ApplicationWatchEvent, 10),
				ManageArgoAppsEnabled: true,
				ManageArgoAppsFilter:  []string{"*"},
				KnownApps:             map[string]map[string]*v1alpha1.Application{},
			}
			argoProcessor.PopulateAppsToKnownApps(tc.KnowArgoApps)
			as.PopulateApps(tc.KnowArgoApps)
			hlth.BackOffFactory = func() backoff.BackOff { return backoff.NewConstantBackOff(0) }
			errCh := make(chan error)
			go func() {
				errCh <- ConsumeArgo(ctx, hlth.Reporter("consume-argo"), as, argoProcessor.ArgoApps)
			}()

			go func() {
				errCh <- argoProcessor.Consume(ctx, hlth.Reporter("consume"))
			}()

			for _, ov := range tc.ArgoOverview {
				err := argoProcessor.Push(ctx, ov)
				if err != nil {
					t.Fatalf("unexpected error on Push: %v", err)
				}
			}
			err1 := <-errCh
			if err1 != nil {
				t.Fatalf("unexpected error on channel: %v", err1)
			}
			err2 := <-errCh
			if err2 != nil {
				t.Fatalf("unexpected error on channel: %v", err2)
			}

			as.checkApplications(t, tc.ExpectedArgoApps)
		})
	}
}

func (a *ArgoAppProcessor) PopulateAppsToKnownApps(appInfo []ArgoAppMetadata) {
	for _, currentAppInfo := range appInfo {
		if a.KnownApps[currentAppInfo.Environment] == nil {
			a.KnownApps[currentAppInfo.Environment] = map[string]*v1alpha1.Application{}
		}
		a.KnownApps[currentAppInfo.Environment][currentAppInfo.Name] = &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name: currentAppInfo.Environment + "-" + currentAppInfo.Name,
				Annotations: map[string]string{
					"com.freiheit.kuberpult/application": currentAppInfo.Name,
					"com.freiheit.kuberpult/environment": currentAppInfo.Environment,
				},
			},
		}
	}
}

func (a *mockApplicationServiceClient) PopulateApps(appInfo []ArgoAppMetadata) {
	for _, currentAppInfo := range appInfo {
		a.Apps = append(a.Apps, &ArgoApp{
			App: &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name: currentAppInfo.Environment + "-" + currentAppInfo.Name,
					Annotations: map[string]string{
						"com.freiheit.kuberpult/application":           currentAppInfo.Name,
						"com.freiheit.kuberpult/environment":           currentAppInfo.Environment,
						"com.freiheit.kuberpult/aa-parent-environment": currentAppInfo.ParentEnvironment,
						"argocd.argoproj.io/manifest-generate-paths":   currentAppInfo.ManifestPath,
					},
				},
			},
			LastEvent: "ADDED",
		})
	}
}
