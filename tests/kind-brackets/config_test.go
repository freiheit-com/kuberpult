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
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// local port used to reach the Cloud SQL proxy sidecar in GKE mode.
const gkeDBPortForwardPort = "5433"

// port the cloud-sql-proxy sidecar listens on inside the pod.
const gkeDBProxyPort = "5432"

// testConfig extends Config with DB-access credentials (GKE only).
// Tests use this; cmd/main.go uses Config directly.
type testConfig struct {
	Config
	dbUser     string
	dbPassword string
	dbName     string
}

// mustLoadConfig calls MustLoadConfig for the shared fields, then adds
// DB credentials for GKE mode.
func mustLoadConfig() testConfig {
	cfg := MustLoadConfig()
	tc := testConfig{Config: cfg}
	if cfg.Mode != ModeGKE {
		return tc
	}

	tc.dbUser = ReadSecretKey("kuberpult-db", "tools", "username")
	tc.dbPassword = ReadSecretKey("kuberpult-db", "tools", "password")

	// Range over all containers to find KUBERPULT_DB_NAME (the cloud-sql-proxy
	// sidecar may appear before the kuberpult-cd-service container).
	dbNameOut, err := exec.Command("kubectl", "get", "deployment", "kuberpult-cd-service",
		"-n", "tools", "-o",
		`jsonpath={range .spec.template.spec.containers[*]}{range .env[*]}{.name}{"="}{.value}{"\n"}{end}{end}`,
	).Output()
	if err != nil {
		log.Fatalf("gke mode: get env vars from kuberpult-cd-service deployment: %v", err)
	}
	for _, line := range strings.Split(string(dbNameOut), "\n") {
		if after, ok := strings.CutPrefix(line, "KUBERPULT_DB_NAME="); ok {
			tc.dbName = strings.TrimSpace(after)
			break
		}
	}
	if tc.dbName == "" {
		log.Fatalf("gke mode: KUBERPULT_DB_NAME not found in kuberpult-cd-service deployment")
	}

	EnsureArgoAppProjects(cfg.KuberpultNamespace)

	return tc
}

// runPsql executes psql with the given args against the kuberpult database.
// Kind: via kubectl exec into the postgres pod.
// GKE: directly against the Cloud SQL proxy sidecar; the caller must have opened
// the port-forward first with startDBPortForward.
func (c testConfig) runPsql(psqlArgs ...string) ([]byte, error) {
	if c.Mode == ModeKind {
		base := []string{
			"exec", "deployment/postgres",
			"-n", c.KuberpultNamespace, "--",
			"psql", "-U", "postgres", "-d", "kuberpult",
		}
		return exec.Command("kubectl", append(base, psqlArgs...)...).CombinedOutput()
	}
	base := []string{"-h", "127.0.0.1", "-p", gkeDBPortForwardPort, "-U", c.dbUser, "-d", c.dbName, "-w"}
	cmd := exec.Command("psql", append(base, psqlArgs...)...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+c.dbPassword)
	return cmd.CombinedOutput()
}

// startDBPortForward opens a kubectl port-forward to the Cloud SQL proxy sidecar
// inside the cd-service pod and waits until psql can connect.
// In kind mode this is a no-op and returns a no-op stop func.
func (c testConfig) startDBPortForward(t *testing.T) func() {
	t.Helper()
	if c.Mode == ModeKind {
		return func() {}
	}

	// If a previous test run left services scaled to 0 (e.g. crashed between resetDB
	// and helmUpgrade), run a neutral helmUpgrade to bring them back up with the
	// correct image version before we try to port-forward. Using raw kubectl scale
	// would start the pod from the last Helm release, which may have a stale image tag.
	readyOut, _ := exec.Command("kubectl", "get", "deployment", "kuberpult-cd-service",
		"-n", c.KuberpultNamespace,
		"-o", "jsonpath={.status.readyReplicas}").Output()
	if r := strings.TrimSpace(string(readyOut)); r == "" || r == "0" {
		t.Logf("startDBPortForward: services at 0 replicas, running recovery helmUpgrade")
		helmUpgrade(t, helmUpgradeParams{bracketsEnabled: false, channelSize: 50})
	}

	pf := exec.Command("kubectl", "port-forward",
		"-n", c.KuberpultNamespace,
		"deployment/kuberpult-cd-service",
		gkeDBPortForwardPort+":"+gkeDBProxyPort)
	if err := pf.Start(); err != nil {
		t.Fatalf("startDBPortForward: %v", err)
	}
	stop := func() {
		if pf.Process != nil {
			_ = pf.Process.Kill()
		}
	}
	// Probe until the tunnel is ready; retry to avoid depending on a fixed sleep.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		probe := exec.Command("psql", "-h", "127.0.0.1", "-p", gkeDBPortForwardPort,
			"-U", c.dbUser, "-d", c.dbName, "-w", "-c", "SELECT 1")
		probe.Env = append(os.Environ(), "PGPASSWORD="+c.dbPassword)
		if err := probe.Run(); err == nil {
			return stop
		}
		time.Sleep(500 * time.Millisecond)
	}
	stop()
	t.Fatalf("startDBPortForward: port-forward not ready after 15 seconds")
	return func() {}
}
