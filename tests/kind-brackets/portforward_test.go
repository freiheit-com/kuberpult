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
	"os"
	"os/exec"
	"sync"
	"time"
)

// portForwardManager keeps a kubectl port-forward process alive. When the process exits
// (pod replaced, network blip) the goroutine restarts it after a short guard
// delay. Calling restart() kills the current process so the loop immediately
// connects to the newly rolled-out pod.
type portForwardManager struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

// startPFManager kills any shell-managed port-forward for kuberpult-frontend-service
// and starts a self-healing manager that owns port 5002 for the test run.
func startPFManager() *portForwardManager {
	// Take ownership from any shell background loop; ignore exit code.
	_ = exec.Command("pkill", "-f", "port-forward.*kuberpult-frontend-service").Run()
	ctx, cancel := context.WithCancel(context.Background())
	m := &portForwardManager{cancel: cancel}
	go m.loop(ctx)
	return m
}

func (m *portForwardManager) loop(ctx context.Context) {
	for ctx.Err() == nil {
		cmd := exec.CommandContext(ctx, "kubectl", "port-forward",
			"-n", "default",
			"deployment/kuberpult-frontend-service",
			"5002:8081")
		m.mu.Lock()
		m.cmd = cmd
		m.mu.Unlock()
		if err := cmd.Run(); err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "port-forward exited: %v — restarting\n", err)
		}
		// Brief guard to avoid a tight spin when kubectl fails immediately
		// (e.g. port still releasing). The concrete readiness signal is
		// waitForFrontendHTTPReady at the call site.
		select {
		case <-ctx.Done():
			return
		case <-time.After(300 * time.Millisecond):
		}
	}
}

// restart kills the running port-forward process so the loop reconnects to the
// freshly rolled-out pod. The caller should then call waitForFrontendHTTPReady.
func (m *portForwardManager) restart() {
	m.mu.Lock()
	cmd := m.cmd
	m.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func (m *portForwardManager) stop() {
	m.cancel()
}
