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

package kindbracketstest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
)

const cdServiceGrpcAddr = "localhost:5004"

// helmUpgrade restarts the port-forward, then delegates to the shared HelmUpgrade.
func helmUpgrade(t *testing.T, p HelmUpgradeParams) {
	t.Helper()
	globalPFM.restart()
	globalCDPFM.restart()
	HelmUpgrade(t, globalCfg.Config, p)
}

// waitForArgoApp polls until the named Argo Application exists in the default namespace.
func waitForArgoApp(t *testing.T, name string) {
	t.Helper()
	deadline := time.Now().Add(argoAppWaitTimeout)
	for time.Now().Before(deadline) {
		out, err := exec.Command("kubectl", "get", "application", name, "-n", globalCfg.KuberpultNamespace).Output()
		if err == nil && strings.Contains(string(out), name) {
			tLogf(t, "  Argo app present: %s", name)
			return
		}
		time.Sleep(argoAppPollInterval)
	}
	t.Fatalf("Argo app %q never appeared after 2 minutes", name)
}

// assertArgoAppStaysPresent polls until the named Argo Application exists in the default namespace.
func assertArgoAppStaysPresent(t *testing.T, envName, appName string) {
	t.Helper()
	deadline := time.Now().Add(argoAppPresentMinDuration)
	combinedName := envName + "-" + appName
	for time.Now().Before(deadline) {
		out, err := exec.Command("kubectl", "get", "application", combinedName, "-n", globalCfg.KuberpultNamespace).CombinedOutput()
		if err != nil {
			t.Fatalf("kubectl get: %v\nout:\n%s", err, string(out))
			return
		}
		time.Sleep(argoAppPresentInterval)
	}
	t.Logf("Argo app %q stays present for %v", combinedName, argoAppPresentMinDuration)
}

// waitForArgoAppGone polls until the named Argo Application no longer exists.
func waitForArgoAppGone(t *testing.T, name string) {
	t.Helper()
	deadline := time.Now().Add(argoAppGoneTimeout)
	for time.Now().Before(deadline) {
		_, err := exec.Command("kubectl", "get", "application", name, "-n", globalCfg.KuberpultNamespace).Output()
		if err != nil {
			tLogf(t, "  Argo app gone: %s", name)
			return
		}
		time.Sleep(argoAppPollInterval)
	}
	dep := "deployment/kuberpult-rollout-service"
	if out, err := exec.Command("kubectl", "logs", "-n", globalCfg.KuberpultNamespace, dep, "--since=5m").CombinedOutput(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "creating ArgoApp failed after maximum number of attempts") &&
				strings.Contains(line, "PermissionDenied") && strings.Contains(line, name) {
				return // correctly log the retry attempts
			}
		}
	}
	t.Fatalf("Argo app %q still present after 3 minutes", name)
}

// TestBracketMigration is the regression test for the individual-to-bracket transition.
//
// Scenario:
//   - Two apps share a bracket, deployed to development and staging.
//   - The cluster starts with staging=false (individual Argo apps per app on staging).
//   - After verifying individual apps and recording pod start times, the cluster is
//     upgraded to staging=true (bracket Argo app for staging).
//   - Expected: the bracket Argo app appears, the individual apps are deleted,
//     and the K8s Deployments on staging are UNTOUCHED (pod start times unchanged).
//
// This exercises deleteAppNoCascade + pendingDeletions/drainPendingDeletions: the
// individual Argo Application objects are removed without pruning the K8s resources,
// so the bracket app can take over ownership without a downtime gap.
func TestBracketMigration(t *testing.T) {
	cleanupCluster(t)
	tLogf(t, "runSuffix: %s", runSuffix)
	app1 := "bmt-app1-" + runSuffix
	app2 := "bmt-app2-" + runSuffix
	apps := []string{app1, app2}
	migrationBracket := "bmt-bracket-" + runSuffix

	// Argo app name convention (from argo.go CreateArgoApplication):
	//   individual: "{env}-{appName}"   e.g. "staging-bmt-app1-XXXXX"
	//   bracket:    "{env}-{bracketName}" e.g. "staging-bmt-bracket-XXXXX"

	tLog(t, "step 1: upgrade to staging=false (individual Argo apps mode for staging)")
	helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, DevelopmentEnabled: false, StagingEnabled: false, ChannelSize: 50})

	tLog(t, "step 2: create v1 releases (dev + staging manifests)")
	for _, app := range apps {
		createRelease(t, app, "sreteam", migrationBracket, "1", map[string]string{
			devNamespace:     stableManifest(app, devNamespace, "1"),
			stagingNamespace: stableManifest(app, stagingNamespace, "1"),
		})
	}

	tLog(t, "step 3: run staging release train (deploys v1 from development to staging)")
	releaseTrain(t, stagingNamespace)

	tLog(t, "step 4: wait for v1 synced in staging (ArgoCD applied the deployment)")
	for _, app := range apps {
		waitForDeploymentAnnotation(t, stagingNamespace, app+"-bracket-dep", "1")
	}

	tLog(t, "step 5: verify individual Argo apps exist on staging")
	for _, app := range apps {
		waitForArgoApp(t, "staging-"+app)
	}

	tLog(t, "step 6: record deployment creation times")
	creationTimes := map[deploymentKey]string{}
	for _, app := range apps {
		for _, ns := range []string{devNamespace, stagingNamespace} {
			k := deploymentKey{ns, app + "-bracket-dep"}
			creationTimes[k] = deploymentCreationTime(t, ns, app+"-bracket-dep")
			tLogf(t, "  %s/%s: %s", ns, app+"-bracket-dep", creationTimes[k])
		}
	}

	tLog(t, "step 7: upgrade to staging=true (migrate to bracket Argo app)")
	helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, DevelopmentEnabled: false, StagingEnabled: true, ChannelSize: 50})

	tLog(t, "step 8: wait for bracket Argo app to appear on staging")
	waitForArgoApp(t, "staging-"+migrationBracket)

	tLog(t, "step 9: wait for individual Argo apps to be deleted from staging")
	for _, app := range apps {
		waitForArgoAppGone(t, "staging-"+app)
	}

	tLog(t, "step 10: assert deployment creation times stable (bracket migration)")
	assertDeploymentCreationTimesStable(t, creationTimes, "bracket migration")
}

// processBatch sends a BatchRequest to the cd-service, retrying on Unavailable
// (port-forward restarting after pod replacement) for up to 30 s.
func processBatch(t *testing.T, req *api.BatchRequest) {
	t.Helper()
	ctx := auth.WriteUserToGrpcContext(context.Background(), auth.User{
		Email: "test@kuberpult.example.com",
		Name:  "Kind Test",
	})
	deadline := time.Now().Add(grpcRetryTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := grpc.NewClient(cdServiceGrpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			lastErr = err
			time.Sleep(grpcRetryInterval)
			continue
		}
		_, lastErr = api.NewBatchServiceClient(conn).ProcessBatch(ctx, req)
		if err := conn.Close(); err != nil {
			tLogf(t, "grpc conn.Close: %v", err)
		}
		if lastErr == nil {
			return
		}
		if status.Code(lastErr) == codes.Unavailable {
			time.Sleep(grpcRetryInterval)
			continue
		}
		break
	}
	t.Fatalf("processBatch: %v", lastErr)
}

func deleteEnvFromApp(t *testing.T, env, app string) {
	t.Helper()
	processBatch(t, &api.BatchRequest{Actions: []*api.BatchAction{
		{Action: &api.BatchAction_DeleteEnvFromApp{
			DeleteEnvFromApp: &api.DeleteEnvironmentFromAppRequest{
				Environment: env,
				Application: app,
			},
		}},
	}})
}

// TestBracketDeleteEnvFromApp verifies that removing the last app's env from a bracket
// (via deleteEnvFromApp) cascade-deletes the per-env bracket Argo Application: once no
// member of the bracket is deployed in that env, the <env>-<bracket> Argo app is torn
// down. The bracket itself persists in brackets_history for future deployments.
// TestBracketDeleteEnvFromApp verifies deleting the staging env from app1:
//   - "sole member": app1 is the only member of the bracket, so the staging bracket
//     Argo app is cascade-deleted (bracketEmptyInEnv == true).
//   - "shared bracket": app2 is also a member, so the staging bracket Argo app
//     SURVIVES and only app1's staging resources are pruned (bracketEmptyInEnv == false).
//
// In both cases app1 stays deployed on development (only its staging env is removed).
func TestBracketDeleteEnvFromApp(t *testing.T) {
	tcs := []struct {
		Name string
		// WithSecondApp keeps app2 in the bracket, so it stays populated on staging.
		WithSecondApp bool
	}{
		{
			Name:          "one app",
			WithSecondApp: false,
		},
		{
			Name:          "two apps",
			WithSecondApp: true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			cleanupCluster(t)
			tLogf(t, "runSuffix: %s", runSuffix)
			app1 := "bde-app1-" + runSuffix
			app2 := "bde-app2-" + runSuffix // shared case only: second member of the bracket
			bracket := "bde-bracket-" + runSuffix

			tLog(t, "step 1: upgrade to staging=true (bracket mode)")
			helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, DevelopmentEnabled: false, StagingEnabled: true, ChannelSize: 50})

			tLog(t, "step 2: create v1 release(s) (dev + staging manifests)")
			createRelease(t, app1, "sreteam", bracket, "1", map[string]string{
				devNamespace:     stableManifest(app1, devNamespace, "1"),
				stagingNamespace: stableManifest(app1, stagingNamespace, "1"),
			})
			if tc.WithSecondApp {
				createRelease(t, app2, "sreteam", bracket, "1", map[string]string{
					devNamespace:     stableManifest(app2, devNamespace, "1"),
					stagingNamespace: stableManifest(app2, stagingNamespace, "1"),
				})
			}

			tLog(t, "step 3: run staging release train")
			releaseTrain(t, stagingNamespace)

			tLog(t, "step 4: wait for v1 synced in staging + bracket Argo app present")
			waitForDeploymentAnnotation(t, stagingNamespace, app1+"-bracket-dep", "1")
			waitForArgoApp(t, "staging-"+bracket)
			if tc.WithSecondApp {
				waitForDeploymentAnnotation(t, stagingNamespace, app2+"-bracket-dep", "1")
			}

			tLog(t, "step 5: record creation times (app1 on dev always survives; app2 staging when shared)")
			creationTimes := map[deploymentKey]string{
				{devNamespace, app1 + "-bracket-dep"}: deploymentCreationTime(t, devNamespace, app1+"-bracket-dep"),
			}
			if tc.WithSecondApp {
				k := deploymentKey{stagingNamespace, app2 + "-bracket-dep"}
				creationTimes[k] = deploymentCreationTime(t, stagingNamespace, app2+"-bracket-dep")
			}

			tLog(t, "step 6: delete staging env from app1")
			deleteEnvFromApp(t, stagingNamespace, app1)

			tLogf(t, "step 7: %s buffer for rollout-service to reconcile", reconcileBuffer)
			time.Sleep(reconcileBuffer)

			if tc.WithSecondApp {
				tLog(t, "step 8: staging bracket survives (app2 remains) but stops tracking app1")
				waitForArgoApp(t, "staging-"+bracket)
				waitForArgoAppNotTracking(t, "staging-"+bracket, "Deployment/"+app1+"-bracket-dep")
				resources := argoAppTrackedResources(t, "staging-"+bracket)
				if !strings.Contains(resources, "Deployment/"+app2+"-bracket-dep\n") {
					t.Errorf("staging bracket Argo app lost app2's deployment, tracked resources:\n%s", resources)
				}
			} else {
				tLog(t, "step 8: verify staging bracket Argo app is cascade-deleted (no member deployed in env)")
				waitForArgoAppGone(t, "staging-"+bracket)
			}

			tLog(t, "step 9: app1 still deployed on development; pods stable")
			waitForDeploymentAnnotation(t, devNamespace, app1+"-bracket-dep", "1")
			assertDeploymentCreationTimesStable(t, creationTimes, "delete staging env from app1 should not bother dev or siblings")
		})
	}
}

// TestBracketReverseMigration is the regression test for the bracket → individual rollback.
//
// Scenario:
//   - Two apps share a bracket, deployed to staging in bracket mode (staging=true).
//   - After verifying the bracket Argo app and recording pod start times, the cluster
//     is downgraded to staging=false (individual Argo apps per app on staging).
//   - Expected: individual Argo apps appear, the bracket Argo app is still PRESENT
//     (apps are still members of the bracket in brackets_history), and the K8s
//     Deployment pods are UNTOUCHED (pod start times unchanged).
func TestBracketReverseMigration(t *testing.T) {
	cleanupCluster(t)
	tLogf(t, "runSuffix: %s", runSuffix)
	app1 := "brm-app1-" + runSuffix
	app2 := "brm-app2-" + runSuffix
	apps := []string{app1, app2}
	bracket := "brm-bracket-" + runSuffix

	tLog(t, "step 1: upgrade to staging=true (bracket mode)")
	helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, DevelopmentEnabled: false, StagingEnabled: true, ChannelSize: 50})

	tLog(t, "step 2: create v1 releases (dev + staging manifests)")
	for _, app := range apps {
		createRelease(t, app, "sreteam", bracket, "1", map[string]string{
			devNamespace:     stableManifest(app, devNamespace, "1"),
			stagingNamespace: stableManifest(app, stagingNamespace, "1"),
		})
	}

	tLog(t, "step 3: run staging release train (deploys v1 from development to staging)")
	releaseTrain(t, stagingNamespace)

	tLog(t, "step 4: wait for v1 synced in staging")
	for _, app := range apps {
		waitForDeploymentAnnotation(t, stagingNamespace, app+"-bracket-dep", "1")
	}

	tLog(t, "step 5: wait for bracket Argo app to appear on staging")
	waitForArgoApp(t, "staging-"+bracket)

	tLog(t, "step 6: record deployment creation times")
	creationTimes := map[deploymentKey]string{}
	for _, app := range apps {
		k := deploymentKey{stagingNamespace, app + "-bracket-dep"}
		creationTimes[k] = deploymentCreationTime(t, stagingNamespace, app+"-bracket-dep")
		tLogf(t, "  %s/%s: %s", stagingNamespace, app+"-bracket-dep", creationTimes[k])
	}

	tLog(t, "step 7: upgrade to staging=false (revert to individual Argo apps)")
	helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, DevelopmentEnabled: false, StagingEnabled: false, ChannelSize: 50})

	tLog(t, "step 8: wait for individual Argo apps to appear on staging")
	for _, app := range apps {
		waitForArgoApp(t, "staging-"+app)
	}

	tLogf(t, "step 9: %s buffer for rollout-service to reconcile", reconcileBuffer)
	time.Sleep(reconcileBuffer)

	tLog(t, "step 10: verify bracket Argo app is still present (apps still in brackets_history)")
	waitForArgoApp(t, "staging-"+bracket)

	tLog(t, "step 11: assert pod start times stable (bracket reverse migration)")
	assertDeploymentCreationTimesStable(t, creationTimes, "bracket reverse migration")
}

// TestBracketAddAppToExistingBracket verifies that adding a second app to an already-running
// bracket does not delete the bracket Argo Application or restart existing pods.
//
// Scenario:
//   - app1 is deployed to a bracket; the bracket Argo app is established.
//   - app2 is released into the same bracket and promoted to staging.
//   - Expected: bracket Argo app persists, bracket version string updates to include
//     both apps (colon-separated), and app1 pods are UNTOUCHED.
func TestBracketAddAppToExistingBracket(t *testing.T) {
	cleanupCluster(t)
	tLogf(t, "runSuffix: %s", runSuffix)
	app1 := "bae-app1-" + runSuffix
	app2 := "bae-app2-" + runSuffix
	bracket := "bae-bracket-" + runSuffix

	tLog(t, "step 1: upgrade to staging=true (bracket mode)")
	helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, DevelopmentEnabled: false, StagingEnabled: true, ChannelSize: 50})

	tLog(t, "step 2: create v1 release for app1 only")
	createRelease(t, app1, "sreteam", bracket, "1", map[string]string{
		devNamespace:     stableManifest(app1, devNamespace, "1"),
		stagingNamespace: stableManifest(app1, stagingNamespace, "1"),
	})

	tLog(t, "step 3: run staging release train (deploys app1 v1 to staging)")
	releaseTrain(t, stagingNamespace)

	tLog(t, "step 4: wait for app1 v1 synced in staging")
	waitForDeploymentAnnotation(t, stagingNamespace, app1+"-bracket-dep", "1")

	tLog(t, "step 5: wait for bracket Argo app to appear on staging")
	waitForArgoApp(t, "staging-"+bracket)

	tLog(t, "step 6: record app1 deployment creation time")
	creationTimes := map[deploymentKey]string{
		{stagingNamespace, app1 + "-bracket-dep"}: deploymentCreationTime(t, stagingNamespace, app1+"-bracket-dep"),
	}
	tLogf(t, "  %s/%s: %s", stagingNamespace, app1+"-bracket-dep", creationTimes[deploymentKey{stagingNamespace, app1 + "-bracket-dep"}])

	tLog(t, "step 7: create v1 release for app2 (same bracket, new member)")
	createRelease(t, app2, "sreteam", bracket, "1", map[string]string{
		devNamespace:     stableManifest(app2, devNamespace, "1"),
		stagingNamespace: stableManifest(app2, stagingNamespace, "1"),
	})

	tLog(t, "step 8: run staging release train (deploys app2 v1 to staging)")
	releaseTrain(t, stagingNamespace)

	tLog(t, "step 9: wait for app2 v1 synced in staging (bracket version string updated to include both apps)")
	waitForDeploymentAnnotation(t, stagingNamespace, app2+"-bracket-dep", "1")

	tLog(t, "step 10: verify bracket Argo app still exists (must not be deleted)")
	waitForArgoApp(t, "staging-"+bracket)

	tLog(t, "step 11: assert app1 pod start time stable (adding app2 to bracket)")
	assertDeploymentCreationTimesStable(t, creationTimes, "add app to existing bracket")
}

// TestBracketPartialUpdate verifies that a release train updating only one of two apps
// in a bracket does not restart the other app's pods.
//
// Scenario:
//   - Two apps share a bracket on staging, both at v1 (bracket version "1:1").
//   - A new v2 release is created for app1 only; the staging release train is run.
//   - staging's bracket version advances to "2:1" (app1=v2, app2=v1).
//   - Expected: app2 pods on staging are UNTOUCHED (no cascade deletion of bracket).
//
// This differs from TestBracketPodStability, where development changes and staging is
// skipped entirely.  Here staging itself receives a partial update.
func TestBracketPartialUpdate(t *testing.T) {
	cleanupCluster(t)
	tLogf(t, "runSuffix: %s", runSuffix)
	app1 := "bpu-app1-" + runSuffix
	app2 := "bpu-app2-" + runSuffix
	bracket := "bpu-bracket-" + runSuffix

	tLog(t, "step 1: upgrade to staging=true (bracket mode)")
	helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, DevelopmentEnabled: false, StagingEnabled: true, ChannelSize: 50})

	tLog(t, "step 2: create v1 releases for both apps")
	for _, app := range []string{app1, app2} {
		createRelease(t, app, "sreteam", bracket, "1", map[string]string{
			devNamespace:     stableManifest(app, devNamespace, "1"),
			stagingNamespace: stableManifest(app, stagingNamespace, "1"),
		})
	}

	tLog(t, "step 3: run staging release train (deploys both apps v1)")
	releaseTrain(t, stagingNamespace)

	tLog(t, "step 4: wait for v1 synced in staging for both apps")
	for _, app := range []string{app1, app2} {
		waitForDeploymentAnnotation(t, stagingNamespace, app+"-bracket-dep", "1")
	}

	tLog(t, "step 5: record app2 deployment creation time")
	creationTimes := map[deploymentKey]string{
		{stagingNamespace, app2 + "-bracket-dep"}: deploymentCreationTime(t, stagingNamespace, app2+"-bracket-dep"),
	}
	tLogf(t, "  %s/%s: %s", stagingNamespace, app2+"-bracket-dep", creationTimes[deploymentKey{stagingNamespace, app2 + "-bracket-dep"}])

	tLog(t, "step 6: create v2 release for app1 only (app2 stays at v1)")
	createRelease(t, app1, "sreteam", bracket, "2", map[string]string{
		devNamespace:     stableManifest(app1, devNamespace, "2"),
		stagingNamespace: stableManifest(app1, stagingNamespace, "2"),
	})

	tLog(t, "step 7: run staging release train (promotes app1 v2, app2 stays at v1)")
	releaseTrain(t, stagingNamespace)

	tLog(t, "step 8: wait for app1 v2 synced in staging (bracket version advances to mixed state)")
	waitForDeploymentAnnotation(t, stagingNamespace, app1+"-bracket-dep", "2")

	tLog(t, "step 9: assert app2 pod start time stable (partial bracket version update)")
	assertDeploymentCreationTimesStable(t, creationTimes, "partial bracket version update")
}

func undeployApp(t *testing.T, app string) {
	t.Helper()
	processBatch(t, &api.BatchRequest{Actions: []*api.BatchAction{
		{Action: &api.BatchAction_Undeploy{
			Undeploy: &api.UndeployRequest{Application: app},
		}},
	}})
}

// TestBracketUndeploy verifies undeploying app1 from bracket1:
//   - "sole member": app1 is the only member of bracket1, so the bracket1 Argo app
//     is cascade-deleted (the bracketEmptyInEnv == true branch).
//   - "shared bracket": app3 is also a member of bracket1, so the bracket1 Argo app
//     SURVIVES and only app1's resources are pruned from it (bracketEmptyInEnv == false).
//
// In both cases a separate bracket2 (app2) must be completely untouched.
func TestBracketUndeploy(t *testing.T) {
	tcs := []struct {
		Name string
		// WithThirdApp keeps app3 in bracket1, so it stays populated after app1's undeploy.
		WithThirdApp bool
	}{
		{
			Name:         "sole member: bracket cascade-deleted",
			WithThirdApp: false,
		},
		{
			Name:         "shared bracket: survives, sibling untouched",
			WithThirdApp: true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			cleanupCluster(t)
			tLogf(t, "runSuffix: %s", runSuffix)
			app1 := "bu-app1-" + runSuffix
			bracket1 := "bu-bracket1-" + runSuffix
			app2 := "bu-app2-" + runSuffix
			bracket2 := "bu-bracket2-" + runSuffix
			app3 := "bu-app3-" + runSuffix // shared case only: second member of bracket1

			tLog(t, "step 1: upgrade to staging=true (bracket mode)")
			helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, DevelopmentEnabled: false, StagingEnabled: true, ChannelSize: 50})

			tLog(t, "step 2: create v1 releases")
			createRelease(t, app1, "sreteam", bracket1, "1", map[string]string{
				devNamespace:     stableManifest(app1, devNamespace, "1"),
				stagingNamespace: stableManifest(app1, stagingNamespace, "1"),
			})
			createRelease(t, app2, "sreteam", bracket2, "1", map[string]string{
				devNamespace:     stableManifest(app2, devNamespace, "1"),
				stagingNamespace: stableManifest(app2, stagingNamespace, "1"),
			})
			if tc.WithThirdApp {
				createRelease(t, app3, "sreteam", bracket1, "1", map[string]string{
					devNamespace:     stableManifest(app3, devNamespace, "1"),
					stagingNamespace: stableManifest(app3, stagingNamespace, "1"),
				})
			}

			tLog(t, "step 3: run staging release train (deploys all apps)")
			releaseTrain(t, stagingNamespace)

			tLog(t, "step 4: wait for bracket Argo apps + v1 synced")
			waitForArgoApp(t, "staging-"+bracket1)
			waitForArgoApp(t, "staging-"+bracket2)
			waitForDeploymentAnnotation(t, stagingNamespace, app2+"-bracket-dep", "1")
			waitForDeploymentAnnotation(t, devNamespace, app2+"-bracket-dep", "1")
			if tc.WithThirdApp {
				waitForDeploymentAnnotation(t, stagingNamespace, app3+"-bracket-dep", "1")
			}

			tLog(t, "step 5: record deployment creation times (control app2; app3 when shared)")
			creationTimes := map[deploymentKey]string{
				{stagingNamespace, app2 + "-bracket-dep"}: deploymentCreationTime(t, stagingNamespace, app2+"-bracket-dep"),
				{devNamespace, app2 + "-bracket-dep"}:     deploymentCreationTime(t, devNamespace, app2+"-bracket-dep"),
			}
			if tc.WithThirdApp {
				k := deploymentKey{stagingNamespace, app3 + "-bracket-dep"}
				creationTimes[k] = deploymentCreationTime(t, stagingNamespace, app3+"-bracket-dep")
			}

			tLog(t, "step 6: undeploy app1 entirely")
			undeployApp(t, app1)

			tLogf(t, "step 7: %s buffer for rollout-service to reconcile", reconcileBuffer)
			time.Sleep(reconcileBuffer)

			if tc.WithThirdApp {
				tLog(t, "step 8: bracket1 survives (app3 remains) but stops tracking app1")
				waitForArgoApp(t, "staging-"+bracket1)
				waitForArgoAppNotTracking(t, "staging-"+bracket1, "Deployment/"+app1+"-bracket-dep")
				resources := argoAppTrackedResources(t, "staging-"+bracket1)
				if !strings.Contains(resources, "Deployment/"+app3+"-bracket-dep\n") {
					t.Errorf("bracket1 Argo app lost app3's deployment, tracked resources:\n%s", resources)
				}
			} else {
				tLog(t, "step 8: verify bracket1 Argo app is cascade-deleted (no apps remain)")
				waitForArgoAppGone(t, "staging-"+bracket1)
			}

			tLog(t, "step 9: assert pod start times stable after undeploy of app1")
			assertDeploymentCreationTimesStable(t, creationTimes, "undeploy app1 should not bother siblings")
		})
	}
}

// TestBracketEnableAllClusters is the regression test for the bug where enabling
// experimentalBrackets for the first time (going from enabled=false to enabled=true
// with development=true and staging=true simultaneously) cascade-deletes all apps
// on both environments.
//
// Scenario:
//   - Two apps share a bracket, deployed to development and staging in
//     NON-bracket mode (experimentalBrackets.enabled=false).
//   - Pod start times are recorded for both envs.
//   - The cluster is upgraded to experimentalBrackets.enabled=true with both
//     development=true and staging=true (the exact user-reported config change).
//   - Expected: bracket Argo apps appear on both envs, individual Argo apps are
//     deleted, and all K8s Deployments are UNTOUCHED (pod start times unchanged).
func TestBracketEnableAllClusters(t *testing.T) {
	cleanupCluster(t)
	tLogf(t, "runSuffix: %s", runSuffix)
	apps := make([]string, numTestApps)
	for i := range apps {
		apps[i] = fmt.Sprintf("beac-app%d-%s", i+1, runSuffix)
	}
	bracket := "beac-bracket-" + runSuffix

	tLog(t, "step 1: upgrade to experimentalBrackets.enabled=false (fully disabled brackets)")
	helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: false, DevelopmentEnabled: false, StagingEnabled: false, ChannelSize: 1})

	tLog(t, "step 2: create v1 releases (dev + dev2 + staging manifests)")
	for _, app := range apps {
		createRelease(t, app, "sreteam", bracket, "1", map[string]string{
			devNamespace:     stableManifest(app, devNamespace, "1"),
			stagingNamespace: stableManifest(app, stagingNamespace, "1"),
		})
	}

	tLog(t, "step 3: run staging release train (deploys v1 from development2 to staging)")
	releaseTrain(t, stagingNamespace)

	// diagnostic: show what ArgoCD apps and staging deployments exist after release train
	if out, err := exec.Command("kubectl", "get", "applications", "-n", globalCfg.KuberpultNamespace, "--no-headers").CombinedOutput(); err == nil {
		tLogf(t, "ArgoCD apps after releaseTrain:\n%s", out)
	}
	if out, err := exec.Command("kubectl", "get", "deployments", "-n", stagingNamespace, "--no-headers").CombinedOutput(); err == nil {
		tLogf(t, "Deployments in staging after releaseTrain:\n%s", out)
	}
	if out, err := exec.Command("kubectl", "logs", "-n", globalCfg.KuberpultNamespace, "deployment/kuberpult-rollout-service", "--tail=40").CombinedOutput(); err == nil {
		tLogf(t, "rollout-service tail after releaseTrain:\n%s", out)
	}

	tLog(t, "step 4: wait for v1 synced in both development and staging")
	for _, app := range apps {
		for _, ns := range []string{devNamespace, stagingNamespace} {
			waitForDeploymentAnnotation(t, ns, app+"-bracket-dep", "1")
		}
	}

	tLog(t, "step 5: verify individual Argo apps exist on both development and staging")
	for _, app := range apps {
		waitForArgoApp(t, devNamespace+"-"+app)
		waitForArgoApp(t, stagingNamespace+"-"+app)
	}

	tLog(t, "step 6: record deployment creation times in both envs")
	creationTimes := map[deploymentKey]string{}
	for _, app := range apps {
		for _, ns := range []string{devNamespace, stagingNamespace} {
			k := deploymentKey{ns, app + "-bracket-dep"}
			creationTimes[k] = deploymentCreationTime(t, ns, app+"-bracket-dep")
			tLogf(t, "  %s/%s: %s", ns, app+"-bracket-dep", creationTimes[k])
		}
	}

	tLog(t, "step 7: upgrade to experimentalBrackets.enabled=true, development=true, staging=true")
	helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, DevelopmentEnabled: true, StagingEnabled: true, ChannelSize: 1})

	var individualArgoApps []string
	for _, app := range apps {
		individualArgoApps = append(individualArgoApps, devNamespace+"-"+app, stagingNamespace+"-"+app)
	}

	tLog(t, "step 7.5: dump state immediately after helmUpgrade (capture drain activity)")
	dumpBracketDiagnosticsExpanded(t,
		[]string{devNamespace + "-" + bracket, stagingNamespace + "-" + bracket},
		individualArgoApps,
		[]string{devNamespace, stagingNamespace},
	)

	tLog(t, "step 8: wait for bracket Argo apps to appear on both envs")
	waitForArgoApp(t, devNamespace+"-"+bracket)
	waitForArgoApp(t, stagingNamespace+"-"+bracket)

	tLog(t, "step 9: wait for individual Argo apps to be deleted from both envs")
	for _, app := range apps {
		waitForArgoAppGone(t, devNamespace+"-"+app)
		waitForArgoAppGone(t, stagingNamespace+"-"+app)
	}

	tLog(t, "step 9.5: dump diagnostics after transition (ArgoCD state, events, service logs)")
	dumpBracketDiagnosticsExpanded(t,
		[]string{devNamespace + "-" + bracket, stagingNamespace + "-" + bracket},
		individualArgoApps,
		[]string{devNamespace, stagingNamespace},
	)

	tLog(t, "step 10: assert pod start times stable (enable all clusters)")
	assertDeploymentCreationTimesStable(t, creationTimes, "enable all clusters")
}

func TestBracketDeploymentAndBracketMoveSimultaneous(t *testing.T) {
	cleanupCluster(t)
	tLogf(t, "runSuffix: %s", runSuffix)
	app1 := "bdabms-app1-" + runSuffix
	app2 := "bdabms-app2-" + runSuffix
	bracket1 := "bdabms-bracket1-" + runSuffix
	bracket2 := "bdabms-bracket2-" + runSuffix

	tLog(t, "step 1: upgrade to development=true (bracket mode)")
	helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, DevelopmentEnabled: true, StagingEnabled: false, ChannelSize: 50})

	tLog(t, "step 2: create v1 release for app1+2 in bracket1")
	createRelease(t, app1, "sreteam", bracket1, "1", map[string]string{
		devNamespace: stableManifest(app1, devNamespace, "1"),
	})
	createRelease(t, app2, "sreteam", bracket1, "1", map[string]string{
		devNamespace: stableManifest(app2, devNamespace, "1"),
	})

	tLog(t, "step 3: wait for bracket1 Argo app to appear on development")
	waitForArgoApp(t, "development-"+bracket1)

	tLog(t, "step 4.1: wait for v1 synced in dev (app1)")
	waitForDeploymentAnnotation(t, devNamespace, app1+"-bracket-dep", "1")

	tLog(t, "step 4.2: wait for v1 synced in dev (app2)")
	waitForDeploymentAnnotation(t, devNamespace, app2+"-bracket-dep", "1")

	tLog(t, "step 5: record deployment creation time")
	creationTimes := map[deploymentKey]string{
		{devNamespace, app1 + "-bracket-dep"}: deploymentCreationTime(t, devNamespace, app1+"-bracket-dep"),
		{devNamespace, app2 + "-bracket-dep"}: deploymentCreationTime(t, devNamespace, app2+"-bracket-dep"),
	}
	tLogf(t, "  %s/%s: %s", devNamespace, app1+"-bracket-dep", creationTimes[deploymentKey{devNamespace, app1 + "-bracket-dep"}])
	tLogf(t, "  %s/%s: %s", devNamespace, app2+"-bracket-dep", creationTimes[deploymentKey{devNamespace, app2 + "-bracket-dep"}])

	tLog(t, "step 6: create v2 release for app1 in bracket2")
	createRelease(t, app1, "sreteam", bracket2, "2", map[string]string{
		devNamespace: stableManifest(app1, devNamespace, "2"),
	})

	tLog(t, "step 7: wait for bracket2 Argo app to appear on development")
	waitForArgoApp(t, "development-"+bracket1)
	waitForArgoApp(t, "development-"+bracket2)

	tLog(t, "step 8: wait for v2 synced in development")
	waitForDeploymentAnnotation(t, devNamespace, app1+"-bracket-dep", "2") // newer version
	waitForDeploymentAnnotation(t, devNamespace, app2+"-bracket-dep", "1") // same as before

	tLog(t, "step 9: assert pod start time stable (app moved bracket1→bracket2)")
	assertDeploymentCreationTimesStable(t, creationTimes, "move bracket1→bracket2")

}

// TestRealityCheck simulates the real-world client upgrade path:
//
//  1. Start on v13.47.6 (bracket DB storage supported, but rollout-service has no
//     experimentalBrackets config — individual Argo apps only).
//  2. Create bracket-grouped releases (2 brackets × 2 apps each, dev + staging).
//  3. Upgrade to the current build with brackets disabled in all clusters.
//     The new rollout-service must leave every individual Argo app untouched.
//  4. Enable brackets on dev only.
//     Dev migrates to bracket Argo apps (cascade=false); staging stays individual.
//
// Detection: Deployment creationTimestamps must be identical across all three
// kuberpult versions — any change means a pod was deleted and recreated (downtime).
func TestRealityCheck(t *testing.T) {
	cleanupCluster(t)
	tLogf(t, "runSuffix: %s", runSuffix)

	app1 := "rc-b1a1-" + runSuffix
	app2 := "rc-b1a2-" + runSuffix
	app3 := "rc-b2a1-" + runSuffix
	app4 := "rc-b2a2-" + runSuffix
	bracket1 := "rc-bracket1-" + runSuffix
	bracket2 := "rc-bracket2-" + runSuffix
	allApps := []string{app1, app2, app3, app4}

	tLog(t, "step 1: install v13.47.6 (no rollout-service bracket support)")
	helmUpgrade(t, HelmUpgradeParams{OldVersion: "v13.47.6", ChannelSize: 50})

	tLog(t, "step 2: create v1 releases — bracket1 (app1+app2) and bracket2 (app3+app4)")
	for _, app := range []string{app1, app2} {
		createRelease(t, app, "sreteam", bracket1, "1", map[string]string{
			devNamespace:     stableManifest(app, devNamespace, "1"),
			stagingNamespace: stableManifest(app, stagingNamespace, "1"),
		})
	}
	for _, app := range []string{app3, app4} {
		createRelease(t, app, "sreteam", bracket2, "1", map[string]string{
			devNamespace:     stableManifest(app, devNamespace, "1"),
			stagingNamespace: stableManifest(app, stagingNamespace, "1"),
		})
	}

	tLog(t, "step 3: run staging release train (dev auto-deploys via upstream=latest)")
	releaseTrain(t, stagingNamespace)

	tLog(t, "step 4: wait for v1 synced in dev and staging (all 4 apps)")
	for _, app := range allApps {
		for _, ns := range []string{devNamespace, stagingNamespace} {
			waitForDeploymentAnnotation(t, ns, app+"-bracket-dep", "1")
		}
	}

	tLog(t, "step 5: record baseline deployment creation times (8 deployments)")
	creationTimes := map[deploymentKey]string{}
	for _, app := range allApps {
		for _, ns := range []string{devNamespace, stagingNamespace} {
			k := deploymentKey{ns, app + "-bracket-dep"}
			creationTimes[k] = deploymentCreationTime(t, ns, app+"-bracket-dep")
			tLogf(t, "  %s/%s: %s", ns, app+"-bracket-dep", creationTimes[k])
		}
	}

	tLog(t, "step 6: upgrade to new version — brackets disabled in all clusters")
	helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: false, ChannelSize: 50})

	tLog(t, "step 7: assert creation times unchanged after kuberpult upgrade")
	assertDeploymentCreationTimesStable(t, creationTimes, "upgrade to new version")

	tLog(t, "step 8: enable brackets on dev only")
	helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, DevelopmentEnabled: true, StagingEnabled: false, ChannelSize: 50})

	tLog(t, "step 9: wait for bracket Argo apps to appear on dev")
	waitForArgoApp(t, devNamespace+"-"+bracket1)
	waitForArgoApp(t, devNamespace+"-"+bracket2)

	tLog(t, "step 10: wait for individual Argo apps to disappear from dev")
	for _, app := range allApps {
		waitForArgoAppGone(t, devNamespace+"-"+app)
	}

	tLog(t, "step 11: assert creation times still unchanged after bracket migration on dev")
	assertDeploymentCreationTimesStable(t, creationTimes, "enable dev brackets")
}

// TestBracketMoveBetweenBrackets is the regression test for the bug where moving
// an app from one bracket to another causes the deployment to be briefly deleted.
//
// Scenario:
//   - An app is deployed to staging inside bracket1.
//   - A new release is created for the same app, now assigned to bracket2.
//   - The staging release train is run.
//   - Expected: the bracket2 Argo app appears, and the app's pods are UNTOUCHED
//     (pod start time unchanged) — the old bracket1 Argo app must NOT cascade-delete
//     the K8s resources before bracket2 takes ownership.
func TestBracketMoveBetweenBrackets(t *testing.T) {
	cleanupCluster(t)
	tLogf(t, "runSuffix: %s", runSuffix)
	app := "bmb-app1-" + runSuffix
	bracket1 := "bmb-bracket1-" + runSuffix
	bracket2 := "bmb-bracket2-" + runSuffix

	tLog(t, "step 1: upgrade to staging=true (bracket mode)")
	helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, DevelopmentEnabled: false, StagingEnabled: true, ChannelSize: 50})

	tLog(t, "step 2: create v1 release for app in bracket1")
	createRelease(t, app, "sreteam", bracket1, "1", map[string]string{
		devNamespace:     stableManifest(app, devNamespace, "1"),
		stagingNamespace: stableManifest(app, stagingNamespace, "1"),
	})

	tLog(t, "step 3: run staging release train (deploys v1 from development2 to staging)")
	releaseTrain(t, stagingNamespace)

	tLog(t, "step 4: wait for v1 synced in staging")
	waitForDeploymentAnnotation(t, stagingNamespace, app+"-bracket-dep", "1")

	tLog(t, "step 5: wait for bracket1 Argo app to appear on staging")
	waitForArgoApp(t, "staging-"+bracket1)

	tLog(t, "step 6: record deployment creation time")
	creationTimes := map[deploymentKey]string{
		{stagingNamespace, app + "-bracket-dep"}: deploymentCreationTime(t, stagingNamespace, app+"-bracket-dep"),
	}
	tLogf(t, "  %s/%s: %s", stagingNamespace, app+"-bracket-dep", creationTimes[deploymentKey{stagingNamespace, app + "-bracket-dep"}])

	tLog(t, "step 7: create v2 release for same app, now assigned to bracket2")
	createRelease(t, app, "sreteam", bracket2, "2", map[string]string{
		devNamespace:     stableManifest(app, devNamespace, "2"),
		stagingNamespace: stableManifest(app, stagingNamespace, "2"),
	})

	tLog(t, "step 8: run staging release train (promotes v2, now in bracket2)")
	releaseTrain(t, stagingNamespace)

	tLog(t, "step 9: wait for bracket2 Argo app to appear on staging")
	waitForArgoApp(t, "staging-"+bracket2)

	tLog(t, "step 10: wait for v2 synced in staging")
	waitForDeploymentAnnotation(t, stagingNamespace, app+"-bracket-dep", "2")

	tLog(t, "step 11: assert pod start time stable (app moved bracket1→bracket2)")
	assertDeploymentCreationTimesStable(t, creationTimes, "move bracket1→bracket2")
}

// TestBracketMoveAndBack is the regression test for the scenario where an app
// moves from bracket1 to bracket2 and then back to bracket1.
// Previously this had lead to deployment deletions.
func TestBracketMoveAndBack(t *testing.T) {
	tcs := []struct {
		Name    string
		NumApps int
	}{
		{Name: "single app", NumApps: 1},
		{Name: "ten apps", NumApps: 10},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			cleanupCluster(t)
			tLogf(t, "runSuffix: %s", runSuffix)

			apps := make([]string, tc.NumApps)
			for i := range apps {
				apps[i] = fmt.Sprintf("bmab-app%d-%s", i+1, runSuffix)
			}
			bracket1 := "bmab-bracket1-" + runSuffix
			bracket2 := "bmab-bracket2-" + runSuffix

			tLog(t, "step 1: upgrade to staging=true (bracket mode)")
			helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, DevelopmentEnabled: false, StagingEnabled: true, ChannelSize: 50})

			tLog(t, "step 2: create v1 releases for all apps in bracket1")
			for _, app := range apps {
				createRelease(t, app, "sreteam", bracket1, "1", map[string]string{
					devNamespace:     stableManifest(app, devNamespace, "1"),
					stagingNamespace: stableManifest(app, stagingNamespace, "1"),
				})
			}

			tLog(t, "step 3: run staging release train (deploys v1 from development to staging)")
			releaseTrain(t, stagingNamespace)

			tLog(t, "step 4: wait for v1 synced in staging and bracket1 Argo app present")
			for _, app := range apps {
				waitForDeploymentAnnotation(t, stagingNamespace, app+"-bracket-dep", "1")
			}
			waitForArgoApp(t, "staging-"+bracket1)

			tLog(t, "step 5: record initial deployment creation times")
			creationTimes := map[deploymentKey]string{}
			for _, app := range apps {
				k := deploymentKey{stagingNamespace, app + "-bracket-dep"}
				creationTimes[k] = deploymentCreationTime(t, stagingNamespace, app+"-bracket-dep")
				tLogf(t, "  initial %s: %s", app, creationTimes[k])
			}

			// ---------- Move 1: bracket1 → bracket2 ----------

			tLog(t, "step 6: create v2 releases for all apps now in bracket2")
			for _, app := range apps {
				createRelease(t, app, "sreteam", bracket2, "2", map[string]string{
					devNamespace:     stableManifest(app, devNamespace, "2"),
					stagingNamespace: stableManifest(app, stagingNamespace, "2"),
				})
			}

			tLog(t, "step 7: run staging release train (promotes v2 in bracket2)")
			releaseTrain(t, stagingNamespace)

			tLog(t, "step 8: wait for bracket2 Argo app to appear and v2 synced for all apps")
			waitForArgoApp(t, "staging-"+bracket2)
			for _, app := range apps {
				waitForDeploymentAnnotation(t, stagingNamespace, app+"-bracket-dep", "2")
			}

			tLog(t, "step 9: assert pod start times stable after move 1 (b1→b2)")
			assertDeploymentCreationTimesStable(t, creationTimes, "move 1 (b1→b2)")

			// ---------- Move 2: bracket2 → bracket1 ----------

			tLog(t, "step 10: create v3 releases for all apps back in bracket1")
			for _, app := range apps {
				createRelease(t, app, "sreteam", bracket1, "3", map[string]string{
					devNamespace:     stableManifest(app, devNamespace, "3"),
					stagingNamespace: stableManifest(app, stagingNamespace, "3"),
				})
			}

			tLog(t, "step 11: run staging release train (promotes v3 back in bracket1)")
			releaseTrain(t, stagingNamespace)

			tLog(t, "step 12: wait for bracket1 Argo app to reappear and v3 synced for all apps")
			waitForArgoApp(t, "staging-"+bracket1)
			for _, app := range apps {
				waitForDeploymentAnnotation(t, stagingNamespace, app+"-bracket-dep", "3")
			}

			tLog(t, "step 13: assert pod start times stable after move 2 (b2→b1)")
			assertDeploymentCreationTimesStable(t, creationTimes, "move 2 (b2→b1)")
		})
	}
}

// argoAppTrackedResources returns the resources the named Argo Application
// currently tracks, one "Kind/name" per line (from .status.resources).
func argoAppTrackedResources(t *testing.T, name string) string {
	t.Helper()
	out, err := exec.Command(
		"kubectl", "get", "application", name,
		"-n", globalCfg.KuberpultNamespace,
		"-o", `jsonpath={range .status.resources[*]}{.kind}/{.name}{"\n"}{end}`,
	).Output()
	if err != nil {
		t.Fatalf("get application %s resources: %v", name, err)
	}
	return string(out)
}

// waitForArgoAppNotTracking polls until the named Argo Application no longer
// tracks the given "Kind/name" resource in .status.resources.
func waitForArgoAppNotTracking(t *testing.T, argoApp, kindSlashName string) {
	t.Helper()
	deadline := time.Now().Add(argoAppGoneTimeout)
	for time.Now().Before(deadline) {
		resources := argoAppTrackedResources(t, argoApp)
		if !strings.Contains(resources, kindSlashName+"\n") {
			tLogf(t, "  Argo app %s no longer tracks %s", argoApp, kindSlashName)
			return
		}
		time.Sleep(argoAppPollInterval)
	}
	dumpBracketDiagnostics(t, []string{argoApp}, []string{devNamespace})
	t.Fatalf("Argo app %q still tracks %q after 3 minutes — the old bracket was never spec-refreshed after the move", argoApp, kindSlashName)
}

// waitForArgoAppTracking polls until the named Argo Application DOES track the
// given "Kind/name" resource in .status.resources (the inverse of
// waitForArgoAppNotTracking) — e.g. a gainer bracket adopting a moved app.
func waitForArgoAppTracking(t *testing.T, argoApp, kindSlashName string) {
	t.Helper()
	deadline := time.Now().Add(argoAppGoneTimeout)
	for time.Now().Before(deadline) {
		resources := argoAppTrackedResources(t, argoApp)
		if strings.Contains(resources, kindSlashName+"\n") {
			tLogf(t, "  Argo app %s tracks %s", argoApp, kindSlashName)
			return
		}
		time.Sleep(argoAppPollInterval)
	}
	dumpBracketDiagnostics(t, []string{argoApp}, []string{devNamespace})
	t.Fatalf("Argo app %q never tracked %q after 3 minutes", argoApp, kindSlashName)
}

// TestBracketMoveOutOfSharedBracket is the regression test for the bug where
// moving an app OUT of a bracket that still has other members left the app in
// BOTH brackets: the old bracket's Argo app spec was never refreshed (it kept
// pinning the stale brackets_history snapshot), so the reposerver kept rendering
// the moved app's manifests in the old bracket while the new bracket rendered
// them too — Argo CD then fought over the app's resources.
//
// Scenario:
//   - app1 and app2 share bracket1 on development.
//   - app2 gets a new release assigned to bracket2 (bracket1 keeps app1).
//   - Expected: bracket1's Argo app stops tracking app2's resources (only
//     bracket2 owns them), and no Deployment is deleted+recreated.
func TestBracketMoveOutOfSharedBracket(t *testing.T) {
	cleanupCluster(t)
	tLogf(t, "runSuffix: %s", runSuffix)
	app1 := "bmosb-app1-" + runSuffix
	app2 := "bmosb-app2-" + runSuffix
	bracket1 := "bmosb-bracket1-" + runSuffix
	bracket2 := "bmosb-bracket2-" + runSuffix

	tLog(t, "step 1: upgrade to development=true (bracket mode)")
	helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, DevelopmentEnabled: true, StagingEnabled: false, ChannelSize: 50})

	tLog(t, "step 2: create v1 releases for app1+app2 in bracket1")
	createRelease(t, app1, "sreteam", bracket1, "1", map[string]string{
		devNamespace: stableManifest(app1, devNamespace, "1"),
	})
	createRelease(t, app2, "sreteam", bracket1, "1", map[string]string{
		devNamespace: stableManifest(app2, devNamespace, "1"),
	})

	tLog(t, "step 3: wait for bracket1 Argo app and v1 synced in dev (both apps)")
	waitForArgoApp(t, devNamespace+"-"+bracket1)
	waitForDeploymentAnnotation(t, devNamespace, app1+"-bracket-dep", "1")
	waitForDeploymentAnnotation(t, devNamespace, app2+"-bracket-dep", "1")

	tLog(t, "step 4: record deployment creation times")
	creationTimes := map[deploymentKey]string{
		{devNamespace, app1 + "-bracket-dep"}: deploymentCreationTime(t, devNamespace, app1+"-bracket-dep"),
		{devNamespace, app2 + "-bracket-dep"}: deploymentCreationTime(t, devNamespace, app2+"-bracket-dep"),
	}

	tLog(t, "step 5: create v2 release for app2, now in bracket2 (bracket1 keeps app1)")
	createRelease(t, app2, "sreteam", bracket2, "2", map[string]string{
		devNamespace: stableManifest(app2, devNamespace, "2"),
	})

	tLog(t, "step 6: wait for bracket2 Argo app and v2 synced (app1 stays at v1)")
	waitForArgoApp(t, devNamespace+"-"+bracket2)
	waitForDeploymentAnnotation(t, devNamespace, app2+"-bracket-dep", "2")
	waitForDeploymentAnnotation(t, devNamespace, app1+"-bracket-dep", "1")

	tLog(t, "step 7: assert bracket1 no longer tracks app2's resources (no fighting)")
	waitForArgoAppNotTracking(t, devNamespace+"-"+bracket1, "Deployment/"+app2+"-bracket-dep")

	tLog(t, "step 8: sanity: bracket1 still tracks app1, bracket2 tracks app2")
	bracket1Resources := argoAppTrackedResources(t, devNamespace+"-"+bracket1)
	if !strings.Contains(bracket1Resources, "Deployment/"+app1+"-bracket-dep\n") {
		t.Errorf("bracket1 Argo app lost app1's deployment, tracked resources:\n%s", bracket1Resources)
	}
	bracket2Resources := argoAppTrackedResources(t, devNamespace+"-"+bracket2)
	if !strings.Contains(bracket2Resources, "Deployment/"+app2+"-bracket-dep\n") {
		t.Errorf("bracket2 Argo app does not track app2's deployment, tracked resources:\n%s", bracket2Resources)
	}

	tLog(t, "step 9: assert pod start times stable (app2 moved out of shared bracket1)")
	assertDeploymentCreationTimesStable(t, creationTimes, "move out of shared bracket")
}

// TestBracketMoveIntoPopulatedBracket verifies that moving an app INTO a bracket
// that already owns a live member adopts the moved app without disturbing the
// existing member, while the loser bracket (still populated) stops tracking the
// moved app. All three workloads must stay up (no pod restart).
//
// Scenario (development, bracket mode):
//   - app1 and app2 share bracket1; appX is the sole member of bracket2. All deployed.
//   - app1 gets a new release assigned to bracket2 (bracket1 keeps app2).
//   - Expected: bracket2 tracks BOTH appX and app1; bracket1 stops tracking app1 but
//     keeps app2; and app1, app2, appX Deployment creationTimestamps are unchanged.
func TestBracketMoveIntoPopulatedBracket(t *testing.T) {
	cleanupCluster(t)
	tLogf(t, "runSuffix: %s", runSuffix)
	app1 := "bmip-app1-" + runSuffix
	app2 := "bmip-app2-" + runSuffix
	appX := "bmip-appx-" + runSuffix
	bracket1 := "bmip-bracket1-" + runSuffix
	bracket2 := "bmip-bracket2-" + runSuffix

	tLog(t, "step 1: upgrade to development=true (bracket mode)")
	helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, DevelopmentEnabled: true, StagingEnabled: false, ChannelSize: 50})

	tLog(t, "step 2: create v1 releases — app1+app2 in bracket1, appX in bracket2")
	createRelease(t, app1, "sreteam", bracket1, "1", map[string]string{
		devNamespace: stableManifest(app1, devNamespace, "1"),
	})
	createRelease(t, app2, "sreteam", bracket1, "1", map[string]string{
		devNamespace: stableManifest(app2, devNamespace, "1"),
	})
	createRelease(t, appX, "sreteam", bracket2, "1", map[string]string{
		devNamespace: stableManifest(appX, devNamespace, "1"),
	})

	tLog(t, "step 3: wait for both bracket Argo apps and v1 synced in dev")
	waitForArgoApp(t, devNamespace+"-"+bracket1)
	waitForArgoApp(t, devNamespace+"-"+bracket2)
	waitForDeploymentAnnotation(t, devNamespace, app1+"-bracket-dep", "1")
	waitForDeploymentAnnotation(t, devNamespace, app2+"-bracket-dep", "1")
	waitForDeploymentAnnotation(t, devNamespace, appX+"-bracket-dep", "1")

	tLog(t, "step 4: record deployment creation times for all three apps")
	creationTimes := map[deploymentKey]string{
		{devNamespace, app1 + "-bracket-dep"}: deploymentCreationTime(t, devNamespace, app1+"-bracket-dep"),
		{devNamespace, app2 + "-bracket-dep"}: deploymentCreationTime(t, devNamespace, app2+"-bracket-dep"),
		{devNamespace, appX + "-bracket-dep"}: deploymentCreationTime(t, devNamespace, appX+"-bracket-dep"),
	}

	tLog(t, "step 5: create v2 for app1, now in bracket2 (already holding appX); bracket1 keeps app2")
	createRelease(t, app1, "sreteam", bracket2, "2", map[string]string{
		devNamespace: stableManifest(app1, devNamespace, "2"),
	})

	tLog(t, "step 6: wait for app1 v2 synced (adopted by bracket2)")
	waitForDeploymentAnnotation(t, devNamespace, app1+"-bracket-dep", "2")

	tLog(t, "step 7: bracket2 (populated gainer) tracks BOTH appX and app1")
	waitForArgoAppTracking(t, devNamespace+"-"+bracket2, "Deployment/"+app1+"-bracket-dep")
	b2 := argoAppTrackedResources(t, devNamespace+"-"+bracket2)
	if !strings.Contains(b2, "Deployment/"+appX+"-bracket-dep\n") {
		t.Errorf("gainer bracket2 lost its existing member appX, tracked resources:\n%s", b2)
	}

	tLog(t, "step 8: bracket1 (loser, still populated) stops tracking app1 but keeps app2")
	waitForArgoAppNotTracking(t, devNamespace+"-"+bracket1, "Deployment/"+app1+"-bracket-dep")
	b1 := argoAppTrackedResources(t, devNamespace+"-"+bracket1)
	if !strings.Contains(b1, "Deployment/"+app2+"-bracket-dep\n") {
		t.Errorf("loser bracket1 lost app2's deployment, tracked resources:\n%s", b1)
	}

	tLog(t, "step 9: assert pod start times stable (gainer's member, moved app, loser's member)")
	assertDeploymentCreationTimesStable(t, creationTimes, "move into populated bracket")
}

// TestBracketMoveOutOfSharedBracketActiveActive is the active-active variant of
// TestBracketMoveOutOfSharedBracket. The bracket lives in the AA env "aa-test"
// (commonEnvPrefix "aa", concrete configs "dev-1"/"dev-2"), so each bracket is
// realized as TWO Argo Applications (aa-aa-test-dev-1-<bracket>,
// aa-aa-test-dev-2-<bracket>) that both render the same bracket manifest path and,
// in the single kind cluster, both manage the one Deployment per app in ns aa-test.
//
// The pause/retarget/resume protocol runs per Argo Application, so a move must keep
// EVERY concrete app consistent — if one concrete app lagged, it would prune the
// moved app's workload mid-handover. This test moves app2 out of a shared bracket1
// and asserts, for BOTH concrete envs, that bracket1 stops tracking app2 (keeps
// app1) and bracket2 adopts app2, with no Deployment delete+recreate.
//
// aa-test upstreams from development2 (upstream.latest), so releases are created with
// a development2 manifest (auto-deployed there) and promoted into aa-test via a
// release train.
func TestBracketMoveOutOfSharedBracketActiveActive(t *testing.T) {
	cleanupCluster(t)
	tLogf(t, "runSuffix: %s", runSuffix)
	app1 := "bmaa-a1-" + runSuffix
	app2 := "bmaa-a2-" + runSuffix
	bracket1 := "bmaa-b1-" + runSuffix
	bracket2 := "bmaa-b2-" + runSuffix
	// Concrete env names from aa-test/config.json. The bracket Argo app name is
	// "aa-<aaNamespace>-<concrete>-<bracket>" (commonEnvPrefix-env-concrete-bracket).
	concreteEnvs := []string{"dev-1", "dev-2"}
	aaBracketApp := func(concrete, bracket string) string {
		return fmt.Sprintf("aa-%s-%s-%s", aaNamespace, concrete, bracket)
	}

	tLog(t, "step 1: enable bracket mode on the active-active aa-test env")
	helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, AATestEnabled: true, ChannelSize: 50})

	tLog(t, "step 2: create v1 releases for app1+app2 in bracket1 (development2 upstream + aa-test)")
	for _, app := range []string{app1, app2} {
		createRelease(t, app, "sreteam", bracket1, "1", map[string]string{
			devTwoNamespace: stableManifest(app, devTwoNamespace, "1"),
			aaNamespace:     stableManifest(app, aaNamespace, "1"),
		})
	}

	tLog(t, "step 3: release train into aa-test (promotes v1 from development2)")
	releaseTrain(t, aaNamespace)

	tLog(t, "step 4: wait for both concrete bracket1 Argo apps + v1 synced on aa-test")
	for _, c := range concreteEnvs {
		waitForArgoApp(t, aaBracketApp(c, bracket1))
	}
	waitForDeploymentAnnotation(t, aaNamespace, app1+"-bracket-dep", "1")
	waitForDeploymentAnnotation(t, aaNamespace, app2+"-bracket-dep", "1")

	tLog(t, "step 5: record creation times (one shared Deployment per app in ns aa-test)")
	creationTimes := map[deploymentKey]string{
		{aaNamespace, app1 + "-bracket-dep"}: deploymentCreationTime(t, aaNamespace, app1+"-bracket-dep"),
		{aaNamespace, app2 + "-bracket-dep"}: deploymentCreationTime(t, aaNamespace, app2+"-bracket-dep"),
	}

	tLog(t, "step 6: move app2 to bracket2 (bracket1 keeps app1)")
	createRelease(t, app2, "sreteam", bracket2, "2", map[string]string{
		devTwoNamespace: stableManifest(app2, devTwoNamespace, "2"),
		aaNamespace:     stableManifest(app2, aaNamespace, "2"),
	})

	tLog(t, "step 7: release train into aa-test (promotes app2 v2 into bracket2)")
	releaseTrain(t, aaNamespace)

	tLog(t, "step 8: wait for both concrete bracket2 Argo apps + app2 v2 synced")
	for _, c := range concreteEnvs {
		waitForArgoApp(t, aaBracketApp(c, bracket2))
	}
	waitForDeploymentAnnotation(t, aaNamespace, app2+"-bracket-dep", "2")

	tLog(t, "step 9: each concrete env — bracket1 stops tracking app2 (keeps app1), bracket2 tracks app2")
	for _, c := range concreteEnvs {
		waitForArgoAppNotTracking(t, aaBracketApp(c, bracket1), "Deployment/"+app2+"-bracket-dep")
		b1 := argoAppTrackedResources(t, aaBracketApp(c, bracket1))
		if !strings.Contains(b1, "Deployment/"+app1+"-bracket-dep\n") {
			t.Errorf("[%s] bracket1 lost app1's deployment, tracked resources:\n%s", c, b1)
		}
		waitForArgoAppTracking(t, aaBracketApp(c, bracket2), "Deployment/"+app2+"-bracket-dep")
	}

	tLog(t, "step 10: assert pod start times stable (per-concrete pause protocol kept both Argo apps consistent)")
	assertDeploymentCreationTimesStable(t, creationTimes, "active-active move out of shared bracket")
}

// TestBracketMigrateDevelopment is the regression test for the bug where enabling
// experimentalBrackets for the development cluster (while staging stays disabled)
// caused all development apps to vanish without bracket apps replacing them.
//
// Scenario:
//   - Two apps share a bracket, deployed to development in NON-bracket mode.
//   - Pod start times are recorded.
//   - The cluster is upgraded to experimentalBrackets.clusters.development=true
//     (staging stays false) — the exact user-reported config change.
//   - Expected: bracket Argo app appears on development, individual Argo apps are
//     deleted, and all K8s Deployments are UNTOUCHED (pod start times unchanged).
func TestBracketMigrateDevelopment(t *testing.T) {
	cleanupCluster(t)
	tLogf(t, "runSuffix: %s", runSuffix)
	app1 := "bmd-app1-" + runSuffix
	app2 := "bmd-app2-" + runSuffix
	apps := []string{app1, app2}
	bracket := "bmd-bracket-" + runSuffix

	tLog(t, "step 1: upgrade to brackets enabled, development=false (individual-app mode)")
	helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, DevelopmentEnabled: false, StagingEnabled: false, ChannelSize: 1})

	tLog(t, "step 2: create v1 releases (dev + dev2 manifests)")
	for _, app := range apps {
		createRelease(t, app, "sreteam", bracket, "1", map[string]string{
			devNamespace: stableManifest(app, devNamespace, "1"),
		})
	}

	tLog(t, "step 3: wait for v1 synced in development")
	for _, app := range apps {
		waitForDeploymentAnnotation(t, devNamespace, app+"-bracket-dep", "1")
	}

	tLog(t, "step 4: verify individual Argo apps exist on development")
	for _, app := range apps {
		waitForArgoApp(t, devNamespace+"-"+app)
	}

	tLog(t, "step 5: record deployment creation times in development")
	creationTimes := map[deploymentKey]string{}
	for _, app := range apps {
		creationTimes[deploymentKey{devNamespace, app + "-bracket-dep"}] = deploymentCreationTime(t, devNamespace, app+"-bracket-dep")
		tLogf(t, "  %s/%s: %s", devNamespace, app+"-bracket-dep", creationTimes[deploymentKey{devNamespace, app + "-bracket-dep"}])
	}

	tLog(t, "step 6: upgrade to development=true (the bug-triggering upgrade)")
	helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, DevelopmentEnabled: true, StagingEnabled: false, ChannelSize: 1})

	tLog(t, "step 7: wait for bracket Argo app to appear on development")
	waitForArgoApp(t, devNamespace+"-"+bracket)

	tLog(t, "step 8: wait for individual Argo apps to be deleted from development")
	for _, app := range apps {
		waitForArgoAppGone(t, devNamespace+"-"+app)
	}

	tLog(t, "step 9: assert pod start times stable (enable development brackets)")
	assertDeploymentCreationTimesStable(t, creationTimes, "enable development brackets")
}

// dumpBracketDiagnostics dumps ArgoCD app status, namespace events, and service logs
// to help diagnose unexpected pod restarts during bracket transitions.
func dumpBracketDiagnostics(t *testing.T, argoApps []string, namespaces []string) {
	t.Helper()
	for _, app := range argoApps {
		if out, err := exec.Command("kubectl", "describe", "application", app, "-n", globalCfg.KuberpultNamespace).CombinedOutput(); err == nil {
			tLogf(t, "ArgoCD app describe %s:\n%s", app, out)
		}
	}
	for _, ns := range namespaces {
		if out, err := exec.Command("kubectl", "get", "events", "-n", ns, "--sort-by=.lastTimestamp").CombinedOutput(); err == nil {
			tLogf(t, "events in %s:\n%s", ns, out)
		}
	}
	for _, svc := range []string{"rollout-service", "manifest-repo-export-service"} {
		dep := "deployment/kuberpult-" + svc
		if out, err := exec.Command("kubectl", "logs", "-n", globalCfg.KuberpultNamespace, dep, "--tail=100").CombinedOutput(); err == nil {
			tLogf(t, "%s logs (tail 100):\n%s", svc, out)
		}
	}
	if out, err := exec.Command("kubectl", "logs", "-n", globalCfg.KuberpultNamespace, "statefulset/argocd-application-controller", "--tail=100").CombinedOutput(); err == nil {
		tLogf(t, "argocd-application-controller logs (tail 100):\n%s", out)
	}
}

// dumpBracketDiagnosticsExpanded is like dumpBracketDiagnostics but also describes
// individual (non-bracket) ArgoCD apps and uses --since=5m for logs so the full
// bracket-migration window is captured regardless of log volume.
func dumpBracketDiagnosticsExpanded(t *testing.T, bracketApps []string, individualApps []string, namespaces []string) {
	t.Helper()
	for _, app := range bracketApps {
		if out, err := exec.Command("kubectl", "describe", "application", app, "-n", globalCfg.KuberpultNamespace).CombinedOutput(); err == nil {
			tLogf(t, "ArgoCD app describe %s:\n%s", app, out)
		}
	}
	for _, app := range individualApps {
		if out, err := exec.Command("kubectl", "describe", "application", app, "-n", globalCfg.KuberpultNamespace).CombinedOutput(); err == nil {
			tLogf(t, "ArgoCD app describe (individual) %s:\n%s", app, out)
		}
	}
	for _, ns := range namespaces {
		if out, err := exec.Command("kubectl", "get", "events", "-n", ns, "--sort-by=.lastTimestamp").CombinedOutput(); err == nil {
			tLogf(t, "events in %s:\n%s", ns, out)
		}
	}
	if out, err := exec.Command("kubectl", "logs", "-n", globalCfg.KuberpultNamespace, "deployment/kuberpult-rollout-service", "--since=5m").CombinedOutput(); err == nil {
		tLogf(t, "rollout-service logs (since 5m):\n%s", out)
	}
	if out, err := exec.Command("kubectl", "logs", "-n", globalCfg.KuberpultNamespace, "statefulset/argocd-application-controller", "--since=5m").CombinedOutput(); err == nil {
		tLogf(t, "argocd-application-controller logs (since 5m):\n%s", out)
	}
	if out, err := exec.Command("kubectl", "logs", "-n", globalCfg.KuberpultNamespace, "deployment/argocd-server", "--since=5m").CombinedOutput(); err == nil {
		tLogf(t, "argocd-server logs (since 5m):\n%s", out)
	}
	if out, err := exec.Command("kubectl", "logs", "-n", globalCfg.KuberpultNamespace, "deployment/kuberpult-reposerver-service", "--since=5m").CombinedOutput(); err == nil {
		tLogf(t, "kuberpult-reposerver-service logs (since 5m):\n%s", out)
	}
	if out, err := exec.Command("kubectl", "logs", "-n", globalCfg.KuberpultNamespace, "deployment/kuberpult-rollout-service", "--since=5m", "--previous").CombinedOutput(); err == nil {
		tLogf(t, "rollout-service logs (previous container, since 5m):\n%s", out)
	}
	if out, err := exec.Command("kubectl", "get", "appprojects", "-n", globalCfg.KuberpultNamespace, "-o", "yaml").CombinedOutput(); err == nil {
		tLogf(t, "AppProjects in %s:\n%s", globalCfg.KuberpultNamespace, out)
	}
}

// argoRBACBasePolicy is the policy.csv a freshly-installed ArgoCD has (see
// charts/kuberpult/run-kind.sh). denyArgoAppCreate rewrites policy.csv from this
// base plus a deny rule, and first asserts the live policy still matches this
// constant so a drift in the ArgoCD setup fails loudly here.
const argoRBACBasePolicy = `p, role:kuberpult, applications, get, */*, allow
     p, role:kuberpult, applications, create, */*, allow
     p, role:kuberpult, applications, update, */*, allow
     p, role:kuberpult, applications, sync, */*, allow
     p, role:kuberpult, applications, delete, */*, allow
     g, kuberpult, role:kuberpult
     `

// argoRBACConfigMap holds ArgoCD's RBAC policy.csv. It lives in the same
// namespace as ArgoCD (== the kuberpult namespace): "default" in kind.
const argoRBACConfigMap = "argocd-rbac-cm"

func undoDenyArgoAppCreate(t *testing.T) {
	t.Helper()
	ns := globalCfg.KuberpultNamespace

	// Guard: fail loudly if ArgoCD's default policy drifted from our constant.
	out, err := exec.Command("kubectl", "get", "configmap", argoRBACConfigMap,
		"-n", ns, "-o", `jsonpath={.data.policy\.csv}`).CombinedOutput()
	if err != nil {
		t.Fatalf("denyArgoAppCreate: read %s: %v: %s", argoRBACConfigMap, err, out)
	}
	if got, want := normalizePolicy(string(out)), normalizePolicy(argoRBACBasePolicy); got == want {
		t.Fatalf("undoDenyArgoAppCreate: pointless call, configmap is in original state")
	}

	newPolicy := argoRBACBasePolicy
	patch, err := json.Marshal(map[string]any{
		"data": map[string]string{"policy.csv": newPolicy},
	})
	if err != nil {
		t.Fatalf("undoDenyArgoAppCreate: marshal patch: %v", err)
	}
	if out, err := exec.Command("kubectl", "apply", "configmap", argoRBACConfigMap,
		"-n", ns, string(patch)).CombinedOutput(); err != nil {
		t.Fatalf("undoDenyArgoAppCreate: patch %s: %v: %s", argoRBACConfigMap, err, out)
	}
	tLogf(t, "undoDenyArgoAppCreate: done")
}

// setArgoRBACPolicy sets ArgoCD's RBAC policy.csv by upgrading the argo-cd
// helm release itself (release "argocd", installed by run-kind.sh). Going
// through helm keeps it the sole field-manager of the argocd-rbac-cm
// ConfigMap — a kubectl patch would grab ownership of .data.policy.csv and
// make every later helm upgrade of argo-cd fail with a field conflict.
func setArgoRBACPolicy(t *testing.T, policyCSV string) {
	t.Helper()
	// --set cannot carry the policy (it contains commas, helm's --set
	// separator), so write it to a file and use --set-file.
	policyFile := filepath.Join(t.TempDir(), "policy.csv")
	if err := os.WriteFile(policyFile, []byte(policyCSV), 0644); err != nil {
		t.Fatalf("setArgoRBACPolicy: write policy file: %v", err)
	}
	out, err := exec.Command("helm", "upgrade", "argocd", "argo-cd/argo-cd",
		"--version", "5.36.0", // must match run-kind.sh
		"--history-max", "1",
		"--reuse-values",
		"--set-file", `configs.rbac.policy\.csv=`+policyFile,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("setArgoRBACPolicy: helm upgrade: %v\n%s", err, out)
	}
	tLogf(t, "setArgoRBACPolicy: policy is now:\n%s", policyCSV)
}
func denyArgoAppCreate(t *testing.T, envName, appName string) {
	t.Helper()
	combinedName := envName + "-" + appName
	denyLine := fmt.Sprintf("p, role:kuberpult, applications, create, */%s, deny\n", combinedName)
	setArgoRBACPolicy(t, argoRBACBasePolicy+denyLine)
	t.Cleanup(func() { setArgoRBACPolicy(t, argoRBACBasePolicy) })
}

//// denyArgoAppCreate forbids the kuberpult role from creating the Argo
//// Application named argoAppName, by rewriting policy.csv of argocd-rbac-cm to
//// the known base plus a casbin "deny" rule. An explicit deny overrides the broad
//// "create */*, allow", so only this one app is blocked.
////
//// ArgoCD reloads argocd-rbac-cm without a restart, so none is triggered here.
//func denyArgoAppCreate(t *testing.T, argoAppName string) {
//	t.Helper()
//	ns := globalCfg.KuberpultNamespace
//
//	// Guard: fail loudly if ArgoCD's default policy drifted from our constant.
//	out, err := exec.Command("kubectl", "get", "configmap", argoRBACConfigMap,
//		"-n", ns, "-o", `jsonpath={.data.policy\.csv}`).CombinedOutput()
//	if err != nil {
//		t.Fatalf("denyArgoAppCreate: read %s: %v: %s", argoRBACConfigMap, err, out)
//	}
//	if got, want := normalizePolicy(string(out)), normalizePolicy(argoRBACBasePolicy); got != want {
//		t.Fatalf("denyArgoAppCreate: live ArgoCD policy.csv differs from argoRBACBasePolicy; "+
//			"update the constant.\n--- live ---\n%s\n--- expected ---\n%s", got, want)
//	}
//
//	denyLine := fmt.Sprintf("p, role:kuberpult, applications, create, */%s, deny", argoAppName)
//	newPolicy := argoRBACBasePolicy + denyLine + "\n"
//
//	patch, err := json.Marshal(map[string]any{
//		"data": map[string]string{"policy.csv": newPolicy},
//	})
//	if err != nil {
//		t.Fatalf("denyArgoAppCreate: marshal patch: %v", err)
//	}
//	if out, err := exec.Command("kubectl", "patch", "configmap", argoRBACConfigMap,
//		"-n", ns, "--type", "merge", "-p", string(patch)).CombinedOutput(); err != nil {
//		t.Fatalf("denyArgoAppCreate: patch %s: %v: %s", argoRBACConfigMap, err, out)
//	}
//	tLogf(t, "denyArgoAppCreate: denied Argo app creation for %s", argoAppName)
//}

func normalizePolicy(s string) string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return strings.Join(lines, "\n")
}
