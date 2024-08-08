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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func Test_handleGRPCError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedResp *http.Response
		expectedBody string
	}{
		{
			name: "not a gRPC status error",
			err:  errors.New("test"),
			expectedResp: &http.Response{
				StatusCode: http.StatusInternalServerError,
			},
			expectedBody: "Internal Server Error\n",
		},
		{
			name: "known gRPC status error (InvalidArgument)",
			err:  status.Error(codes.InvalidArgument, "test message"),
			expectedResp: &http.Response{
				StatusCode: http.StatusBadRequest,
			},
			expectedBody: "test message\n",
		},
		{
			name: "known gRPC status error (DeadlineExceeded)",
			err:  status.Error(codes.DeadlineExceeded, "test message"),
			expectedResp: &http.Response{
				StatusCode: http.StatusRequestTimeout,
			},
			expectedBody: "test message\n",
		},
		{
			name: "unknown gRPC status error",
			err:  status.Error(codes.Canceled, "test message"),
			expectedResp: &http.Response{
				StatusCode: http.StatusInternalServerError,
			},
			expectedBody: "Internal Server Error\n",
		},
		{
			name: "permission denied",
			err:  status.Error(codes.PermissionDenied, "test message"),
			expectedResp: &http.Response{
				StatusCode: http.StatusForbidden,
			},
			expectedBody: "test message\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			handleGRPCError(context.Background(), w, tt.err)
			resp := w.Result()

			if d := cmp.Diff(tt.expectedResp, resp, cmpopts.IgnoreFields(http.Response{}, "Status", "Proto", "ProtoMajor", "ProtoMinor", "Header", "Body", "ContentLength")); d != "" {
				t.Errorf("response mismatch: %s", d)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("error reading response body: %s", err)
			}
			if d := cmp.Diff(tt.expectedBody, string(body)); d != "" {
				t.Errorf("response body mismatch: %s", d)
			}
		})
	}
}
