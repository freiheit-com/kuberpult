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
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"
)

// TB is the subset of *testing.T used by these helpers, so that the same
// functions can be called from both test code and CLI tools.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
	Logf(format string, args ...any)
}

// TestMode selects the cluster backend.
type TestMode string

const (
	ModeKind TestMode = "kind"
	ModeGKE  TestMode = "gke"
)

// FrontendPort is the local port used for the kuberpult frontend port-forward.
const FrontendPort = "5002"

// GKEContext is the required kubectl context when running in GKE mode.
const GKEContext = "gke_fdc-standard-setup-dev-env_europe-west1-d_tools-migrationtest"

// imageRegistry is the public GCP Artifact Registry that hosts all kuberpult service images.
const imageRegistry = "europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult"

// Config holds all infrastructure-dependent settings needed by the shared ops.
// Test code uses this embedded inside testConfig, which adds DB-access fields.
type Config struct {
	Mode               TestMode
	KuberpultNamespace string // "default" (kind) | "tools" (gke)
	HelmReleaseName    string // "kuberpult-local" (kind) | derived from helm list (gke)
	TestVersion        string // chart version whose images exist in the registry (GKE only)
	GKEProjectNumber   string // kept as string; --reuse-values re-introduces it as a number
}

// HelmUpgradeParams configures bracket settings for a helm upgrade.
type HelmUpgradeParams struct {
	BracketsEnabled    bool
	DevelopmentEnabled bool
	StagingEnabled     bool
	// AATestEnabled toggles bracket mode for the active-active "aa-test" env.
	AATestEnabled bool
	ChannelSize   int
	// OldVersion, if non-empty, installs this specific chart version from GitHub
	// releases instead of the locally built chart. Kind mode only: images for
	// that version are pulled from the public registry and loaded into kind.
	// Example: "v13.47.6"
	// If not set, the current version will be used.
	OldVersion string
}

// MustLoadConfig reads KUBERPULT_TEST_MODE and derives all cluster settings
// that do not require database credentials. Suitable for use in cmd/main.go.
// Uses log.Fatalf because it is typically called outside a test context.
func MustLoadConfig() Config {
	mode := TestMode(os.Getenv("KUBERPULT_TEST_MODE"))
	if mode == "" {
		mode = ModeKind
	}
	if mode != ModeKind && mode != ModeGKE {
		log.Fatalf("KUBERPULT_TEST_MODE must be 'kind' or 'gke', got %q", mode)
	}
	if mode == ModeKind {
		return Config{
			Mode:               ModeKind,
			KuberpultNamespace: "default",
			HelmReleaseName:    "kuberpult-local",
		}
	}

	cfg := Config{Mode: ModeGKE, KuberpultNamespace: "tools"}

	ctxOut, err := exec.Command("kubectl", "config", "current-context").Output()
	if err != nil {
		log.Fatalf("gke mode: kubectl config current-context: %v", err)
	}
	if got := strings.TrimSpace(string(ctxOut)); got != GKEContext {
		log.Fatalf("gke mode: wrong kubectl context %q\nExpected: %q\nRun: kubectl config use-context %s",
			got, GKEContext, GKEContext)
	}

	relOut, err := exec.Command("helm", "list", "-n", "tools", "--short").Output()
	if err != nil {
		log.Fatalf("gke mode: helm list -n tools: %v", err)
	}
	var kuberpultReleases []string
	for _, r := range strings.Fields(string(relOut)) {
		if strings.Contains(r, "kuberpult") {
			kuberpultReleases = append(kuberpultReleases, r)
		}
	}
	if len(kuberpultReleases) != 1 {
		log.Fatalf("gke mode: expected exactly 1 kuberpult helm release in namespace tools, got %d: %v",
			len(kuberpultReleases), kuberpultReleases)
	}
	cfg.HelmReleaseName = kuberpultReleases[0]

	cfg.TestVersion = os.Getenv("KUBERPULT_TEST_VERSION")
	if cfg.TestVersion == "" {
		log.Fatalf("gke mode: KUBERPULT_TEST_VERSION is required (e.g. v13.52.0)")
	}

	cfg.GKEProjectNumber = ReadHelmValue(cfg.HelmReleaseName, "tools", "gke", "project_number")

	return cfg
}

// ReadHelmValue reads a nested key from a helm release's computed values and
// returns it as a string. Helm stores large numbers as JSON numbers; this
// function always returns a string so callers can pass it back via --set-string.
func ReadHelmValue(release, namespace, topKey, subKey string) string {
	out, err := exec.Command("helm", "get", "values", release,
		"-n", namespace, "--all", "-o", "json").Output()
	if err != nil {
		log.Fatalf("gke mode: helm get values %s -n %s: %v", release, namespace, err)
	}
	var vals map[string]any
	if err := json.Unmarshal(out, &vals); err != nil {
		log.Fatalf("gke mode: parse helm values: %v", err)
	}
	top, ok := vals[topKey].(map[string]any)
	if !ok {
		log.Fatalf("gke mode: helm values: key %q not found or not a map", topKey)
	}
	v, ok := top[subKey]
	if !ok {
		log.Fatalf("gke mode: helm values: key %q.%q not found", topKey, subKey)
	}
	return fmt.Sprintf("%v", v)
}

// ReadSecretKey reads a base64-encoded key from a Kubernetes secret.
func ReadSecretKey(secret, namespace, key string) string {
	out, err := exec.Command("kubectl", "get", "secret", secret,
		"-n", namespace, "-o", "jsonpath={.data."+key+"}").Output()
	if err != nil {
		log.Fatalf("gke mode: kubectl get secret %s/%s key %q: %v", namespace, secret, key, err)
	}
	dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
	if err != nil {
		log.Fatalf("gke mode: base64 decode %s/%s: %v", secret, key, err)
	}
	return string(dec)
}

// EnsurePortForwards ensures kubectl port-forwards for the kuberpult frontend
// and cd-service are reachable. If both ports are already up it returns
// immediately; otherwise it kills any stale processes, starts fresh detached
// ones, and waits until both are reachable. Processes are started detached so
// they outlive the command and remain available for subsequent runs in the same
// shell session.
func EnsurePortForwards(tb TB, cfg Config) {
	tb.Helper()
	tb.Logf("EnsurePortForwards: checking 5002 reachability...")
	reach5002 := isFrontendHTTPReachable()
	tb.Logf("EnsurePortForwards: 5002 reachable=%v", reach5002)
	reach5004 := isTCPReachable("127.0.0.1:5004")
	tb.Logf("EnsurePortForwards: 5004 reachable=%v", reach5004)
	// If existing port-forwards are already working, reuse them.
	if reach5002 && reach5004 {
		tb.Logf("EnsurePortForwards: both ports already reachable, skipping restart")
		return
	}
	// Kind mode: portForwardManagers (started in TestMain) handle restart automatically.
	// pkilling them races with their restart loop — just wait for them to reconnect.
	if cfg.Mode == ModeKind {
		tb.Logf("EnsurePortForwards: kind mode — waiting for managers to reconnect")
		WaitForFrontendReady(tb)
		waitForTCPPort(tb, "127.0.0.1:5004", 30*time.Second)
		return
	}
	for _, target := range []string{"kuberpult-frontend-service", "kuberpult-cd-service"} {
		tb.Logf("EnsurePortForwards: pkill port-forward.*%s", target)
		_ = exec.Command("pkill", "-f", "port-forward.*"+target).Run()
	}
	startDetached := func(target, portMapping string) {
		tb.Logf("EnsurePortForwards: starting port-forward for %s %s in namespace %s", target, portMapping, cfg.KuberpultNamespace)
		cmd := exec.Command("kubectl", "port-forward",
			"-n", cfg.KuberpultNamespace,
			"deployment/"+target,
			portMapping)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			tb.Fatalf("start port-forward for %s: %v", target, err)
		}
		pid := cmd.Process.Pid
		tb.Logf("EnsurePortForwards: started pid=%d for %s", pid, target)
		// Watch for immediate exit (e.g. deployment not found) before detaching.
		exited := make(chan error, 1)
		go func() { exited <- cmd.Wait() }()
		select {
		case err := <-exited:
			tb.Fatalf("port-forward for %s (pid %d) exited immediately: %v", target, pid, err)
		case <-time.After(500 * time.Millisecond):
			// Still running — detach so the process outlives this command.
			_ = cmd.Process.Release()
		}
	}
	startDetached("kuberpult-frontend-service", "5002:8081")
	startDetached("kuberpult-cd-service", "5004:8443")
	tb.Logf("EnsurePortForwards: waiting for frontend at :5002 ...")
	WaitForFrontendReady(tb)
	tb.Logf("EnsurePortForwards: waiting for cd-service at :5004 ...")
	waitForTCPPort(tb, "127.0.0.1:5004", 15*time.Second)
	tb.Logf("EnsurePortForwards: done")
}

func isTCPReachable(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// isFrontendHTTPReachable makes a real HTTP request to verify the port-forward
// tunnel is live end-to-end, not just that the kubectl process is still
// listening locally (a stale tunnel passes TCP but hangs on HTTP).
func isFrontendHTTPReachable() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:" + FrontendPort + "/health") //nolint:gosec
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return true
}

func waitForTCPPort(tb TB, addr string, timeout time.Duration) {
	tb.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	tb.Fatalf("port %s not reachable after %s", addr, timeout)
}

// EnsureArgoAppProjects applies the ArgoCD AppProject resources required by the
// kind-bracket tests. In kind mode these are created by run-kind.sh; in GKE mode
// only the "default" project exists, so we apply the missing ones here.
// The apply is idempotent — safe to call on every test run.
func EnsureArgoAppProjects(namespace string) {
	manifest := fmt.Sprintf(`
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: development
  namespace: %s
spec:
  description: test-env
  destinations:
  - name: dest1
    namespace: '*'
    server: https://kubernetes.default.svc
  sourceRepos:
  - '*'
---
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: staging
  namespace: %s
spec:
  description: staging-normal
  destinations:
  - name: dest1
    namespace: '*'
    server: https://kubernetes.default.svc
  sourceRepos:
  - '*'
`, namespace, namespace)

	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("EnsureArgoAppProjects: kubectl apply: %v\n%s", err, out)
	}
}

// kindLoadOldImages pulls service images for the given kuberpult version from the
// public registry and loads them into the kind cluster. Pulls run sequentially
// to preserve clear per-image error output; loads run in parallel.
func kindLoadOldImages(tb TB, version string) {
	tb.Helper()
	services := []string{
		"kuberpult-cd-service",
		"kuberpult-frontend-service",
		"kuberpult-rollout-service",
		"kuberpult-reposerver-service",
	}
	for _, svc := range services {
		img := imageRegistry + "/" + svc + ":" + version
		tb.Logf("kindLoadOldImages: docker pull %s", img)
		if out, err := exec.Command("docker", "pull", img).CombinedOutput(); err != nil {
			tb.Fatalf("docker pull %s: %v\n%s", img, err, out)
		}
	}
	type loadResult struct {
		image string
		out   []byte
		err   error
	}
	results := make([]loadResult, len(services))
	var wg sync.WaitGroup
	for i, svc := range services {
		img := imageRegistry + "/" + svc + ":" + version
		wg.Add(1)
		go func(idx int, image string) {
			defer wg.Done()
			tb.Logf("kindLoadOldImages: kind load docker-image %s", image)
			out, err := exec.Command("kind", "load", "docker-image", image).CombinedOutput()
			results[idx] = loadResult{image, out, err}
		}(i, img)
	}
	wg.Wait()
	for _, r := range results {
		if r.err != nil {
			tb.Fatalf("kind load docker-image %s: %v\n%s", r.image, r.err, r.out)
		}
	}
}

// HelmUpgrade runs helm upgrade with the given bracket parameters, waits for
// all service rollouts, ensures port-forwards are up, waits for the frontend
// to be reachable, and runs the create-environments script.
// Test code should call globalPFM.restart() before this function so the
// port-forward manager reconnects to the newly rolled-out pod.
func HelmUpgrade(tb TB, cfg Config, p HelmUpgradeParams) {
	tb.Helper()
	repoRoot, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		tb.Fatalf("git rev-parse: %v", err)
	}
	root := strings.TrimSpace(string(repoRoot))

	boolStr := func(b bool) string {
		if b {
			return "true"
		}
		return "false"
	}
	tb.Logf("HelmUpgrade: brackets=%s development=%s staging=%s aa-test=%s channelSize=%d",
		boolStr(p.BracketsEnabled), boolStr(p.DevelopmentEnabled), boolStr(p.StagingEnabled), boolStr(p.AATestEnabled), p.ChannelSize)

	var cmd *exec.Cmd
	switch cfg.Mode {
	case ModeKind:
		valsPath := root + "/charts/kuberpult/vals.yaml"
		var chartPath string
		if p.OldVersion != "" {
			kindLoadOldImages(tb, p.OldVersion)
			v := p.OldVersion
			chartPath = "https://github.com/freiheit-com/kuberpult/releases/download/" + v + "/kuberpult-" + v + ".tgz"
			// Old chart predates rollout.experimentalBrackets — skip those flags.
			cmd = exec.Command("helm", "upgrade", "--install",
				"--values", valsPath,
				"--set", fmt.Sprintf("rollout.kuberpultEventsChannelSize=%d", p.ChannelSize),
				cfg.HelmReleaseName, chartPath)
		} else {
			out, err2 := exec.Command("git", "describe", "--always", "--long", "--tags").Output()
			if err2 != nil {
				tb.Fatalf("git describe: %v", err2)
			}
			chartPath = root + "/charts/kuberpult/kuberpult-" + strings.TrimSpace(string(out)) + ".tgz"
			cmd = exec.Command("helm", "upgrade", "--install",
				"--values", valsPath,
				"--set", "rollout.experimentalBrackets.enabled="+boolStr(p.BracketsEnabled),
				"--set", "rollout.experimentalBrackets.clusters.development="+boolStr(p.DevelopmentEnabled),
				"--set", "rollout.experimentalBrackets.clusters.staging="+boolStr(p.StagingEnabled),
				"--set", "rollout.experimentalBrackets.clusters.aa-test="+boolStr(p.AATestEnabled),
				"--set", fmt.Sprintf("rollout.kuberpultEventsChannelSize=%d", p.ChannelSize),
				cfg.HelmReleaseName, chartPath)
		}
	case ModeGKE:
		v := cfg.TestVersion
		chartPath := "https://github.com/freiheit-com/kuberpult/releases/download/" + v + "/kuberpult-" + v + ".tgz"
		cmd = exec.Command("helm", "upgrade",
			"--reuse-values",
			"-n", cfg.KuberpultNamespace,
			"--set-string", "gke.project_number="+cfg.GKEProjectNumber,
			"--set", "rollout.experimentalBrackets.enabled="+boolStr(p.BracketsEnabled),
			"--set", "rollout.experimentalBrackets.clusters.development="+boolStr(p.DevelopmentEnabled),
			"--set", "rollout.experimentalBrackets.clusters.staging="+boolStr(p.StagingEnabled),
			"--set", "rollout.experimentalBrackets.clusters.aa-test="+boolStr(p.AATestEnabled),
			"--set", fmt.Sprintf("rollout.kuberpultEventsChannelSize=%d", p.ChannelSize),
			cfg.HelmReleaseName, chartPath)
	}
	if out2, err2 := cmd.CombinedOutput(); err2 != nil {
		tb.Fatalf("helm upgrade (brackets=%s dev=%s staging=%s): %v\n%s",
			boolStr(p.BracketsEnabled), boolStr(p.DevelopmentEnabled), boolStr(p.StagingEnabled), err2, out2)
	}

	for _, dep := range []string{
		"deployment/kuberpult-rollout-service",
		"deployment/kuberpult-cd-service",
		"deployment/kuberpult-frontend-service",
		"deployment/kuberpult-reposerver-service",
	} {
		out3, err3 := exec.Command("kubectl", "rollout", "status", dep,
			"-n", cfg.KuberpultNamespace, "--timeout=3m").CombinedOutput()
		if err3 != nil {
			tb.Fatalf("kubectl rollout status %s: %v\n%s", dep, err3, out3)
		}
		tb.Logf("%s rolled out: %s", dep, strings.TrimSpace(string(out3)))
	}

	EnsurePortForwards(tb, cfg)
	WaitForFrontendReady(tb)

	scriptPath := root + "/infrastructure/scripts/create-testdata/create-environments.sh"
	scriptCmd := exec.Command("bash", scriptPath)
	scriptCmd.Env = append(os.Environ(), "FRONTEND_PORT="+FrontendPort)
	if out4, err4 := scriptCmd.CombinedOutput(); err4 != nil {
		tb.Fatalf("create-environments: %v\n%s", err4, out4)
	}
	tb.Logf("create-environments: done")
}

// DBCreds holds database credentials (GKE mode only; kind uses kubectl exec).
type DBCreds struct {
	User     string
	Password string
	DBName   string
}

// MustLoadDBCreds reads DB credentials for the given config.
// Kind mode returns empty DBCreds (psql runs via kubectl exec).
func MustLoadDBCreds(cfg Config) DBCreds {
	if cfg.Mode != ModeGKE {
		return DBCreds{}
	}
	creds := DBCreds{
		User:     ReadSecretKey("kuberpult-db", "tools", "username"),
		Password: ReadSecretKey("kuberpult-db", "tools", "password"),
	}
	out, err := exec.Command("kubectl", "get", "deployment", "kuberpult-cd-service",
		"-n", cfg.KuberpultNamespace, "-o",
		`jsonpath={range .spec.template.spec.containers[*]}{range .env[*]}{.name}{"="}{.value}{"\n"}{end}{end}`,
	).Output()
	if err != nil {
		log.Fatalf("gke mode: get env vars from kuberpult-cd-service deployment: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if after, ok := strings.CutPrefix(line, "KUBERPULT_DB_NAME="); ok {
			creds.DBName = strings.TrimSpace(after)
			break
		}
	}
	if creds.DBName == "" {
		log.Fatalf("gke mode: KUBERPULT_DB_NAME not found in kuberpult-cd-service deployment")
	}
	return creds
}

func runPsql(cfg Config, db DBCreds, psqlArgs ...string) ([]byte, error) {
	if cfg.Mode == ModeKind {
		base := []string{"exec", "deployment/postgres", "-n", cfg.KuberpultNamespace, "--",
			"psql", "-U", "postgres", "-d", "kuberpult"}
		return exec.Command("kubectl", append(base, psqlArgs...)...).CombinedOutput()
	}
	base := []string{"-h", "127.0.0.1", "-p", "5433", "-U", db.User, "-d", db.DBName, "-w"}
	cmd := exec.Command("psql", append(base, psqlArgs...)...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+db.Password)
	return cmd.CombinedOutput()
}

func openDBPortForward(tb TB, cfg Config, db DBCreds) func() {
	tb.Helper()
	if cfg.Mode == ModeKind {
		return func() {}
	}
	pf := exec.Command("kubectl", "port-forward",
		"-n", cfg.KuberpultNamespace,
		"deployment/kuberpult-cd-service",
		"5433:5432")
	if err := pf.Start(); err != nil {
		tb.Fatalf("openDBPortForward: %v", err)
	}
	stop := func() {
		if pf.Process != nil {
			_ = pf.Process.Kill()
		}
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		probe := exec.Command("psql", "-h", "127.0.0.1", "-p", "5433",
			"-U", db.User, "-d", db.DBName, "-w", "-c", "SELECT 1")
		probe.Env = append(os.Environ(), "PGPASSWORD="+db.Password)
		if err := probe.Run(); err == nil {
			return stop
		}
		time.Sleep(500 * time.Millisecond)
	}
	stop()
	tb.Fatalf("openDBPortForward: not ready after 15 seconds")
	return func() {}
}

// ResetDB deletes all ArgoCD applications, drops all DB tables, and scales
// kuberpult services to 0. After this call, run helm-upgrade to bring
// services back up.
func ResetDB(tb TB, cfg Config, db DBCreds) {
	tb.Helper()

	tb.Logf("ResetDB: removing ArgoCD application finalizers")
	names, _ := exec.Command("kubectl", "get", "applications",
		"-n", cfg.KuberpultNamespace, "-o", "name").CombinedOutput()
	for _, name := range strings.Fields(string(names)) {
		if err := exec.Command("kubectl", "patch", "-n", cfg.KuberpultNamespace, name,
			"--type=merge", "-p", `{"metadata":{"finalizers":[]}}`).Run(); err != nil {
			tb.Logf("ResetDB: patch finalizers for %s failed (continuing): %v", name, err)
		}
	}
	tb.Logf("ResetDB: deleting all ArgoCD applications")
	out, err := exec.Command("kubectl", "delete", "applications", "--all",
		"-n", cfg.KuberpultNamespace, "--wait=true", "--cascade=true").CombinedOutput()
	if err != nil {
		tb.Fatalf("ResetDB: delete applications: %v: %s", err, out)
	}
	tb.Logf("ResetDB: %s", strings.TrimSpace(string(out)))

	// In GKE mode, postgres is accessed via the cloud-sql-proxy sidecar in the
	// cd-service pod. If a previous reset-db run scaled cd-service to 0, there
	// are no pods to port-forward to. Bring it back up before opening the tunnel.
	if cfg.Mode == ModeGKE {
		tb.Logf("ResetDB: ensuring kuberpult-cd-service has at least 1 replica for cloud-sql-proxy access")
		if out2, err2 := exec.Command("kubectl", "scale", "deployment/kuberpult-cd-service",
			"-n", cfg.KuberpultNamespace, "--replicas=1").CombinedOutput(); err2 != nil {
			tb.Fatalf("ResetDB: scale cd-service to 1: %v\n%s", err2, out2)
		}
		if out2, err2 := exec.Command("kubectl", "rollout", "status", "deployment/kuberpult-cd-service",
			"-n", cfg.KuberpultNamespace, "--timeout=3m").CombinedOutput(); err2 != nil {
			tb.Fatalf("ResetDB: wait for cd-service rollout: %v\n%s", err2, out2)
		}
	}

	stopDBPF := openDBPortForward(tb, cfg, db)
	defer stopDBPF()

	tb.Logf("ResetDB: querying tables")
	out, err = runPsql(cfg, db, "-At", "-c",
		"SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename;")
	if err != nil {
		tb.Fatalf("ResetDB: list tables: %v: %s", err, out)
	}
	var tables []string
	for _, line := range strings.Fields(string(out)) {
		if line != "" {
			tables = append(tables, line)
		}
	}
	if len(tables) == 0 {
		tb.Logf("ResetDB: no tables found, db is already empty")
	} else {
		tb.Logf("ResetDB: dropping %d tables: %s", len(tables), strings.Join(tables, ", "))
		for batch := range slices.Chunk(tables, 5) {
			var parts []string
			for _, table := range batch {
				parts = append(parts, fmt.Sprintf(`"%s"`, table))
			}
			out2, err2 := runPsql(cfg, db, "-c",
				fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE;", strings.Join(parts, ", ")))
			if err2 != nil {
				tb.Fatalf("ResetDB: drop %s: %v: %s", strings.Join(parts, ", "), err2, out2)
			}
		}
	}

	// frontend-service is kept running to avoid breaking the ingress.
	tb.Logf("ResetDB: scaling kuberpult services to 0")
	for _, dep := range []string{
		"deployment/kuberpult-cd-service",
		"deployment/kuberpult-reposerver-service",
		"deployment/kuberpult-rollout-service",
	} {
		if out3, err3 := exec.Command("kubectl", "scale", dep,
			"-n", cfg.KuberpultNamespace, "--replicas=0").CombinedOutput(); err3 != nil {
			tb.Fatalf("ResetDB: scale %s: %v\n%s", dep, err3, out3)
		}
	}
}

// WaitForFrontendReady polls the /health endpoint until the frontend responds.
func WaitForFrontendReady(tb TB) {
	tb.Helper()
	url := "http://localhost:" + FrontendPort + "/health"
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:gosec
		if err == nil {
			tb.Logf("WaitForFrontendReady: %s responded (status %d)", url, resp.StatusCode)
			_ = resp.Body.Close()
			return
		}
		if err != lastErr {
			tb.Logf("WaitForFrontendReady: %s: %v", url, err)
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	tb.Fatalf("frontend not reachable at %s after 30s (last error: %v)", url, lastErr)
}

// ReleaseTrain triggers a release train for the given environment.
func ReleaseTrain(tb TB, env string) {
	tb.Helper()
	url := "http://localhost:" + FrontendPort + "/api/environments/" + env + "/releasetrain"
	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		tb.Fatalf("build release-train request for %s: %v", env, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		tb.Fatalf("PUT release train %s: %v", env, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode > 299 {
		tb.Fatalf("PUT release train %s: HTTP %d: %s", env, resp.StatusCode, body)
	}
}

// CreateRelease posts a new release to kuberpult. manifests maps environment
// name to the manifest YAML for that environment.
func CreateRelease(tb TB, app, team, bracket, version string, manifests map[string]string) {
	tb.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	mustWrite := func(key, val string) {
		if err := w.WriteField(key, val); err != nil {
			tb.Fatalf("write field %s: %v", key, err)
		}
	}
	mustWrite("application", app)
	mustWrite("version", version)
	mustWrite("team", team)
	mustWrite("source_message", "release "+version)
	if bracket != "" {
		mustWrite("experimentalArgoBracket", bracket)
	}
	for env, manifest := range manifests {
		fw, err := w.CreateFormFile("manifests["+env+"]", "manifests["+env+"]")
		if err != nil {
			tb.Fatalf("create form file for %s: %v", env, err)
		}
		if _, err := io.WriteString(fw, manifest); err != nil {
			tb.Fatalf("write manifest for %s: %v", env, err)
		}
	}
	if err := w.Close(); err != nil {
		tb.Fatalf("close multipart writer: %v", err)
	}

	req, err := http.NewRequest("POST", "http://localhost:"+FrontendPort+"/api/release", &b)
	if err != nil {
		tb.Fatalf("build release request for %s v%s: %v", app, version, err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		tb.Fatalf("POST /api/release for %s v%s: %v", app, version, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode > 299 {
		tb.Fatalf("POST /api/release for %s v%s: HTTP %d: %s", app, version, resp.StatusCode, body)
	}
}

// StableManifest returns a Deployment + ConfigMap whose pod-template spec is
// identical across all version numbers so Kubernetes does not roll pods.
func StableManifest(app, namespace, version string) string {
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
      terminationGracePeriodSeconds: 0
      containers:
      - name: sleep
        image: busybox:latest
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
