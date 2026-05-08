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
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// runSuffix is a short unique string appended to app names so repeated test
// runs do not collide with existing Kuberpult releases.
var runSuffix = fmt.Sprintf("%d", time.Now().Unix()%100000)

const (
	kuberpultFrontendPort = "8081"
	devNamespace          = "development"
	dev2Namespace         = "development2"
	stagingNamespace      = "staging"
	// Bracket name used for the two test apps.
	testBracket = "bracket-stability-test"

	// Polling intervals and deadlines.
	argoAppWaitTimeout  = 2 * time.Minute
	argoAppGoneTimeout  = 3 * time.Minute
	argoAppPollInterval = 5 * time.Second
	podPollInterval     = 3 * time.Second
	podChurnBuffer      = 20 * time.Second
	reconcileBuffer     = 30 * time.Second
	grpcRetryTimeout    = 30 * time.Second
	grpcRetryInterval   = 2 * time.Second
)

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
	return fmt.Sprintf(`---
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s-bracket-cfg
  namespace: %s
data:
  version: "%s"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s-bracket-dep
  namespace: %s
  annotations:
    kuberpult.freiheit.com/release-version: "%s"
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %s-bracket
  template:
    metadata:
      labels:
        app: %s-bracket
    spec:
      containers:
      - name: sleep
        image: alpine:latest
        command: ["/bin/sh", "-c", "trap 'exit 0' SIGTERM; while true; do sleep 1000; done"]
        readinessProbe:
          exec:
            command: ["ls", "/"]
          initialDelaySeconds: 3
          periodSeconds: 5
        resources:
          limits:
            cpu: 100m
            memory: 32Mi
`, app, namespace, version, app, namespace, version, app, app)
}

func createRelease(t *testing.T, app, team, bracketName, version string, manifests map[string]string) {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	mustWriteField(t, w, "application", app)
	mustWriteField(t, w, "version", version)
	mustWriteField(t, w, "team", team)
	if bracketName != "" {
		mustWriteField(t, w, "experimentalArgoBracket", bracketName)
	}
	for env, manifest := range manifests {
		fw, err := w.CreateFormFile("manifests["+env+"]", "manifests["+env+"]")
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := io.WriteString(fw, manifest); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req, err := http.NewRequest("POST", "http://localhost:"+kuberpultFrontendPort+"/api/release", &b)
	if err != nil {
		t.Fatalf("build release request: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/release for %s v%s: %v", app, version, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode > 299 {
		t.Fatalf("POST /api/release for %s v%s: HTTP %d: %s", app, version, resp.StatusCode, body)
	}
}

func mustWriteField(t *testing.T, w *multipart.Writer, key, value string) {
	t.Helper()
	if err := w.WriteField(key, value); err != nil {
		t.Fatalf("write field %s: %v", key, err)
	}
}

func releaseTrain(t *testing.T, env string) {
	t.Helper()
	url := "http://localhost:" + kuberpultFrontendPort + "/api/environments/" + env + "/releasetrain"
	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		t.Fatalf("build release-train request for %s: %v", env, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT release train %s: %v", env, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode > 299 {
		t.Fatalf("PUT release train %s: HTTP %d: %s", env, resp.StatusCode, body)
	}
}

// podStartTime returns the RFC3339 startTime of the first Running pod in
// namespace matching label app=<app>-bracket. Retries for up to 2 minutes.
func podStartTime(t *testing.T, namespace, app string) string {
	t.Helper()
	label := "app=" + app + "-bracket"
	deadline := time.Now().Add(argoAppWaitTimeout)
	for time.Now().Before(deadline) {
		out, err := exec.Command(
			"kubectl", "get", "pods",
			"-n", namespace,
			"-l", label,
			"--field-selector=status.phase=Running",
			"-o", "jsonpath={.items[0].status.startTime}",
		).Output()
		s := strings.TrimSpace(string(out))
		if err == nil && s != "" {
			return s
		}
		time.Sleep(podPollInterval)
	}
	t.Fatalf("no Running pod with label %s in namespace %s after 2 minutes", label, namespace)
	return ""
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
	t.Fatalf("deployment %s/%s annotation release-version never reached %q after 3 minutes",
		namespace, deploymentName, wantVersion)
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
	t.Logf("runSuffix: %s", runSuffix)
	app1 := "bst-app1-" + runSuffix
	app2 := "bst-app2-" + runSuffix
	apps := []string{app1, app2}

	// staging's upstream is development2; development2's upstream is "latest".
	// We must provide a development2 manifest so that Kuberpult records a
	// deployment there, enabling the staging release train to pick it up.
	manifestsV := func(version string) map[string]string {
		m := map[string]string{}
		for _, ns := range []string{devNamespace, dev2Namespace, stagingNamespace} {
			m[ns] = stableManifest("", ns, version) // app filled in by caller
		}
		return m
	}

	t.Log("step 1: create initial releases (version 1)")
	for _, app := range apps {
		manifests := map[string]string{
			devNamespace:     stableManifest(app, devNamespace, "1"),
			dev2Namespace:    stableManifest(app, dev2Namespace, "1"),
			stagingNamespace: stableManifest(app, stagingNamespace, "1"),
		}
		createRelease(t, app, "sreteam", testBracket, "1", manifests)
	}
	_ = manifestsV // suppress unused warning from the helper above

	// development and development2 both have upstream=latest → auto-deploy v1.
	// staging needs an explicit release train that reads from development2.
	t.Log("step 2: run release train for staging (deploys v1 from development2)")
	releaseTrain(t, stagingNamespace)

	t.Log("step 3: wait for v1 to be synced in development and staging")
	for _, app := range apps {
		for _, ns := range []string{devNamespace, stagingNamespace} {
			waitForDeploymentAnnotation(t, ns, app+"-bracket-dep", "1")
		}
	}

	t.Log("step 4: record pod start times")
	type podKey struct{ ns, app string }
	startTimes := map[podKey]string{}
	for _, app := range apps {
		for _, ns := range []string{devNamespace, stagingNamespace} {
			k := podKey{ns, app}
			startTimes[k] = podStartTime(t, ns, app)
			t.Logf("  %s/%s: %s", ns, app, startTimes[k])
		}
	}

	t.Log("step 5: create version 2 (annotation changes, pod spec identical)")
	for _, app := range apps {
		createRelease(t, app, "sreteam", testBracket, "2", map[string]string{
			devNamespace:     stableManifest(app, devNamespace, "2"),
			dev2Namespace:    stableManifest(app, dev2Namespace, "2"),
			stagingNamespace: stableManifest(app, stagingNamespace, "2"),
		})
	}

	// Intentionally do NOT run the staging release train: staging stays at v1.
	// development auto-deploys v2 (upstream=latest).
	// This is the seenVersions scenario: development changes, staging is stable.
	t.Log("step 6: wait for development to sync v2 (staging intentionally stays at v1)")
	for _, app := range apps {
		waitForDeploymentAnnotation(t, devNamespace, app+"-bracket-dep", "2")
	}

	// Extra buffer: if a pod was deleted give it time to come back, so a
	// transient absence is reliably caught by the startTime diff.
	t.Log("step 7: 20s buffer to let any accidental pod churn complete")
	time.Sleep(podChurnBuffer)

	t.Log("step 8: verify staging pod start times have not changed")
	for _, app := range apps {
		k := podKey{stagingNamespace, app}
		got := podStartTime(t, stagingNamespace, app)
		if got != startTimes[k] {
			t.Errorf("REGRESSION: pod %s/%s was restarted unexpectedly\n  before: %s\n  after:  %s",
				stagingNamespace, app, startTimes[k], got)
		} else {
			t.Logf("  OK %s/%s start time stable at %s", stagingNamespace, app, got)
		}
	}
}
