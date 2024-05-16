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

package parser

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/run/v1"
)

func TestParseManifest(t *testing.T) {
	for _, test := range []struct {
		testName        string
		yamlData        []byte
		expectedService run.Service
	}{
		{
			testName:        "Test empty manifest",
			yamlData:        []byte(``),
			expectedService: run.Service{},
		},
		{
			testName: "Test service metadata",
			yamlData: []byte(`
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: test-service
  namespace: gcp-proj
  labels:
    label1: value1
    label2: value2
`),
			expectedService: run.Service{
				ApiVersion: "serving.knative.dev/v1",
				Kind:       "Service",
				Metadata: &run.ObjectMeta{
					Name:      "test-service",
					Namespace: "gcp-proj",
					Labels: map[string]string{
						"label1": "value1",
						"label2": "value2",
					},
				},
			},
		},
		{
			testName: "Test service spec",
			yamlData: []byte(`
apiVersion: serving.knative.dev/v1
kind: Service
spec:
  template:
    metadata:
      labels:
        service_label: value
      annotations:
        service_annotation: value
    spec:
      containerConcurrency: 2
      timeoutSeconds: 20
      serviceAccountName: test@test
      containers:
      - image: image.uri:image.tag
        ports:
        - name: http1
          containerPort: 8080
        env:
        - name: ENV_VAR_1
          value: '8080'
        resources:
          limits:
            cpu: '1'
            memory: 512Gi
`),
			expectedService: run.Service{
				ApiVersion: "serving.knative.dev/v1",
				Kind:       "Service",
				Spec: &run.ServiceSpec{
					Template: &run.RevisionTemplate{
						Metadata: &run.ObjectMeta{
							Labels: map[string]string{
								"service_label": "value",
							},
							Annotations: map[string]string{
								"service_annotation": "value",
							},
						},
						Spec: &run.RevisionSpec{
							ContainerConcurrency: 2,
							TimeoutSeconds:       20,
							ServiceAccountName:   "test@test",
							Containers: []*run.Container{
								{
									Image: "image.uri:image.tag",
									Ports: []*run.ContainerPort{
										{
											Name:          "http1",
											ContainerPort: 8080,
										},
									},
									Resources: &run.ResourceRequirements{
										Limits: map[string]string{
											"cpu":    "1",
											"memory": "512Gi",
										},
									},
									Env: []*run.EnvVar{
										{
											Name:  "ENV_VAR_1",
											Value: "8080",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	} {
		testCase := test
		t.Run(testCase.testName, func(t *testing.T) {
			t.Parallel()
			var service run.Service
			err := ParseManifest(testCase.yamlData, &service)
			if err != nil {
				t.Fatalf("Expected no error, but got %v", err)
			}
			if diff := cmp.Diff(testCase.expectedService, service); diff != "" {
				t.Errorf("Service mismatch (-want, +got) %s", diff)
			}

		})
	}
}
