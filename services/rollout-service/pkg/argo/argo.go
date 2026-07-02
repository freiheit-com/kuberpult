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

/*
Package argo manages kuberpult's Argo CD Applications from the rollout-service.

# How the rollout is intended to work (especially for brackets) - high level description
This section describes the main rules for the rollout-service.
As such it touches on more than just what happens in this module, but also in the cd-service (overview).

Overarching goal: a running workload must never go down because of a bracket transition, and the
rollout-service must converge to the correct Argo CD state even after missed messages or restarts.

For non-bracket apps, kuberpult manages all app creations and deletions,
including the cascade=true option to remove the k8s resources on undeploy.
Kuberpult's rollout-service decides about all deletions of argo apps.
However, what is actually part of a kuberpult app's manifest is up the operator.

For bracket-apps, this is also true, however, when switching from brackets
back to apps, we do not (necessarily) need to remove the bracket itself, the
operator can do that.

The cd-service is responsible to send the right brackets to the rollout-service. This includes
CHANGING brackets. Each change is sent once, as a fast path.

Reliability does not depend on never missing a message: a deletion (e.g. an emptied bracket) is the
absence of something and cannot be re-derived from a resync. So the rollout-service must converge by
reconciling — comparing the brackets that should exist (from the overview) with the Argo apps that
actually exist, and removing the stragglers — rather than relying solely on the one-shot delete event.
After a restart, this reconciliation is what restores correctness. However,
in some error cases, an empty bracket app may not be removed completely.

The brackets_history table defines which brackets exist GLOBALLY (by member-app membership). A
per-env bracket Argo Application (<env>-<bracket>) is, however, only required when at least one
member of the bracket has a deployment in that env. When the last deployment of a bracket in env
E disappears, kuberpult cascade-deletes the per-env bracket Argo app — removing its workload —
while the bracket itself persists in brackets_history for future deployments. The Argo Application
object is not the source of truth; the workload's existence is.

Kuberpult ensures that the k8s deployment is not deleted, for example when:
* A service moves to another bracket.
* An environment is now configured with bracketMode=true.
* An environment is now configured with bracketMode=false.
* ...

The only reasons to delete a k8s deployment is when
* the service is deleted in kuberpult
* the environment of the service is deleted in kuberpult (delete env from app)
* the services manifests literally does not contain the deployment anymore (but this is outside our control).

# Implementation Details

* All resource-removing (cascade=true) deletions of Argo Applications go through one place: the
  rollout_should_undeploy_cascade table, written by the cd-service and consumed by the ESL-gated
  undeploy worker. The is_bracket column says whether the row targets an individual app or a
  bracket. Argo CD's auto-sync is never the actor that removes a whole bracket's workload.
* Bracket Argo apps are created with Automated{Prune:true, SelfHeal:true, AllowEmpty:false}.
  Prune:true reconciles individual resource changes within a populated bracket. AllowEmpty:false
  prevents Argo CD from auto-pruning a bracket down to zero resources — the prune-to-empty that
  caused workload downtime on bracket moves. Non-bracket apps keep AllowEmpty:true.
* The brackets_history table defines which brackets exist globally. A per-env bracket Argo
  Application exists iff at least one member has a deployment in that env. When the last such
  deployment goes away (undeploy of the last member, or DeleteEnvFromApp on the last member's
  only env), kuberpult cascade-deletes the per-env bracket Argo app via the cascade table.
* combineBracketDeployments (cd-service/pkg/service/overview.go) emits the BracketVersionDelete
  sentinel for env E when no member has a deployment in E, so the rollout-service tears down the
  per-env bracket Argo app instead of recreating it.
* The pause protocol (prune-vs-adopt protection on bracket moves): while a bracket that lost a
  member still pins its old snapshot, BOTH the losing and the gaining bracket render the moved
  app's manifests — each apply steals Argo's resource tracking, the other side goes OutOfSync
  and re-steals after sync backoff, and a spec refresh of the loser during such a flap prunes
  the moved app's workload. The cd-service names the gainers in
  GetBracketDetailsResponse.lost_members_to; for such a loser, ProcessAppChange replaces the
  normal spec refresh with a three-step handover (pendingSpecUpdates, advanced on watch events):
  1. PAUSE in place — auto-sync disabled, manifest path unchanged. An Argo sync operation
     executes against the spec path read at execution time (with the prune flag from its
     initiation), so the path must not move while an operation can be running.
  2. RETARGET — once the app is provably quiet (controller reconciled the paused spec: the
     refresh annotation requested by the pause is gone, and no operation is requested or
     running), the path moves to the new snapshot, still paused.
  3. RESUME — auto-sync is restored only once the loser's own sync status (compared against the
     new path) reports no resource as requiring pruning — derived from the same cluster cache
     that computes prune tasks, so the resume is provably prune-free.
  The paused-for-move marker annotation records the phase, driving recovery after a
  rollout-service restart via the watch ADDED replay (a loser recovered in the paused phase
  stays paused until the next overview tick rebuilds the retarget payload). Known limitation:
  an unrelated member resource legitimately requiring pruning during the move window blocks
  the resume ("bracket.resume.blocked" log) until a later change resolves it; the workload is
  unaffected.

*/

package argo

import (
	"context"
	"database/sql"
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
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/pkg/sorting"
)

// BracketPausedForMoveAnnotation marks a bracket Argo app whose auto-sync has been
// temporarily disabled while another bracket adopts members it lost (bracket move).
// Its value is the protocol phase (BracketMovePhase*), which makes the paused state
// recoverable after a rollout-service restart via the watch ADDED replay.
const BracketPausedForMoveAnnotation = "com.freiheit.kuberpult/paused-for-move"

// The pause-protocol phases stored as BracketPausedForMoveAnnotation values.
const (
	// BracketMovePhasePaused: auto-sync disabled, manifest path still the old
	// snapshot. An in-flight sync operation executes against the spec path at
	// execution time, so the path may only move once the app is provably quiet.
	BracketMovePhasePaused = "paused"
	// BracketMovePhaseRetargeted: still paused, manifest path moved to the new
	// snapshot; waiting for the disown before auto-sync is restored.
	BracketMovePhaseRetargeted = "retargeted"
)

// argocdRefreshAnnotation requests an app reconcile from the Argo CD application
// controller, which removes the annotation when it processes the request. The
// pause update sets it so the quiet check can prove the controller has reconciled
// the paused spec (per-app reconciles are serialised, so once it is removed no
// reconcile of the pre-pause spec can still initiate a sync operation).
const argocdRefreshAnnotation = "argocd.argoproj.io/refresh"

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
	DeleteArgoApps(ctx context.Context, argoApps map[string]*v1alpha1.Application, appName, envName, parentEnvName string, deployment *api.Deployment)
	GetManageArgoAppsFilter() []string
	GetManageArgoAppsEnabled() bool
}

type PendingDeletion struct {
	EnvironmentName       string // key into KnownApps for the DeleteArgoApps call
	ParentEnvironmentName string // key for isBracketEnv / knowsBracketApp check
	AppName               string
}

// PendingSpecUpdate tracks a bracket Argo app that lost members to other brackets
// (a bracket move) and is going through the pause protocol (see the package
// comment): paused in place, retargeted to the new snapshot once quiet, and
// resumed once its own sync status — compared against ExpectedPath — no longer
// attributes any resource as requiring pruning, i.e. the gainers have adopted the
// moved resources and the controller's cluster cache has registered it.
type PendingSpecUpdate struct {
	EnvironmentName string
	ApplicationName string
	// ExpectedPath is the manifest path the app is (or will be) retargeted to.
	ExpectedPath string
	// Phase is BracketMovePhasePaused (waiting for quiet, then send Retarget) or
	// BracketMovePhaseRetargeted (waiting for the disown, then resume).
	Phase string
	// Retarget is the prepared paused application at the new path. Nil after a
	// restart recovery in the paused phase — rebuilt by the next overview tick.
	Retarget *v1alpha1.Application
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
	// DBHandler is read by ProcessArgoOverview to look up the current brackets_history
	// snapshot so its source_transformer_esl_id can be embedded in each bracket Argo CD
	// app's Spec.Source.Path (see CreateArgoApplication). Nil in unit tests; non-nil in
	// production. When nil, the legacy path format (no @<esl_id> suffix) is emitted.
	DBHandler *db.DBHandler
	//
	ExperimentalBracketsClusters []string
	// The apps that will be recreated as brackets.
	// We store them, so we can delete them only once the bracket is there.
	pendingDeletions []PendingDeletion
	// Brackets paused for a member move, waiting to have auto-sync restored
	// once they no longer own the moved resources (see PendingSpecUpdate).
	pendingSpecUpdates []*PendingSpecUpdate
}

func New(appClient application.ApplicationServiceClient, manageArgoApplicationEnabled, kuberpultMetricsEnabled, argoAppsMetricsEnabled bool, manageArgoApplicationFilter []string, triggerChannelSize, argoAppsChannelSize int, ddMetrics statsd.ClientInterface, experimentalBracketsClusters []string, dbHandler *db.DBHandler) ArgoAppProcessor {
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
		DBHandler:                    dbHandler,
		//
		ExperimentalBracketsClusters: experimentalBracketsClusters,
		pendingDeletions:             []PendingDeletion{},
		pendingSpecUpdates:           []*PendingSpecUpdate{},
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
	// LostMembersTo maps a bracket name to the brackets that gained members it
	// lost in this change (GetBracketDetailsResponse.lost_members_to). The
	// loser's Argo app spec refresh is deferred until those gainers are Synced.
	LostMembersTo map[string][]string
}

func (a *ArgoAppProcessor) ProcessArgoOverview(ctx context.Context, l *zap.Logger, argoOv *ArgoOverview) {
	overview := argoOv.Overview
	// All bracket Argo CD apps emitted from this overview tick should pin the same
	// brackets_history snapshot, so the reposerver can read the exact app list each
	// bracket was last spec-updated against. Read it once up-front.
	bracketSnapshotEslId := a.lookupBracketSnapshotEslId(ctx, l)
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
							BracketSnapshotEslId:         bracketSnapshotEslId,
							LostMembersTo:                argoOv.LostMembersTo[currentApp],
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
						BracketSnapshotEslId:         bracketSnapshotEslId,
						LostMembersTo:                argoOv.LostMembersTo[currentApp],
					}
					a.ProcessAppChange(ctx, appInfo, currentAppDetails, overview, argoOv.AppDetails)
				}

			}
		}
		span.Finish()
	}
}

// lookupBracketSnapshotEslId returns the source_transformer_esl_id of the latest
// brackets_history row, or 0 if the lookup is skipped (no DB handler) or fails
// (no rows, or DB error). A zero value triggers the legacy path format in
// CreateArgoApplication, which the reposerver still understands.
func (a *ArgoAppProcessor) lookupBracketSnapshotEslId(ctx context.Context, l *zap.Logger) db.TransformerID {
	if a.DBHandler == nil {
		return 0
	}
	var result db.TransformerID
	err := a.DBHandler.WithTransaction(ctx, true, func(ctx context.Context, tx *sql.Tx) error {
		row, err := db.DBSelectBracketHistoryLatest(ctx, a.DBHandler, tx)
		if err != nil {
			return err
		}
		if row != nil {
			result = row.SourceTransformerEslId
		}
		return nil
	})
	if err != nil {
		l.Warn("brackets.history.lookup.failed", zap.Error(err))
		return 0
	}
	return result
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
	drainSpan, ctx := tracer.StartSpanFromContext(ctx, "DrainPendingDeletions")
	defer drainSpan.Finish()
	l := logger.FromContext(ctx)

	drainSpan.SetTag("pendingDeletionsBefore", len(a.pendingDeletions))
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

	l.Info("pendingDeletion",
		zap.Int("pendingDeletionsBefore", len(a.pendingDeletions)),
		zap.Int("pendingDeletionsAfter", len(remaining)))

	a.pendingDeletions = remaining
	drainSpan.SetTag("pendingDeletionsAfter", len(remaining))
}

// bracketDisowned reports whether the paused bracket app's sync status —
// computed against expectedPath — no longer attributes any resource as
// requiring pruning. RequiresPruning is derived from the same cluster cache the
// application controller computes prune tasks from, so once it is clear,
// re-enabling auto-sync cannot prune the moved resources. The path check guards
// against reading a stale status from before the retarget (whose desired
// manifests still contained the moved apps — "nothing to prune" for the wrong
// reason).
func bracketDisowned(app *v1alpha1.Application, expectedPath string) string {
	if app.Status.Sync.ComparedTo.Source.Path != expectedPath {
		return fmt.Sprintf("source path mismatch %s != %s", app.Status.Sync.ComparedTo.Source.Path, expectedPath)
	}
	for _, res := range app.Status.Resources {
		if res.RequiresPruning {
			return fmt.Sprintf("resource %s/%s/%s requires pruning", res.Namespace, res.Kind, res.Name)
		}
	}
	return ""
}

func (a *ArgoAppProcessor) findPendingSpecUpdate(envName, appName string) *PendingSpecUpdate {
	for _, pu := range a.pendingSpecUpdates {
		if pu.EnvironmentName == envName && pu.ApplicationName == appName {
			return pu
		}
	}
	return nil
}

func (a *ArgoAppProcessor) removePendingSpecUpdate(envName, appName string) {
	remaining := a.pendingSpecUpdates[:0]
	for _, pu := range a.pendingSpecUpdates {
		if pu.EnvironmentName != envName || pu.ApplicationName != appName {
			remaining = append(remaining, pu)
		}
	}
	a.pendingSpecUpdates = remaining
}

// bracketPausedAndQuiet reports whether the watched app provably can no longer
// start a sync operation: the spec is paused (no automated sync policy), the
// controller has reconciled the paused spec — it removed the refresh annotation
// the pause update requested, and per-app reconciles are serialised, so no
// reconcile of the pre-pause spec can still initiate an operation — and no
// requested or running operation remains. Only then is it safe to move the
// manifest path: an in-flight sync operation executes against the spec path at
// execution time, with the prune flag it was initiated with.
func bracketPausedAndQuiet(app *v1alpha1.Application) string {
	if app.Spec.SyncPolicy != nil && app.Spec.SyncPolicy.Automated != nil {
		return "automated sync policy exists"
	}
	if _, refreshPending := app.Annotations[argocdRefreshAnnotation]; refreshPending {
		return "refresh pending"
	}
	if app.Operation != nil {
		return "requested operation exists"
	}
	if state := app.Status.OperationState; state != nil {
		if state.Phase == "Running" || state.Phase == "Terminating" {
			return "operation running"
		}
	}
	return ""
}

// recoverPendingSpecUpdate re-enters the pause protocol for an app carrying the
// paused-for-move marker when no pending entry exists for it (e.g. after a
// rollout-service restart). In the retargeted phase the disown-wait condition is
// fully derivable from the app's status; in the paused phase the retarget
// payload is gone and only the next overview tick can rebuild it — until then
// the app stays safely paused. Returns true when the app is in the protocol.
func (a *ArgoAppProcessor) recoverPendingSpecUpdate(ctx context.Context, envName, appName string, app *v1alpha1.Application) bool {
	marker := app.Annotations[BracketPausedForMoveAnnotation]
	if marker == "" {
		return false
	}
	if a.findPendingSpecUpdate(envName, appName) != nil {
		return true
	}
	expectedPath := ""
	phase := BracketMovePhaseRetargeted
	if marker == BracketMovePhasePaused {
		phase = BracketMovePhasePaused
	} else if app.Spec.Source != nil {
		expectedPath = app.Spec.Source.Path
	}
	logger.FromContext(ctx).Info("bracket.move.pause.recovered",
		zap.String("app", appName),
		zap.String("env", envName),
		zap.String("phase", phase),
		zap.String("expected-path", expectedPath))
	a.pendingSpecUpdates = append(a.pendingSpecUpdates, &PendingSpecUpdate{
		EnvironmentName: envName,
		ApplicationName: appName,
		ExpectedPath:    expectedPath,
		Phase:           phase,
		Retarget:        nil,
	})
	return true
}

// upsertExistingArgoApp resolves a create conflict: the app already exists in
// Argo CD but its watch event has not arrived yet, so it is missing from
// KnownApps and ProcessAppChange took the create path. Dropping the change
// would pin the app to its old spec forever — the fast path sends each change
// exactly once — so the desired spec is applied as an update instead. An app
// paused for a bracket move is handed back to the pause protocol.
func (a *ArgoAppProcessor) upsertExistingArgoApp(ctx context.Context, appInfo *AppInfo, desired *v1alpha1.Application) {
	//exhaustruct:ignore
	existing, err := a.ApplicationClient.Get(ctx, &application.ApplicationQuery{Name: conversion.FromString(desired.Name)})
	if err != nil {
		logger.FromContext(ctx).Error("argo.create.conflict.get.failed",
			zap.String("argo.app", desired.Name), zap.Error(err))
		return
	}
	if a.recoverPendingSpecUpdate(ctx, appInfo.EnvironmentName, appInfo.ApplicationName, existing) {
		return
	}
	_ = a.updateApplication(ctx, desired, "argo.create.conflict.update")
}

// isGoneErr reports whether an application RPC failed because the app does not
// exist. Argo CD reports operations on unknown apps as PermissionDenied (it
// hides existence from unauthorised callers), so both codes mean "gone" here.
func isGoneErr(err error) bool {
	code := status.Code(err)
	return code == codes.NotFound || code == codes.PermissionDenied
}

// createPausedApplication creates (upsert) an application in its paused
// pause-protocol state, e.g. when the loser's app object is unknown or has
// vanished underneath the protocol. A fresh app with the same name re-owns any
// still-labelled k8s resources, so it must not sync before the disown either.
func (a *ArgoAppProcessor) createPausedApplication(ctx context.Context, paused *v1alpha1.Application) {
	upsert := true
	validate := false
	appCreateRequest := &application.ApplicationCreateRequest{
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     nil, //nolint:misspell
		XXX_sizecache:        0,
		Application:          paused,
		Upsert:               &upsert,
		Validate:             &validate,
	}
	logger.FromContext(ctx).Info("bracket.move.pause.create",
		zap.String("argo.app", paused.Name))
	if _, err := a.ApplicationClient.Create(ctx, appCreateRequest); err != nil {
		logger.FromContext(ctx).Error("bracket.move.pause.create.failed",
			zap.String("argo.app", paused.Name), zap.Error(err))
	}
}

// updateApplication sends an application update for the pause protocol and logs it.
func (a *ArgoAppProcessor) updateApplication(ctx context.Context, app *v1alpha1.Application, logMsg string) error {
	path := ""
	if app.Spec.Source != nil {
		path = app.Spec.Source.Path
	}
	logger.FromContext(ctx).Info(logMsg,
		zap.String("argo.app", app.Name),
		zap.String("path", path))
	appUpdateRequest := &application.ApplicationUpdateRequest{
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     nil, //nolint:misspell
		XXX_sizecache:        0,
		Validate:             conversion.Bool(false),
		Application:          app,
		Project:              conversion.FromString(app.Spec.Project),
	}
	_, err := a.ApplicationClient.Update(ctx, appUpdateRequest)
	if err != nil {
		logger.FromContext(ctx).Error(logMsg+".failed", zap.String("argo.app", app.Name), zap.Error(err))
	}
	return err
}

// deferSpecUpdateIfWaiting implements the PAUSE step of the pause protocol for a
// bracket that lost members to other brackets (a bracket move). Instead of the
// normal spec refresh — whose auto-sync would prune the moved apps' resources
// while Argo's tracking ownership is still in flux — the bracket's auto-sync is
// disabled in place (path unchanged) and the prepared retarget to the new
// snapshot is stored on the pending entry; drainPendingSpecUpdates applies it
// once the app is provably quiet. Returns true when the normal create/update
// must be skipped. A bracket with a pending entry always stays in the protocol,
// even when a later tick carries no member loss, so sequential moves cannot
// bypass an unfinished handover.
func (a *ArgoAppProcessor) deferSpecUpdateIfWaiting(ctx context.Context, appInfo *AppInfo, overview *api.GetOverviewResponse) bool {
	existing := a.findPendingSpecUpdate(appInfo.EnvironmentName, appInfo.ApplicationName)
	if existing == nil && len(appInfo.LostMembersTo) == 0 {
		return false
	}
	retarget := CreateArgoApplication(overview, appInfo)
	retarget.Spec.SyncPolicy.Automated = nil
	retarget.Annotations[BracketPausedForMoveAnnotation] = BracketMovePhaseRetargeted
	path := retarget.Spec.Source.Path
	if existing != nil {
		// Merge: the newest payload wins. If the target moved on after the
		// retarget was already applied, re-send it so the disown check matches
		// the spec on the cluster.
		resend := existing.Phase == BracketMovePhaseRetargeted && existing.ExpectedPath != path
		existing.Retarget = retarget
		existing.ExpectedPath = path
		if resend {
			if err := a.updateApplication(ctx, retarget, "bracket.move.retarget"); err != nil && isGoneErr(err) {
				// The app vanished underneath the protocol (e.g. it was emptied
				// and deleted while its DELETED watch event is still queued
				// behind this burst of overview ticks). Recreate it paused at
				// the target path; the drain resumes it once its own status
				// shows nothing to prune.
				a.createPausedApplication(ctx, retarget)
			}
		}
		return true
	}
	logger.FromContext(ctx).Info("bracket.move.pause",
		zap.String("app", appInfo.ApplicationName),
		zap.String("env", appInfo.EnvironmentName),
		zap.String("target-path", path),
		zap.Strings("lost-members-to", appInfo.LostMembersTo))
	argoApp := a.isKnownArgoApp(appInfo.ApplicationName, appInfo.EnvironmentName, a.KnownApps[appInfo.EnvironmentName])
	if argoApp == nil {
		// The loser's Argo app object is unknown (e.g. lost watch state after a
		// restart). Create it directly in the retargeted paused state: a fresh
		// app has no in-flight sync operations.
		a.createPausedApplication(ctx, retarget)
		a.pendingSpecUpdates = append(a.pendingSpecUpdates, &PendingSpecUpdate{
			EnvironmentName: appInfo.EnvironmentName,
			ApplicationName: appInfo.ApplicationName,
			ExpectedPath:    path,
			Phase:           BracketMovePhaseRetargeted,
			Retarget:        retarget,
		})
		return true
	}
	// PAUSE in place: disable auto-sync without touching the path — an in-flight
	// sync operation executes against the spec path at execution time, so the
	// path may only move once the app is provably quiet (bracketPausedAndQuiet).
	pausedApp := argoApp.DeepCopy()
	if pausedApp.Spec.SyncPolicy != nil {
		pausedApp.Spec.SyncPolicy.Automated = nil
	}
	if pausedApp.Annotations == nil {
		pausedApp.Annotations = map[string]string{}
	}
	pausedApp.Annotations[BracketPausedForMoveAnnotation] = BracketMovePhasePaused
	pausedApp.Annotations[argocdRefreshAnnotation] = "normal"
	phase := BracketMovePhasePaused
	if err := a.updateApplication(ctx, pausedApp, "bracket.move.pause.update"); err != nil && isGoneErr(err) {
		// The app vanished underneath us (stale KnownApps): skip the in-place
		// pause and create it paused at the target path directly — a fresh app
		// has no in-flight sync operations.
		a.createPausedApplication(ctx, retarget)
		phase = BracketMovePhaseRetargeted
	}
	a.pendingSpecUpdates = append(a.pendingSpecUpdates, &PendingSpecUpdate{
		EnvironmentName: appInfo.EnvironmentName,
		ApplicationName: appInfo.ApplicationName,
		ExpectedPath:    path,
		Phase:           phase,
		Retarget:        retarget,
	})
	return true
}

// resumeBracketAutoSync re-enables auto-sync on a paused bracket app and removes
// the paused-for-move marker (the RESUME step of the pause protocol).
func (a *ArgoAppProcessor) resumeBracketAutoSync(ctx context.Context, app *v1alpha1.Application) error {
	resumed := app.DeepCopy()
	if resumed.Spec.SyncPolicy == nil {
		//exhaustruct:ignore
		resumed.Spec.SyncPolicy = &v1alpha1.SyncPolicy{}
	}
	resumed.Spec.SyncPolicy.Automated = &v1alpha1.SyncPolicyAutomated{
		Prune:    true,
		SelfHeal: true,
		// AllowEmpty=false is the bracket default, see CreateArgoApplication.
		AllowEmpty: false,
	}
	delete(resumed.Annotations, BracketPausedForMoveAnnotation)
	return a.updateApplication(ctx, resumed, "bracket.move.resume")
}

// drainPendingSpecUpdates advances the pause protocol on watch events: paused
// brackets are retargeted to the new snapshot once provably quiet, and
// retargeted brackets get auto-sync restored once their own sync status proves
// the moved resources are no longer attributed to them.
func (a *ArgoAppProcessor) drainPendingSpecUpdates(ctx context.Context) {
	drainSpan, ctx := tracer.StartSpanFromContext(ctx, "DrainPendingSpecUpdates")
	defer drainSpan.Finish()
	l := logger.FromContext(ctx)

	drainSpan.SetTag("pendingSpecUpdateCountBefore", len(a.pendingSpecUpdates))
	remaining := a.pendingSpecUpdates[:0]

	for _, pu := range a.pendingSpecUpdates {
		puSpan, ctx := tracer.StartSpanFromContext(ctx, "ProcessingPendingSpecUpdate")
		puSpan.SetTag("app", pu.ApplicationName)
		puSpan.SetTag("env", pu.EnvironmentName)
		puSpan.SetTag("phase", pu.Phase)

		var app *v1alpha1.Application
		if knownApps := a.KnownApps[pu.EnvironmentName]; knownApps != nil {
			app = knownApps[pu.ApplicationName]
		}
		if app == nil {
			remaining = append(remaining, pu)
			puSpan.SetTag("skipReason", "Argo app is nil")
			puSpan.Finish()
			continue
		}
		switch pu.Phase {
		case BracketMovePhasePaused:
			bracketPausedReason := bracketPausedAndQuiet(app)
			if bracketPausedReason != "" || pu.Retarget == nil {
				l.Info("bracket.retarget.blocked",
					zap.String("app", pu.ApplicationName),
					zap.String("env", pu.EnvironmentName),
					zap.Bool("retarget-known", pu.Retarget != nil),
					zap.String("reason", bracketPausedReason))
				remaining = append(remaining, pu)
				puSpan.SetTag("skipReason", bracketPausedReason)
				puSpan.Finish()
				continue
			}
			if err := a.updateApplication(ctx, pu.Retarget, "bracket.move.retarget"); err != nil {
				remaining = append(remaining, pu)
				puSpan.SetTag("skipReason", fmt.Sprintf("updateApplication failed %s", err.Error()))
				puSpan.Finish(tracer.WithError(err))
				continue
			}
			pu.Phase = BracketMovePhaseRetargeted
			remaining = append(remaining, pu)
			puSpan.SetTag("skipReason", "bracket move retargeted")
			puSpan.Finish()
		default: // BracketMovePhaseRetargeted
			bracketDisownedReason := bracketDisowned(app, pu.ExpectedPath)
			if bracketDisownedReason != "" {
				l.Info("bracket.resume.blocked",
					zap.String("app", pu.ApplicationName),
					zap.String("env", pu.EnvironmentName),
					zap.String("expected-path", pu.ExpectedPath),
					zap.String("reason", bracketDisownedReason))
				remaining = append(remaining, pu)
				puSpan.SetTag("skipReason", bracketDisownedReason)
				puSpan.Finish()
				continue
			}
			if err := a.resumeBracketAutoSync(ctx, app); err != nil {
				remaining = append(remaining, pu)
				puSpan.SetTag("skipReason", fmt.Sprintf("resumeBracketAutoSync failed %s", err.Error()))
				puSpan.Finish(tracer.WithError(err))
				continue
			}
		}
		puSpan.SetTag("skipReason", "none")
		puSpan.Finish()
	}
	l.Info("specUpdate",
		zap.Int("pendingSpecUpdatesBefore", len(a.pendingSpecUpdates)),
		zap.Int("pendingSpecUpdatesAfter", len(remaining)),
	)
	a.pendingSpecUpdates = remaining
	drainSpan.SetTag("pendingSpecUpdateCountAfter", len(a.pendingSpecUpdates))
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
				deleteWithNoCascade := false
				// only consider deletion, if the current app is not deployed anymore:
				if appInfo.IsBracket && currentAppDetails.Deployments[appInfo.ParentEnvironmentName] == nil {
					for _, otherApp := range sorting.SortKeys(allAppDetails) {
						otherDetails := allAppDetails[otherApp]
						if otherApp != appInfo.ApplicationName &&
							otherDetails.Application != nil &&
							otherDetails.Application.ArgoBracket == otherApp &&
							otherDetails.Deployments[appInfo.ParentEnvironmentName] != nil {
							deleteWithNoCascade = true
							break
						}
					}
				}
				if deleteWithNoCascade {
					if err := a.deleteAppNoCascade(ctx, ok, appInfo.ApplicationName); err != nil {
						logger.FromContext(ctx).Error("bracket.move.delete.failed",
							zap.String("app", appInfo.ApplicationName), zap.Error(err))
					}
				} else if !appInfo.IsBracket {
					// Non-bracket app with no deployment: cascade=false safety net for a
					// transient cd-service overview (e.g. mid helm-upgrade).
					a.DeleteArgoApps(ctx, ok, appInfo.ApplicationName, appInfo.EnvironmentName, appInfo.ParentEnvironmentName, currentAppDetails.Deployments[appInfo.ParentEnvironmentName])
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
		// Pause protocol (prune-vs-adopt protection on bracket moves): a bracket
		// that lost members is retargeted with auto-sync disabled instead of the
		// normal refresh — see the package comment and deferSpecUpdateIfWaiting.
		if appInfo.IsBracket && a.deferSpecUpdateIfWaiting(ctx, appInfo, overview) {
			return
		}
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
		l.Info("event.ignored",
			zap.String("source", "argocd"),
			zap.String("env", envName),
			zap.String("reason", "app-not-tagged"),
			zap.Any("annotations", ev.Application.Annotations),
		)
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
			// Restart recovery for the pause protocol: a bracket paused for a move
			// whose pending entry was lost (rollout-service restart) re-enters the
			// protocol at the phase recorded in the marker annotation.
			a.recoverPendingSpecUpdate(ctx, envName, appName, &ev.Application)
			a.drainPendingDeletions(ctx, envName)
			a.drainPendingSpecUpdates(ctx)
		}
	case "DELETED":
		l.Info("deleted:kuberpult.application:" + ev.Application.Name + ",kuberpult.environment:" + envName)
		delete(a.KnownApps[envName], appName)
		// Drop any pause-protocol entry for the deleted app — e.g. the bracket
		// was emptied mid-move and the delete-sentinel path removed its object.
		a.removePendingSpecUpdate(envName, appName)
	}
}

type AppInfo struct {
	ApplicationName              string
	TeamName                     string
	EnvironmentName              string
	ParentEnvironmentName        string
	ArgoEnvironmentConfiguration *api.ArgoCDEnvironmentConfiguration
	IsBracket                    bool
	// BracketSnapshotEslId is the brackets_history.source_transformer_esl_id that
	// will be embedded as "@<esl_id>" in the bracket Argo CD app's Spec.Source.Path.
	// Zero means "emit the legacy path with no suffix" (e.g. for non-bracket apps
	// or when the rollout-service runs without DB access in tests).
	BracketSnapshotEslId db.TransformerID
	// LostMembersTo names the brackets that gained members this bracket lost in
	// the current change (only set for bracket apps; see ArgoOverview.LostMembersTo).
	LostMembersTo []string
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
			if status.Code(err) != codes.InvalidArgument {
				logger.FromContext(ctx).Sugar().Errorf("creating %s, env %s: %v", appToCreate.Name, appInfo.EnvironmentName, err)
			} else {
				// The app exists with a different spec — its watch event has not
				// arrived yet (KnownApps lag), so the update path was missed.
				a.upsertExistingArgoApp(ctx, appInfo, appToCreate)
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
			if isGoneErr(err) {
				// The app vanished underneath us (stale KnownApps; e.g. its
				// DELETED watch event is still queued behind a burst of overview
				// ticks). Recreate it with the desired spec — the change would
				// otherwise be lost for good (the fast path sends each change
				// exactly once).
				a.CreateArgoApp(ctx, overview, appInfo)
			}
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

func (a *ArgoAppProcessor) DeleteArgoApps(ctx context.Context, argoApps map[string]*v1alpha1.Application, appName, envName, parentEnvName string, deployment *api.Deployment) {
	toDelete := make([]*v1alpha1.Application, 0)
	deleteSpan, ctx := tracer.StartSpanFromContext(ctx, "DeleteApplications")
	defer deleteSpan.Finish()
	if argoApps[appName] != nil && deployment == nil {
		toDelete = append(toDelete, argoApps[appName])
	}
	for i := range toDelete {
		deleteAppSpan, ctx := tracer.StartSpanFromContext(ctx, "DeleteApplication")
		deleteAppSpan.SetTag("application", toDelete[i].Name)
		deleteAppSpan.SetTag("environment", envName)
		deleteAppSpan.SetTag("parentEnvironment", parentEnvName)
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
			deleteAppSpan.Finish(tracer.WithError(err))
		} else {
			deleteAppSpan.Finish()
		}
	}
}

func CreateArgoApplication(overview *api.GetOverviewResponse, appInfo *AppInfo) *v1alpha1.Application {
	applicationNs := ""

	annotations := make(map[string]string)
	labels := make(map[string]string)

	var manifestPath string
	if appInfo.IsBracket {
		// Append "@<source_transformer_esl_id>" so the reposerver reads the exact
		// brackets_history snapshot this Argo CD app was last spec-updated against.
		// Zero means "no snapshot known yet" (e.g. tests without DB) — emit the legacy
		// path so the reposerver falls back to DBSelectBracketHistoryLatest.
		bracketName := appInfo.ApplicationName
		if appInfo.BracketSnapshotEslId != 0 {
			bracketName = fmt.Sprintf("%s@%d", bracketName, appInfo.BracketSnapshotEslId)
		}
		manifestPath = filepath.Join("environments", appInfo.ParentEnvironmentName, "brackets", bracketName)
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
