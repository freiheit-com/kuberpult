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

package integration_tests

import (
	"bytes"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v3"
)

type simplifiedArgoApp struct {
	Metadata struct {
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
	Status struct {
		OperationState struct {
			Phase string `yaml:"phase"`
		} `yaml:"operationState"`
	} `yaml:"status"`
}

type simplifiedConfigMapMeta struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type simplifiedConfigMap struct {
	Metadata   simplifiedConfigMapMeta `yaml:"metadata"`
	Data       map[string]string       `yaml:"data"`
	Kind       string                  `yaml:"kind"`
	ApiVersion string                  `yaml:"apiVersion"`
}

func TestArgoRolloutWork(t *testing.T) {
	testCases := []struct {
		name string
		app  string
	}{
		{
			name: "it can sync manifests into the cluster",
			app:  "rollout-" + appSuffix,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			appName := "development2-" + tc.app
			expectedConfig := simplifiedConfigMap{
				ApiVersion: "v1",
				Kind:       "ConfigMap",
				Metadata: simplifiedConfigMapMeta{
					Name:      tc.app,
					Namespace: "development2",
				},
				Data: map[string]string{
					"key": tc.app,
				},
			}
			data, err := yaml.Marshal(expectedConfig)
			if err != nil {
				t.Fatal(err)
			}
			// Release a new app that we start completely fresh
			releaseApp(t, tc.app, map[string]string{
				"development2": string(data),
			})
			time.Sleep(5 * time.Second)
			// We have to sync the root app once because we have created a new app
			runArgo(t, "app", "sync", "root")
			// The sync may already be in progress, therefore we wait here for pending operations to finish
			// runArgo(t, "app", "wait", appName, "--operation")
			// runArgo(t, "app", "sync", appName)
			_, appData := runArgo(t, "app", "get", appName, "-o", "yaml")
			var app simplifiedArgoApp
			err = yaml.Unmarshal(appData, &app)
			if err != nil {
				t.Fatal(err)
			}
			appAnnotation := app.Metadata.Annotations["com.freiheit.kuberpult/application"]
			if appAnnotation != tc.app {
				t.Errorf("wrong value for annotation \"com.freiheit.kuberpult/application\": expected %q but got %q", tc.app, appAnnotation)
			}
			if app.Status.OperationState.Phase != "Succeeded" {
				t.Errorf("wrong value for operation state phase, expected %q got %q", "Succeeded", app.Status.OperationState.Phase)
			}
			_, manifestData := runArgo(t, "app", "manifests", appName)
			var actualConfig simplifiedConfigMap
			err = yaml.Unmarshal(manifestData, &actualConfig)
			if err != nil {
				t.Fatal(err)
			}
			d := cmp.Diff(expectedConfig, actualConfig)
			if d != "" {
				t.Errorf("unexpected diff between config maps: %s", d)
			}
		})
	}
}

func runArgo(t *testing.T, args ...string) (*exec.Cmd, []byte) {
	var out, stderr bytes.Buffer
	args = append([]string{"--port-forward", "--grpc-web"}, args...)
	cmd := exec.Command("argocd", args...)
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("argocd %q command exited with code %d\nerr: %s\nstderr: %s", strings.Join(args, " "), cmd.ProcessState.ExitCode(), err, stderr.String())
	}
	if cmd.ProcessState.ExitCode() != 0 {
		t.Fatalf("argocd %q command exited with code %d\nstderr: %s", strings.Join(args, " "), cmd.ProcessState.ExitCode(), stderr.String())
	}
	return cmd, out.Bytes()
}

func releaseApp(t *testing.T, application string, manifests map[string]string) {
	values := map[string]io.Reader{
		"application":      strings.NewReader(application),
		"version":          strings.NewReader("12"),
		"source_commit_id": strings.NewReader("0123456789abcdef0123456789abcdef01234567"),
		"team":             strings.NewReader("team"),
	}
	files := map[string]io.Reader{}
	for env, data := range manifests {
		files["manifests["+env+"]"] = strings.NewReader(data)
	}
	actualStatusCode, body, err := callRelease(values, files, "/api/release")
	if err != nil {
		t.Fatal(err)
	}
	if actualStatusCode > 299 {
		t.Fatalf("bad status code: %d, body: %s", actualStatusCode, body)
	}
}
