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
	"fmt"
	"io"
	"os/exec"
	"strings"
	"testing"

	"math/rand"

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
		// TODO this will be fixed in Ref SRX-PA568W
		{
			name: "can create application",
			app:  "rollout-" + appSuffix,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			appName := "development-" + tc.app
			expectedConfig := simplifiedConfigMap{
				ApiVersion: "v1",
				Kind:       "ConfigMap",
				Metadata: simplifiedConfigMapMeta{
					Name:      tc.app,
					Namespace: "development",
				},
				Data: map[string]string{
					"key": tc.app,
				},
			}
			data, err := yaml.Marshal(expectedConfig)
			if err != nil {
				t.Fatal(err)
			}
			releaseVersion := rand.Int() % 1000
			// Release a new app that we start completely fresh
			releaseApp(t, tc.app, map[string]string{
				"development": string(data),
			}, releaseVersion)
			// We have to sync the root app once because we have created a new app
			_, err = runArgo("app", "sync", "root")
			if err != nil {
				t.Fatalf("error while syncing root app: %v", err)
			}
			// runArgo(t, "app", "get", appName)
			appData, err := runArgo("app", "get", appName, "-o", "yaml")
			if err != nil {
				t.Errorf("expected no error but got %v", err)
			}
			var app simplifiedArgoApp
			err = yaml.Unmarshal(appData, &app)
			if err != nil {
				t.Fatal(err)
			}
			appAnnotation := app.Metadata.Annotations["com.freiheit.kuberpult/application"]
			if appAnnotation != tc.app {
				t.Errorf("wrong value for annotation \"com.freiheit.kuberpult/application\": expected %q but got %q", tc.app, appAnnotation)
			}
			// manifestData, err := runArgo("app", "manifests", appName)
			// if err != nil {
			// 	t.Errorf("error while getting app manifest: %v", err)
			// }
			// var actualConfig simplifiedConfigMap
			// err = yaml.Unmarshal(manifestData, &actualConfig)
			// if err != nil {
			// 	t.Fatal(err)
			// }
			// d := cmp.Diff(expectedConfig, actualConfig)
			// if d != "" {
			// 	t.Errorf("unexpected diff between config maps: %s", d)
			// }
		})
	}
}

func runArgo(args ...string) ([]byte, error) {
	var out, stderr bytes.Buffer
	args = append([]string{"--port-forward"}, args...)
	args = append([]string{"--grpc-web"}, args...)
	cmd := exec.Command("argocd", args...)
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return []byte{}, fmt.Errorf("argocd %q command exited with code %d\nerr: %s\nstderr: %s", strings.Join(args, " "), cmd.ProcessState.ExitCode(), err, stderr.String())
	}
	return out.Bytes(), nil
}

func releaseApp(t *testing.T, application string, manifests map[string]string, version int) {
	values := map[string]io.Reader{
		"application": strings.NewReader(application),
		"version":     strings.NewReader(fmt.Sprint(version)),
	}
	files := map[string]io.Reader{}
	for env, data := range manifests {
		files["manifests["+env+"]"] = strings.NewReader(data)
		files["signatures["+env+"]"] = strings.NewReader(CalcSignature(t, data))
	}
	actualStatusCode, body, err := callRelease(values, files)
	if err != nil {
		t.Fatal(err)
	}
	if actualStatusCode > 299 {
		t.Fatalf("bad status code: %d, body: %s", actualStatusCode, body)
	}
}
