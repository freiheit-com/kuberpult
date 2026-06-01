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
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	auth "github.com/freiheit-com/kuberpult/pkg/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const cdServiceGrpcAddr = "localhost:5004"
const argoNamespace = "default"

type helmUpgradeParams struct {
	bracketsEnabled    bool
	developmentEnabled bool
	stagingEnabled     bool
	channelSize        int
}

// helmUpgrade calls helm upgrade with the given bracket configuration and waits
// for all services to finish rolling out.
func helmUpgrade(t *testing.T, p helmUpgradeParams) {
	t.Helper()
	out, err := exec.Command("git", "describe", "--always", "--long", "--tags").Output()
	if err != nil {
		t.Fatalf("git describe: %v", err)
	}
	version := strings.TrimSpace(string(out))
	repoRoot, err2 := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err2 != nil {
		t.Fatalf("git rev-parse: %v", err2)
	}
	root := strings.TrimSpace(string(repoRoot))
	chartPath := root + "/charts/kuberpult/kuberpult-" + version + ".tgz"
	valsPath := root + "/charts/kuberpult/vals.yaml"

	boolStr := func(b bool) string {
		if b {
			return "true"
		}
		return "false"
	}
	tLogf(t, "helmUpgrade: enabled=%s development=%s staging=%s channelSize=%d chart=%s",
		boolStr(p.bracketsEnabled), boolStr(p.developmentEnabled), boolStr(p.stagingEnabled), p.channelSize, chartPath)

	cmd := exec.Command("helm", "upgrade", "--install",
		"--values", valsPath,
		"--set", "rollout.experimentalBrackets.enabled="+boolStr(p.bracketsEnabled),
		"--set", "rollout.experimentalBrackets.clusters.development="+boolStr(p.developmentEnabled),
		"--set", "rollout.experimentalBrackets.clusters.staging="+boolStr(p.stagingEnabled),
		"--set", fmt.Sprintf("rollout.kuberpultEventsChannelSize=%d", p.channelSize),
		"kuberpult-local", chartPath)
	if out2, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helm upgrade (enabled=%s dev=%s staging=%s): %v\n%s",
			boolStr(p.bracketsEnabled), boolStr(p.developmentEnabled), boolStr(p.stagingEnabled), err, out2)
	}

	for _, dep := range []string{
		"deployment/kuberpult-rollout-service",
		"deployment/kuberpult-cd-service",
		"deployment/kuberpult-frontend-service",
		"deployment/kuberpult-reposerver-service",
	} {
		out3, err := exec.Command("kubectl", "rollout", "status", dep, "--timeout=3m").CombinedOutput()
		if err != nil {
			t.Fatalf("kubectl rollout status %s: %v\n%s", dep, err, out3)
		}
		tLogf(t, "%s rolled out: %s", dep, strings.TrimSpace(string(out3)))
	}
	globalPFM.restart()
	waitForFrontendHTTPReady(t)
	scriptPath := root + "/infrastructure/scripts/create-testdata/create-environments.sh"
	if out4, err := exec.Command("bash", scriptPath).CombinedOutput(); err != nil {
		t.Fatalf("create-environments: %v\n%s", err, out4)
	}
	tLog(t, "create-environments: done")
}

// waitForArgoApp polls until the named Argo Application exists in the default namespace.
func waitForArgoApp(t *testing.T, name string) {
	t.Helper()
	deadline := time.Now().Add(argoAppWaitTimeout)
	for time.Now().Before(deadline) {
		out, err := exec.Command("kubectl", "get", "application", name, "-n", argoNamespace).Output()
		if err == nil && strings.Contains(string(out), name) {
			tLogf(t, "  Argo app present: %s", name)
			return
		}
		time.Sleep(argoAppPollInterval)
	}
	t.Fatalf("Argo app %q never appeared after 2 minutes", name)
}

// waitForArgoAppGone polls until the named Argo Application no longer exists.
func waitForArgoAppGone(t *testing.T, name string) {
	t.Helper()
	deadline := time.Now().Add(argoAppGoneTimeout)
	for time.Now().Before(deadline) {
		_, err := exec.Command("kubectl", "get", "application", name, "-n", argoNamespace).Output()
		if err != nil {
			tLogf(t, "  Argo app gone: %s", name)
			return
		}
		time.Sleep(argoAppPollInterval)
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
	helmUpgrade(t, helmUpgradeParams{bracketsEnabled: true, developmentEnabled: false, stagingEnabled: false, channelSize: 50})

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
	helmUpgrade(t, helmUpgradeParams{bracketsEnabled: true, developmentEnabled: false, stagingEnabled: true, channelSize: 50})

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
func TestBracketDeleteEnvFromApp(t *testing.T) {
	cleanupCluster(t)
	tLogf(t, "runSuffix: %s", runSuffix)
	app := "bde-app1-" + runSuffix
	bracket := "bde-bracket-" + runSuffix

	tLog(t, "step 1: upgrade to staging=true (bracket mode)")
	helmUpgrade(t, helmUpgradeParams{bracketsEnabled: true, developmentEnabled: false, stagingEnabled: true, channelSize: 50})

	tLog(t, "step 2: create v1 release (dev + staging manifests)")
	createRelease(t, app, "sreteam", bracket, "1", map[string]string{
		devNamespace:     stableManifest(app, devNamespace, "1"),
		stagingNamespace: stableManifest(app, stagingNamespace, "1"),
	})

	tLog(t, "step 3: run staging release train")
	releaseTrain(t, stagingNamespace)

	tLog(t, "step 4: wait for v1 synced in staging")
	waitForDeploymentAnnotation(t, stagingNamespace, app+"-bracket-dep", "1")

	tLog(t, "step 5: wait for bracket Argo app to appear on staging")
	waitForArgoApp(t, "staging-"+bracket)

	tLog(t, "step 6: delete env from app")
	deleteEnvFromApp(t, stagingNamespace, app)

	tLogf(t, "step 7: %s buffer for rollout-service to reconcile", reconcileBuffer)
	time.Sleep(reconcileBuffer)

	tLog(t, "step 8: verify bracket Argo app is cascade-deleted (no member deployed in env)")
	waitForArgoAppGone(t, "staging-"+bracket)
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
	helmUpgrade(t, helmUpgradeParams{bracketsEnabled: true, developmentEnabled: false, stagingEnabled: true, channelSize: 50})

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
	helmUpgrade(t, helmUpgradeParams{bracketsEnabled: true, developmentEnabled: false, stagingEnabled: false, channelSize: 50})

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
	helmUpgrade(t, helmUpgradeParams{bracketsEnabled: true, developmentEnabled: false, stagingEnabled: true, channelSize: 50})

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
	helmUpgrade(t, helmUpgradeParams{bracketsEnabled: true, developmentEnabled: false, stagingEnabled: true, channelSize: 50})

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

// TestBracketUndeploy verifies that undeploying the only app in bracket1:
// (a) deletes the bracket1 Argo Application, and
// (b) does not restart pods belonging to a separate bracket2.
func TestBracketUndeploy(t *testing.T) {
	cleanupCluster(t)
	tLogf(t, "runSuffix: %s", runSuffix)
	app1 := "bu-app1-" + runSuffix
	bracket1 := "bu-bracket1-" + runSuffix
	app2 := "bu-app2-" + runSuffix
	bracket2 := "bu-bracket2-" + runSuffix

	tLog(t, "step 1: upgrade to staging=true (bracket mode)")
	helmUpgrade(t, helmUpgradeParams{bracketsEnabled: true, developmentEnabled: false, stagingEnabled: true, channelSize: 50})

	tLog(t, "step 2: create v1 releases for both apps")
	createRelease(t, app1, "sreteam", bracket1, "1", map[string]string{
		devNamespace:     stableManifest(app1, devNamespace, "1"),
		stagingNamespace: stableManifest(app1, stagingNamespace, "1"),
	})
	createRelease(t, app2, "sreteam", bracket2, "1", map[string]string{
		devNamespace:     stableManifest(app2, devNamespace, "1"),
		stagingNamespace: stableManifest(app2, stagingNamespace, "1"),
	})

	tLog(t, "step 3: run staging release train (deploys both apps)")
	releaseTrain(t, stagingNamespace)

	tLog(t, "step 4: wait for both bracket Argo apps to appear on staging")
	waitForArgoApp(t, "staging-"+bracket1)
	waitForArgoApp(t, "staging-"+bracket2)

	waitForDeploymentAnnotation(t, stagingNamespace, app2+"-bracket-dep", "1")
	waitForDeploymentAnnotation(t, devNamespace, app2+"-bracket-dep", "1")

	tLog(t, "step 5: record deployment creation times for app2")
	creationTimes := map[deploymentKey]string{
		{stagingNamespace, app2 + "-bracket-dep"}: deploymentCreationTime(t, stagingNamespace, app2+"-bracket-dep"),
		{devNamespace, app2 + "-bracket-dep"}:     deploymentCreationTime(t, devNamespace, app2+"-bracket-dep"),
	}
	tLogf(t, "  %s/%s: %s", stagingNamespace, app2+"-bracket-dep", creationTimes[deploymentKey{stagingNamespace, app2 + "-bracket-dep"}])
	tLogf(t, "  %s/%s: %s", devNamespace, app2+"-bracket-dep", creationTimes[deploymentKey{devNamespace, app2 + "-bracket-dep"}])

	tLog(t, "step 6: undeploy app1 entirely")
	undeployApp(t, app1)

	tLogf(t, "step 7: %s buffer for rollout-service to reconcile", reconcileBuffer)
	time.Sleep(reconcileBuffer)

	tLog(t, "step 8: verify bracket1 Argo app is deleted on staging (no apps remain)")
	waitForArgoAppGone(t, "staging-"+bracket1)

	tLog(t, "step 9: assert pod start times stable after undeploy of app1")
	assertDeploymentCreationTimesStable(t, creationTimes, "undeploy app1 should not bother app2")
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
	helmUpgrade(t, helmUpgradeParams{bracketsEnabled: false, developmentEnabled: false, stagingEnabled: false, channelSize: 1})

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
	if out, err := exec.Command("kubectl", "get", "applications", "-n", argoNamespace, "--no-headers").CombinedOutput(); err == nil {
		tLogf(t, "ArgoCD apps after releaseTrain:\n%s", out)
	}
	if out, err := exec.Command("kubectl", "get", "deployments", "-n", stagingNamespace, "--no-headers").CombinedOutput(); err == nil {
		tLogf(t, "Deployments in staging after releaseTrain:\n%s", out)
	}
	if out, err := exec.Command("kubectl", "logs", "-n", argoNamespace, "deployment/kuberpult-rollout-service", "--tail=40").CombinedOutput(); err == nil {
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
	helmUpgrade(t, helmUpgradeParams{bracketsEnabled: true, developmentEnabled: true, stagingEnabled: true, channelSize: 1})

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
	helmUpgrade(t, helmUpgradeParams{bracketsEnabled: true, developmentEnabled: true, stagingEnabled: false, channelSize: 50})

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
	helmUpgrade(t, helmUpgradeParams{bracketsEnabled: true, developmentEnabled: false, stagingEnabled: true, channelSize: 50})

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
	cleanupCluster(t)
	tLogf(t, "runSuffix: %s", runSuffix)
	app := "bmab-app1-" + runSuffix
	bracket1 := "bmab-bracket1-" + runSuffix
	bracket2 := "bmab-bracket2-" + runSuffix

	tLog(t, "step 1: upgrade to staging=true (bracket mode)")
	helmUpgrade(t, helmUpgradeParams{bracketsEnabled: true, developmentEnabled: false, stagingEnabled: true, channelSize: 50})

	tLog(t, "step 2: create v1 release for app in bracket1")
	createRelease(t, app, "sreteam", bracket1, "1", map[string]string{
		devNamespace:     stableManifest(app, devNamespace, "1"),
		stagingNamespace: stableManifest(app, stagingNamespace, "1"),
	})

	tLog(t, "step 3: run staging release train (deploys v1 from development2 to staging)")
	releaseTrain(t, stagingNamespace)

	tLog(t, "step 4: wait for v1 synced in staging and bracket1 Argo app present")
	waitForDeploymentAnnotation(t, stagingNamespace, app+"-bracket-dep", "1")
	waitForArgoApp(t, "staging-"+bracket1)

	tLog(t, "step 5: record initial deployment creation time")
	creationTimes := map[deploymentKey]string{
		{stagingNamespace, app + "-bracket-dep"}: deploymentCreationTime(t, stagingNamespace, app+"-bracket-dep"),
	}
	tLogf(t, "  initial: %s", creationTimes[deploymentKey{stagingNamespace, app + "-bracket-dep"}])

	// ---------- Move 1: bracket1 → bracket2 ----------

	tLog(t, "step 6: create v2 release for app now in bracket2")
	createRelease(t, app, "sreteam", bracket2, "2", map[string]string{
		devNamespace:     stableManifest(app, devNamespace, "2"),
		stagingNamespace: stableManifest(app, stagingNamespace, "2"),
	})

	tLog(t, "step 7: run staging release train (promotes v2 in bracket2)")
	releaseTrain(t, stagingNamespace)

	tLog(t, "step 8: wait for bracket2 Argo app to appear and v2 synced")
	waitForArgoApp(t, "staging-"+bracket2)
	waitForDeploymentAnnotation(t, stagingNamespace, app+"-bracket-dep", "2")

	tLog(t, "step 9: assert pod start time stable after move 1 (b1→b2)")
	assertDeploymentCreationTimesStable(t, creationTimes, "move 1 (b1→b2)")

	// ---------- Move 2: bracket2 → bracket1 ----------

	tLog(t, "step 10: create v3 release for app back in bracket1")
	createRelease(t, app, "sreteam", bracket1, "3", map[string]string{
		devNamespace:     stableManifest(app, devNamespace, "3"),
		stagingNamespace: stableManifest(app, stagingNamespace, "3"),
	})

	tLog(t, "step 11: run staging release train (promotes v3 back in bracket1)")
	releaseTrain(t, stagingNamespace)

	tLog(t, "step 12: wait for bracket1 Argo app to reappear and v3 synced")
	waitForArgoApp(t, "staging-"+bracket1)
	waitForDeploymentAnnotation(t, stagingNamespace, app+"-bracket-dep", "3")

	tLog(t, "step 13: assert pod start time stable after move 2 (b2→b1)")
	assertDeploymentCreationTimesStable(t, creationTimes, "move 2 (b2→b1)")
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
	helmUpgrade(t, helmUpgradeParams{bracketsEnabled: true, developmentEnabled: false, stagingEnabled: false, channelSize: 1})

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
	helmUpgrade(t, helmUpgradeParams{bracketsEnabled: true, developmentEnabled: true, stagingEnabled: false, channelSize: 1})

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
		if out, err := exec.Command("kubectl", "describe", "application", app, "-n", argoNamespace).CombinedOutput(); err == nil {
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
		if out, err := exec.Command("kubectl", "logs", "-n", argoNamespace, dep, "--tail=100").CombinedOutput(); err == nil {
			tLogf(t, "%s logs (tail 100):\n%s", svc, out)
		}
	}
	if out, err := exec.Command("kubectl", "logs", "-n", argoNamespace, "statefulset/argocd-application-controller", "--tail=100").CombinedOutput(); err == nil {
		tLogf(t, "argocd-application-controller logs (tail 100):\n%s", out)
	}
}

// dumpBracketDiagnosticsExpanded is like dumpBracketDiagnostics but also describes
// individual (non-bracket) ArgoCD apps and uses --since=5m for logs so the full
// bracket-migration window is captured regardless of log volume.
func dumpBracketDiagnosticsExpanded(t *testing.T, bracketApps []string, individualApps []string, namespaces []string) {
	t.Helper()
	for _, app := range bracketApps {
		if out, err := exec.Command("kubectl", "describe", "application", app, "-n", argoNamespace).CombinedOutput(); err == nil {
			tLogf(t, "ArgoCD app describe %s:\n%s", app, out)
		}
	}
	for _, app := range individualApps {
		if out, err := exec.Command("kubectl", "describe", "application", app, "-n", argoNamespace).CombinedOutput(); err == nil {
			tLogf(t, "ArgoCD app describe (individual) %s:\n%s", app, out)
		}
	}
	for _, ns := range namespaces {
		if out, err := exec.Command("kubectl", "get", "events", "-n", ns, "--sort-by=.lastTimestamp").CombinedOutput(); err == nil {
			tLogf(t, "events in %s:\n%s", ns, out)
		}
	}
	if out, err := exec.Command("kubectl", "logs", "-n", argoNamespace, "deployment/kuberpult-rollout-service", "--since=5m").CombinedOutput(); err == nil {
		tLogf(t, "rollout-service logs (since 5m):\n%s", out)
	}
	if out, err := exec.Command("kubectl", "logs", "-n", argoNamespace, "statefulset/argocd-application-controller", "--since=5m").CombinedOutput(); err == nil {
		tLogf(t, "argocd-application-controller logs (since 5m):\n%s", out)
	}
	if out, err := exec.Command("kubectl", "logs", "-n", argoNamespace, "deployment/argocd-server", "--since=5m").CombinedOutput(); err == nil {
		tLogf(t, "argocd-server logs (since 5m):\n%s", out)
	}
	if out, err := exec.Command("kubectl", "logs", "-n", argoNamespace, "deployment/kuberpult-reposerver-service", "--since=5m").CombinedOutput(); err == nil {
		tLogf(t, "kuberpult-reposerver-service logs (since 5m):\n%s", out)
	}
	if out, err := exec.Command("kubectl", "logs", "-n", argoNamespace, "deployment/kuberpult-rollout-service", "--since=5m", "--previous").CombinedOutput(); err == nil {
		tLogf(t, "rollout-service logs (previous container, since 5m):\n%s", out)
	}
	if out, err := exec.Command("kubectl", "get", "appprojects", "-n", argoNamespace, "-o", "yaml").CombinedOutput(); err == nil {
		tLogf(t, "AppProjects in %s:\n%s", argoNamespace, out)
	}
}
