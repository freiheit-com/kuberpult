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

package cloudrun

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/api/run/v1"
)

const errorMessage = "Expected %s, but got %s"

func TestIsPresent(t *testing.T) {
	for _, test := range []struct {
		testName         string
		servicesResponse []*run.Service
		serviceName      string
		expectedResponse bool
	}{
		{
			testName:         "No services returned",
			servicesResponse: []*run.Service{},
			serviceName:      "myService",
			expectedResponse: false,
		},
		{
			testName: "Service does not exist",
			servicesResponse: []*run.Service{
				{Metadata: &run.ObjectMeta{Name: "notMyService"}},
				{Metadata: &run.ObjectMeta{Name: "alsoNotMyService"}},
				{Metadata: &run.ObjectMeta{Name: "notIt"}},
			},
			serviceName:      "myService",
			expectedResponse: false,
		},
		{
			testName: "Service exists",
			servicesResponse: []*run.Service{
				{Metadata: &run.ObjectMeta{Name: "notMyService"}},
				{Metadata: &run.ObjectMeta{Name: "myService"}},
				{Metadata: &run.ObjectMeta{Name: "notIt"}},
			},
			serviceName:      "myService",
			expectedResponse: true,
		},
	} {
		testCase := test
		t.Run(testCase.testName, func(t *testing.T) {
			t.Parallel()
			exists := isPresent(testCase.servicesResponse, testCase.serviceName)
			if exists != testCase.expectedResponse {
				t.Errorf("Expected %t, but got %t", testCase.expectedResponse, exists)
			}
		})
	}
}

func TestGetParent(t *testing.T) {
	for _, test := range []struct {
		testName       string
		service        *run.Service
		expectedParent string
		expectedError  error
	}{
		{
			testName: "Namespace does not exist",
			service: &run.Service{
				Metadata: &run.ObjectMeta{
					Name:   "testService",
					Labels: map[string]string{serviceLocationLabel: "europe-west1"},
				},
			},
			expectedParent: "",
			expectedError:  serviceConfigError{name: "testService", namespaceMissing: true},
		},
		{
			testName: "Location does not exist",
			service: &run.Service{
				Metadata: &run.ObjectMeta{Name: "testService", Namespace: "proj_id"},
			},
			expectedParent: "",
			expectedError:  serviceConfigError{name: "testService", locationMissing: true},
		},
		{
			testName: "Namespace and Location do not exist",
			service: &run.Service{
				Metadata: &run.ObjectMeta{Name: "testService"},
			},
			expectedParent: "",
			expectedError:  serviceConfigError{name: "testService", namespaceMissing: true},
		},
		{
			testName: "Namespace and Location do exist",
			service: &run.Service{
				Metadata: &run.ObjectMeta{
					Name:      "testService",
					Namespace: "proj_id",
					Labels:    map[string]string{serviceLocationLabel: "europe-west1"},
				},
			},
			expectedParent: "projects/proj_id/locations/europe-west1",
			expectedError:  nil,
		},
	} {
		testCase := test
		t.Run(testCase.testName, func(t *testing.T) {
			t.Parallel()
			parent, err := getParent(testCase.service)
			if diff := cmp.Diff(testCase.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			if parent != testCase.expectedParent {
				t.Errorf(errorMessage, testCase.expectedParent, parent)
			}
		})
	}
}

func TestGetServiceConditions(t *testing.T) {
	for _, test := range []struct {
		testName          string
		service           *run.Service
		expectedCondition ServiceReadyCondition
		expectedError     error
	}{
		{
			testName: "Service ready",
			service: &run.Service{
				Metadata: &run.ObjectMeta{
					Name: "example-service",
				},
				Status: &run.ServiceStatus{
					ObservedGeneration: 2,
					Conditions: []*run.GoogleCloudRunV1Condition{
						{
							Type:    "Ready",
							Status:  "True",
							Message: "",
							Reason:  "",
						},
						{
							Type:   "ConfigurationsReady",
							Status: "False",
						},
						{
							Type:   "RoutesReady",
							Status: "Unknown",
						},
					},
				},
			},
			expectedCondition: ServiceReadyCondition{
				Name:     "example-service",
				Revision: 2,
				Status:   "True",
				Reason:   "",
				Message:  "",
			},
			expectedError: nil,
		},
		{
			testName: "Service not ready",
			service: &run.Service{
				Metadata: &run.ObjectMeta{
					Name: "example-service",
				},
				Status: &run.ServiceStatus{
					ObservedGeneration: 3,
					Conditions: []*run.GoogleCloudRunV1Condition{
						{
							Type:    "Ready",
							Status:  "False",
							Message: "service not ready",
							Reason:  "ErrHealthCheck",
						},
						{
							Type:   "ConfigurationsReady",
							Status: "True",
						},
						{
							Type:   "RoutesReady",
							Status: "True",
						},
					},
				},
			},
			expectedCondition: ServiceReadyCondition{
				Name:     "example-service",
				Revision: 3,
				Status:   "False",
				Message:  "service not ready",
				Reason:   "ErrHealthCheck",
			},
			expectedError: nil,
		},
		{
			testName: "Ready condition not present",
			service: &run.Service{
				Metadata: &run.ObjectMeta{
					Name: "example-service",
				},
				Status: &run.ServiceStatus{
					ObservedGeneration: 3,
					Conditions: []*run.GoogleCloudRunV1Condition{
						{
							Type:   "ConfigurationsReady",
							Status: "True",
						},
						{
							Type:   "RoutesReady",
							Status: "True",
						},
					},
				},
			},
			expectedCondition: ServiceReadyCondition{
				Name:     "example-service",
				Revision: 3,
				Status:   "",
				Message:  "",
				Reason:   "",
			},
			expectedError: serviceReadyConditionError{name: "example-service"},
		},
	} {
		testCase := test
		t.Run(testCase.testName, func(t *testing.T) {
			t.Parallel()
			conditions, err := GetServiceReadyCondition(testCase.service)
			if diff := cmp.Diff(testCase.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			if conditions != testCase.expectedCondition {
				t.Errorf(errorMessage, testCase.expectedCondition, conditions)
			}
		})
	}
}

func TestGetOperationId(t *testing.T) {
	parent := "projects/proj/locations/loc"
	for _, test := range []struct {
		testName      string
		service       *run.Service
		expectedId    string
		expectedError error
	}{
		{
			testName: "Correct operation ID",
			service: &run.Service{
				Metadata: &run.ObjectMeta{
					Name: "test-service",
					Annotations: map[string]string{
						"run.googleapis.com/operation-id": "1234-1234-1234",
					},
				},
			},
			expectedId:    parent + "/operations/1234-1234-1234",
			expectedError: nil,
		},
		{
			testName: "Operation id does not exist",
			service: &run.Service{
				Metadata: &run.ObjectMeta{
					Name:        "test-service",
					Annotations: map[string]string{},
				},
			},
			expectedId:    "",
			expectedError: operationIdMissingError{"test-service"},
		},
	} {
		testCase := test
		t.Run(testCase.testName, func(t *testing.T) {
			t.Parallel()
			operationId, err := getOperationId(parent, testCase.service)
			if diff := cmp.Diff(testCase.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			if operationId != testCase.expectedId {
				t.Errorf(errorMessage, testCase.expectedId, operationId)
			}
		})
	}
}

func TestValidateService(t *testing.T) {
	for _, test := range []struct {
		testName      string
		manifest      []byte
		expectedError error
	}{
		{
			testName:      "Empty manifest",
			manifest:      []byte(""),
			expectedError: serviceManifestError{metadataMissing: true, nameEmpty: false},
		},
		{
			testName: "Missing metadata",
			manifest: []byte(`
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
			expectedError: serviceManifestError{metadataMissing: true, nameEmpty: false},
		},
		{
			testName: "Empty service name",
			manifest: []byte(`
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name:
  namespace: gcp-proj
  labels:
  cloud.googleapis.com/location: value1
`),
			expectedError: serviceManifestError{metadataMissing: false, nameEmpty: true},
		},
		{
			testName: "Metadata exists and service name not empty",
			manifest: []byte(`
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: test-service
  namespace: gcp-proj
  labels:
    cloud.googleapis.com/location: value1
`),
			expectedError: nil,
		},
	} {
		testCase := test
		t.Run(testCase.testName, func(t *testing.T) {
			t.Parallel()
			var svc serviceConfig
			err := validateService(testCase.manifest, &svc)
			if diff := cmp.Diff(testCase.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
