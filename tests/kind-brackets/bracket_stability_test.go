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

// Package kindbracketstest verifies bracket Argo app stability against the KinD
// cluster started by charts/kuberpult/run-kind.sh.
//
// Run after the cluster is up:
//
//	go test -v ./tests/kind-brackets/ -timeout 10m
package kindbracketstest

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

// runSuffix is a short unique string appended to app names so repeated test
// runs do not collide with existing Kuberpult releases.
var runSuffix = fmt.Sprintf("%d", time.Now().Unix()%100000)

const (
	kuberpultFrontendPort = "5002"
	devNamespace          = "development"
	stagingNamespace      = "staging"
	// devTwoNamespace is the upstream env of the active-active aa-test env;
	// aaNamespace is the active-active env itself (both pre-created by run-kind.sh).
	devTwoNamespace = "development2"
	aaNamespace     = "aa-test"
	// Bracket name used for the two test apps.
	testBracket = "bracket-stability-test"

	// numTestApps is the number of apps used in load-test scenarios.
	// Increase this to stress-test bracket operations at scale.
	// 2 is "the default"
	numTestApps = 2

	// Polling intervals and deadlines.
	argoAppWaitTimeout   = 2 * time.Minute
	argoAppGoneTimeout   = 3 * time.Minute
	argoAppAbsentTimeout = 45 * time.Second
	argoAppPollInterval  = 5 * time.Second
	podPollInterval      = 3 * time.Second
	reconcileBuffer      = 10 * time.Second
	grpcRetryTimeout     = 30 * time.Second
	grpcRetryInterval    = 2 * time.Second
)

// deploymentKey identifies a Deployment by namespace and name.
type deploymentKey struct{ namespace, name string }

// waitForFrontendHTTPReady polls the frontend /health endpoint until it returns
// a valid HTTP response, covering the race where the port-forward reconnects
// after pods are replaced. Mirrors the processBatch retry logic.
func waitForFrontendHTTPReady(t *testing.T) {
	t.Helper()
	WaitForFrontendReady(t)
}

// stableManifest returns a Deployment + ConfigMap for app/namespace/version.
//
// The release number is written to the Deployment's metadata.annotations so
// that ArgoCD detects a diff and syncs on every new version — exercising the
// full bracket code path.  Crucially, the pod template spec is IDENTICAL across
// all versions, so Kubernetes will NOT restart pods due to a rolling update.
//
// This means: if a pod's startTime changes between versions, its Deployment was
// deleted (i.e. the bracket Argo app was cascade-deleted unexpectedly).
func stableManifest(app, namespace, version string) string {
	return StableManifest(app, namespace, version)
}

func createRelease(t *testing.T, app, team, bracketName, version string, manifests map[string]string) {
	t.Helper()
	CreateRelease(t, app, team, bracketName, version, manifests)
}

// dumpKuberpultLogs prints the last 300 log lines (current + previous container)
// of every pod belonging to the frontend and cd services to stderr.
// Uses per-pod kubectl calls so all replicas are covered (kubectl logs deployment/X
// silently picks just one pod when multiple exist).
func dumpKuberpultLogs(t *testing.T) {
	t.Helper()
	fmt.Fprintln(os.Stderr, "=== CRITICAL: dumping kuberpult service logs ===")
	podsOut, err := exec.Command("kubectl", "get", "pods", "-n", globalCfg.KuberpultNamespace, "-o", "name").CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "kubectl get pods: %v\n", err)
	}
	for _, dep := range []string{"kuberpult-frontend-service", "kuberpult-cd-service", "kuberpult-rollout-service", "kuberpult-reposerver-service"} {
		for _, line := range strings.Fields(string(podsOut)) {
			podName := strings.TrimPrefix(line, "pod/")
			if !strings.HasPrefix(podName, dep+"-") {
				continue
			}
			for _, prev := range []bool{false, true} {
				args := []string{"logs", "-n", globalCfg.KuberpultNamespace, podName, "--tail=300"}
				label := podName
				if prev {
					args = append(args, "--previous")
					label = podName + " (previous)"
				}
				out, err := exec.Command("kubectl", args...).CombinedOutput()
				if err != nil {
					fmt.Fprintf(os.Stderr, "kubectl logs %s: %v\n", label, err)
				} else {
					fmt.Fprintf(os.Stderr, "=== logs: %s ===\n%s\n", label, out)
				}
			}
		}
	}
}

// fatalWithServiceLogs dumps service logs then exits the entire test binary so
// no further tests run and the cluster stays in its failed state for inspection.
func fatalWithServiceLogs(t *testing.T, format string, args ...any) {
	t.Helper()
	dumpKuberpultLogs(t)
	fmt.Fprintf(os.Stderr, "FATAL: "+format+"\n", args...)
	os.Exit(1)
}

func releaseTrain(t *testing.T, env string) {
	t.Helper()
	ReleaseTrain(t, env)
}

// deploymentCreationTime returns the creationTimestamp of the named Deployment.
// Fails the test if the Deployment does not exist.
func deploymentCreationTime(t *testing.T, namespace, name string) string {
	t.Helper()
	out, err := exec.Command(
		"kubectl", "get", "deployment", name,
		"-n", namespace,
		"-o", "jsonpath={.metadata.creationTimestamp}",
	).Output()
	if err != nil {
		t.Fatalf("get deployment %s/%s creationTimestamp: %v", namespace, name, err)
	}
	return strings.TrimSpace(string(out))
}

// assertDeploymentCreationTimesStable checks that no Deployment was deleted+recreated
// (which would cause downtime) by verifying that creationTimestamps are unchanged.
func assertDeploymentCreationTimesStable(t *testing.T, times map[deploymentKey]string, context string) {
	t.Helper()
	for k, want := range times {
		got := deploymentCreationTime(t, k.namespace, k.name)
		if got != want {
			t.Fatalf("REGRESSION (%s): deployment %s/%s was deleted and recreated\n  before: %s\n  after:  %s",
				context, k.namespace, k.name, want, got)
		}
	}
	for k, want := range times {
		tLogf(t, "  OK %s/%s creation time stable at %s", k.namespace, k.name, want)
	}
}

// waitForDeploymentAnnotation polls until the named Deployment's annotation
// kuberpult.freiheit.com/release-version matches wantVersion, or the 3-minute
// deadline is exceeded.  This confirms ArgoCD has synced the release.
func waitForDeploymentAnnotation(t *testing.T, namespace, deploymentName, wantVersion string) {
	t.Helper()
	deadline := time.Now().Add(argoAppGoneTimeout)
	for time.Now().Before(deadline) {
		out, _ := exec.Command(
			"kubectl", "get", "deployment", deploymentName,
			"-n", namespace,
			"-o", `jsonpath={.metadata.annotations.kuberpult\.freiheit\.com/release-version}`,
		).Output()
		if strings.TrimSpace(string(out)) == wantVersion {
			return
		}
		time.Sleep(podPollInterval)
	}
	if out2, _ := exec.Command("kubectl", "describe", "deployment", deploymentName, "-n", namespace).CombinedOutput(); len(out2) > 0 {
		tLogf(t, "deployment describe:\n%s", out2)
	}
	if out3, _ := exec.Command("kubectl", "get", "pods", "-n", namespace, "-o", "wide").CombinedOutput(); len(out3) > 0 {
		tLogf(t, "pods in %s:\n%s", namespace, out3)
	}
	t.Fatalf("deployment %s/%s annotation release-version never reached %q after 3 minutes",
		namespace, deploymentName, wantVersion)
}

// cleanupCluster deletes all ArgoCD applications and all pods/deployments in
// the three test namespaces, then waits for each deletion to complete.
// Call this at the start of every test to ensure a clean cluster state.
func removeAllArgoAppFinalizers(t *testing.T) {
	t.Helper()
	names, _ := exec.Command("kubectl", "get", "applications", "-n", globalCfg.KuberpultNamespace, "-o", "name").CombinedOutput()
	for _, name := range strings.Fields(string(names)) {
		err := exec.Command("kubectl", "patch", "-n", globalCfg.KuberpultNamespace, name,
			"--type=merge", "-p", `{"metadata":{"finalizers":[]}}`).Run()
		if err != nil {
			tLog(t, "kubectl patch finalizers failed (continuing)[%s]: %v", name, err)
		} else {
			tLog(t, "deleted finalizer for application %s", name)
		}
	}
}

func cleanupCluster(t *testing.T) {
	t.Helper()
	tLog(t, "cleanupCluster: deleting all ArgoCD applications")
	removeAllArgoAppFinalizers(t)
	out, err := exec.Command("kubectl", "delete", "applications", "--all", "-n", globalCfg.KuberpultNamespace, "--wait=true").CombinedOutput()
	if err != nil {
		t.Fatalf("kubectl delete applications failed: %v: %s", err, out)
	}
	tLogf(t, "  %s", strings.TrimSpace(string(out)))
	for _, ns := range []string{devNamespace, stagingNamespace} {
		out2, err := exec.Command("kubectl", "delete", "deployments,pods", "--all", "-n", ns, "--wait=true").CombinedOutput()
		if err != nil {
			t.Fatalf("kubectl delete deployment/etc failed: %v: %s", err, out2)
		}
		tLogf(t, "  %s: %s", ns, strings.TrimSpace(string(out2)))
	}

	resetDB(t)
}

func resetDB(t *testing.T) {
	t.Helper()

	stopDBPF := globalCfg.startDBPortForward(t)
	defer stopDBPF()

	// Step 1: query all table names in the public schema
	tLog(t, "resetDB: querying tables")
	out, err := globalCfg.runPsql("-At", "-c", "SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename;")
	if err != nil {
		t.Fatalf("resetDB: list tables: %v: %s", err, out)
	}
	var tables []string
	for _, line := range strings.Fields(string(out)) {
		if line != "" {
			tables = append(tables, line)
		}
	}

	// Step 2: log which tables will be dropped
	if len(tables) == 0 {
		tLog(t, "resetDB: no tables found, db is already empty")
	} else {
		tLogf(t, "resetDB: will drop tables: %s", strings.Join(tables, ", "))

		// Step 3: drop tables in batches of 10 to reduce round-trips (~1 sec per call).
		// Dropping (instead of truncating) includes schema_migrations, so each
		// deployed kuberpult version re-creates the schema from its own migration set.
		for batch := range slices.Chunk(tables, 10) {
			var parts []string
			for _, table := range batch {
				parts = append(parts, fmt.Sprintf(`"%s"`, table))
			}
			tLogf(t, "resetDB: dropping tables %s", strings.Join(parts, ", "))
			out2, err2 := globalCfg.runPsql("-c", fmt.Sprintf(`DROP TABLE IF EXISTS %s CASCADE;`, strings.Join(parts, ", ")))
			if err2 != nil {
				t.Fatalf("resetDB: drop %s: %v: %s", strings.Join(parts, ", "), err2, out2)
			}
		}
	}

	// Scale kuberpult services to 0 to clear in-memory state and db connection.
	// Scale-up is handled by helmUpgrade in each test.
	tLog(t, "resetDB: scaling kuberpult services to 0")
	kuberpultDeployments := []string{
		"deployment/kuberpult-cd-service",
		"deployment/kuberpult-frontend-service",
		"deployment/kuberpult-reposerver-service",
		"deployment/kuberpult-rollout-service",
	}
	for _, dep := range kuberpultDeployments {
		if out3, err3 := exec.Command("kubectl", "scale", dep, "-n", globalCfg.KuberpultNamespace, "--replicas=0").CombinedOutput(); err3 != nil {
			t.Fatalf("resetDB: scale %s: %v\n%s", dep, err3, out3)
		}
	}
}

// TestBracketPodStability is the regression test for the seenVersions cascade-
// deletion bug.
//
// Scenario:
//   - Two apps share a bracket, deployed to development (bracket-cluster) and
//     staging (bracket-cluster).
//   - After both envs are on version 1, a new release v2 is created.
//   - development (upstream=latest) auto-deploys v2; staging stays at v1.
//   - The rollout-service now sees: development changed ("1:1" → "2:2"),
//     staging unchanged (seenVersions skips it).
//   - Bug: staging's bracket Argo app is cascade-deleted → pods disappear.
//   - Fix: backfill staging in the Argo push → pods survive.
//
// Detection: because the Deployment pod-template spec is identical across
// versions, Kubernetes does NOT roll pods on a version update.  Any change in
// pod startTime means the Deployment was deleted — the bug.
func TestBracketPodStability(t *testing.T) {
	cleanupCluster(t)
	helmUpgrade(t, HelmUpgradeParams{BracketsEnabled: true, DevelopmentEnabled: false, StagingEnabled: true, ChannelSize: 50})
	tLogf(t, "runSuffix: %s", runSuffix)
	app1 := "bst-app1-" + runSuffix
	app2 := "bst-app2-" + runSuffix
	apps := []string{app1, app2}

	tLog(t, "step 1: create initial releases (version 1)")
	for _, app := range apps {
		manifests := map[string]string{
			devNamespace:     stableManifest(app, devNamespace, "1"),
			stagingNamespace: stableManifest(app, stagingNamespace, "1"),
		}
		createRelease(t, app, "sreteam", testBracket, "1", manifests)
	}

	// development has upstream=latest → auto-deploy v1.
	// staging needs an explicit release train that reads from development.
	tLog(t, "step 2: run release train for staging (deploys v1 from development)")
	releaseTrain(t, stagingNamespace)

	tLog(t, "step 3: wait for v1 to be synced in development and staging")
	for _, app := range apps {
		for _, ns := range []string{devNamespace, stagingNamespace} {
			waitForDeploymentAnnotation(t, ns, app+"-bracket-dep", "1")
		}
	}

	tLog(t, "step 4: record deployment creation times")
	creationTimes := map[deploymentKey]string{}
	for _, app := range apps {
		for _, ns := range []string{devNamespace, stagingNamespace} {
			k := deploymentKey{ns, app + "-bracket-dep"}
			creationTimes[k] = deploymentCreationTime(t, ns, app+"-bracket-dep")
			tLogf(t, "  %s/%s: %s", ns, app+"-bracket-dep", creationTimes[k])
		}
	}

	tLog(t, "step 5: create version 2 (annotation changes, pod spec identical)")
	for _, app := range apps {
		createRelease(t, app, "sreteam", testBracket, "2", map[string]string{
			devNamespace:     stableManifest(app, devNamespace, "2"),
			stagingNamespace: stableManifest(app, stagingNamespace, "2"),
		})
	}

	// Intentionally do NOT run the staging release train: staging stays at v1.
	// development auto-deploys v2 (upstream=latest).
	// This is the seenVersions scenario: development changes, staging is stable.
	tLog(t, "step 6: wait for development to sync v2 (staging intentionally stays at v1)")
	for _, app := range apps {
		waitForDeploymentAnnotation(t, devNamespace, app+"-bracket-dep", "2")
	}

	tLog(t, "step 7: assert staging pod start times stable")
	assertDeploymentCreationTimesStable(t, creationTimes, "pod stability")
}
