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
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"path/filepath"
	"slices"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/conversion"
	"github.com/freiheit-com/kuberpult/pkg/logger"
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

type Processor interface {
	Push(ctx context.Context, last *ArgoOverview)
	Consume(ctx context.Context, hlth *setup.HealthReporter) error
	CreateOrUpdateApp(ctx context.Context, overview *api.GetOverviewResponse, appName, team string, env *api.Environment, appsKnownToArgo map[string]*v1alpha1.Application)
	ConsumeArgo(ctx context.Context, hlth *setup.HealthReporter) error
	DeleteArgoApps(ctx context.Context, argoApps map[string]*v1alpha1.Application, appName string, deployment *api.Deployment)
	GetManageArgoAppsFilter() []string
	GetManageArgoAppsEnabled() bool
}

type ArgoAppProcessor struct {
	trigger               chan *ArgoOverview
	lastOverview          *ArgoOverview
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
		trigger:               make(chan *ArgoOverview),
		argoApps:              make(chan *v1alpha1.ApplicationWatchEvent),
	}
}

type Key struct {
	AppName     string
	EnvName     string
	Application *api.Environment_Application
	Environment *api.Environment
}

func (a *ArgoAppProcessor) GetManageArgoAppsFilter() []string {
	return a.ManageArgoAppsFilter
}

func (a *ArgoAppProcessor) GetManageArgoAppsEnabled() bool {
	return a.ManageArgoAppsEnabled
}

func (a *ArgoAppProcessor) Push(ctx context.Context, last *ArgoOverview) {
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
	appsKnownToArgo := map[string]map[string]*v1alpha1.Application{} //EnvName => AppName => Deployment
	envAppsKnownToArgo := make(map[string]*v1alpha1.Application)
	for {
		select {
		case argoOv := <-a.trigger:
			overview := argoOv.Overview
			for currentApp, currentAppDetails := range argoOv.AppDetails {
				for _, envGroup := range overview.EnvironmentGroups {
					for _, env := range envGroup.Environments {
						if ok := appsKnownToArgo[env.Name]; ok != nil {
							envAppsKnownToArgo = appsKnownToArgo[env.Name]
							a.DeleteArgoApps(ctx, envAppsKnownToArgo, currentApp, currentAppDetails.Deployments[env.Name])
						}

						a.CreateOrUpdateApp(ctx, overview, currentApp, currentAppDetails.Application.Team, env, envAppsKnownToArgo)
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

type ArgoOverview struct {
	AppDetails map[string]*api.GetAppDetailsResponse
	Overview   *api.GetOverviewResponse
}

func (a *ArgoAppProcessor) CreateOrUpdateApp(ctx context.Context, overview *api.GetOverviewResponse, appName, team string, env *api.Environment, appsKnownToArgo map[string]*v1alpha1.Application) {
	t := team

	var existingApp *v1alpha1.Application
	selfManaged, err := IsSelfManagedFilterActive(t, a)
	if err != nil {
		logger.FromContext(ctx).Error("detecting self manage:", zap.Error(err))
	}
	if selfManaged {
		for _, argoApp := range appsKnownToArgo {
			if argoApp.Annotations["com.freiheit.kuberpult/application"] == appName && argoApp.Annotations["com.freiheit.kuberpult/environment"] == env.Name {
				existingApp = argoApp
				break
			}
		}

		if existingApp == nil {
			createSpan, ctx := tracer.StartSpanFromContext(ctx, "CreateApplication")
			createSpan.SetTag("application", appName)
			createSpan.SetTag("environment", env.Name)
			createSpan.SetTag("operation", "create")
			appToCreate := CreateArgoApplication(overview, appName, team, env)
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
			createSpan.Finish()
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
				updateSpan, ctx := tracer.StartSpanFromContext(ctx, "UpdateApplications")
				updateSpan.SetTag("application", appName)
				updateSpan.SetTag("environment", env.Name)
				updateSpan.SetTag("operation", "update")
				updateSpan.SetTag("argoDiff", diff)
				_, err := a.ApplicationClient.Update(ctx, appUpdateRequest)
				if err != nil {
					logger.FromContext(ctx).Error("updating application: "+appToUpdate.Name+",env "+env.Name, zap.Error(err))
				}
				updateSpan.Finish()
			}
		}
	}
}

func IsSelfManagedFilterActive(team string, processor Processor) (bool, error) {
	managedAppsFilter := processor.GetManageArgoAppsFilter()
	managedAppsEnabled := processor.GetManageArgoAppsEnabled()
	if len(managedAppsFilter) > 1 && slices.Contains(managedAppsFilter, "*") {
		return false, fmt.Errorf("filter can only have length of 1 when `*` is active")
	}

	isSelfManaged := managedAppsEnabled && (slices.Contains(managedAppsFilter, team) || slices.Contains(managedAppsFilter, "*"))

	return isSelfManaged, nil
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

func (a *ArgoAppProcessor) DeleteArgoApps(ctx context.Context, argoApps map[string]*v1alpha1.Application, appName string, deployment *api.Deployment) {
	toDelete := make([]*v1alpha1.Application, 0)
	deleteSpan, ctx := tracer.StartSpanFromContext(ctx, "DeleteApplications")
	defer deleteSpan.Finish()
	if argoApps[appName] != nil && deployment == nil {
		toDelete = append(toDelete, argoApps[appName])
	}
	for i := range toDelete {
		deleteAppSpan, ctx := tracer.StartSpanFromContext(ctx, "DeleteApplication")
		deleteAppSpan.SetTag("application", toDelete[i].Name)
		deleteAppSpan.SetTag("namespace", toDelete[i].Namespace)
		deleteAppSpan.SetTag("operation", "delete")
		_, err := a.ApplicationClient.Delete(ctx, &application.ApplicationDeleteRequest{
			Cascade:              nil,
			PropagationPolicy:    nil,
			AppNamespace:         nil,
			Project:              nil,
			XXX_NoUnkeyedLiteral: struct{}{},
			XXX_unrecognized:     nil,
			XXX_sizecache:        0,
			Name:                 conversion.FromString(toDelete[i].Name),
		})

		if err != nil {
			logger.FromContext(ctx).Error("deleting application: "+toDelete[i].Name, zap.Error(err))
		}
		deleteAppSpan.Finish()
	}
}

func CreateArgoApplication(overview *api.GetOverviewResponse, appName, team string, env *api.Environment) *v1alpha1.Application {
	applicationNs := ""

	annotations := make(map[string]string)
	labels := make(map[string]string)

	manifestPath := filepath.Join("environments", env.Name, "applications", appName, "manifests")

	annotations["com.freiheit.kuberpult/application"] = appName
	annotations["com.freiheit.kuberpult/environment"] = env.Name
	annotations["com.freiheit.kuberpult/self-managed"] = "true"
	// This annotation is so that argoCd does not invalidate *everything* in the whole repo when receiving a git webhook.
	// It has to start with a "/" to be absolute to the git repo.
	// See https://argo-cd.readthedocs.io/en/stable/operator-manual/high_availability/#webhook-and-manifest-paths-annotation
	annotations["argocd.argoproj.io/manifest-generate-paths"] = "/" + manifestPath
	labels["com.freiheit.kuberpult/team"] = team

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
		Name:        fmt.Sprintf("%s-%s", env.Name, appName),
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

func getEnvironmentAndName(annotations map[string]string) (string, string) {
	return annotations["com.freiheit.kuberpult/environment"], annotations["com.freiheit.kuberpult/application"]
}
