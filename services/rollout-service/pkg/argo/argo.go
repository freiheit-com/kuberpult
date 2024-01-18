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

Copyright 2023 freiheit.com
*/
package argo

import (
	"context"
	"fmt"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/ptr"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path/filepath"
	"slices"
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
	l := logger.FromContext(ctx).With(zap.String("argo-consuming", "ready"))
	a.lastOverview = last
	select {
	case a.trigger <- a.lastOverview:
		l.Info("argocd.pushing")
	default:
	}
}

func (a *ArgoAppProcessor) Consume(ctx context.Context) error {
	l := logger.FromContext(ctx).With(zap.String("event-consuming", "ready"))

	for {
		select {
		case overview := <-a.trigger:
			for _, envGroup := range overview.EnvironmentGroups {
				for _, env := range envGroup.Environments {
					appList, err := a.ApplicationClient.List(ctx, &application.ApplicationQuery{
						Name: ptr.FromString(fmt.Sprintf("%s-", env)),
					})

					if err != nil {
						fmt.Errorf("listing applications: %w", err)
					}
					err = a.DeleteArgoApps(ctx, appList, env.Applications)

					if err != nil {
						fmt.Errorf("deleting applications: %w", err)
					}

					for _, app := range env.Applications {
						t := team(overview, app.Name)
						if a.ManageArgoAppsEnabled && len(a.ManageArgoAppsFilter) > 0 && slices.Contains(a.ManageArgoAppsFilter, t) {
							k := Key{AppName: app.Name, EnvName: env.Name, Application: app, Environment: env}

							argoApp, err := a.ApplicationClient.Get(ctx, &application.ApplicationQuery{
								Name: ptr.FromString(app.Name),
							})

							if err != nil {
								fmt.Errorf("getting application: %w", err)
							}

							if argoApp == nil {
								appToCreate := CreateDeployApplication(overview, app, k.Environment)
								appToCreate.ResourceVersion = ""
								upsert := false
								validate := false
								appCreateRequest := &application.ApplicationCreateRequest{
									Application: appToCreate,
									Upsert:      &upsert,
									Validate:    &validate,
								}
								_, err = a.ApplicationClient.Create(ctx, appCreateRequest)
								if err != nil {
									fmt.Errorf("creating application: %w", err)
								}
							} else {
								appToUpdate := CreateDeployApplication(overview, app, k.Environment)
								appUpdateRequest := &application.ApplicationUpdateSpecRequest{
									Name:         &appToUpdate.Name,
									Spec:         &appToUpdate.Spec,
									AppNamespace: &appToUpdate.Namespace,
								}
								_, err = a.ApplicationClient.UpdateSpec(ctx, appUpdateRequest)
								if err != nil {
									fmt.Errorf("updating application: %w", err)
								}
							}

						}
					}
				}
			}

		case ev := <-a.argoApps:
			appName, envName := getEnvironmentAndName(ev.Application.Annotations)
			k := Key{AppName: appName, EnvName: envName}
			if k.AppName == "" {
				continue
			}
			switch ev.Type {
			case "ADDED":
				l.Info(fmt.Sprintf("argocd.created-%s", k.AppName))
			case "MODIFIED":
				l.Info(fmt.Sprintf("argocd.updated-%s", k.AppName))
			case "DELETED":
				l.Info(fmt.Sprintf("argocd.deleted-%s", k.AppName))
			}

		case <-ctx.Done():
			return nil
		}
	}
}

func (a *ArgoAppProcessor) ConsumeArgo(ctx context.Context, hlth *setup.HealthReporter) error {
	watch, err := a.ApplicationClient.Watch(ctx, &application.ApplicationQuery{})
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

		switch ev.Type {
		case "ADDED", "MODIFIED", "DELETED":
			a.argoApps <- ev
		}
	}
}

func calculateFinalizers() []string {
	return []string{
		"resources-finalizer.argocd.argoproj.io",
	}
}

func (a ArgoAppProcessor) DeleteArgoApps(ctx context.Context, appList *v1alpha1.ApplicationList, apps map[string]*api.Environment_Application) error {
	argoApps := appList.Items
	toDelete := make([]*v1alpha1.Application, 0)
	for _, argoApp := range argoApps {
		for i, app := range apps {
			if argoApp.Name == fmt.Sprintf("%s-%s", i, app.Name) {
				break
			}
		}
		toDelete = append(toDelete, &argoApp)
	}

	for i := range toDelete {
		_, err := a.ApplicationClient.Delete(ctx, &application.ApplicationDeleteRequest{
			Name: ptr.FromString(toDelete[i].Name),
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func CreateDeployApplication(overview *api.GetOverviewResponse, app *api.Environment_Application, env *api.Environment) *v1alpha1.Application {
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

	syncWindows := v1alpha1.SyncWindows{}

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

	for _, w := range env.Config.Argocd.SyncWindows {
		apps := []string{"*"}
		if len(w.Applications) > 0 {
			apps = w.Applications
		}
		syncWindows = append(syncWindows, &v1alpha1.SyncWindow{
			Applications: apps,
			Schedule:     w.Schedule,
			Duration:     w.Duration,
			Kind:         w.Kind,
			ManualSync:   true,
		})
	}

	deployApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:        app.Name,
			Annotations: annotations,
			Labels:      labels,
			Finalizers:  calculateFinalizers(),
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: env.Name,
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        overview.ManifestRepoUrl,
				Path:           manifestPath,
				TargetRevision: overview.Branch,
			},
			Destination: applicationDestination,
			SyncPolicy: &v1alpha1.SyncPolicy{
				Automated: &v1alpha1.SyncPolicyAutomated{
					Prune:    false,
					SelfHeal: false,
					// We always allow empty, because it makes it easier to delete apps/environments
					AllowEmpty: true,
				},
				SyncOptions: env.Config.Argocd.SyncOptions,
			},
			IgnoreDifferences: ignoreDifferences,
		},
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
