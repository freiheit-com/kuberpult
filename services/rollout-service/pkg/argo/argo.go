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
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/setup"
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
	trigger           chan *v1alpha1.Application
	lastOverview      *api.GetOverviewResponse
	argoApps          chan *v1alpha1.ApplicationWatchEvent
	ApplicationClient application.ApplicationServiceClient
	HealthReporter    *setup.HealthReporter
}

type Key struct {
	Application string
	Environment string
}

func (a *ArgoAppProcessor) Push(last *api.GetOverviewResponse, appToPush *v1alpha1.Application) {
	a.lastOverview = last
	select {
	case a.trigger <- appToPush:
	default:
	}
}

func (a *ArgoAppProcessor) Consume(ctx context.Context) error {
	seen := map[Key]*api.Environment_Application{}

	for {
		select {
		case <-a.trigger:
			err := a.ConsumeArgo(ctx, a.HealthReporter)
			if err != nil {
				return err
			}
		case ev := <-a.argoApps:
			appName, envName := getEnvironmentAndName(ev.Application.Annotations)
			key := Key{Application: appName, Environment: envName}

			if ok := seen[key]; ok == nil {

				switch ev.Type {
				case "ADDED":
					upsert := false
					validate := false

					appCreateRequest := &application.ApplicationCreateRequest{
						Application: &ev.Application,
						Upsert:      &upsert,
						Validate:    &validate,
					}
					_, err := a.ApplicationClient.Create(ctx, appCreateRequest)
					if err != nil {
						return fmt.Errorf(err.Error())
					}
				case "MODIFIED":
					appUpdateRequest := &application.ApplicationUpdateSpecRequest{
						Name:         &ev.Application.Name,
						Spec:         &ev.Application.Spec,
						AppNamespace: &ev.Application.Namespace,
					}
					_, err := a.ApplicationClient.UpdateSpec(ctx, appUpdateRequest)
					if err != nil {
						return fmt.Errorf(err.Error())
					}
				case "DELETED":
					appDeleteRequest := &application.ApplicationDeleteRequest{
						Name:         &ev.Application.Name,
						AppNamespace: &ev.Application.Namespace,
					}
					_, err := a.ApplicationClient.Delete(ctx, appDeleteRequest)
					if err != nil {
						return fmt.Errorf(err.Error())
					}
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

func getEnvironmentAndName(annotations map[string]string) (string, string) {
	return annotations["com.freiheit.kuberpult/environment"], annotations["com.freiheit.kuberpult/application"]
}

func calculateFinalizers() []string {
	return []string{
		"resources-finalizer.argocd.argoproj.io",
	}
}

func CreateDeployApplication(overview *api.GetOverviewResponse, app *api.Environment_Application, env *api.Environment,
	annotations, labels map[string]string, manifestPath string) *v1alpha1.Application {
	applicationNs := ""

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
