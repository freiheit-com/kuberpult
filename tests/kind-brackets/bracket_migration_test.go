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
	"os/exec"
	"strings"
	"testing"
	"time"
)

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

	out3, err := exec.Command("kubectl", "rollout", "status",
		"deployment/kuberpult-rollout-service", "--timeout=3m").CombinedOutput()
	if err != nil {
		t.Fatalf("kubectl rollout status: %v\n%s", err, out3)
	}
	t.Logf("rollout-service rolled out: %s", strings.TrimSpace(string(out3)))
}

// waitForArgoApp polls until the named Argo Application exists in the default namespace.
func waitForArgoApp(t *testing.T, name string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		out, err := exec.Command("kubectl", "get", "application", name, "-n", "default").Output()
		if err == nil && strings.Contains(string(out), name) {
			t.Logf("  Argo app present: %s", name)
			return
		}
		time.Sleep(5 * time.Second)
	}
	t.Fatalf("Argo app %q never appeared after 2 minutes", name)
}

// waitForArgoAppGone polls until the named Argo Application no longer exists.
func waitForArgoAppGone(t *testing.T, name string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		_, err := exec.Command("kubectl", "get", "application", name, "-n", "default").Output()
		if err != nil {
			t.Logf("  Argo app gone: %s", name)
			return
		}
		time.Sleep(5 * time.Second)
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
	time.Sleep(20 * time.Second)

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
