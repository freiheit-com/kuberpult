
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
			name: "known gRPC status error",
			err:  status.Error(codes.InvalidArgument, "test message"),
			expectedResp: &http.Response{
				StatusCode: http.StatusBadRequest,
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
