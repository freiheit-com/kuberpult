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

package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
)

type mockRolloutCient struct {
	resp *api.GetStatusResponse
	req  *api.GetStatusRequest
	err  error
}

func (b *mockRolloutCient) StreamStatus(ctx context.Context, req *api.StreamStatusRequest, opts ...grpc.CallOption) (api.RolloutService_StreamStatusClient, error) {
	return nil, fmt.Errorf("not implemented")
}

func (b *mockRolloutCient) GetStatus(ctx context.Context, req *api.GetStatusRequest, opts ...grpc.CallOption) (*api.GetStatusResponse, error) {
	b.req = req
	return b.resp, b.err
}

var _ api.RolloutServiceClient = (*mockRolloutCient)(nil)

func TestHandleRolloutStatus(t *testing.T) {

	tcs := []struct {
		Name                       string
		Request                    *http.Request
		RolloutResponse            *api.GetStatusResponse
		DontConfigureRolloutClient bool
		AzureAuth                  bool

		ExpectedStatus int
		ExpectedBody   string
	}{
		{
			Name:                       "returns not implemented when not configured",
			DontConfigureRolloutClient: true,
			ExpectedStatus:             http.StatusNotImplemented,
			ExpectedBody:               "not implemented\n",
		},
		{
			Name: "fails on invalid json",
			Request: &http.Request{
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(``)),
			},
			ExpectedStatus: http.StatusBadRequest,
			ExpectedBody:   "invalid json in request\n",
		},
		{
			Name: "fails on missing sig",
			Request: &http.Request{
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{}`)),
			},
			AzureAuth:      true,
			ExpectedStatus: http.StatusBadRequest,
			ExpectedBody:   "Missing signature in request body - this is required with AzureAuth enabled\n",
		},
		{
			Name: "succedes with a simple request",
			Request: &http.Request{
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{}`)),
			},
			RolloutResponse: &api.GetStatusResponse{
				Status: api.RolloutStatus_RolloutStatusSuccesful,
			},
			ExpectedStatus: http.StatusOK,
			ExpectedBody:   `{"status":"succesful","applications":[]}`,
		},
		{
			Name: "succedes with a larger request",
			Request: &http.Request{
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{}`)),
			},
			RolloutResponse: &api.GetStatusResponse{
				Status:       api.RolloutStatus_RolloutStatusSuccesful,
				Applications: applicationsWithAllStates(),
			},
			ExpectedStatus: http.StatusOK,
			ExpectedBody: `{"status":"succesful","applications":[` +
				`{"application":"RolloutStatusUnknown","environment":"","status":"unknown"},` +
				`{"application":"RolloutStatusSuccesful","environment":"","status":"succesful"},` +
				`{"application":"RolloutStatusProgressing","environment":"","status":"progressing"},` +
				`{"application":"RolloutStatusError","environment":"","status":"error"},` +
				`{"application":"RolloutStatusPending","environment":"","status":"pending"},` +
				`{"application":"RolloutStatusUnhealthy","environment":"","status":"unhealthy"}` +
				`]}`,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			var rc api.RolloutServiceClient = nil
			if !tc.DontConfigureRolloutClient {
				rc = &mockRolloutCient{resp: tc.RolloutResponse}
			}
			srv := Server{
				RolloutClient: rc,
				AzureAuth:     tc.AzureAuth,
			}

			w := httptest.NewRecorder()
			srv.handleEnvironmentGroupRolloutStatus(w, tc.Request, "foo")
			result := w.Result()
			if result.StatusCode != tc.ExpectedStatus {
				t.Errorf("wrong status received, expected %d but got %d", tc.ExpectedStatus, result.StatusCode)
			}
			body, _ := io.ReadAll(result.Body)
			if d := cmp.Diff(tc.ExpectedBody, string(body)); d != "" {
				t.Errorf("response body mismatch:\ngot:  %s\nwant: %s\ndiff: \n%s", string(body), tc.ExpectedBody, d)
			}
		})
	}
}

func applicationsWithAllStates() []*api.GetStatusResponse_ApplicationStatus {
	states := []int{}
	for k := range api.RolloutStatus_name {
		states = append(states, int(k))
	}
	sort.Ints(states)
	result := []*api.GetStatusResponse_ApplicationStatus{}
	for _, i := range states {
		result = append(result, &api.GetStatusResponse_ApplicationStatus{
			Application:   api.RolloutStatus_name[int32(i)],
			RolloutStatus: api.RolloutStatus(i),
		})
	}
	return result
}
