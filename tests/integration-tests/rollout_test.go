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

Copyright 2023 freiheit.com*/

package integration_tests

import (
	"bytes"
	"os/exec"
	"testing"

	"gopkg.in/yaml.v3"
)

type simplifiedArgoApp struct {
	Metadata struct {
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
	Status struct {
		OperationState struct {
			Operation struct {
				Sync struct {
					Revision string `yaml:"revision"`
				} `yaml:"sync"`
			} `yaml:"operation"`
			Phase string `yaml:"phase"`
		} `yaml:"operationState"`
	} `yaml:"status"`
}

func TestArgoRolloutWork(t *testing.T) {
	testCases := []struct {
		name string
		app  string
	}{
		{
			name: "it can sync manifests into the cluster",
			app:  "echo",
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			runArgo(t, "app", "sync", "root")
			runArgo(t, "app", "sync", "development-"+tc.app)
			_, appData := runArgo(t, "app", "get", "development-"+tc.app, "-o", "yaml")
			var app simplifiedArgoApp
			err := yaml.Unmarshal(appData, &app)
			if err != nil {
				t.Fatal(err)
			}
			appAnnotation := app.Metadata.Annotations["com.freiheit.kuberpult/application"]
			if appAnnotation != tc.app {
				t.Errorf("wrong value for annotation \"com.freiheit.kuberpult/application\": expected %q but got %q", tc.app, appAnnotation)
			}
		})
	}
}

func runArgo(t *testing.T, args ...string) (*exec.Cmd, []byte) {
	var out bytes.Buffer
	args = append([]string{"--port-forward"}, args...)
	cmd := exec.Command("argocd", args...)
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		t.Fatalf("error running argocd: %s", err)
	}
	if cmd.ProcessState.ExitCode() != 0 {
		t.Fatalf("argocd command exited with code %d", cmd.ProcessState.ExitCode())
	}
	return cmd, out.Bytes()
}
