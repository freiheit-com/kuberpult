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
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argorepo "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/cenkalti/backoff/v4"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	core "k8s.io/api/core/v1"
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
	Steps             []step
	Apps              []*ArgoApp
	current           int
	t                 *testing.T
	cancel            context.CancelFunc
	lastUpdateRequest *application.ApplicationUpdateRequest
	updateRequests    []*application.ApplicationUpdateRequest
	deleteErr         error
	// updateErr makes Update fail with this error (the request is still recorded).
	updateErr error
	// createConflictOnExisting makes Create fail with InvalidArgument for apps
	// that already exist, like the real Argo CD server does when the specs differ.
	createConflictOnExisting bool
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
	if in.Name != nil {
		for i := len(m.Apps) - 1; i >= 0; i-- {
			if m.Apps[i].App.Name == *in.Name {
				if m.Apps[i].LastEvent == "DELETED" {
					break
				}
				return m.Apps[i].App, nil
			}
		}
	}
	return nil, status.Error(codes.NotFound, "app not found")
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
	NoCascade bool
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
	if m.deleteErr != nil {
		return nil, m.deleteErr
	}
	noCascade := req.Cascade != nil && !*req.Cascade
	for _, app := range m.Apps {
		if app.App.Name == *req.Name {
			deleteApp := &ArgoApp{App: app.App, LastEvent: "DELETED", NoCascade: noCascade}
			m.Apps = append(m.Apps, deleteApp)
			return nil, nil
		}
	}
	return nil, nil
}

func (m *mockApplicationServiceClient) Update(ctx context.Context, req *application.ApplicationUpdateRequest, opts ...grpc.CallOption) (*v1alpha1.Application, error) {
	m.lastUpdateRequest = req
	m.updateRequests = append(m.updateRequests, req)
	if m.updateErr != nil {
		return nil, m.updateErr
	}
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
	// Only skip creation if the app is currently active (last event was not DELETED).
	lastEvent := ""
	for _, existingArgoApp := range m.Apps {
		if existingArgoApp.App.Name == req.Application.Name {
			lastEvent = existingArgoApp.LastEvent
		}
	}
	if lastEvent == "ADDED" || lastEvent == "MODIFIED" {
		if m.createConflictOnExisting {
			return nil, status.Error(codes.InvalidArgument, "existing application spec is different, use upsert flag to force update")
		}
		return nil, nil
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

func ArgoAppToMetaData(app *ArgoApp) ArgoAppMetadata {
	return ArgoAppMetadata{
		Name:              app.App.Annotations["com.freiheit.kuberpult/application"],
		Environment:       app.App.Annotations["com.freiheit.kuberpult/environment"],
		ParentEnvironment: app.App.Annotations["com.freiheit.kuberpult/aa-parent-environment"],
		Event:             app.LastEvent,
		ManifestPath:      app.App.Annotations["argocd.argoproj.io/manifest-generate-paths"],
		NoCascade:         app.NoCascade,
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
									DeployTime: timestamppb.New(time.Unix(123456789, 0)),
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
										Argocd: &api.ArgoCDEnvironmentConfiguration{
											Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
									DeployTime: timestamppb.New(time.Unix(123456789, 0)),
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
										Argocd: &api.ArgoCDEnvironmentConfiguration{
											Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
									DeployTime: timestamppb.New(time.Unix(123456789, 0)),
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
										Argocd: &api.ArgoCDEnvironmentConfiguration{
											Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
									DeployTime: timestamppb.New(time.Unix(123456789, 0)),
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
										Argocd: &api.ArgoCDEnvironmentConfiguration{
											Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
									DeployTime: timestamppb.New(time.Unix(123456789, 0)),
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
									DeployTime: timestamppb.New(time.Unix(1234567892, 0)),
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
										Argocd: &api.ArgoCDEnvironmentConfiguration{
											Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
									DeployTime: timestamppb.New(time.Unix(123456789, 0)),
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
									DeployTime: timestamppb.New(time.Unix(1234567892, 0)),
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
										Argocd: &api.ArgoCDEnvironmentConfiguration{
											Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
										Argocd: &api.ArgoCDEnvironmentConfiguration{
											Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
									DeployTime: timestamppb.New(time.Unix(123456789, 0)),
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
				Steps:  tc.Steps,
				cancel: cancel,
				t:      t,
			}
			hlth := &setup.HealthServer{}
			argoProcessor := &ArgoAppProcessor{
				ApplicationClient: as,
				trigger:           make(chan argoTrigger, 10),
				ArgoApps:          make(chan *v1alpha1.ApplicationWatchEvent, 10),

				maxProcessedTransformerEslId: &atomic.Int64{},

				ManageArgoAppsEnabled: true,
				ManageArgoAppsFilter:  []string{"*"},
				KnownApps:             map[string]map[string]*v1alpha1.Application{},
			}

			hlth.BackOffFactory = func() backoff.BackOff { return backoff.NewConstantBackOff(0) }
			errChConsume := make(chan error)
			errChConsumeArgo := make(chan error)
			go func() {
				errChConsume <- argoProcessor.Consume(ctx, hlth.Reporter("consume"), nil)
			}()

			go func() {
				errChConsumeArgo <- ConsumeArgo(ctx, hlth.Reporter("consume-argo"), as, argoProcessor.ArgoApps)
			}()

			err := argoProcessor.Push(ctx, tc.ArgoOverview, 0)
			if err != nil {
				t.Fatalf("error running Push: %v", err)
			}

			err = <-errChConsumeArgo

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
		ApplicationName   string
	}{
		{
			Name:            "when filter has `*` and a team name",
			ApplicationName: "foo",
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
										Argocd: &api.ArgoCDEnvironmentConfiguration{
											Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
					Argocd: &api.ArgoCDEnvironmentConfiguration{
						Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
										Argocd: &api.ArgoCDEnvironmentConfiguration{
											Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
					Argocd: &api.ArgoCDEnvironmentConfiguration{
						Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
				cancel: cancel,
				t:      t,
			}
			hlth := &setup.HealthServer{}
			argoProcessor := &ArgoAppProcessor{
				ApplicationClient:     as,
				trigger:               make(chan argoTrigger, 10),
				ArgoApps:              make(chan *v1alpha1.ApplicationWatchEvent, 10),
				ManageArgoAppsEnabled: tc.ArgoManageEnabled,
				ManageArgoAppsFilter:  tc.ArgoManageFilter,
				KnownApps:             map[string]map[string]*v1alpha1.Application{},

				maxProcessedTransformerEslId: &atomic.Int64{},
			}
			hlth.BackOffFactory = func() backoff.BackOff { return backoff.NewConstantBackOff(0) }

			isActive, err := IsSelfManagedFilterActive(tc.ApplicationName, argoProcessor)
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
	IsBracket         bool
	NoCascade         bool
}

// Receiving information kuberpult applications triggers changes in
func TestReactToKuberpultEvents(t *testing.T) {
	tcs := []struct {
		Name                         string
		KnowArgoApps                 []ArgoAppMetadata
		ExpectedArgoApps             []ArgoAppMetadata
		ArgoOverview                 []*ArgoOverview
		ExperimentalBracketsClusters []string
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
										DeployTime: timestamppb.New(time.Unix(123456789, 0)),
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
											Argocd: &api.ArgoCDEnvironmentConfiguration{
												Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
										DeployTime: timestamppb.New(time.Unix(123456789, 0)),
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
												Configs: []*api.ArgoCDEnvironmentConfiguration{
													{
														ConcreteEnvName: "de-1",
														Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
															Name:   "staging",
															Server: "test-server",
														},
													},
													{
														ConcreteEnvName: "de-2",
														Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
										DeployTime: timestamppb.New(time.Unix(123456789, 0)),
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
												Configs: []*api.ArgoCDEnvironmentConfiguration{
													{
														ConcreteEnvName: "de-1",
														Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
															Name:   "staging",
															Server: "test-server",
														},
													},
													{
														ConcreteEnvName: "de-2",
														Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
												Configs: []*api.ArgoCDEnvironmentConfiguration{
													{
														ConcreteEnvName: "de-1",
														Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
															Name:   "staging",
															Server: "test-server",
														},
													},
													{
														ConcreteEnvName: "de-2",
														Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
					// rollout-service no longer cascade-deletes here — cascade=true on real
					// undeploy is the cd-service → rollout_should_undeploy_cascade path.
					Name:              "foo",
					Environment:       "test-staging-de-1",
					ParentEnvironment: "staging",
					Event:             "DELETED",
					ManifestPath:      "/environments/staging/applications/foo/manifests",
					NoCascade:         true,
				},
				{
					Name:              "foo",
					Environment:       "test-staging-de-2",
					ParentEnvironment: "staging",
					Event:             "DELETED",
					ManifestPath:      "/environments/staging/applications/foo/manifests",
					NoCascade:         true,
				},
			},
		},
		{
			Name:         "works with empty argo configs",
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
										SourceCommitId: "00012",
									},
								},
							},
							Deployments: map[string]*api.Deployment{
								"staging": &api.Deployment{},
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
												Configs:         []*api.ArgoCDEnvironmentConfiguration{},
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
			ExpectedArgoApps: []ArgoAppMetadata{},
		},
		{
			// Regression test: when staging is bracket-enabled and an app uses the default
			// single-app bracket naming (bracketName == appName), the development ArgoCD
			// app must NOT be deleted.
			Name: "single-app bracket: staging bracket does not delete development app",
			KnowArgoApps: []ArgoAppMetadata{
				{
					Name:              "myapp",
					Environment:       "development",
					ParentEnvironment: "development",
					Event:             "ADDED",
					ManifestPath:      "/environments/development/applications/myapp/manifests",
				},
			},
			ExperimentalBracketsClusters: []string{"staging"},
			ArgoOverview: []*ArgoOverview{
				{
					AppDetails: map[string]*api.GetAppDetailsResponse{
						"myapp": {
							Application: &api.Application{
								Name:        "myapp",
								ArgoBracket: "myapp", // single-app bracket: bracket name == app name
								Team:        "myteam",
							},
							// Merged state produced by addBracketToChange when key already exists:
							// development has a real deployment, staging has the bracket marker.
							Deployments: map[string]*api.Deployment{
								"development": {
									Version: 1,
									DeploymentMetaData: &api.Deployment_DeploymentMetaData{
										DeployTime: timestamppb.New(time.Unix(123456789, 0)),
									},
								},
								"staging": {}, //exhaustruct:ignore
							},
						},
					},
					Overview: &api.GetOverviewResponse{
						EnvironmentGroups: []*api.EnvironmentGroup{
							{
								EnvironmentGroupName: "development-group",
								Environments: []*api.Environment{
									{
										Name:     "development",
										Priority: api.Priority_UPSTREAM,
										Config: &api.EnvironmentConfig{
											Argocd: &api.ArgoCDEnvironmentConfiguration{
												Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
													Name:   "development",
													Server: "test-server",
												},
											},
										},
									},
								},
							},
							{
								EnvironmentGroupName: "staging-group",
								Environments: []*api.Environment{
									{
										Name:     "staging",
										Priority: api.Priority_UPSTREAM,
										Config: &api.EnvironmentConfig{
											Argocd: &api.ArgoCDEnvironmentConfiguration{
												Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
				// Pre-existing development app (from PopulateApps).
				{
					Name:              "myapp",
					Environment:       "development",
					ParentEnvironment: "development",
					Event:             "ADDED",
					ManifestPath:      "/environments/development/applications/myapp/manifests",
				},
				// Development app is UPDATED (not deleted) with the regular manifest path.
				{
					Name:              "myapp",
					Environment:       "development",
					ParentEnvironment: "development",
					Event:             "MODIFIED",
					ManifestPath:      "/environments/development/applications/myapp/manifests",
				},
				// Staging bracket app is CREATED with the bracket manifest path.
				{
					Name:              "myapp",
					Environment:       "staging",
					ParentEnvironment: "staging",
					Event:             "ADDED",
					ManifestPath:      "/environments/staging/brackets/myapp",
				},
			},
		},
		{
			// When staging is switched back from true→false and no deployment data has
			// arrived yet, the bracket app must not be touched (race window).
			Name: "bracket rollback race window: bracket app not deleted when deployment is nil",
			KnowArgoApps: []ArgoAppMetadata{
				{
					Name:              "myapp",
					Environment:       "staging",
					ParentEnvironment: "staging",
					Event:             "ADDED",
					ManifestPath:      "/environments/staging/brackets/myapp",
					IsBracket:         true,
				},
			},
			ExperimentalBracketsClusters: []string{},
			ArgoOverview: []*ArgoOverview{
				{
					AppDetails: map[string]*api.GetAppDetailsResponse{
						"myapp": {
							Application: &api.Application{
								Name: "myapp",
								Team: "myteam",
							},
							Deployments: map[string]*api.Deployment{
								"staging": nil,
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
											Argocd: &api.ArgoCDEnvironmentConfiguration{
												Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
				// Bracket app is untouched.
				{Name: "myapp", Environment: "staging", ParentEnvironment: "staging", Event: "ADDED", ManifestPath: "/environments/staging/brackets/myapp"},
			},
		},
		{
			// When staging is switched back from true→false and deployment data is present,
			// the bracket app must be deleted without cascade so k8s resources persist,
			// and the individual app must be created in the same cycle.
			Name: "bracket rollback race resolved: bracket app deleted without cascade when deployment exists",
			KnowArgoApps: []ArgoAppMetadata{
				{
					Name:              "myapp",
					Environment:       "staging",
					ParentEnvironment: "staging",
					Event:             "ADDED",
					ManifestPath:      "/environments/staging/brackets/myapp",
					IsBracket:         true,
				},
			},
			ExperimentalBracketsClusters: []string{},
			ArgoOverview: []*ArgoOverview{
				{
					AppDetails: map[string]*api.GetAppDetailsResponse{
						"myapp": {
							Application: &api.Application{
								Name: "myapp",
								Team: "myteam",
							},
							Deployments: map[string]*api.Deployment{
								"staging": {
									Version: 1,
									DeploymentMetaData: &api.Deployment_DeploymentMetaData{
										DeployTime: timestamppb.New(time.Unix(123456789, 0)),
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
											Argocd: &api.ArgoCDEnvironmentConfiguration{
												Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
				// Bracket app present initially.
				{Name: "myapp", Environment: "staging", ParentEnvironment: "staging", Event: "ADDED", ManifestPath: "/environments/staging/brackets/myapp"},
				// Bracket app deleted without cascade (no k8s resource disruption).
				{Name: "myapp", Environment: "staging", ParentEnvironment: "staging", Event: "DELETED", ManifestPath: "/environments/staging/brackets/myapp", NoCascade: true},
				// Individual app created in the same cycle.
				{Name: "myapp", Environment: "staging", ParentEnvironment: "staging", Event: "ADDED", ManifestPath: "/environments/staging/applications/myapp/manifests"},
			},
		},
		{
			// When a bracket becomes fully empty (no member has a deployment in this env), the
			// rollout-service does NOT delete it from here — that would race ahead of the
			// ESL-gated cascade=true delete via rollout_should_undeploy_cascade and orphan
			// the workload. The cascade table is the sole authority for removing an empty
			// bracket together with its workload. (See argo.go package comment, # Implementation
			// Details, "per-env bracket existence" rule.)
			Name: "empty bracket: rollout-service does NOT delete it — cascade table is the authority",
			KnowArgoApps: []ArgoAppMetadata{
				{
					Name:              "bracket-one",
					Environment:       "staging",
					ParentEnvironment: "staging",
					Event:             "ADDED",
					ManifestPath:      "/environments/staging/brackets/bracket-one",
					IsBracket:         true,
				},
			},
			ExperimentalBracketsClusters: []string{"staging"},
			ArgoOverview: []*ArgoOverview{
				{
					AppDetails: map[string]*api.GetAppDetailsResponse{
						"bracket-one": {
							Application: &api.Application{
								Name:        "bracket-one",
								ArgoBracket: "bracket-one",
							},
							// Empty deployments map: bracket has no apps left.
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
											Argocd: &api.ArgoCDEnvironmentConfiguration{
												Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
				// Bracket app present, NOT deleted by ProcessArgoOverview. The cascade-table
				// consumer (undeploy package) is what will eventually cascade=true delete it.
				{Name: "bracket-one", Environment: "staging", ParentEnvironment: "staging", Event: "ADDED", ManifestPath: "/environments/staging/brackets/bracket-one"},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			const MAX = 100
			for i := 0; i < MAX; i++ {
				testutil.WrapTestRoutine(t, context.Background(), "INFO", func(ctx context.Context) {
					t.Logf("------- Test Start")
					ctx, cancel := context.WithCancel(ctx)
					mockClient := &mockApplicationServiceClient{
						Steps: []step{
							{
								RecvErr:       status.Error(codes.Canceled, "context cancelled"),
								CancelContext: true,
							},
						},
						cancel: cancel,
						t:      t,
					}
					hlth := &setup.HealthServer{}
					argoProcessor := &ArgoAppProcessor{
						ApplicationClient:            mockClient,
						trigger:                      make(chan argoTrigger, 10),
						ArgoApps:                     make(chan *v1alpha1.ApplicationWatchEvent, 10),
						ManageArgoAppsEnabled:        true,
						ManageArgoAppsFilter:         []string{"*"},
						KnownApps:                    map[string]map[string]*v1alpha1.Application{},
						ExperimentalBracketsClusters: tc.ExperimentalBracketsClusters,

						maxProcessedTransformerEslId: &atomic.Int64{},
					}
					argoProcessor.PopulateAppsToKnownApps(tc.KnowArgoApps)
					mockClient.PopulateApps(tc.KnowArgoApps)
					hlth.BackOffFactory = func() backoff.BackOff { return backoff.NewConstantBackOff(0) }
					errChConsumeArgo := make(chan error)
					errChConsume := make(chan error)

					for _, ov := range tc.ArgoOverview {
						err := argoProcessor.Push(ctx, ov, 0)
						if err != nil {
							t.Fatalf("unexpected error on Push: %v", err)
						}
					}

					var wg sync.WaitGroup
					wg.Add(1)
					var tmp = make(chan struct{})
					var waitChannel = &tmp
					go func() {
						defer wg.Done()
						errChConsume <- argoProcessor.Consume(ctx, hlth.Reporter("consume"), waitChannel)
					}()

					wg.Add(1)
					go func() {
						defer wg.Done()

						// we can only start ConsumeArgo once Consume is already running
						<-tmp

						t.Logf("ConsumeArgo start")
						errChConsumeArgo <- ConsumeArgo(ctx, hlth.Reporter("consume-argo"), mockClient, argoProcessor.ArgoApps)
						t.Logf("ConsumeArgo end")
					}()

					err2 := <-errChConsume
					if err2 != nil {
						t.Fatalf("unexpected error on channel: %v", err2)
					}

					err1 := <-errChConsumeArgo
					if err1 != nil {
						t.Fatalf("unexpected error on channel: %v", err1)
					}

					wg.Wait() // this ensures that we have no confusing test logs when running this multiple times.

					if len(mockClient.Apps) != len(tc.ExpectedArgoApps) {
						t.Fatalf("mismatch on number of applications, want %d got %d\n%v",
							len(tc.ExpectedArgoApps),
							len(mockClient.Apps),
							mockClient.Apps,
						)
					}
					for idx, app := range mockClient.Apps {
						currAppMetadata := ArgoAppToMetaData(app)
						if diff := cmp.Diff(tc.ExpectedArgoApps[idx], currAppMetadata); diff != "" {
							t.Errorf("argo app mismatch (-want, +got):\n%s", diff)
						}
					}
				})
			}
		})
	}
}

func (a *ArgoAppProcessor) PopulateAppsToKnownApps(appInfo []ArgoAppMetadata) {
	for _, currentAppInfo := range appInfo {
		if a.KnownApps[currentAppInfo.Environment] == nil {
			a.KnownApps[currentAppInfo.Environment] = map[string]*v1alpha1.Application{}
		}
		annotations := map[string]string{
			"com.freiheit.kuberpult/application": currentAppInfo.Name,
			"com.freiheit.kuberpult/environment": currentAppInfo.Environment,
		}
		if currentAppInfo.IsBracket {
			annotations["com.freiheit.kuberpult/is-bracket"] = "true"
		}
		a.KnownApps[currentAppInfo.Environment][currentAppInfo.Name] = &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:        currentAppInfo.Environment + "-" + currentAppInfo.Name,
				Annotations: annotations,
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

func TestUpdateArgoAppPreservesSyncPolicy(t *testing.T) {
	tcs := []struct {
		Name               string
		ExistingSyncPolicy *v1alpha1.SyncPolicy
	}{
		{
			Name:               "preserves nil SyncPolicy when operator removed auto-sync",
			ExistingSyncPolicy: nil,
		},
		{
			Name: "preserves custom SyncPolicy set by operator",
			ExistingSyncPolicy: &v1alpha1.SyncPolicy{
				Automated: &v1alpha1.SyncPolicyAutomated{
					Prune:    false,
					SelfHeal: false,
				},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			testutil.WrapTestRoutine(t, context.Background(), "INFO", func(ctx context.Context) {
				mockClient := &mockApplicationServiceClient{
					t:      t,
					cancel: func() {},
				}
				argoProcessor := &ArgoAppProcessor{
					ApplicationClient:     mockClient,
					ManageArgoAppsEnabled: true,
					ManageArgoAppsFilter:  []string{"*"},
					KnownApps:             map[string]map[string]*v1alpha1.Application{},

					maxProcessedTransformerEslId: &atomic.Int64{},
				}

				overview := &api.GetOverviewResponse{
					ManifestRepoUrl: "https://git.example.com/repo",
					Branch:          "new-branch",
				}
				appInfo := &AppInfo{
					ApplicationName:       "myapp",
					TeamName:              "myteam",
					EnvironmentName:       "staging",
					ParentEnvironmentName: "staging",
					ArgoEnvironmentConfiguration: &api.ArgoCDEnvironmentConfiguration{
						Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
							Server: "https://kubernetes.default.svc",
						},
					},
				}
				// existingApp has a different TargetRevision to trigger an update,
				// and whatever SyncPolicy the operator set.
				//exhaustruct:ignore
				existingApp := &v1alpha1.Application{
					//exhaustruct:ignore
					Spec: v1alpha1.ApplicationSpec{
						//exhaustruct:ignore
						Source: &v1alpha1.ApplicationSource{
							TargetRevision: "old-branch",
						},
						SyncPolicy: tc.ExistingSyncPolicy,
					},
				}

				argoProcessor.UpdateArgoApp(ctx, overview, appInfo, existingApp)

				if mockClient.lastUpdateRequest == nil {
					t.Fatal("expected Update to be called but it was not")
				}
				if diff := cmp.Diff(tc.ExistingSyncPolicy, mockClient.lastUpdateRequest.Application.Spec.SyncPolicy); diff != "" {
					t.Errorf("SyncPolicy mismatch (-want, +got):\n%s", diff)
				}
			})
		})
	}
}

func TestCreateArgoApplicationAllowEmpty(t *testing.T) {
	tcs := []struct {
		Name           string
		IsBracket      bool
		WantAllowEmpty bool
	}{
		{
			// Brackets must not auto-prune to empty: that prune-to-empty is what
			// deletes a moved-away bracket's workload and causes downtime.
			Name:           "bracket app disables allow-empty",
			IsBracket:      true,
			WantAllowEmpty: false,
		},
		{
			Name:           "non-bracket app keeps allow-empty",
			IsBracket:      false,
			WantAllowEmpty: true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			appInfo := &AppInfo{
				ApplicationName:       "myapp",
				TeamName:              "myteam",
				EnvironmentName:       "staging",
				ParentEnvironmentName: "staging",
				IsBracket:             tc.IsBracket,
				ArgoEnvironmentConfiguration: &api.ArgoCDEnvironmentConfiguration{
					Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
						Server: "https://kubernetes.default.svc",
					},
				},
			}
			overview := &api.GetOverviewResponse{
				ManifestRepoUrl: "https://git.example.com/repo",
				Branch:          "main",
			}

			app := CreateArgoApplication(overview, appInfo)

			if app.Spec.SyncPolicy == nil || app.Spec.SyncPolicy.Automated == nil {
				t.Fatalf("expected an automated sync policy")
			}
			if got := app.Spec.SyncPolicy.Automated.AllowEmpty; got != tc.WantAllowEmpty {
				t.Errorf("AllowEmpty = %v, want %v", got, tc.WantAllowEmpty)
			}
			// Prune stays true regardless: within a non-empty app/bracket, Argo still
			// reconciles removed resources.
			if !app.Spec.SyncPolicy.Automated.Prune {
				t.Errorf("Prune = false, want true")
			}
		})
	}
}

func TestCreateArgoApplicationBracketPath(t *testing.T) {
	tcs := []struct {
		Name                 string
		IsBracket            bool
		BracketSnapshotEslId db.TransformerID
		WantPath             string
	}{
		{
			Name:                 "non-bracket app keeps its applications/<app>/manifests path regardless of esl_id",
			IsBracket:            false,
			BracketSnapshotEslId: 42,
			WantPath:             "environments/staging/applications/myapp/manifests",
		},
		{
			Name:                 "bracket with zero esl_id emits legacy path (no @ suffix)",
			IsBracket:            true,
			BracketSnapshotEslId: 0,
			WantPath:             "environments/staging/brackets/myapp",
		},
		{
			Name:                 "bracket with esl_id embeds @<esl_id> for the reposerver",
			IsBracket:            true,
			BracketSnapshotEslId: 42,
			WantPath:             "environments/staging/brackets/myapp@42",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			appInfo := &AppInfo{
				ApplicationName:       "myapp",
				TeamName:              "myteam",
				EnvironmentName:       "staging",
				ParentEnvironmentName: "staging",
				IsBracket:             tc.IsBracket,
				BracketSnapshotEslId:  tc.BracketSnapshotEslId,
				ArgoEnvironmentConfiguration: &api.ArgoCDEnvironmentConfiguration{
					Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
						Server: "https://kubernetes.default.svc",
					},
				},
			}
			overview := &api.GetOverviewResponse{
				ManifestRepoUrl: "https://git.example.com/repo",
				Branch:          "main",
			}

			app := CreateArgoApplication(overview, appInfo)

			if app.Spec.Source == nil {
				t.Fatalf("expected non-nil Source")
			}
			if got := app.Spec.Source.Path; got != tc.WantPath {
				t.Errorf("Spec.Source.Path = %q, want %q", got, tc.WantPath)
			}
			// The manifest-generate-paths annotation must mirror the source path (with leading /).
			wantAnnotation := "/" + tc.WantPath
			if got := app.Annotations["argocd.argoproj.io/manifest-generate-paths"]; got != wantAnnotation {
				t.Errorf("manifest-generate-paths annotation = %q, want %q", got, wantAnnotation)
			}
		})
	}
}

func TestDrainPendingDeletionsRetryOnError(t *testing.T) {
	tcs := []struct {
		Name              string
		DeleteErr         error
		ExpectedRemaining int
	}{
		{
			Name:              "successful delete removes item from pending",
			DeleteErr:         nil,
			ExpectedRemaining: 0,
		},
		{
			Name:              "failed delete keeps item in pending for retry",
			DeleteErr:         fmt.Errorf("argocd rejected delete"),
			ExpectedRemaining: 1,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			appName := "my-app"
			argoAppName := "staging-1-my-app"
			envName := "staging-1"
			parentEnvName := "staging"

			mockClient := &mockApplicationServiceClient{
				deleteErr: tc.DeleteErr,
			}
			argoProcessor := &ArgoAppProcessor{
				ApplicationClient: mockClient,
				KnownApps: map[string]map[string]*v1alpha1.Application{
					parentEnvName: {
						"bracket-app": {
							//exhaustruct:ignore
							ObjectMeta: metav1.ObjectMeta{
								Name:        "staging-bracket",
								Annotations: map[string]string{"com.freiheit.kuberpult/is-bracket": "true"},
							},
						},
					},
					envName: {
						appName: {
							//exhaustruct:ignore
							ObjectMeta: metav1.ObjectMeta{
								Name: argoAppName,
							},
						},
					},
				},
				pendingDeletions: []PendingDeletion{
					{
						EnvironmentName:       envName,
						ParentEnvironmentName: parentEnvName,
						AppName:               appName,
					},
				},

				maxProcessedTransformerEslId: &atomic.Int64{},
			}

			argoProcessor.drainPendingDeletions(ctx, parentEnvName)

			if got := len(argoProcessor.pendingDeletions); got != tc.ExpectedRemaining {
				t.Errorf("pendingDeletions length: want %d, got %d", tc.ExpectedRemaining, got)
			}
		})
	}
}

// TestBracketMoveNoCascadeDelete verifies how ProcessArgoOverview handles an empty bracket:
//   - move case (another bracket has the deployment): ProcessAppChange deletes the old
//     bracket with NoCascade=true so the new bracket adopts the workload.
//   - undeploy case (no replacement): ProcessAppChange does NOT delete the bracket; the
//     rollout_should_undeploy_cascade table is the sole authority for cascade=true delete
//     of the bracket together with its workload. A no-cascade delete here would race ahead
//     of the ESL-gated cascade=true consumer and orphan the k8s workload.
func TestBracketMoveNoCascadeDelete(t *testing.T) {
	tcs := []struct {
		Name string
		// bracket1 is the pre-existing bracket in KnownApps.
		// bracket2Deployment is the replacement (non-nil = move case, nil = undeploy case).
		bracket2Deployment *api.Deployment
		WantDeleted        bool // whether ProcessArgoOverview itself should issue a Delete for bracket1
		WantNoCascade      bool // only checked when WantDeleted is true
	}{
		{
			Name:               "app moves from bracket1 to bracket2: bracket1 deleted without cascade",
			bracket2Deployment: &api.Deployment{}, //exhaustruct:ignore
			WantDeleted:        true,
			WantNoCascade:      true,
		},
		{
			Name:               "app undeployed from bracket1 (no replacement): bracket1 NOT deleted by ProcessArgoOverview — cascade table is the authority",
			bracket2Deployment: nil,
			WantDeleted:        false,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()

			mockClient := &mockApplicationServiceClient{
				deleteErr: nil,
			}

			argoProcessor := &ArgoAppProcessor{
				ApplicationClient:            mockClient,
				ManageArgoAppsEnabled:        true,
				ManageArgoAppsFilter:         []string{"*"},
				KnownApps:                    map[string]map[string]*v1alpha1.Application{},
				ExperimentalBracketsClusters: []string{"staging"},
				trigger:                      make(chan argoTrigger, 10),
				ArgoApps:                     make(chan *v1alpha1.ApplicationWatchEvent, 10),
				pendingDeletions:             []PendingDeletion{},

				maxProcessedTransformerEslId: &atomic.Int64{},
			}

			// Seed KnownApps and mockClient.Apps with the pre-existing bracket1 app.
			argoProcessor.PopulateAppsToKnownApps([]ArgoAppMetadata{
				{
					Name: "bracket1", Environment: "staging", ParentEnvironment: "staging",
					Event: "ADDED", ManifestPath: "/environments/staging/brackets/bracket1",
					IsBracket: true,
				},
			})
			mockClient.PopulateApps([]ArgoAppMetadata{
				{
					Name: "bracket1", Environment: "staging", ParentEnvironment: "staging",
					Event: "ADDED", ManifestPath: "/environments/staging/brackets/bracket1",
				},
			})

			// Build AppDetails: bracket1 is empty; bracket2 has a deployment only when
			// tc.bracket2Deployment != nil (the move case).
			appDetails := map[string]*api.GetAppDetailsResponse{
				"bracket1": {
					//exhaustruct:ignore
					Application: &api.Application{Name: "bracket1", ArgoBracket: "bracket1"},
					Deployments: map[string]*api.Deployment{},
				},
			}
			if tc.bracket2Deployment != nil {
				appDetails["bracket2"] = &api.GetAppDetailsResponse{
					//exhaustruct:ignore
					Application: &api.Application{Name: "bracket2", ArgoBracket: "bracket2"},
					Deployments: map[string]*api.Deployment{"staging": tc.bracket2Deployment},
				}
			}

			argoOv := &ArgoOverview{
				AppDetails: appDetails,
				Overview: &api.GetOverviewResponse{
					EnvironmentGroups: []*api.EnvironmentGroup{
						{
							EnvironmentGroupName: "staging-group",
							Environments: []*api.Environment{
								{
									Name:     "staging",
									Priority: api.Priority_UPSTREAM,
									Config: &api.EnvironmentConfig{
										Argocd: &api.ArgoCDEnvironmentConfiguration{
											Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
			}

			l := logger.FromContext(ctx)
			argoProcessor.ProcessArgoOverview(ctx, l, argoOv)

			// Find the DELETED entry for bracket1 in mockClient.Apps.
			var bracket1Delete *ArgoApp
			for _, app := range mockClient.Apps {
				name := app.App.Annotations["com.freiheit.kuberpult/application"]
				if name == "bracket1" && app.LastEvent == "DELETED" {
					bracket1Delete = app
					break
				}
			}
			if tc.WantDeleted {
				if bracket1Delete == nil {
					t.Fatal("bracket1 was not deleted at all")
				}
				if diff := cmp.Diff(tc.WantNoCascade, bracket1Delete.NoCascade); diff != "" {
					t.Errorf("bracket1 NoCascade mismatch (-want +got):\n%s", diff)
				}
			} else if bracket1Delete != nil {
				t.Fatalf("bracket1 was deleted by ProcessArgoOverview but the cascade table should be the sole authority (NoCascade=%v)", bracket1Delete.NoCascade)
			}
		})
	}
}

// TestBracketMoveDefersLoserSpecUpdate verifies the prune-vs-adopt protection on a
// bracket move (the "pause protocol"): a bracket that lost members to another
// bracket (LostMembersTo) is first PAUSED in place (auto-sync disabled, path
// unchanged — an in-flight sync operation executes against the spec path at
// execution time, so the path must not move while one can be running), then
// RETARGETED to the new snapshot once it is provably quiet (controller reconciled
// the paused spec — refresh annotation cleared — and no operation is running),
// and auto-sync is RESUMED only once its own sync status proves the moved
// resources are no longer attributed to it (compared against the new path, no
// entry requires pruning).
func TestBracketMoveDefersLoserSpecUpdate(t *testing.T) {
	// The old pinned path of the live bracket1 app, and the path
	// CreateArgoApplication emits with no DB handler (snapshot id 0 = legacy format).
	oldPath := "environments/staging/brackets/bracket1@1"
	newPath := "environments/staging/brackets/bracket1"
	type watchEvent struct {
		RefreshPending  bool // the argocd refresh annotation is still on the app
		OpRunning       bool
		ComparedToPath  string
		RequiresPruning bool
		// WantUpdates is the total number of update requests expected after this
		// event was processed (1 = only the pause, 2 = +retarget, 3 = +resume).
		WantUpdates int
	}
	tcs := []struct {
		Name   string
		Events []watchEvent
	}{
		{
			Name: "retarget only once quiet, resume only after disown",
			Events: []watchEvent{
				// Controller has not reconciled the paused spec yet: no retarget.
				{RefreshPending: true, OpRunning: false, ComparedToPath: oldPath, RequiresPruning: false, WantUpdates: 1},
				// Pre-pause sync operation still running: no retarget.
				{RefreshPending: false, OpRunning: true, ComparedToPath: oldPath, RequiresPruning: false, WantUpdates: 1},
				// Quiet: retarget fires.
				{RefreshPending: false, OpRunning: false, ComparedToPath: oldPath, RequiresPruning: false, WantUpdates: 2},
				// Still attributed the moved resources: no resume.
				{RefreshPending: false, OpRunning: false, ComparedToPath: newPath, RequiresPruning: true, WantUpdates: 2},
				// Disowned: resume fires.
				{RefreshPending: false, OpRunning: false, ComparedToPath: newPath, RequiresPruning: false, WantUpdates: 3},
			},
		},
		{
			Name: "stale comparison against the old pinned path never resumes",
			Events: []watchEvent{
				{RefreshPending: false, OpRunning: false, ComparedToPath: oldPath, RequiresPruning: false, WantUpdates: 2},
				{RefreshPending: false, OpRunning: false, ComparedToPath: oldPath, RequiresPruning: false, WantUpdates: 2},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()

			mockClient := &mockApplicationServiceClient{
				deleteErr: nil,
			}

			argoProcessor := &ArgoAppProcessor{
				ApplicationClient:            mockClient,
				ManageArgoAppsEnabled:        true,
				ManageArgoAppsFilter:         []string{"*"},
				KnownApps:                    map[string]map[string]*v1alpha1.Application{},
				ExperimentalBracketsClusters: []string{"staging"},
				trigger:                      make(chan argoTrigger, 10),
				ArgoApps:                     make(chan *v1alpha1.ApplicationWatchEvent, 10),
				pendingDeletions:             []PendingDeletion{},

				maxProcessedTransformerEslId: &atomic.Int64{},
			}

			// Seed KnownApps and mockClient.Apps with the pre-existing bracket1 app.
			argoProcessor.PopulateAppsToKnownApps([]ArgoAppMetadata{
				{
					Name: "bracket1", Environment: "staging", ParentEnvironment: "staging",
					Event: "ADDED", ManifestPath: "/environments/staging/brackets/bracket1",
					IsBracket: true,
				},
			})
			mockClient.PopulateApps([]ArgoAppMetadata{
				{
					Name: "bracket1", Environment: "staging", ParentEnvironment: "staging",
					Event: "ADDED", ManifestPath: "/environments/staging/brackets/bracket1",
				},
			})
			// The live bracket1 app pins the old snapshot path.
			//exhaustruct:ignore
			argoProcessor.KnownApps["staging"]["bracket1"].Spec.Source = &v1alpha1.ApplicationSource{Path: oldPath}

			// Both brackets are live: app moved from bracket1 to bracket2, but bracket1
			// still has another member deployed.
			appDetails := map[string]*api.GetAppDetailsResponse{
				"bracket1": {
					//exhaustruct:ignore
					Application: &api.Application{Name: "bracket1", ArgoBracket: "bracket1"},
					Deployments: map[string]*api.Deployment{"staging": {}}, //exhaustruct:ignore
				},
				"bracket2": {
					//exhaustruct:ignore
					Application: &api.Application{Name: "bracket2", ArgoBracket: "bracket2"},
					Deployments: map[string]*api.Deployment{"staging": {}}, //exhaustruct:ignore
				},
			}

			argoOv := &ArgoOverview{
				AppDetails:    appDetails,
				LostMembersTo: map[string][]string{"bracket1": {"bracket2"}},
				Overview: &api.GetOverviewResponse{
					EnvironmentGroups: []*api.EnvironmentGroup{
						{
							EnvironmentGroupName: "staging-group",
							Environments: []*api.Environment{
								{
									Name:     "staging",
									Priority: api.Priority_UPSTREAM,
									Config: &api.EnvironmentConfig{
										Argocd: &api.ArgoCDEnvironmentConfiguration{
											Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
			}

			l := logger.FromContext(ctx)
			argoProcessor.ProcessArgoOverview(ctx, l, argoOv)

			// PAUSE: exactly one update — bracket1 with auto-sync disabled, path
			// UNCHANGED (an in-flight sync op executes against the live spec path),
			// the paused marker, and a refresh request so the quiet check can prove
			// the controller reconciled the paused spec.
			if got := len(mockClient.updateRequests); got != 1 {
				t.Fatalf("got %d update requests in the move tick, want exactly 1 (pause)", got)
			}
			paused := mockClient.updateRequests[0].Application
			if paused.Name != "staging-bracket1" {
				t.Fatalf("pause update targeted %q, want %q", paused.Name, "staging-bracket1")
			}
			if got := paused.Spec.Source.Path; got != oldPath {
				t.Errorf("pause update path = %q, want unchanged old path %q", got, oldPath)
			}
			if paused.Spec.SyncPolicy != nil && paused.Spec.SyncPolicy.Automated != nil {
				t.Errorf("pause update must disable auto-sync (Automated=nil), got %+v", paused.Spec.SyncPolicy)
			}
			if got := paused.Annotations[BracketPausedForMoveAnnotation]; got != BracketMovePhasePaused {
				t.Errorf("pause update annotation %q = %q, want %q", BracketPausedForMoveAnnotation, got, BracketMovePhasePaused)
			}
			if got := paused.Annotations[argocdRefreshAnnotation]; got == "" {
				t.Errorf("pause update must request a refresh via the %q annotation", argocdRefreshAnnotation)
			}

			for i, ev := range tc.Events {
				marker := BracketMovePhasePaused
				if len(mockClient.updateRequests) >= 2 {
					marker = BracketMovePhaseRetargeted
				}
				annotations := map[string]string{
					"com.freiheit.kuberpult/application": "bracket1",
					"com.freiheit.kuberpult/environment": "staging",
					"com.freiheit.kuberpult/is-bracket":  "true",
					BracketPausedForMoveAnnotation:       marker,
				}
				if ev.RefreshPending {
					annotations[argocdRefreshAnnotation] = "normal"
				}
				var opState *v1alpha1.OperationState
				if ev.OpRunning {
					//exhaustruct:ignore
					opState = &v1alpha1.OperationState{Phase: "Running"}
				}
				argoProcessor.ProcessArgoWatchEvent(ctx, l, &v1alpha1.ApplicationWatchEvent{
					Type: "MODIFIED",
					//exhaustruct:ignore
					Application: v1alpha1.Application{
						//exhaustruct:ignore
						ObjectMeta: metav1.ObjectMeta{
							Name:        "staging-bracket1",
							Annotations: annotations,
						},
						//exhaustruct:ignore
						Spec: v1alpha1.ApplicationSpec{
							//exhaustruct:ignore
							Source: &v1alpha1.ApplicationSource{Path: oldPath},
						},
						//exhaustruct:ignore
						Status: v1alpha1.ApplicationStatus{
							OperationState: opState,
							//exhaustruct:ignore
							Sync: v1alpha1.SyncStatus{
								Status: v1alpha1.SyncStatusCodeSynced,
								//exhaustruct:ignore
								ComparedTo: v1alpha1.ComparedTo{
									//exhaustruct:ignore
									Source: v1alpha1.ApplicationSource{Path: ev.ComparedToPath},
								},
							},
							Resources: []v1alpha1.ResourceStatus{
								//exhaustruct:ignore
								{Kind: "Deployment", Name: "app1-dep", RequiresPruning: ev.RequiresPruning},
							},
						},
					},
				})
				if got := len(mockClient.updateRequests); got != ev.WantUpdates {
					t.Fatalf("after event %d: got %d update requests, want %d", i+1, got, ev.WantUpdates)
				}
			}

			if len(mockClient.updateRequests) >= 2 {
				// RETARGET: new path, still paused, marker advanced.
				retargeted := mockClient.updateRequests[1].Application
				if got := retargeted.Spec.Source.Path; got != newPath {
					t.Errorf("retarget update path = %q, want %q", got, newPath)
				}
				if retargeted.Spec.SyncPolicy == nil || retargeted.Spec.SyncPolicy.Automated != nil {
					t.Errorf("retarget update must keep auto-sync disabled, got %+v", retargeted.Spec.SyncPolicy)
				}
				if got := retargeted.Annotations[BracketPausedForMoveAnnotation]; got != BracketMovePhaseRetargeted {
					t.Errorf("retarget update annotation = %q, want %q", got, BracketMovePhaseRetargeted)
				}
			}
			if len(mockClient.updateRequests) >= 3 {
				// RESUME: policy restored, marker removed.
				resumed := mockClient.updateRequests[2].Application
				if resumed.Spec.SyncPolicy == nil {
					t.Fatal("resume update has no SyncPolicy")
				}
				wantPolicy := &v1alpha1.SyncPolicyAutomated{Prune: true, SelfHeal: true, AllowEmpty: false}
				if diff := cmp.Diff(wantPolicy, resumed.Spec.SyncPolicy.Automated); diff != "" {
					t.Errorf("resume Automated policy mismatch (-want, +got):\n%s", diff)
				}
				if _, ok := resumed.Annotations[BracketPausedForMoveAnnotation]; ok {
					t.Errorf("resume update must remove the %q annotation", BracketPausedForMoveAnnotation)
				}
			}
		})
	}
}

// TestBracketMovePauseRecoveryAfterRestart verifies that a rollout-service restart
// does not strand a paused bracket: the watch ADDED replay re-enters the disown
// wait for any bracket app carrying the paused-for-move annotation.
func TestBracketMovePauseRecoveryAfterRestart(t *testing.T) {
	pausedPath := "environments/staging/brackets/bracket1"
	tcs := []struct {
		Name            string
		Marker          string
		RequiresPruning bool
		WantResume      bool
	}{
		{
			Name:            "retargeted, still attributed the moved resources: stays paused",
			Marker:          BracketMovePhaseRetargeted,
			RequiresPruning: true,
			WantResume:      false,
		},
		{
			Name:            "retargeted and already disowned: resumes immediately",
			Marker:          BracketMovePhaseRetargeted,
			RequiresPruning: false,
			WantResume:      true,
		},
		{
			Name: "paused but not yet retargeted: stays paused until the next tick",
			// The retarget payload was lost with the restart; only a new overview
			// tick can rebuild it, so recovery must not resume here.
			Marker:          BracketMovePhasePaused,
			RequiresPruning: false,
			WantResume:      false,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()

			mockClient := &mockApplicationServiceClient{
				deleteErr: nil,
			}

			argoProcessor := &ArgoAppProcessor{
				ApplicationClient:            mockClient,
				ManageArgoAppsEnabled:        true,
				ManageArgoAppsFilter:         []string{"*"},
				KnownApps:                    map[string]map[string]*v1alpha1.Application{},
				ExperimentalBracketsClusters: []string{"staging"},
				trigger:                      make(chan argoTrigger, 10),
				ArgoApps:                     make(chan *v1alpha1.ApplicationWatchEvent, 10),
				pendingDeletions:             []PendingDeletion{},

				maxProcessedTransformerEslId: &atomic.Int64{},
			}

			l := logger.FromContext(ctx)
			argoProcessor.ProcessArgoWatchEvent(ctx, l, &v1alpha1.ApplicationWatchEvent{
				Type: "ADDED",
				//exhaustruct:ignore
				Application: v1alpha1.Application{
					//exhaustruct:ignore
					ObjectMeta: metav1.ObjectMeta{
						Name: "staging-bracket1",
						Annotations: map[string]string{
							"com.freiheit.kuberpult/application": "bracket1",
							"com.freiheit.kuberpult/environment": "staging",
							"com.freiheit.kuberpult/is-bracket":  "true",
							BracketPausedForMoveAnnotation:       tc.Marker,
						},
					},
					//exhaustruct:ignore
					Spec: v1alpha1.ApplicationSpec{
						//exhaustruct:ignore
						Source: &v1alpha1.ApplicationSource{Path: pausedPath},
					},
					//exhaustruct:ignore
					Status: v1alpha1.ApplicationStatus{
						//exhaustruct:ignore
						Sync: v1alpha1.SyncStatus{
							Status: v1alpha1.SyncStatusCodeSynced,
							//exhaustruct:ignore
							ComparedTo: v1alpha1.ComparedTo{
								//exhaustruct:ignore
								Source: v1alpha1.ApplicationSource{Path: pausedPath},
							},
						},
						Resources: []v1alpha1.ResourceStatus{
							//exhaustruct:ignore
							{Kind: "Deployment", Name: "app1-dep", RequiresPruning: tc.RequiresPruning},
						},
					},
				},
			})

			wantUpdates := 0
			if tc.WantResume {
				wantUpdates = 1
			}
			if got := len(mockClient.updateRequests); got != wantUpdates {
				t.Fatalf("got %d update requests after recovery event, want %d", got, wantUpdates)
			}
			if tc.WantResume {
				resumed := mockClient.updateRequests[0].Application
				if resumed.Spec.SyncPolicy == nil || resumed.Spec.SyncPolicy.Automated == nil || !resumed.Spec.SyncPolicy.Automated.Prune {
					t.Errorf("resume update must restore the Automated sync policy, got %+v", resumed.Spec.SyncPolicy)
				}
				if _, ok := resumed.Annotations[BracketPausedForMoveAnnotation]; ok {
					t.Errorf("resume update must remove the %q annotation", BracketPausedForMoveAnnotation)
				}
			}
		})
	}
}

// TestCreateConflictFallsBackToUpdate verifies that a spec change is not lost
// when the Argo app already exists but its watch event has not arrived yet
// (KnownApps lag): ProcessAppChange then takes the create path and Argo rejects
// the create with InvalidArgument ("existing application spec is different").
// The rollout-service must fall back to updating the app with its desired spec —
// except when the existing app is paused for a bracket move, in which case the
// pause protocol takes over via a recovered pending entry.
func TestCreateConflictFallsBackToUpdate(t *testing.T) {
	tcs := []struct {
		Name             string
		ExistingPaused   bool
		WantUpdate       bool
		WantPendingEntry bool
	}{
		{
			Name:             "existing app: create conflict falls back to update",
			ExistingPaused:   false,
			WantUpdate:       true,
			WantPendingEntry: false,
		},
		{
			Name:             "existing paused app: pause protocol recovered instead",
			ExistingPaused:   true,
			WantUpdate:       false,
			WantPendingEntry: true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()

			mockClient := &mockApplicationServiceClient{
				deleteErr:                nil,
				createConflictOnExisting: true,
			}

			argoProcessor := &ArgoAppProcessor{
				ApplicationClient:            mockClient,
				ManageArgoAppsEnabled:        true,
				ManageArgoAppsFilter:         []string{"*"},
				KnownApps:                    map[string]map[string]*v1alpha1.Application{},
				ExperimentalBracketsClusters: []string{"staging"},
				trigger:                      make(chan argoTrigger, 10),
				ArgoApps:                     make(chan *v1alpha1.ApplicationWatchEvent, 10),
				pendingDeletions:             []PendingDeletion{},

				maxProcessedTransformerEslId: &atomic.Int64{},
			}

			// The app exists in Argo CD, but its watch event has not arrived yet:
			// it is in mockClient.Apps but NOT in KnownApps.
			mockClient.PopulateApps([]ArgoAppMetadata{
				{
					Name: "bracket1", Environment: "staging", ParentEnvironment: "staging",
					Event: "ADDED", ManifestPath: "/environments/staging/brackets/bracket1",
				},
			})
			if tc.ExistingPaused {
				existing := mockClient.Apps[0].App
				existing.Annotations[BracketPausedForMoveAnnotation] = BracketMovePhaseRetargeted
				//exhaustruct:ignore
				existing.Spec.Source = &v1alpha1.ApplicationSource{Path: "environments/staging/brackets/bracket1"}
			}

			appDetails := map[string]*api.GetAppDetailsResponse{
				"bracket1": {
					//exhaustruct:ignore
					Application: &api.Application{Name: "bracket1", ArgoBracket: "bracket1"},
					Deployments: map[string]*api.Deployment{"staging": {}}, //exhaustruct:ignore
				},
			}

			argoOv := &ArgoOverview{
				AppDetails:    appDetails,
				LostMembersTo: nil,
				Overview: &api.GetOverviewResponse{
					EnvironmentGroups: []*api.EnvironmentGroup{
						{
							EnvironmentGroupName: "staging-group",
							Environments: []*api.Environment{
								{
									Name:     "staging",
									Priority: api.Priority_UPSTREAM,
									Config: &api.EnvironmentConfig{
										Argocd: &api.ArgoCDEnvironmentConfiguration{
											Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
			}

			l := logger.FromContext(ctx)
			argoProcessor.ProcessArgoOverview(ctx, l, argoOv)

			wantUpdates := 0
			if tc.WantUpdate {
				wantUpdates = 1
			}
			if got := len(mockClient.updateRequests); got != wantUpdates {
				t.Fatalf("got %d update requests, want %d", got, wantUpdates)
			}
			if tc.WantUpdate {
				if got := mockClient.updateRequests[0].Application.Name; got != "staging-bracket1" {
					t.Errorf("conflict fallback updated %q, want %q", got, "staging-bracket1")
				}
			}
			wantPending := 0
			if tc.WantPendingEntry {
				wantPending = 1
			}
			if got := len(argoProcessor.pendingSpecUpdates); got != wantPending {
				t.Errorf("got %d pending spec updates, want %d", got, wantPending)
			}
		})
	}
}

// TestVanishedAppFallsBackToCreate verifies that a spec change is not lost when
// the Argo app was deleted but its DELETED watch event has not been processed
// yet (stale KnownApps; e.g. the event is queued behind a burst of overview
// ticks). The update then fails with NotFound/PermissionDenied and the app must
// be recreated: in the paused-retargeted state when it is going through the
// pause protocol, with the normal spec otherwise.
func TestVanishedAppFallsBackToCreate(t *testing.T) {
	newPath := "environments/staging/brackets/bracket1"
	tcs := []struct {
		Name             string
		WithPendingEntry bool
		WantPausedCreate bool
	}{
		{
			Name:             "stale pause-protocol entry: recreated paused at the target path",
			WithPendingEntry: true,
			WantPausedCreate: true,
		},
		{
			Name:             "no pending entry: recreated with the normal spec",
			WithPendingEntry: false,
			WantPausedCreate: false,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()

			mockClient := &mockApplicationServiceClient{
				deleteErr: nil,
				updateErr: status.Error(codes.PermissionDenied, "permission denied"),
			}

			argoProcessor := &ArgoAppProcessor{
				ApplicationClient:            mockClient,
				ManageArgoAppsEnabled:        true,
				ManageArgoAppsFilter:         []string{"*"},
				KnownApps:                    map[string]map[string]*v1alpha1.Application{},
				ExperimentalBracketsClusters: []string{"staging"},
				trigger:                      make(chan argoTrigger, 10),
				ArgoApps:                     make(chan *v1alpha1.ApplicationWatchEvent, 10),
				pendingDeletions:             []PendingDeletion{},

				maxProcessedTransformerEslId: &atomic.Int64{},
			}

			// KnownApps still contains bracket1, but the app is gone from Argo CD
			// (mockClient.Apps is empty): the DELETED watch event is still queued.
			argoProcessor.PopulateAppsToKnownApps([]ArgoAppMetadata{
				{
					Name: "bracket1", Environment: "staging", ParentEnvironment: "staging",
					Event: "ADDED", ManifestPath: "/environments/staging/brackets/bracket1",
					IsBracket: true,
				},
			})
			if tc.WithPendingEntry {
				argoProcessor.pendingSpecUpdates = []*PendingSpecUpdate{
					{
						EnvironmentName: "staging",
						ApplicationName: "bracket1",
						ExpectedPath:    "environments/staging/brackets/bracket1@1",
						Phase:           BracketMovePhaseRetargeted,
						Retarget:        nil,
					},
				}
			}

			appDetails := map[string]*api.GetAppDetailsResponse{
				"bracket1": {
					//exhaustruct:ignore
					Application: &api.Application{Name: "bracket1", ArgoBracket: "bracket1"},
					Deployments: map[string]*api.Deployment{"staging": {}}, //exhaustruct:ignore
				},
			}

			argoOv := &ArgoOverview{
				AppDetails:    appDetails,
				LostMembersTo: nil,
				Overview: &api.GetOverviewResponse{
					EnvironmentGroups: []*api.EnvironmentGroup{
						{
							EnvironmentGroupName: "staging-group",
							Environments: []*api.Environment{
								{
									Name:     "staging",
									Priority: api.Priority_UPSTREAM,
									Config: &api.EnvironmentConfig{
										Argocd: &api.ArgoCDEnvironmentConfiguration{
											Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
			}

			l := logger.FromContext(ctx)
			argoProcessor.ProcessArgoOverview(ctx, l, argoOv)

			if got := len(mockClient.Apps); got != 1 {
				t.Fatalf("got %d apps in Argo after the tick, want 1 (recreated)", got)
			}
			created := mockClient.Apps[0]
			if created.LastEvent != "ADDED" {
				t.Fatalf("app event = %q, want ADDED", created.LastEvent)
			}
			if created.App.Name != "staging-bracket1" {
				t.Fatalf("created app %q, want %q", created.App.Name, "staging-bracket1")
			}
			if got := created.App.Spec.Source.Path; got != newPath {
				t.Errorf("created app path = %q, want %q", got, newPath)
			}
			marker := created.App.Annotations[BracketPausedForMoveAnnotation]
			if tc.WantPausedCreate {
				if marker != BracketMovePhaseRetargeted {
					t.Errorf("created app marker = %q, want %q", marker, BracketMovePhaseRetargeted)
				}
				if created.App.Spec.SyncPolicy == nil || created.App.Spec.SyncPolicy.Automated != nil {
					t.Errorf("created app must be paused (Automated=nil), got %+v", created.App.Spec.SyncPolicy)
				}
				if got := len(argoProcessor.pendingSpecUpdates); got != 1 {
					t.Errorf("got %d pending spec updates, want 1", got)
				}
			} else {
				if marker != "" {
					t.Errorf("created app must not carry the paused marker, got %q", marker)
				}
				if created.App.Spec.SyncPolicy == nil || created.App.Spec.SyncPolicy.Automated == nil || !created.App.Spec.SyncPolicy.Automated.Prune {
					t.Errorf("created app must have the normal Automated policy, got %+v", created.App.Spec.SyncPolicy)
				}
			}
		})
	}
}

func TestProcessAppChangeDeferDeletion(t *testing.T) {
	tcs := []struct {
		Name string
	}{
		{Name: "bracket not yet established: app deferred to pendingDeletions"},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			envName := "development"
			appName := "my-app"

			argoProcessor := &ArgoAppProcessor{
				ApplicationClient:            &mockApplicationServiceClient{},
				ManageArgoAppsEnabled:        false,
				KnownApps:                    map[string]map[string]*v1alpha1.Application{},
				ExperimentalBracketsClusters: []string{envName},
				pendingDeletions:             []PendingDeletion{},
				trigger:                      make(chan argoTrigger, 10),
				ArgoApps:                     make(chan *v1alpha1.ApplicationWatchEvent, 10),

				maxProcessedTransformerEslId: &atomic.Int64{},
			}
			appDetails := &api.GetAppDetailsResponse{
				//exhaustruct:ignore
				Application: &api.Application{Name: appName, Team: "test-team"},
				Deployments: map[string]*api.Deployment{},
			}
			appInfo := &AppInfo{
				ApplicationName:       appName,
				EnvironmentName:       envName,
				ParentEnvironmentName: envName,
				IsBracket:             false,
				TeamName:              "test-team",
				ArgoEnvironmentConfiguration: &api.ArgoCDEnvironmentConfiguration{
					//exhaustruct:ignore
					Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
						Name: envName,
					},
				},
			}

			argoProcessor.ProcessAppChange(ctx, appInfo, appDetails, &api.GetOverviewResponse{}, map[string]*api.GetAppDetailsResponse{})

			if got := len(argoProcessor.pendingDeletions); got != 1 {
				t.Fatalf("pendingDeletions: want 1, got %d", got)
			}
			if diff := testutil.CmpDiff(appName, argoProcessor.pendingDeletions[0].AppName); diff != "" {
				t.Errorf("pendingDeletion AppName mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestProcessArgoOverviewEmptyBracketsNotDeleted verifies the contract documented in
// the argo.go package comment (# Implementation Details): an empty bracket (no member
// has a deployment in env E, and no other bracket has it either — i.e. not a move) is
// NOT deleted by ProcessAppChange. Removing a bracket together with its workload is
// the exclusive job of the rollout_should_undeploy_cascade table (cascade=true). A
// no-cascade delete here would race ahead of the ESL-gated cascade=true consumer and
// orphan the k8s workload (Argo cascade-delete only fires when it deletes the
// Application object itself; if we already deleted it no-cascade, the resources
// remain). The AppDetails iteration order is still deterministic (sorting.SortKeys),
// but it has no observable effect on this path.
func TestProcessArgoOverviewEmptyBracketsNotDeleted(t *testing.T) {
	tcs := []struct {
		Name             string
		AppKeys          []string // app names added to AppDetails (no deployment)
		WantDeletedOrder []string // expected argo app names DELETED by ProcessArgoOverview
	}{
		{
			Name:             "two empty brackets are NOT deleted here — cascade table is the authority",
			AppKeys:          []string{"bracket-z", "bracket-a"},
			WantDeletedOrder: nil,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()

			mockClient := &mockApplicationServiceClient{deleteErr: nil}

			argoProcessor := &ArgoAppProcessor{
				ApplicationClient:            mockClient,
				ManageArgoAppsEnabled:        true,
				ManageArgoAppsFilter:         []string{"*"},
				KnownApps:                    map[string]map[string]*v1alpha1.Application{},
				ExperimentalBracketsClusters: []string{"staging"},
				trigger:                      make(chan argoTrigger, 10),
				ArgoApps:                     make(chan *v1alpha1.ApplicationWatchEvent, 10),
				pendingDeletions:             []PendingDeletion{},

				maxProcessedTransformerEslId: &atomic.Int64{},
			}

			// Seed KnownApps and mock with all bracket apps.
			for _, name := range tc.AppKeys {
				argoProcessor.PopulateAppsToKnownApps([]ArgoAppMetadata{
					{
						Name: name, Environment: "staging", ParentEnvironment: "staging",
						Event: "ADDED", IsBracket: true,
					},
				})
				mockClient.PopulateApps([]ArgoAppMetadata{
					{Name: name, Environment: "staging", ParentEnvironment: "staging", Event: "ADDED"},
				})
			}

			// Build AppDetails: all apps have no deployment in staging.
			appDetails := map[string]*api.GetAppDetailsResponse{}
			for _, name := range tc.AppKeys {
				appDetails[name] = &api.GetAppDetailsResponse{
					//exhaustruct:ignore
					Application: &api.Application{Name: name, ArgoBracket: name},
					Deployments: map[string]*api.Deployment{},
				}
			}

			argoOv := &ArgoOverview{
				AppDetails: appDetails,
				Overview: &api.GetOverviewResponse{
					EnvironmentGroups: []*api.EnvironmentGroup{
						{
							EnvironmentGroupName: "staging-group",
							Environments: []*api.Environment{
								{
									Name:     "staging",
									Priority: api.Priority_UPSTREAM,
									Config: &api.EnvironmentConfig{
										Argocd: &api.ArgoCDEnvironmentConfiguration{
											Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
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
			}

			l := logger.FromContext(ctx)
			argoProcessor.ProcessArgoOverview(ctx, l, argoOv)

			// Collect DELETED entries in the order they were appended to mockClient.Apps.
			var gotOrder []string
			for _, app := range mockClient.Apps {
				if app.LastEvent == "DELETED" {
					gotOrder = append(gotOrder, app.App.Name)
				}
			}
			if diff := testutil.CmpDiff(tc.WantDeletedOrder, gotOrder); diff != "" {
				t.Errorf("delete order mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDrainPendingDeletionsByName(t *testing.T) {
	tcs := []struct {
		Name string
	}{
		{Name: "app not in KnownApps: deleted by constructed name without cascade"},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			appName := "my-app"
			envName := "staging-1"
			parentEnvName := "staging"
			argoAppName := envName + "-" + appName

			mockClient := &mockApplicationServiceClient{}
			mockClient.PopulateApps([]ArgoAppMetadata{
				{Name: appName, Environment: envName, ParentEnvironment: parentEnvName, Event: "ADDED"},
			})
			argoProcessor := &ArgoAppProcessor{
				ApplicationClient: mockClient,
				KnownApps: map[string]map[string]*v1alpha1.Application{
					parentEnvName: {
						"bracket": {
							//exhaustruct:ignore
							ObjectMeta: metav1.ObjectMeta{
								Name: "staging-bracket",
								Annotations: map[string]string{
									"com.freiheit.kuberpult/is-bracket": "true",
								},
							},
						},
					},
				},
				pendingDeletions: []PendingDeletion{
					{EnvironmentName: envName, ParentEnvironmentName: parentEnvName, AppName: appName},
				},

				maxProcessedTransformerEslId: &atomic.Int64{},
			}

			argoProcessor.drainPendingDeletions(ctx, parentEnvName)

			if got := len(argoProcessor.pendingDeletions); got != 0 {
				t.Errorf("pendingDeletions after drain: want 0, got %d", got)
			}
			var deleted *ArgoApp
			for _, app := range mockClient.Apps {
				if app.App.Name == argoAppName && app.LastEvent == "DELETED" {
					deleted = app
					break
				}
			}
			if deleted == nil {
				t.Fatalf("expected app %q to be deleted by name, but no DELETED entry found", argoAppName)
			}
			if diff := testutil.CmpDiff(true, deleted.NoCascade); diff != "" {
				t.Errorf("NoCascade mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
