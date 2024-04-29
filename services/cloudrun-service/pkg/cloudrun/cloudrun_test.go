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

package cloudrun

import (
	"testing"

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
		expectedError  string
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
			expectedError:  serviceConfigError{name: "testService", namespaceMissing: true}.Error(),
		},
		{
			testName: "Location does not exist",
			service: &run.Service{
				Metadata: &run.ObjectMeta{Name: "testService", Namespace: "proj_id"},
			},
			expectedParent: "",
			expectedError:  serviceConfigError{name: "testService", locationMissing: true}.Error(),
		},
		{
			testName: "Namespace and Location do not exist",
			service: &run.Service{
				Metadata: &run.ObjectMeta{Name: "testService"},
			},
			expectedParent: "",
			expectedError:  serviceConfigError{name: "testService", namespaceMissing: true, locationMissing: true}.Error(),
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
			expectedError:  "",
		},
	} {
		testCase := test
		t.Run(testCase.testName, func(t *testing.T) {
			t.Parallel()
			parent, err := getParent(testCase.service)
			if err != nil {
				if err.Error() != testCase.expectedError {
					t.Errorf(errorMessage, testCase.expectedError, err.Error())
				}
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
		expectedError     string
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
			expectedError: "",
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
			expectedError: "",
		},
	} {
		testCase := test
		t.Run(testCase.testName, func(t *testing.T) {
			t.Parallel()
			conditions, err := GetServiceReadyCondition(testCase.service)
			if err != nil {
				if err.Error() != testCase.expectedError {
					t.Errorf(errorMessage, testCase.expectedError, err.Error())
				}
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
		expectedError string
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
			expectedError: "",
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
			expectedError: "failed to get operation-id for service test-service",
		},
	} {
		testCase := test
		t.Run(testCase.testName, func(t *testing.T) {
			t.Parallel()
			operationId, err := getOperationId(parent, testCase.service)
			if err != nil {
				if err.Error() != testCase.expectedError {
					t.Errorf(errorMessage, testCase.expectedError, err.Error())
				}
			}
			if operationId != testCase.expectedId {
				t.Errorf(errorMessage, testCase.expectedId, operationId)
			}
		})
	}
}
