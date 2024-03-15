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
	"github.com/google/go-cmp/cmp"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"path/filepath"
	"slices"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/ptr"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// this is a simpler version of ApplicationServiceClient from the application package
type SimplifiedApplicationServiceClient interface {
	Watch(ctx context.Context, qry *application.ApplicationQuery, opts ...grpc.CallOption) (application.ApplicationService_WatchClient, error)
}

type ArgoAppProcessor struct {
	trigger               chan *api.GetOverviewResponse
	lastOverview          *api.GetOverviewResponse
	argoApps              chan *v1alpha1.ApplicationWatchEvent
	ApplicationClient     application.ApplicationServiceClient
	ManageArgoAppsEnabled bool
	ManageArgoAppsFilter  []string
}

func New(appClient application.ApplicationServiceClient, manageArgoApplicationEnabled bool, manageArgoApplicationFilter []string) ArgoAppProcessor {
	return ArgoAppProcessor{
		lastOverview:          nil,
		ApplicationClient:     appClient,
		ManageArgoAppsEnabled: manageArgoApplicationEnabled,
		ManageArgoAppsFilter:  manageArgoApplicationFilter,
		trigger:               make(chan *api.GetOverviewResponse),
		argoApps:              make(chan *v1alpha1.ApplicationWatchEvent),
	}
}

type Key struct {
	AppName     string
	EnvName     string
	Application *api.Environment_Application
	Environment *api.Environment
}

func (a *ArgoAppProcessor) Push(ctx context.Context, last *api.GetOverviewResponse) {
	l := logger.FromContext(ctx).With(zap.String("argo-pushing", "ready"))
	a.lastOverview = last
	select {
	case a.trigger <- a.lastOverview:
		l.Info("argocd.pushed")
	default:
	}
}

func (a *ArgoAppProcessor) Consume(ctx context.Context, hlth *setup.HealthReporter) error {
	hlth.ReportReady("event-consuming")
	l := logger.FromContext(ctx).With(zap.String("self-manage", "consuming"))
	appsKnownToArgo := map[string]map[string]*v1alpha1.Application{}
	envAppsKnownToArgo := make(map[string]*v1alpha1.Application)
	for {
		select {
		case overview := <-a.trigger:
			for _, envGroup := range overview.EnvironmentGroups {
				for _, env := range envGroup.Environments {
					if ok := appsKnownToArgo[env.Name]; ok != nil {
						envAppsKnownToArgo = appsKnownToArgo[env.Name]
						err := a.DeleteArgoApps(ctx, envAppsKnownToArgo, env.Applications)
						if err != nil {
							l.Error("deleting applications", zap.Error(err))
							continue
						}
					}

					for _, app := range env.Applications {
						a.CreateOrUpdateApp(ctx, overview, app, env, envAppsKnownToArgo)
					}
				}
			}

		case ev := <-a.argoApps:
			envName, appName := getEnvironmentAndName(ev.Application.Annotations)
			if appName == "" {
				continue
			}
			if appsKnownToArgo[envName] == nil {
				appsKnownToArgo[envName] = map[string]*v1alpha1.Application{}
			}
			envKnownToArgo := appsKnownToArgo[envName]
			switch ev.Type {
			case "ADDED", "MODIFIED":
				l.Info("created/updated:kuberpult.application:" + ev.Application.Name + ",kuberpult.environment:" + envName)
				envKnownToArgo[appName] = &ev.Application
			case "DELETED":
				l.Info("deleted:kuberpult.application:" + ev.Application.Name + ",kuberpult.environment:" + envName)
				delete(envKnownToArgo, appName)
			}
			appsKnownToArgo[envName] = envKnownToArgo
		case <-ctx.Done():
			return nil
		}
	}
}

func (a ArgoAppProcessor) CreateOrUpdateApp(ctx context.Context, overview *api.GetOverviewResponse, app *api.Environment_Application, env *api.Environment, appsKnownToArgo map[string]*v1alpha1.Application) {
	//exhaustruct:ignore
	t := team(overview, app.Name)
	span, ctx := tracer.StartSpanFromContext(ctx, "Create or Update Applications")
	defer span.Finish()

	var existingApp *v1alpha1.Application
	if a.ManageArgoAppsEnabled && len(a.ManageArgoAppsFilter) > 0 && slices.Contains(a.ManageArgoAppsFilter, t) {

		for _, argoApp := range appsKnownToArgo {
			if argoApp.Annotations["com.freiheit.kuberpult/application"] == app.Name && argoApp.Annotations["com.freiheit.kuberpult/environment"] == env.Name {
				existingApp = argoApp
				break
			}
		}

		if existingApp == nil {
			appToCreate := CreateArgoApplication(overview, app, env)
			appToCreate.ResourceVersion = ""
			upsert := false
			validate := false
			appCreateRequest := &application.ApplicationCreateRequest{
				XXX_NoUnkeyedLiteral: struct{}{},
				XXX_unrecognized:     nil,
				XXX_sizecache:        0,
				Application:          appToCreate,
				Upsert:               &upsert,
				Validate:             &validate,
			}
			_, err := a.ApplicationClient.Create(ctx, appCreateRequest)
			if err != nil {
				// We check if the application was created in the meantime
				if status.Code(err) != codes.InvalidArgument {
					logger.FromContext(ctx).Error("creating "+appToCreate.Name+",env "+env.Name, zap.Error(err))
				}
			}
		} else {
			appToUpdate := CreateArgoApplication(overview, app, env)
			appUpdateRequest := &application.ApplicationUpdateRequest{
				XXX_NoUnkeyedLiteral: struct{}{},
				XXX_unrecognized:     nil,
				XXX_sizecache:        0,
				Validate:             ptr.Bool(false),
				Application:          appToUpdate,
				Project:              ptr.FromString(appToUpdate.Spec.Project),
			}
			//We have to exclude the unexported type isServerInferred. It is managed by Argo.

			//exhaustruct:ignore
			if !cmp.Equal(appUpdateRequest.Application.Spec, existingApp.Spec, cmp.AllowUnexported(v1alpha1.ApplicationSpec{}.Destination)) {
				_, err := a.ApplicationClient.Update(ctx, appUpdateRequest)
				if err != nil {
					logger.FromContext(ctx).Error("updating application: "+appToUpdate.Name+",env "+env.Name, zap.Error(err))
				}
			}
		}
	}
}

func (a *ArgoAppProcessor) ConsumeArgo(ctx context.Context, hlth *setup.HealthReporter) error {
	return hlth.Retry(ctx, func() error {
		//exhaustruct:ignore
		watch, err := a.ApplicationClient.Watch(ctx, &application.ApplicationQuery{})
		if err != nil {
			if status.Code(err) == codes.Canceled {
				// context is cancelled -> we are shutting down
				return setup.Permanent(nil)
			}
			return fmt.Errorf("watching applications: %w", err)
		}
		hlth.ReportReady("consuming argo events")
		for {
			ev, err := watch.Recv()
			if err != nil {
				if status.Code(err) == codes.Canceled {
					// context is cancelled -> we are shutting down
					return setup.Permanent(nil)
				}
				return err
			}

			switch ev.Type {
			case "ADDED", "MODIFIED", "DELETED":
				a.argoApps <- ev
			}
		}
	})
}

func calculateFinalizers() []string {
	return []string{
		"resources-finalizer.argocd.argoproj.io",
	}
}

func (a ArgoAppProcessor) DeleteArgoApps(ctx context.Context, argoApps map[string]*v1alpha1.Application, apps map[string]*api.Environment_Application) error {
	toDelete := make([]*v1alpha1.Application, 0)
	for _, argoApp := range argoApps {
		if apps[argoApp.Annotations["com.freiheit.kuberpult/application"]] == nil {
			toDelete = append(toDelete, argoApp)
		}
	}

	for i := range toDelete {
		_, err := a.ApplicationClient.Delete(ctx, &application.ApplicationDeleteRequest{
			Cascade:              nil,
			PropagationPolicy:    nil,
			AppNamespace:         nil,
			Project:              nil,
			XXX_NoUnkeyedLiteral: struct{}{},
			XXX_unrecognized:     nil,
			XXX_sizecache:        0,
			Name:                 ptr.FromString(toDelete[i].Name),
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func CreateArgoApplication(overview *api.GetOverviewResponse, app *api.Environment_Application, env *api.Environment) *v1alpha1.Application {
	applicationNs := ""

	annotations := make(map[string]string)
	labels := make(map[string]string)

	manifestPath := filepath.Join("environments", env.Name, "applications", app.Name, "manifests")

	annotations["com.freiheit.kuberpult/application"] = app.Name
	annotations["com.freiheit.kuberpult/environment"] = env.Name
	annotations["com.freiheit.kuberpult/self-managed"] = "true"
	// This annotation is so that argoCd does not invalidate *everything* in the whole repo when receiving a git webhook.
	// It has to start with a "/" to be absolute to the git repo.
	// See https://argo-cd.readthedocs.io/en/stable/operator-manual/high_availability/#webhook-and-manifest-paths-annotation
	annotations["argocd.argoproj.io/manifest-generate-paths"] = "/" + manifestPath
	labels["com.freiheit.kuberpult/team"] = team(overview, app.Name)

	if env.Config.Argocd.Destination.Namespace != nil {
		applicationNs = *env.Config.Argocd.Destination.Namespace
	} else if env.Config.Argocd.Destination.ApplicationNamespace != nil {
		applicationNs = *env.Config.Argocd.Destination.ApplicationNamespace
	}

	applicationDestination := v1alpha1.ApplicationDestination{
		Name:      env.Config.Argocd.Destination.Name,
		Namespace: applicationNs,
		Server:    env.Config.Argocd.Destination.Server,
	}

	ignoreDifferences := make([]v1alpha1.ResourceIgnoreDifferences, len(env.Config.Argocd.IgnoreDifferences))
	for index, value := range env.Config.Argocd.IgnoreDifferences {
		difference := v1alpha1.ResourceIgnoreDifferences{
			Group:                 value.Group,
			Kind:                  value.Kind,
			Name:                  value.Name,
			Namespace:             value.Namespace,
			JSONPointers:          value.JsonPointers,
			JQPathExpressions:     value.JqPathExpressions,
			ManagedFieldsManagers: value.ManagedFieldsManagers,
		}
		ignoreDifferences[index] = difference
	}
	//exhaustruct:ignore
	ObjectMeta := metav1.ObjectMeta{
		Name:        fmt.Sprintf("%s-%s", env.Name, app.Name),
		Annotations: annotations,
		Labels:      labels,
		Finalizers:  calculateFinalizers(),
	}
	//exhaustruct:ignore
	Source := &v1alpha1.ApplicationSource{
		RepoURL:        overview.ManifestRepoUrl,
		Path:           manifestPath,
		TargetRevision: overview.Branch,
	}
	//exhaustruct:ignore
	SyncPolicy := &v1alpha1.SyncPolicy{
		Automated: &v1alpha1.SyncPolicyAutomated{
			Prune:    true,
			SelfHeal: true,
			// We always allow empty, because it makes it easier to delete apps/environments
			AllowEmpty: true,
		},
		SyncOptions: env.Config.Argocd.SyncOptions,
	}
	//exhaustruct:ignore
	Spec := v1alpha1.ApplicationSpec{
		Source:            Source,
		SyncPolicy:        SyncPolicy,
		Project:           env.Name,
		Destination:       applicationDestination,
		IgnoreDifferences: ignoreDifferences,
	}
	//exhaustruct:ignore
	deployApp := &v1alpha1.Application{
		ObjectMeta: ObjectMeta,
		Spec:       Spec,
	}

	return deployApp
}

func team(overview *api.GetOverviewResponse, app string) string {
	a := overview.Applications[app]
	if a == nil {
		return ""
	}
	return a.Team
}

func getEnvironmentAndName(annotations map[string]string) (string, string) {
	return annotations["com.freiheit.kuberpult/environment"], annotations["com.freiheit.kuberpult/application"]
}
