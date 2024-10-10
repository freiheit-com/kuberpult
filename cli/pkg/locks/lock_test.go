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

package locks

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/freiheit-com/kuberpult/cli/pkg/kuberpult_utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

type mockHttpServer struct {
	response int
	header   http.Header
	body     LockJsonData
}

func (s *mockHttpServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	if err := json.NewDecoder(req.Body).Decode(&s.body); err != nil {
		panic(fmt.Errorf("error while parsing the json body in the mock HTTP server, error: %w", err))
	}
	s.header = req.Header
	w.WriteHeader(s.response)
}

func strPtr(s string) *string {
	return &s
}

func intPtr(n uint64) *uint64 {
	return &n
}

func TestRequestCreation(t *testing.T) {
	type testCase struct {
		name             string
		params           LockParameters
		authParams       kuberpult_utils.AuthenticationParameters
		expectedHeaders  http.Header
		expectedJson     LockJsonData
		expectedErrorMsg error
		responseCode     int
	}

	tcs := []testCase{
		{
			name: "create env lock with message",
			params: &CreateEnvironmentLockParameters{
				Environment: "production",
				LockId:      "a-lock",
				Message:     "my special message",
			},
			expectedHeaders: http.Header{
				"Content-Type": {"application/json"},
			}, expectedJson: LockJsonData{
				Message: "my special message",
			},
			responseCode: http.StatusOK,
		},
		{
			name: "create env lock with ci link",
			params: &CreateEnvironmentLockParameters{
				Environment: "production",
				LockId:      "a-lock",
				CiLink:      strPtr("https://localhost:8000"),
			},
			expectedHeaders: http.Header{
				"Content-Type": {"application/json"},
			}, expectedJson: LockJsonData{
				CiLink: "https://localhost:8000",
			},
			responseCode: http.StatusOK,
		},
		{
			name: "create app lock with message",
			params: &CreateAppLockParameters{
				Environment: "production",
				LockId:      "a-lock",
				Application: "potato",
				Message:     "my special message",
			},
			expectedHeaders: http.Header{
				"Content-Type": {"application/json"},
			}, expectedJson: LockJsonData{
				Message: "my special message",
			},
			responseCode: http.StatusOK,
		},
		{
			name: "create app lock with ci link",
			params: &CreateAppLockParameters{
				Environment: "production",
				LockId:      "a-lock",
				Application: "potato",
				CiLink:      strPtr("https://localhost:8000"),
			},
			expectedHeaders: http.Header{
				"Content-Type": {"application/json"},
			}, expectedJson: LockJsonData{
				CiLink: "https://localhost:8000",
			},
			responseCode: http.StatusOK,
		},
		{
			name: "create team lock with message",
			params: &CreateTeamLockParameters{
				Environment: "production",
				LockId:      "a-lock",
				Team:        "potato-devs",
				Message:     "my special message",
			},
			expectedHeaders: http.Header{
				"Content-Type": {"application/json"},
			}, expectedJson: LockJsonData{
				Message: "my special message",
			},
			responseCode: http.StatusOK,
		},
		{
			name: "create team lock with ci link",
			params: &CreateTeamLockParameters{
				Environment: "production",
				LockId:      "a-lock",
				Team:        "potato-devs",
				CiLink:      strPtr("https://localhost:8000"),
			},
			expectedHeaders: http.Header{
				"Content-Type": {"application/json"},
			}, expectedJson: LockJsonData{
				CiLink: "https://localhost:8000",
			},
			responseCode: http.StatusOK,
		},
		{
			name: "create group lock with message",
			params: &CreateEnvironmentGroupLockParameters{
				EnvironmentGroup: "production",
				LockId:           "a-lock",
				Message:          "my special message",
			},
			expectedHeaders: http.Header{
				"Content-Type": {"application/json"},
			}, expectedJson: LockJsonData{
				Message: "my special message",
			},
			responseCode: http.StatusOK,
		},
		{
			name: "create group lock with ci link",
			params: &CreateEnvironmentGroupLockParameters{
				EnvironmentGroup: "production",
				LockId:           "a-lock",
				CiLink:           strPtr("https://localhost:8000"),
			},
			expectedHeaders: http.Header{
				"Content-Type": {"application/json"},
			}, expectedJson: LockJsonData{
				CiLink: "https://localhost:8000",
			},
			responseCode: http.StatusOK,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockServer := &mockHttpServer{
				response: tc.responseCode,
			}
			server := httptest.NewServer(mockServer)

			authParams := tc.authParams
			requestParams := kuberpult_utils.RequestParameters{
				Url: &server.URL,
			}
			err := HandleLockRequest(requestParams, authParams, tc.params)
			// check errors
			if tc.expectedErrorMsg != nil {
				if diff := cmp.Diff(tc.expectedErrorMsg, err, cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				return
			}

			// check headers, note that we cannot compare with cmp.Diff because there are some default headers that we shouldn't bother checking (like Accept-Encoding etc)
			for key, expectedVal := range tc.expectedHeaders {
				actualVal, ok := mockServer.header[key]
				if !ok {
					t.Fatalf("there is a key in the expected headers that does not exist in the received headers\nexpected:\n  %v\nreceived:\n  %v\nmissing key:\n  %s", tc.expectedHeaders, mockServer.header, key)
				}
				if diff := cmp.Diff(expectedVal, actualVal); diff != "" {
					t.Fatalf("there is a mismatch between the expected headers and the received headers\nexpected:\n  %v\nreceived:\n  %v\ndiffering key:\n  %s\ndiff:\n  %s", tc.expectedHeaders, mockServer.header, key, diff)
				}
			}

			if mockServer.body != tc.expectedJson {
				t.Fatalf("there is a mismatch between the expected json and the received json\nexpected:\n	%+v\nreceived:\n	%+v", tc.expectedJson, mockServer.body)
			}
		})
	}
}
