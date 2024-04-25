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
					t.Errorf("Expected %s, but got %s", testCase.expectedError, err.Error())
				}
			}
			if parent != testCase.expectedParent {
				t.Errorf("Expected %s, but got %s", testCase.expectedParent, parent)
			}
		})
	}
}
