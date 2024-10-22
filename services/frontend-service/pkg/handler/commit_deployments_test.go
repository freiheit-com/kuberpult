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

package handler

import (
	"context"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"google.golang.org/grpc"
)

type mockCommitDeploymentServiceClient struct {
	failGrpcCall bool
}

func (m mockCommitDeploymentServiceClient) GetCommitDeploymentInfo(ctx context.Context, in *api.GetCommitDeploymentInfoRequest, opts ...grpc.CallOption) (*api.GetCommitDeploymentInfoResponse, error) {
	if m.failGrpcCall == true {
		return nil, fmt.Errorf("some error")
	}
	return &api.GetCommitDeploymentInfoResponse{
		DeploymentStatus: map[string]*api.AppCommitDeploymentStatus{
			"app1": {
				DeploymentStatus: map[string]api.CommitDeploymentStatus{
					"dev":   api.CommitDeploymentStatus_DEPLOYED,
					"stage": api.CommitDeploymentStatus_PENDING,
					"prod":  api.CommitDeploymentStatus_UNKNOWN,
				},
			},
		},
	}, nil
}

func TestHandleCommitDeployments(t *testing.T) {
	tcs := []struct {
		name               string
		inputTail          string
		failGrpcCall       bool
		expectedStatusCode int
		expectedResponse   string
	}{
		{
			name:               "No commit provided",
			inputTail:          "",
			failGrpcCall:       false,
			expectedStatusCode: http.StatusBadRequest,
			expectedResponse:   "missing commit hash\n",
		},
		{
			name:               "trailling paths after commit",
			inputTail:          "123456/invalid",
			failGrpcCall:       false,
			expectedStatusCode: http.StatusNotFound,
			expectedResponse:   "invalid path\n",
		},
		{
			name:               "grpc call failed",
			inputTail:          "123456/",
			failGrpcCall:       true,
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponse:   "failed to get commit deployments from server: some error\n",
		},
		{
			name:               "grpc call success",
			inputTail:          "123456/",
			failGrpcCall:       false,
			expectedStatusCode: http.StatusOK,
			expectedResponse:   "{\"deploymentStatus\":{\"app1\":{\"deploymentStatus\":{\"dev\":\"DEPLOYED\", \"prod\":\"UNKNOWN\", \"stage\":\"PENDING\"}}}}\n",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/api/commit-deployments", nil)
			w := httptest.NewRecorder()
			s := Server{
				CommitDeploymentsClient: mockCommitDeploymentServiceClient{
					failGrpcCall: tc.failGrpcCall,
				},
			}
			s.handleCommitDeployments(w, req, tc.inputTail)
			if w.Code != tc.expectedStatusCode {
				t.Errorf("expected status code %d, got %d", tc.expectedStatusCode, w.Code)
			}
			if diff := cmp.Diff(tc.expectedResponse, w.Body.String()); diff != "" {
				t.Errorf("response mismatch (-want, +got):\\n%s", diff)
			}
		})
	}
}
