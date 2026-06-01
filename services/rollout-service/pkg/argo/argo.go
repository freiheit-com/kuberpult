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
	"path/filepath"
	"slices"
	"sync/atomic"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/conversion"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/pkg/sorting"
)

// this is a simpler version of ApplicationServiceClient from the application package
type SimplifiedApplicationServiceClient interface {
	Watch(ctx context.Context, qry *application.ApplicationQuery, opts ...grpc.CallOption) (application.ApplicationService_WatchClient, error)
}

type WriteOnceCh = *chan struct{}

// argoTrigger carries an ArgoOverview together with the ESL ID of the
// cd-service event that triggered the push. Consume updates
// maxProcessedTransformerEslId after ProcessArgoOverview returns so the cascade
// consumer can gate safely.
type argoTrigger struct {
	overview *ArgoOverview
	eslId    int64
}

type Processor interface {
	Push(ctx context.Context, last *ArgoOverview, eslId int64) error
	Consume(ctx context.Context, hlth *setup.HealthReporter, chPtr WriteOnceCh) error
	CreateArgoApp(ctx context.Context, overview *api.GetOverviewResponse, appInfo *AppInfo)
	UpdateArgoApp(ctx context.Context, overview *api.GetOverviewResponse, appInfo *AppInfo, existingApp *v1alpha1.Application)
	DeleteArgoApps(ctx context.Context, argoApps map[string]*v1alpha1.Application, appName string, deployment *api.Deployment)
	GetManageArgoAppsFilter() []string
	GetManageArgoAppsEnabled() bool
}

type PendingDeletion struct {
	EnvironmentName       string // key into KnownApps for the DeleteArgoApps call
	ParentEnvironmentName string // key for isBracketEnv / knowsBracketApp check
	AppName               string
}

type ArgoAppProcessor struct {
	trigger chan argoTrigger

	// the eslId stored here signifies that all create/update operations are already done
	maxProcessedTransformerEslId *atomic.Int64

	ArgoApps                chan *v1alpha1.ApplicationWatchEvent
	ApplicationClient       application.ApplicationServiceClient
	ManageArgoAppsEnabled   bool
	KuberpultMetricsEnabled bool
	ArgoAppsMetricsEnabled  bool
	ManageArgoAppsFilter    []string
	DDMetrics               statsd.ClientInterface
	KnownApps               map[string]map[string]*v1alpha1.Application
	//
	ExperimentalBracketsClusters []string
	// The apps that will be recreated as brackets.
	// We store them, so we can delete them only once the bracket is there.
	pendingDeletions []PendingDeletion
}

func New(appClient application.ApplicationServiceClient, manageArgoApplicationEnabled, kuberpultMetricsEnabled, argoAppsMetricsEnabled bool, manageArgoApplicationFilter []string, triggerChannelSize, argoAppsChannelSize int, ddMetrics statsd.ClientInterface, experimentalBracketsClusters []string) ArgoAppProcessor {
	return ArgoAppProcessor{
		ApplicationClient:            appClient,
		ManageArgoAppsEnabled:        manageArgoApplicationEnabled,
		ManageArgoAppsFilter:         manageArgoApplicationFilter,
		KuberpultMetricsEnabled:      kuberpultMetricsEnabled,
		ArgoAppsMetricsEnabled:       argoAppsMetricsEnabled,
		trigger:                      make(chan argoTrigger, triggerChannelSize),
		maxProcessedTransformerEslId: &atomic.Int64{},
		ArgoApps:                     make(chan *v1alpha1.ApplicationWatchEvent, argoAppsChannelSize),
		DDMetrics:                    ddMetrics,
		KnownApps:                    map[string]map[string]*v1alpha1.Application{},
		//
		ExperimentalBracketsClusters: experimentalBracketsClusters,
		pendingDeletions:             []PendingDeletion{},
	}
}

// MaxProcessedTransformerEslId returns a pointer to the atomic that tracks the
// highest transformer ESL ID for which ProcessArgoOverview has fully completed.
// The cascade consumer reads this to ensure it never cascade-deletes a bracket
// before the corresponding new bracket Argo Application has been created.
func (a *ArgoAppProcessor) MaxProcessedTransformerEslId() *atomic.Int64 {
	return a.maxProcessedTransformerEslId
}

// updateMaxProcessedEslId advances the max-processed ESL ID if eslId is larger.
// Only called from Consume (single writer), so a plain Load+Store is safe.
// This function must only be called when the processing for that eslID
// - meaning the update/create for argo apps - is already finished
func (a *ArgoAppProcessor) updateMaxProcessedEslId(eslId int64) {
	if eslId > a.maxProcessedTransformerEslId.Load() {
		a.maxProcessedTransformerEslId.Store(eslId)
	}
}

func (a *ArgoAppProcessor) GetManageArgoAppsFilter() []string {
	return a.ManageArgoAppsFilter
}

func (a *ArgoAppProcessor) GetManageArgoAppsEnabled() bool {
	return a.ManageArgoAppsEnabled
}

func (a *ArgoAppProcessor) Push(ctx context.Context, last *ArgoOverview, eslId int64) error {
	l := logger.FromContext(ctx).With(zap.String("argo-pushing", "ready"))
	select {
	case a.trigger <- argoTrigger{overview: last, eslId: eslId}:
		l.Info("argocd.pushed")
		a.GaugeKuberpultEventsQueueFillRate(ctx)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *ArgoAppProcessor) Consume(ctx context.Context, hlth *setup.HealthReporter, chPtr WriteOnceCh) error {
	if hlth != nil {
		hlth.ReportReady("event-consuming")
	}

	var alreadyWritten = false
	l := logger.FromContext(ctx).With(zap.String("self-manage", "consuming"))
	for {
		select {
		case t := <-a.trigger:
			l.Info("self-manage.trigger")
			a.ProcessArgoOverview(ctx, l, t.overview)
			a.updateMaxProcessedEslId(t.eslId)
			a.GaugeKuberpultEventsQueueFillRate(ctx)
		case <-ctx.Done():
			return nil
		default:
			select {
			case t := <-a.trigger:
				l.Info("self-manage.trigger")
				a.ProcessArgoOverview(ctx, l, t.overview)
				a.updateMaxProcessedEslId(t.eslId)
				a.GaugeKuberpultEventsQueueFillRate(ctx)
			case ev := <-a.ArgoApps:
				a.ProcessArgoWatchEvent(ctx, l, ev)
				a.GaugeArgoAppsQueueFillRate(ctx)
			case <-ctx.Done():
				return nil
			}
		}
		if !alreadyWritten && chPtr != nil {
			ch := *chPtr
			ch <- struct{}{}
			alreadyWritten = true
		}
	}
}

type ArgoOverview struct {
	AppDetails map[string]*api.GetAppDetailsResponse //Map from appName to app Details. Gets filled with information based on what apps have changed.
	Overview   *api.GetOverviewResponse              //Standard overview. Only information regarding environments should be retrieved from this overview.
}

func (a *ArgoAppProcessor) ProcessArgoOverview(ctx context.Context, l *zap.Logger, argoOv *ArgoOverview) {
	overview := argoOv.Overview
	for _, currentApp := range sorting.SortKeys(argoOv.AppDetails) {
		currentAppDetails := argoOv.AppDetails[currentApp]
		span, ctx := tracer.StartSpanFromContext(ctx, "ProcessChangedApp")
		defer span.Finish()
		span.SetTag("kuberpult-app", currentApp)
		for _, envGroup := range overview.EnvironmentGroups {
			for _, parentEnvironment := range envGroup.Environments {
				// isBracket must be per-environment: a single-app bracket (bracketName==appName)
				// is a bracket only in bracket envs; in non-bracket envs it is a regular app.
				isBracket := currentAppDetails.Application.ArgoBracket == currentApp && a.isBracketEnv(parentEnvironment.Name)
				if isAAEnv(parentEnvironment.Config) {
					for _, cfg := range parentEnvironment.Config.ArgoConfigs.Configs { //Active/Active environments have multiple argo cd configurations
						targetEnvName := a.extractFullyQualifiedEnvironmentName(parentEnvironment.Config.ArgoConfigs.CommonEnvPrefix, parentEnvironment.Name, cfg)
						appInfo := &AppInfo{
							ApplicationName:              currentApp,
							EnvironmentName:              targetEnvName,
							TeamName:                     currentAppDetails.Application.Team,
							ParentEnvironmentName:        parentEnvironment.Name,
							ArgoEnvironmentConfiguration: cfg,
							IsBracket:                    isBracket,
						}
						a.ProcessAppChange(ctx, appInfo, currentAppDetails, overview, argoOv.AppDetails)
					}
				} else {
					appInfo := &AppInfo{
						ApplicationName:              currentApp,
						EnvironmentName:              parentEnvironment.Name,
						TeamName:                     currentAppDetails.Application.Team,
						ParentEnvironmentName:        parentEnvironment.Name,
						ArgoEnvironmentConfiguration: parentEnvironment.Config.Argocd,
						IsBracket:                    isBracket,
					}
					a.ProcessAppChange(ctx, appInfo, currentAppDetails, overview, argoOv.AppDetails)
				}

			}
		}
		span.Finish()
	}
}

func (a *ArgoAppProcessor) extractFullyQualifiedEnvironmentName(commonPrefix, envName string, argoCDConfig *api.ArgoCDEnvironmentConfiguration) string {
	return commonPrefix + "-" + envName + "-" + argoCDConfig.ConcreteEnvName
}

func (a *ArgoAppProcessor) isBracketEnv(envName string) bool {
	return slices.Contains(a.ExperimentalBracketsClusters, envName)
}

func (a *ArgoAppProcessor) knowsBracketApp(envName string) bool {
	for _, appName := range sorting.SortKeys(a.KnownApps[envName]) {
		if a.KnownApps[envName][appName].Annotations["com.freiheit.kuberpult/is-bracket"] == "true" {
			return true
		}
	}
	return false
}

// ApplicationDeleter is the minimal subset of application.ApplicationServiceClient
// needed by DeleteApplication. Defined so non-argo packages (e.g. undeploy) can
// pass a small test mock without implementing the full client.
type ApplicationDeleter interface {
	Delete(ctx context.Context, in *application.ApplicationDeleteRequest, opts ...grpc.CallOption) (*application.ApplicationResponse, error)
}

// DeleteApplication is the single point in the rollout-service that calls the
// Argo CD Application Delete RPC. Every other code path (bracket migration,
// bracket move, no-cascade deletes inside argo.go, the cascade-true cleanup
// driven by the undeploy package) goes through here so cascade semantics live
// in one place and are easy to audit.
func DeleteApplication(ctx context.Context, client ApplicationDeleter, argoAppName string, cascadeDelete bool) error {
	cascade := cascadeDelete
	_, err := client.Delete(ctx, &application.ApplicationDeleteRequest{
		Cascade: &cascade,
		Name:    conversion.FromString(argoAppName),
	})
	return err
}

// deleteAppNoCascade deletes an ArgoCD Application object without pruning the k8s resources it manages.
// Used when transitioning an env to bracket mode: the bracket app takes over resource ownership,
// so the individual app object can be removed without touching the live k8s resources.
func (a *ArgoAppProcessor) deleteAppNoCascade(ctx context.Context, knownApps map[string]*v1alpha1.Application, appName string) error {
	argoApp := knownApps[appName]
	if argoApp == nil {
		return nil
	}
	logger.FromContext(ctx).Info("bracket.delete.no-cascade",
		zap.String("argo.app", argoApp.Name),
		zap.String("kuberpult.app", appName))
	return DeleteApplication(ctx, a.ApplicationClient, argoApp.Name, false)
}

// deleteAppNoCascadeByName deletes an ArgoCD Application by its constructed name without
// cascading to k8s resources. Used when the app exists in ArgoCD but its watch event has
// not yet been received (KnownApps cache is stale after rollout-service restart).
func (a *ArgoAppProcessor) deleteAppNoCascadeByName(ctx context.Context, argoAppName string) error {
	logger.FromContext(ctx).Info("bracket.delete.no-cascade.by-name",
		zap.String("argo.app", argoAppName))
	err := DeleteApplication(ctx, a.ApplicationClient, argoAppName, false)
	if err != nil && status.Code(err) != codes.NotFound {
		return err
	}
	return nil
}

/*
drainPendingDeletions deletes normal argo apps that have now been replaced by bracket apps.
*/
func (a *ArgoAppProcessor) drainPendingDeletions(ctx context.Context, bracketEnvName string) {
	l := logger.FromContext(ctx)
	remaining := a.pendingDeletions[:0]
	for _, pd := range a.pendingDeletions {
		// if the app belongs to the bracket:
		if pd.ParentEnvironmentName == bracketEnvName && a.knowsBracketApp(pd.ParentEnvironmentName) {
			l.Info("bracket.drain.pending",
				zap.String("app", pd.AppName),
				zap.String("env", pd.EnvironmentName))
			// Delete with cascade=false so the bracket takes over resource ownership.
			// If the watch event for this app hasn't arrived yet (e.g. because the rollout-service was restarted), fall back to deleting by constructed name.
			knownApps := a.KnownApps[pd.EnvironmentName]
			known := knownApps != nil && knownApps[pd.AppName] != nil
			l.Info("bracket.drain.attempt",
				zap.String("app", pd.AppName),
				zap.String("env", pd.EnvironmentName),
				zap.Bool("known", known))
			var err error
			if known {
				err = a.deleteAppNoCascade(ctx, knownApps, pd.AppName)
			} else {
				err = a.deleteAppNoCascadeByName(ctx, pd.EnvironmentName+"-"+pd.AppName)
			}
			if err != nil {
				code := status.Code(err)
				switch code {
				case codes.NotFound:
					l.Info("bracket.drain.already-gone",
						zap.String("app", pd.AppName),
						zap.String("env", pd.EnvironmentName),
						zap.String("code", code.String()))
				case codes.PermissionDenied:
					l.Warn("bracket.drain.already-gone",
						zap.String("app", pd.AppName),
						zap.String("env", pd.EnvironmentName),
						zap.String("code", code.String()))
				default:
					l.Error("bracket.drain.delete.failed", zap.String("app", pd.AppName), zap.Error(err))
					remaining = append(remaining, pd)
				}
				continue
			}
		} else {
			remaining = append(remaining, pd)
		}
	}
	a.pendingDeletions = remaining
}

func (a *ArgoAppProcessor) ProcessAppChange(ctx context.Context, appInfo *AppInfo, currentAppDetails *api.GetAppDetailsResponse, overview *api.GetOverviewResponse, allAppDetails map[string]*api.GetAppDetailsResponse) {
	logger.FromContext(ctx).Sugar().Debugf("Processing app %q on environment %q", appInfo.ApplicationName, appInfo.EnvironmentName)
	// Bracket-to-individual transition guard (rollback: staging switched from true→false).
	// When the existing KnownApp is a bracket (is-bracket=true) but IsBracket=false, we must
	// not let the normal delete path do a cascading delete (which would leave a deployment gap).
	if !appInfo.IsBracket {
		if knownEnvApps := a.KnownApps[appInfo.EnvironmentName]; knownEnvApps != nil {
			if existingApp := knownEnvApps[appInfo.ApplicationName]; existingApp != nil {
				if existingApp.Annotations["com.freiheit.kuberpult/is-bracket"] == "true" {
					if currentAppDetails.Deployments[appInfo.ParentEnvironmentName] == nil {
						// No deployment recorded for this environment yet. Leave the bracket as-is
						// until deployment data arrives.
						return
					}
					// Deployment data available: delete bracket without cascade so k8s resources
					// persist, then create the individual app in the same cycle.
					if err := a.deleteAppNoCascade(ctx, knownEnvApps, appInfo.ApplicationName); err != nil {
						logger.FromContext(ctx).Error("bracket.rollback.delete.failed",
							zap.String("app", appInfo.ApplicationName), zap.Error(err))
						return
					}
					delete(knownEnvApps, appInfo.ApplicationName)
				}
			}
		}
	}
	// For non-bracket apps in a bracket env: only delete once the bracket app is established in KnownApps.
	// This prevents a downtime gap when transitioning an env to bracket mode.
	allowDelete := appInfo.IsBracket ||
		!a.isBracketEnv(appInfo.ParentEnvironmentName) ||
		a.knowsBracketApp(appInfo.ParentEnvironmentName)
	logger.FromContext(ctx).Info("ProcessAppChange",
		zap.Bool("allow_delete", allowDelete),
		zap.Bool("isBracket", appInfo.IsBracket),
		zap.Bool("isBracketEnv", a.isBracketEnv(appInfo.ParentEnvironmentName)),
		zap.Bool("knowsBracket", a.knowsBracketApp(appInfo.ParentEnvironmentName)),
		zap.String("app", appInfo.ApplicationName),
		zap.String("env", appInfo.EnvironmentName))
	if allowDelete {
		if ok := a.KnownApps[appInfo.EnvironmentName]; ok != nil { //If argo does not know this application, delete it
			if !appInfo.IsBracket && a.isBracketEnv(appInfo.ParentEnvironmentName) {
				// Individual app in a bracket env: delete without cascade so k8s resources
				// remain under the bracket app's management.
				if err := a.deleteAppNoCascade(ctx, ok, appInfo.ApplicationName); err != nil {
					logger.FromContext(ctx).Error("bracket.individual.delete.failed",
						zap.String("app", appInfo.ApplicationName), zap.Error(err))
				}
			} else {
				// Bracket move detection: if another bracket has a deployment for the same env,
				// delete without cascade so the new bracket takes over k8s resource ownership.
				noCascade := false
				if appInfo.IsBracket {
					for _, otherApp := range sorting.SortKeys(allAppDetails) {
						otherDetails := allAppDetails[otherApp]
						if otherApp != appInfo.ApplicationName &&
							otherDetails.Application != nil &&
							otherDetails.Application.ArgoBracket == otherApp &&
							otherDetails.Deployments[appInfo.ParentEnvironmentName] != nil {
							noCascade = true
							break
						}
					}
				}
				if noCascade {
					if err := a.deleteAppNoCascade(ctx, ok, appInfo.ApplicationName); err != nil {
						logger.FromContext(ctx).Error("bracket.move.delete.failed",
							zap.String("app", appInfo.ApplicationName), zap.Error(err))
					}
				} else if !appInfo.IsBracket {
					// Non-bracket app with no deployment: cascade=false safety net for a
					// transient cd-service overview (e.g. mid helm-upgrade).
					a.DeleteArgoApps(ctx, ok, appInfo.ApplicationName, currentAppDetails.Deployments[appInfo.ParentEnvironmentName])
				}
				// Else: bracket with no deployment AND not a move. Do NOT delete here.
				// The rollout_should_undeploy_cascade table is the single authority for
				// removing a bracket together with its workload (cascade=true). A
				// no-cascade delete from here would beat the ESL-gated cascade=true
				// consumer to the punch — the app object goes, the cascade=true call
				// then gets NotFound, and the k8s Deployment is orphaned.
			}
		}
	} else {
		// Bracket not yet confirmed; defer deletion until its watch ADDED event arrives.
		// Guard against duplicates so a second overview before drain doesn't double-queue.
		alreadyPending := false
		for _, existing := range a.pendingDeletions {
			if existing.AppName == appInfo.ApplicationName && existing.ParentEnvironmentName == appInfo.ParentEnvironmentName {
				alreadyPending = true
				break
			}
		}
		if !alreadyPending {
			logger.FromContext(ctx).Info("bracket.defer.deletion",
				zap.String("app", appInfo.ApplicationName),
				zap.String("env", appInfo.EnvironmentName))
			a.pendingDeletions = append(a.pendingDeletions, PendingDeletion{
				EnvironmentName:       appInfo.EnvironmentName,
				ParentEnvironmentName: appInfo.ParentEnvironmentName,
				AppName:               appInfo.ApplicationName,
			})
		}
	}

	if currentAppDetails.Deployments[appInfo.ParentEnvironmentName] != nil { //If there is a deployment for this app on this environment
		argoApp := a.isKnownArgoApp(appInfo.ApplicationName, appInfo.EnvironmentName, a.KnownApps[appInfo.EnvironmentName])
		if argoApp == nil {
			a.CreateArgoApp(ctx, overview, appInfo)
		} else {
			a.UpdateArgoApp(ctx, overview, appInfo, argoApp)
		}

	}
}

// is AAEnv
// Note that there is also a function IsAAEnv in config.go for a similar type.
// Keep them in sync.
func isAAEnv(config *api.EnvironmentConfig) bool {
	if config.IsActiveActive != nil {
		return *config.IsActiveActive
	}
	// for backwards compatibility:
	if config.ArgoConfigs == nil {
		return false
	}
	return config.ArgoConfigs.CommonEnvPrefix != ""
}

func (a *ArgoAppProcessor) ProcessArgoWatchEvent(ctx context.Context, l *zap.Logger, ev *v1alpha1.ApplicationWatchEvent) {
	envName, appName := getEnvironmentAndName(ev.Application.Annotations)
	if appName == "" {
		return
	}
	if a.KnownApps[envName] == nil {
		a.KnownApps[envName] = map[string]*v1alpha1.Application{}
	}
	switch ev.Type {
	case "ADDED", "MODIFIED":
		l.Info("created/updated:kuberpult.application:"+ev.Application.Name+",kuberpult.environment:"+envName,
			zap.String("sync", string(ev.Application.Status.Sync.Status)),
			zap.String("health", string(ev.Application.Status.Health.Status)))
		a.KnownApps[envName][appName] = &ev.Application
		if ev.Application.Annotations["com.freiheit.kuberpult/is-bracket"] == "true" {
			l.Info("bracket.watch.event",
				zap.String("type", string(ev.Type)),
				zap.String("argo.app", ev.Application.Name),
				zap.String("env", envName),
				zap.String("sync", string(ev.Application.Status.Sync.Status)),
				zap.String("health", string(ev.Application.Status.Health.Status)),
				zap.Int("pending.deletions", len(a.pendingDeletions)))
			a.drainPendingDeletions(ctx, envName)
		}
	case "DELETED":
		l.Info("deleted:kuberpult.application:" + ev.Application.Name + ",kuberpult.environment:" + envName)
		delete(a.KnownApps[envName], appName)
	}
}

type AppInfo struct {
	ApplicationName              string
	TeamName                     string
	EnvironmentName              string
	ParentEnvironmentName        string
	ArgoEnvironmentConfiguration *api.ArgoCDEnvironmentConfiguration
	IsBracket                    bool
}

func (a *ArgoAppProcessor) isKnownArgoApp(appName, envName string, appsKnownToArgo map[string]*v1alpha1.Application) *v1alpha1.Application {
	for _, key := range sorting.SortKeys(appsKnownToArgo) {
		argoApp := appsKnownToArgo[key]
		if argoApp.Annotations["com.freiheit.kuberpult/application"] == appName && argoApp.Annotations["com.freiheit.kuberpult/environment"] == envName {
			return argoApp
		}
	}
	return nil
}

func (a *ArgoAppProcessor) CreateArgoApp(ctx context.Context, overview *api.GetOverviewResponse, appInfo *AppInfo) {
	selfManaged, err := IsSelfManagedFilterActive(appInfo.TeamName, a)
	if err != nil {
		logger.FromContext(ctx).Error("detecting self manage:", zap.Error(err))
	}
	if selfManaged {
		createSpan, ctx := tracer.StartSpanFromContext(ctx, "CreateApplication")
		createSpan.SetTag("application", appInfo.ApplicationName)
		createSpan.SetTag("environment", appInfo.EnvironmentName)
		createSpan.SetTag("operation", "create")
		appToCreate := CreateArgoApplication(overview, appInfo)
		appToCreate.ResourceVersion = ""
		upsert := false
		validate := false
		appCreateRequest := &application.ApplicationCreateRequest{
			XXX_NoUnkeyedLiteral: struct{}{},
			XXX_unrecognized:     nil, //nolint:misspell
			XXX_sizecache:        0,
			Application:          appToCreate,
			Upsert:               &upsert,
			Validate:             &validate,
		}
		_, err := a.ApplicationClient.Create(ctx, appCreateRequest)
		if err != nil {
			// We check if the application was created in the meantime
			if status.Code(err) != codes.InvalidArgument {

				logger.FromContext(ctx).Sugar().Errorf("creating %s, env %s: %v", appToCreate.Name, appInfo.EnvironmentName, err)
			}
		}
		createSpan.Finish()
	}
}

func (a *ArgoAppProcessor) UpdateArgoApp(ctx context.Context, overview *api.GetOverviewResponse, appInfo *AppInfo, existingApp *v1alpha1.Application) {
	appToUpdate := CreateArgoApplication(overview, appInfo)
	// Preserve whatever SyncPolicy the operator has set on the live app (e.g. nil when auto-sync was disabled manually).
	appToUpdate.Spec.SyncPolicy = existingApp.Spec.SyncPolicy
	appUpdateRequest := &application.ApplicationUpdateRequest{
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     nil, //nolint:misspell
		XXX_sizecache:        0,
		Validate:             conversion.Bool(false),
		Application:          appToUpdate,
		Project:              conversion.FromString(appToUpdate.Spec.Project),
	}

	//exhaustruct:ignore
	diff := cmp.Diff(appUpdateRequest.Application.Spec, existingApp.Spec,
		cmp.AllowUnexported(v1alpha1.ApplicationDestination{}))
	if diff != "" {
		logger.FromContext(ctx).Info("UpdateArgoApp",
			zap.String("diff", diff),
			zap.Any("newSpec", appUpdateRequest.Application.Spec),
			zap.Any("existingSpec", existingApp.Spec),
		)
		updateSpan, ctx := tracer.StartSpanFromContext(ctx, "UpdateApplications")
		updateSpan.SetTag("application", appInfo.ApplicationName)
		updateSpan.SetTag("environment", appInfo.EnvironmentName)
		updateSpan.SetTag("operation", "update")
		updateSpan.SetTag("argoDiff", diff)
		_, err := a.ApplicationClient.Update(ctx, appUpdateRequest)
		if err != nil {
			logger.FromContext(ctx).Error("updating application: "+appToUpdate.Name+",env "+appInfo.EnvironmentName, zap.Error(err))
		}
		updateSpan.Finish()
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
	// No finalizers: workload cleanup goes through Argo CD's automated sync
	// (prune=true) when manifests disappear from the source, and through the
	// explicit cd-service → rollout_should_undeploy_cascade table → consumer
	// path that issues a cascade=true delete from the undeploy package. The
	// resources-finalizer is unnecessary and only made flaky helm-upgrade
	// races destroy workload Deployments.
	return nil
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
		// Cascade=false here is a safety net: this path can fire on a transient
		// "deployment == nil" in the cd-service overview (e.g. while the
		// cd-service is being helm-upgraded). Cascading delete on a transient
		// signal would destroy the workload Deployment we are trying to
		// protect. Workload cleanup on a *real* undeploy goes through the
		// rollout_should_undeploy_cascade DB table consumed by the undeploy
		// package, which issues cascade=true with explicit cd-service intent.
		logger.FromContext(ctx).Info("argo.delete.no-cascade",
			zap.String("argo.app", toDelete[i].Name),
			zap.String("kuberpult.app", appName))
		if err := DeleteApplication(ctx, a.ApplicationClient, toDelete[i].Name, false); err != nil {
			logger.FromContext(ctx).Error("deleting application: "+toDelete[i].Name, zap.Error(err))
		}
		deleteAppSpan.Finish()
	}
}

func CreateArgoApplication(overview *api.GetOverviewResponse, appInfo *AppInfo) *v1alpha1.Application {
	applicationNs := ""

	annotations := make(map[string]string)
	labels := make(map[string]string)

	var manifestPath string
	if appInfo.IsBracket {
		manifestPath = filepath.Join("environments", appInfo.ParentEnvironmentName, "brackets", appInfo.ApplicationName)
	} else {
		manifestPath = filepath.Join("environments", appInfo.ParentEnvironmentName, "applications", appInfo.ApplicationName, "manifests")
	}

	annotations["com.freiheit.kuberpult/application"] = appInfo.ApplicationName
	annotations["com.freiheit.kuberpult/environment"] = appInfo.EnvironmentName
	annotations["com.freiheit.kuberpult/aa-parent-environment"] = appInfo.ParentEnvironmentName
	annotations["com.freiheit.kuberpult/self-managed"] = "true"
	if appInfo.IsBracket {
		annotations["com.freiheit.kuberpult/is-bracket"] = "true"
	}
	// This annotation is so that argoCd does not invalidate *everything* in the whole repo when receiving a git webhook.
	// It has to start with a "/" to be absolute to the git repo.
	// See https://argo-cd.readthedocs.io/en/stable/operator-manual/high_availability/#webhook-and-manifest-paths-annotation
	annotations["argocd.argoproj.io/manifest-generate-paths"] = "/" + manifestPath
	labels["com.freiheit.kuberpult/team"] = appInfo.TeamName

	if appInfo.ArgoEnvironmentConfiguration.Destination.Namespace != nil {
		applicationNs = *appInfo.ArgoEnvironmentConfiguration.Destination.Namespace
	} else if appInfo.ArgoEnvironmentConfiguration.Destination.ApplicationNamespace != nil {
		applicationNs = *appInfo.ArgoEnvironmentConfiguration.Destination.ApplicationNamespace
	}

	applicationDestination := v1alpha1.ApplicationDestination{
		Name:      appInfo.ArgoEnvironmentConfiguration.Destination.Name,
		Namespace: applicationNs,
		Server:    appInfo.ArgoEnvironmentConfiguration.Destination.Server,
	}

	var ignoreDifferences []v1alpha1.ResourceIgnoreDifferences = nil
	if len(appInfo.ArgoEnvironmentConfiguration.IgnoreDifferences) > 0 {
		ignoreDifferences = make([]v1alpha1.ResourceIgnoreDifferences, len(appInfo.ArgoEnvironmentConfiguration.IgnoreDifferences))
		for index, value := range appInfo.ArgoEnvironmentConfiguration.IgnoreDifferences {
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
		Name:        fmt.Sprintf("%s-%s", appInfo.EnvironmentName, appInfo.ApplicationName),
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
			// For brackets, AllowEmpty=false is deliberate: it keeps Argo CD's auto-sync from pruning a
			// bracket down to zero resources when the bracket's source becomes empty (e.g. its only app
			// moved to another bracket). That auto-prune-to-empty is what caused workload downtime on a
			// bracket move. Whole-bracket resource removal is instead an explicit, kuberpult-decided
			// cascade delete via the rollout_should_undeploy_cascade table. Pruning of individual
			// resources within a still-populated bracket is unaffected (the bracket is not empty).
			// Non-bracket apps keep AllowEmpty=true (it makes deleting apps/environments easier).
			AllowEmpty: !appInfo.IsBracket,
		},
		SyncOptions: appInfo.ArgoEnvironmentConfiguration.SyncOptions,
	}
	//exhaustruct:ignore
	Spec := v1alpha1.ApplicationSpec{
		Source:            Source,
		SyncPolicy:        SyncPolicy,
		Project:           appInfo.EnvironmentName,
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
