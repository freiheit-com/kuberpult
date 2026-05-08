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
	"os/exec"
	"strings"
	"testing"
	"time"

	auth "github.com/freiheit-com/kuberpult/pkg/auth"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const cdServiceGrpcAddr = "localhost:8083"

// helmUpgrade calls helm upgrade with the bracket staging cluster override and waits
// for the rollout-service deployment to finish rolling out.
// It calls helm directly (not install-kuberpult-helm.sh) to avoid port-forward side
// effects and dependency rebuild overhead.
func helmUpgrade(t *testing.T, stagingEnabled bool) {
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

	stagingVal := "false"
	if stagingEnabled {
		stagingVal = "true"
	}
	t.Logf("helmUpgrade: staging=%s, chart=%s", stagingVal, chartPath)

	cmd := exec.Command("helm", "upgrade", "--install",
		"--values", valsPath,
		"--set", "rollout.experimentalBrackets.clusters.staging="+stagingVal,
		"kuberpult-local", chartPath)
	if out2, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helm upgrade (staging=%s): %v\n%s", stagingVal, err, out2)
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
		t.Logf("%s rolled out: %s", dep, strings.TrimSpace(string(out3)))
	}
}

// waitForArgoApp polls until the named Argo Application exists in the default namespace.
func waitForArgoApp(t *testing.T, name string) {
	t.Helper()
	deadline := time.Now().Add(argoAppWaitTimeout)
	for time.Now().Before(deadline) {
		out, err := exec.Command("kubectl", "get", "application", name, "-n", "default").Output()
		if err == nil && strings.Contains(string(out), name) {
			t.Logf("  Argo app present: %s", name)
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
		_, err := exec.Command("kubectl", "get", "application", name, "-n", "default").Output()
		if err != nil {
			t.Logf("  Argo app gone: %s", name)
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
	t.Logf("runSuffix: %s", runSuffix)
	app1 := "bmt-app1-" + runSuffix
	app2 := "bmt-app2-" + runSuffix
	apps := []string{app1, app2}
	migrationBracket := "bmt-bracket-" + runSuffix

	// Argo app name convention (from argo.go CreateArgoApplication):
	//   individual: "{env}-{appName}"   e.g. "staging-bmt-app1-XXXXX"
	//   bracket:    "{env}-{bracketName}" e.g. "staging-bmt-bracket-XXXXX"

	t.Log("step 1: upgrade to staging=false (individual Argo apps mode for staging)")
	helmUpgrade(t, false)

	t.Log("step 2: create v1 releases (dev + dev2 + staging manifests)")
	for _, app := range apps {
		createRelease(t, app, "sreteam", migrationBracket, "1", map[string]string{
			devNamespace:     stableManifest(app, devNamespace, "1"),
			dev2Namespace:    stableManifest(app, dev2Namespace, "1"),
			stagingNamespace: stableManifest(app, stagingNamespace, "1"),
		})
	}

	t.Log("step 3: run staging release train (deploys v1 from development2 to staging)")
	releaseTrain(t, stagingNamespace)

	t.Log("step 4: wait for v1 synced in staging (ArgoCD applied the deployment)")
	for _, app := range apps {
		waitForDeploymentAnnotation(t, stagingNamespace, app+"-bracket-dep", "1")
	}

	t.Log("step 5: verify individual Argo apps exist on staging")
	for _, app := range apps {
		waitForArgoApp(t, "staging-"+app)
	}

	t.Log("step 6: record pod start times")
	type podKey struct{ ns, app string }
	startTimes := map[podKey]string{}
	for _, app := range apps {
		for _, ns := range []string{devNamespace, stagingNamespace} {
			k := podKey{ns, app}
			startTimes[k] = podStartTime(t, ns, app)
			t.Logf("  %s/%s: %s", ns, app, startTimes[k])
		}
	}

	t.Log("step 7: upgrade to staging=true (migrate to bracket Argo app)")
	helmUpgrade(t, true)

	t.Log("step 8: wait for bracket Argo app to appear on staging")
	waitForArgoApp(t, "staging-"+migrationBracket)

	t.Log("step 9: wait for individual Argo apps to be deleted from staging")
	for _, app := range apps {
		waitForArgoAppGone(t, "staging-"+app)
	}

	t.Log("step 10: 20s buffer to let any accidental pod churn complete")
	time.Sleep(podChurnBuffer)

	t.Log("step 11: verify pod start times have not changed (all environments)")
	for _, app := range apps {
		for _, ns := range []string{devNamespace, stagingNamespace} {
			k := podKey{ns, app}
			got := podStartTime(t, ns, app)
			if got != startTimes[k] {
				t.Errorf("REGRESSION: pod %s/%s was restarted during bracket migration\n  before: %s\n  after:  %s",
					ns, app, startTimes[k], got)
			} else {
				t.Logf("  OK %s/%s start time stable at %s", ns, app, got)
			}
		}
	}
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
		conn.Close()
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

// TestBracketDeleteEnvFromApp verifies that removing the last app from a bracket
// (via deleteEnvFromApp) does not cause the bracket Argo Application to be deleted.
// The bracket should persist as long as the brackets_history table has an entry.
func TestBracketDeleteEnvFromApp(t *testing.T) {
	t.Logf("runSuffix: %s", runSuffix)
	app := "bde-app1-" + runSuffix
	bracket := "bde-bracket-" + runSuffix

	t.Log("step 1: upgrade to staging=true (bracket mode)")
	helmUpgrade(t, true)

	t.Log("step 2: create v1 release (dev + dev2 + staging manifests)")
	createRelease(t, app, "sreteam", bracket, "1", map[string]string{
		devNamespace:     stableManifest(app, devNamespace, "1"),
		dev2Namespace:    stableManifest(app, dev2Namespace, "1"),
		stagingNamespace: stableManifest(app, stagingNamespace, "1"),
	})

	t.Log("step 3: run staging release train")
	releaseTrain(t, stagingNamespace)

	t.Log("step 4: wait for v1 synced in staging")
	waitForDeploymentAnnotation(t, stagingNamespace, app+"-bracket-dep", "1")

	t.Log("step 5: wait for bracket Argo app to appear on staging")
	waitForArgoApp(t, "staging-"+bracket)

	t.Log("step 6: delete env from app")
	deleteEnvFromApp(t, stagingNamespace, app)

	t.Log("step 7: 30s buffer for rollout-service to reconcile")
	time.Sleep(reconcileBuffer)

	t.Log("step 8: verify bracket Argo app still exists (must not be deleted)")
	waitForArgoApp(t, "staging-"+bracket)
}

func undeployApp(t *testing.T, app string) {
	t.Helper()
	processBatch(t, &api.BatchRequest{Actions: []*api.BatchAction{
		{Action: &api.BatchAction_Undeploy{
			Undeploy: &api.UndeployRequest{Application: app},
		}},
	}})
}

// TestBracketUndeploy verifies that undeploying the only app in bracket1 does not:
// (a) delete the bracket1 Argo Application, and
// (b) restart pods belonging to a separate bracket2.
func TestBracketUndeploy(t *testing.T) {
	t.Logf("runSuffix: %s", runSuffix)
	app1 := "bu-app1-" + runSuffix
	bracket1 := "bu-bracket1-" + runSuffix
	app2 := "bu-app2-" + runSuffix
	bracket2 := "bu-bracket2-" + runSuffix

	t.Log("step 1: upgrade to staging=true (bracket mode)")
	helmUpgrade(t, true)

	t.Log("step 2: create v1 releases for both apps")
	createRelease(t, app1, "sreteam", bracket1, "1", map[string]string{
		devNamespace:     stableManifest(app1, devNamespace, "1"),
		dev2Namespace:    stableManifest(app1, dev2Namespace, "1"),
		stagingNamespace: stableManifest(app1, stagingNamespace, "1"),
	})
	createRelease(t, app2, "sreteam", bracket2, "1", map[string]string{
		devNamespace:     stableManifest(app2, devNamespace, "1"),
		dev2Namespace:    stableManifest(app2, dev2Namespace, "1"),
		stagingNamespace: stableManifest(app2, stagingNamespace, "1"),
	})

	t.Log("step 3: run staging release train (deploys both apps)")
	releaseTrain(t, stagingNamespace)

	t.Log("step 4: wait for both apps synced in staging")
	waitForDeploymentAnnotation(t, stagingNamespace, app1+"-bracket-dep", "1")
	waitForDeploymentAnnotation(t, stagingNamespace, app2+"-bracket-dep", "1")

	t.Log("step 5: wait for both bracket Argo apps to appear on staging")
	waitForArgoApp(t, "staging-"+bracket1)
	waitForArgoApp(t, "staging-"+bracket2)

	t.Log("step 6: record app2 pod start time")
	app2StartTime := podStartTime(t, stagingNamespace, app2)
	t.Logf("  %s/%s: %s", stagingNamespace, app2, app2StartTime)

	t.Log("step 7: undeploy app1 entirely")
	undeployApp(t, app1)

	t.Log("step 8: 30s buffer for rollout-service to reconcile")
	time.Sleep(reconcileBuffer)

	t.Log("step 9: verify bracket1 Argo app still exists (must not be deleted)")
	waitForArgoApp(t, "staging-"+bracket1)

	t.Log("step 10: verify app2 pod start time has not changed (no accidental restart)")
	got := podStartTime(t, stagingNamespace, app2)
	if got != app2StartTime {
		t.Errorf("REGRESSION: pod %s/%s was restarted during undeploy of %s\n  before: %s\n  after:  %s",
			stagingNamespace, app2, app1, app2StartTime, got)
	} else {
		t.Logf("  OK %s/%s start time stable at %s", stagingNamespace, app2, got)
	}
}
