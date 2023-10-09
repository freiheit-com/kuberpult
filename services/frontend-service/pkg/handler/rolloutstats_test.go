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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"
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
	key, err := openpgp.NewEntity("Test", "", "test@example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	keyRing := openpgp.EntityList{key}
	environmentGroup := "foo"
	tcs := []struct {
		Name                       string
		Request                    *http.Request
		RolloutResponse            *api.GetStatusResponse
		DontConfigureRolloutClient bool
		NoAzureAuth                bool

		ExpectedStatus  int
		ExpectedBody    string
		ExpectedRequest *api.GetStatusRequest
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
			Name: "fails on missing signature",
			Request: &http.Request{
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{}`)),
			},
			ExpectedStatus: http.StatusBadRequest,
			ExpectedBody:   "Missing signature in request body - this is required with AzureAuth enabled\n",
		},
		{
			Name: "fails with wrong signature",
			Request: &http.Request{
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"signature":"fooo"}`)),
			},
			ExpectedStatus: http.StatusUnauthorized,
			ExpectedBody:   "Internal: Invalid Signature: EOF",
		},
		{
			Name: "succedes with correct signature",
			Request: &http.Request{
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"signature":` + toJson(mustSign(key, environmentGroup)) + `}`)),
			},
			RolloutResponse: &api.GetStatusResponse{
				Status: api.RolloutStatus_RolloutStatusSuccesful,
			},
			ExpectedStatus: http.StatusOK,
			ExpectedBody:   `{"status":"succesful","applications":[]}`,
			ExpectedRequest: &api.GetStatusRequest{
				EnvironmentGroup: environmentGroup,
			},
		},
		{
			Name: "succedes without signature when no auth is enabled",
			Request: &http.Request{
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{}`)),
			},
			NoAzureAuth: true,
			RolloutResponse: &api.GetStatusResponse{
				Status: api.RolloutStatus_RolloutStatusSuccesful,
			},
			ExpectedStatus: http.StatusOK,
			ExpectedBody:   `{"status":"succesful","applications":[]}`,
			ExpectedRequest: &api.GetStatusRequest{
				EnvironmentGroup: environmentGroup,
			},
		},
		{
			Name: "succedes with a larger request",
			Request: &http.Request{
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"signature":` + toJson(mustSign(key, environmentGroup)) + `}`)),
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
			ExpectedRequest: &api.GetStatusRequest{
				EnvironmentGroup: environmentGroup,
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			var rc api.RolloutServiceClient = nil
			mrc := &mockRolloutCient{resp: tc.RolloutResponse}
			if !tc.DontConfigureRolloutClient {
				rc = mrc
			}

			srv := Server{
				RolloutClient: rc,
				AzureAuth:     !tc.NoAzureAuth,
				KeyRing:       keyRing,
			}

			w := httptest.NewRecorder()
			srv.handleEnvironmentGroupRolloutStatus(w, tc.Request, environmentGroup)
			result := w.Result()
			if result.StatusCode != tc.ExpectedStatus {
				t.Errorf("wrong status received, expected %d but got %d", tc.ExpectedStatus, result.StatusCode)
			}
			body, _ := io.ReadAll(result.Body)
			if d := cmp.Diff(tc.ExpectedBody, string(body)); d != "" {
				t.Errorf("response body mismatch:\ngot:  %s\nwant: %s\ndiff: \n%s", string(body), tc.ExpectedBody, d)
			}

			if d := cmp.Diff(tc.ExpectedRequest, mrc.req, protocmp.Transform()); d != "" {
				t.Errorf("request mismatch:\ndiff:%s", d)
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

func mustSign(key *openpgp.Entity, data string) string {
	signatureBuffer := bytes.Buffer{}
	err := openpgp.ArmoredDetachSign(&signatureBuffer, key, bytes.NewReader([]byte(data)), nil)
	if err != nil {
		panic(err)
	}
	return signatureBuffer.String()
}

func toJson(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
