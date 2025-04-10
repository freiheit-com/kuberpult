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
	"github.com/DataDog/datadog-go/v5/statsd"
	"path/filepath"
	"slices"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

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
	Push(ctx context.Context, last *ArgoOverview) error
	Consume(ctx context.Context, hlth *setup.HealthReporter) error
	CreateOrUpdateApp(ctx context.Context, overview *api.GetOverviewResponse, appName, team, envName string, argoEnvConfigurationb *api.EnvironmentConfig_ArgoCD, appsKnownToArgo map[string]*v1alpha1.Application)
	DeleteArgoApps(ctx context.Context, argoApps map[string]*v1alpha1.Application, appName string, deployment *api.Deployment)
	GetManageArgoAppsFilter() []string
	GetManageArgoAppsEnabled() bool
}

type ArgoAppProcessor struct {
	trigger                 chan *ArgoOverview
	lastOverview            *ArgoOverview
	ArgoApps                chan *v1alpha1.ApplicationWatchEvent
	ApplicationClient       application.ApplicationServiceClient
	ManageArgoAppsEnabled   bool
	KuberpultMetricsEnabled bool
	ArgoAppsMetricsEnabled  bool
	ManageArgoAppsFilter    []string
	DDMetrics               statsd.ClientInterface
}

func New(appClient application.ApplicationServiceClient, manageArgoApplicationEnabled, kuberpultMetricsEnabled, argoAppsMetricsEnabled bool, manageArgoApplicationFilter []string, triggerChannelSize, argoAppsChannelSize int, ddMetrics statsd.ClientInterface) ArgoAppProcessor {
	return ArgoAppProcessor{
		lastOverview:            nil,
		ApplicationClient:       appClient,
		ManageArgoAppsEnabled:   manageArgoApplicationEnabled,
		ManageArgoAppsFilter:    manageArgoApplicationFilter,
		KuberpultMetricsEnabled: kuberpultMetricsEnabled,
		ArgoAppsMetricsEnabled:  argoAppsMetricsEnabled,
		trigger:                 make(chan *ArgoOverview, triggerChannelSize),
		ArgoApps:                make(chan *v1alpha1.ApplicationWatchEvent, argoAppsChannelSize),
		DDMetrics:               ddMetrics,
	}
}

func (a *ArgoAppProcessor) GetManageArgoAppsFilter() []string {
	return a.ManageArgoAppsFilter
}

func (a *ArgoAppProcessor) GetManageArgoAppsEnabled() bool {
	return a.ManageArgoAppsEnabled
}

func (a *ArgoAppProcessor) Push(ctx context.Context, last *ArgoOverview) error {
	l := logger.FromContext(ctx).With(zap.String("argo-pushing", "ready"))
	a.lastOverview = last
	select {
	case a.trigger <- a.lastOverview:
		l.Info("argocd.pushed")
		a.GaugeKuberpultEventsQueueFillRate(ctx)
		return nil
	default:
		return fmt.Errorf("failed to push to argo app processor: channel full")
	}
}

func (a *ArgoAppProcessor) Consume(ctx context.Context, hlth *setup.HealthReporter) error {
	hlth.ReportReady("event-consuming")
	l := logger.FromContext(ctx).With(zap.String("self-manage", "consuming"))
	appsKnownToArgo := map[string]map[string]*v1alpha1.Application{} //EnvName => AppName => Deployment
	for {
		select {
		case argoOv := <-a.trigger:
			l.Info("self-manage.trigger")
			a.ProcessArgoOverview(ctx, l, appsKnownToArgo, argoOv)
			a.GaugeKuberpultEventsQueueFillRate(ctx)
		case <-ctx.Done():
			return nil
		default:
			select {
			case argoOv := <-a.trigger:
				l.Info("self-manage.trigger")
				a.ProcessArgoOverview(ctx, l, appsKnownToArgo, argoOv)
				a.GaugeKuberpultEventsQueueFillRate(ctx)
			case ev := <-a.ArgoApps:
				a.ProcessArgoWatchEvent(ctx, l, appsKnownToArgo, ev)
				a.GaugeArgoAppsQueueFillRate(ctx)
			case <-ctx.Done():
				return nil
			}
		}
	}
}

type ArgoOverview struct {
	AppDetails map[string]*api.GetAppDetailsResponse //Map from appName to app Details. Gets filled with information based on what apps have changed.
	Overview   *api.GetOverviewResponse              //Standard overview. Only information regarding environments should be retrieved from this overview.
}

func (a *ArgoAppProcessor) ProcessArgoOverview(ctx context.Context, l *zap.Logger, appsKnownToArgo map[string]map[string]*v1alpha1.Application, argoOv *ArgoOverview) {
	overview := argoOv.Overview
	for currentApp, currentAppDetails := range argoOv.AppDetails {
		span, ctx := tracer.StartSpanFromContext(ctx, "ProcessChangedApp")
		defer span.Finish()
		span.SetTag("kuberpult-app", currentApp)
		for _, envGroup := range overview.EnvironmentGroups {
			for _, parentEnvironment := range envGroup.Environments {
				if isAAEnv(parentEnvironment.Config) {
					for _, cfg := range parentEnvironment.Config.ArgoConfigs.Configs {
						targetEnvName := a.extractFullyQualifiedEnvironmentName(parentEnvironment.Config.ArgoConfigs.CommonEnvPrefix, parentEnvironment.Name, cfg)
						a.ProcessAppChange(ctx, appsKnownToArgo, targetEnvName, cfg, currentApp, currentAppDetails, overview)
					}
				} else {
					a.ProcessAppChange(ctx, appsKnownToArgo, parentEnvironment.Name, parentEnvironment.Config.Argocd, currentApp, currentAppDetails, overview)
				}

			}
		}
		span.Finish()
	}
}

func (a *ArgoAppProcessor) extractFullyQualifiedEnvironmentName(commonPrefix, envName string, argoCDConfig *api.EnvironmentConfig_ArgoCD) string {
	return commonPrefix + "-" + envName + "-" + argoCDConfig.ConcreteEnvName
}

func (a *ArgoAppProcessor) ProcessAppChange(ctx context.Context, appsKnownToArgo map[string]map[string]*v1alpha1.Application, targetEnvironmentName string, targetEnvConfig *api.EnvironmentConfig_ArgoCD, currentApp string, currentAppDetails *api.GetAppDetailsResponse, overview *api.GetOverviewResponse) {
	logger.FromContext(ctx).Sugar().Debugf("Processing app %q on environment %q", currentApp, targetEnvironmentName)
	if ok := appsKnownToArgo[targetEnvironmentName]; ok != nil {
		a.DeleteArgoApps(ctx, appsKnownToArgo[targetEnvironmentName], currentApp, currentAppDetails.Deployments[targetEnvironmentName])
	}

	if currentAppDetails.Deployments[targetEnvironmentName] != nil { //If there is a deployment for this app on this environment
		a.CreateOrUpdateApp(ctx, overview, currentApp, currentAppDetails.Application.Team, targetEnvironmentName, targetEnvConfig, appsKnownToArgo[targetEnvironmentName])
	}
}

func isAAEnv(config *api.EnvironmentConfig) bool {
	return config.ArgoConfigs != nil && len(config.ArgoConfigs.Configs) > 1
}

func (a *ArgoAppProcessor) ProcessArgoWatchEvent(ctx context.Context, l *zap.Logger, appsKnownToArgo map[string]map[string]*v1alpha1.Application, ev *v1alpha1.ApplicationWatchEvent) {
	envName, appName := getEnvironmentAndName(ev.Application.Annotations)
	if appName == "" {
		return
	}
	if appsKnownToArgo[envName] == nil {
		appsKnownToArgo[envName] = map[string]*v1alpha1.Application{}
	}
	switch ev.Type {
	case "ADDED", "MODIFIED":
		l.Info("created/updated:kuberpult.application:" + ev.Application.Name + ",kuberpult.environment:" + envName)
		appsKnownToArgo[envName][appName] = &ev.Application
	case "DELETED":
		l.Info("deleted:kuberpult.application:" + ev.Application.Name + ",kuberpult.environment:" + envName)
		delete(appsKnownToArgo[envName], appName)
	}
}

func (a *ArgoAppProcessor) CreateOrUpdateApp(ctx context.Context, overview *api.GetOverviewResponse, appName, team, envName string, argoEnvConfiguration *api.EnvironmentConfig_ArgoCD, appsKnownToArgo map[string]*v1alpha1.Application) {
	t := team

	var existingApp *v1alpha1.Application
	selfManaged, err := IsSelfManagedFilterActive(t, a)
	if err != nil {
		logger.FromContext(ctx).Error("detecting self manage:", zap.Error(err))
	}
	if selfManaged {
		for _, argoApp := range appsKnownToArgo {
			if argoApp.Annotations["com.freiheit.kuberpult/application"] == appName && argoApp.Annotations["com.freiheit.kuberpult/environment"] == envName {
				existingApp = argoApp
				break
			}
		}

		if existingApp == nil {
			createSpan, ctx := tracer.StartSpanFromContext(ctx, "CreateApplication")
			createSpan.SetTag("application", appName)
			createSpan.SetTag("environment", envName)
			createSpan.SetTag("operation", "create")
			appToCreate := CreateArgoApplication(overview, appName, team, envName, argoEnvConfiguration)
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
					logger.FromContext(ctx).Error("creating "+appToCreate.Name+",env "+envName, zap.Error(err))
				}
			}
			createSpan.Finish()
		} else {
			appToUpdate := CreateArgoApplication(overview, appName, team, envName, argoEnvConfiguration)
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
				updateSpan.SetTag("environment", envName)
				updateSpan.SetTag("operation", "update")
				updateSpan.SetTag("argoDiff", diff)
				_, err := a.ApplicationClient.Update(ctx, appUpdateRequest)
				if err != nil {
					logger.FromContext(ctx).Error("updating application: "+appToUpdate.Name+",env "+envName, zap.Error(err))
				}
				updateSpan.Finish()
			}
		}
	}
}

func (a *ArgoAppProcessor) ShouldSendArgoAppsMetrics() bool {
	return a.DDMetrics != nil && a.ArgoAppsMetricsEnabled
}

func (a *ArgoAppProcessor) GaugeArgoAppsQueueFillRate(ctx context.Context) {
	if !a.ShouldSendArgoAppsMetrics() {
		return
	}
	fillRate := 0.0
	if cap(a.ArgoApps) != 0 {
		fillRate = float64(len(a.ArgoApps)) / float64(cap(a.ArgoApps))
	} else {
		fillRate = 1 // If capacity is 0, we are always at 100%
	}
	ddError := a.DDMetrics.Gauge("argo_events_fill_rate", fillRate, []string{}, 1)
	if ddError != nil {
		logger.FromContext(ctx).Sugar().Warnf("could not send argo_events_fill_rate metric to datadog! Err: %v", ddError)
	}
}

func (a *ArgoAppProcessor) GaugeKuberpultEventsQueueFillRate(ctx context.Context) {
	if !a.KuberpultMetricsEnabled || a.DDMetrics == nil {
		return
	}

	fillRate := 0.0
	if cap(a.trigger) != 0 {
		fillRate = float64(len(a.trigger)) / float64(cap(a.trigger))
	} else {
		fillRate = 1 // If capacity is 0, we are always at 100%
	}
	ddError := a.DDMetrics.Gauge("kuberpult_events_fill_rate", fillRate, []string{}, 1)

	if ddError != nil {
		logger.FromContext(ctx).Sugar().Warnf("error sending kuberpult_events_fill_rate to datadog. Err: %w", ddError)
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

func CreateArgoApplication(overview *api.GetOverviewResponse, appName, team, envName string, envArgoConfig *api.EnvironmentConfig_ArgoCD) *v1alpha1.Application {
	applicationNs := ""

	annotations := make(map[string]string)
	labels := make(map[string]string)

	manifestPath := filepath.Join("environments", envName, "applications", appName, "manifests")

	annotations["com.freiheit.kuberpult/application"] = appName
	annotations["com.freiheit.kuberpult/environment"] = envName
	annotations["com.freiheit.kuberpult/self-managed"] = "true"
	// This annotation is so that argoCd does not invalidate *everything* in the whole repo when receiving a git webhook.
	// It has to start with a "/" to be absolute to the git repo.
	// See https://argo-cd.readthedocs.io/en/stable/operator-manual/high_availability/#webhook-and-manifest-paths-annotation
	annotations["argocd.argoproj.io/manifest-generate-paths"] = "/" + manifestPath
	labels["com.freiheit.kuberpult/team"] = team

	if envArgoConfig.Destination.Namespace != nil {
		applicationNs = *envArgoConfig.Destination.Namespace
	} else if envArgoConfig.Destination.ApplicationNamespace != nil {
		applicationNs = *envArgoConfig.Destination.ApplicationNamespace
	}

	applicationDestination := v1alpha1.ApplicationDestination{
		Name:      envArgoConfig.Destination.Name,
		Namespace: applicationNs,
		Server:    envArgoConfig.Destination.Server,
	}

	var ignoreDifferences []v1alpha1.ResourceIgnoreDifferences = nil
	if len(envArgoConfig.IgnoreDifferences) > 0 {
		ignoreDifferences = make([]v1alpha1.ResourceIgnoreDifferences, len(envArgoConfig.IgnoreDifferences))
		for index, value := range envArgoConfig.IgnoreDifferences {
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
	}
	//exhaustruct:ignore
	ObjectMeta := metav1.ObjectMeta{
		Name:        fmt.Sprintf("%s-%s", envName, appName),
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
		SyncOptions: envArgoConfig.SyncOptions,
	}
	//exhaustruct:ignore
	Spec := v1alpha1.ApplicationSpec{
		Source:            Source,
		SyncPolicy:        SyncPolicy,
		Project:           envName,
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
