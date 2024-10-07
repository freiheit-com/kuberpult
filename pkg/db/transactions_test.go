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

package db

import (
	"fmt"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/google/go-cmp/cmp"
	"github.com/lib/pq"
	"testing"
)

func TestIsRetryableError(t *testing.T) {
	const ExpectedRetryable = true
	tcs := []struct {
		InputCode string
	}{
		{
			InputCode: "40000",
		},
		{
			InputCode: "40001",
		},
		{
			InputCode: "40002",
		},
		{
			InputCode: "40003",
		},
		{
			InputCode: "40P10",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(fmt.Sprintf("error_code_is_retryable_%s", tc.InputCode), func(t *testing.T) {
			t.Parallel()

			actualRetryable := IsRetryablePostgresError(&pq.Error{Code: pq.ErrorCode(tc.InputCode)})

			if diff := cmp.Diff(ExpectedRetryable, actualRetryable); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestIsNotRetryableError(t *testing.T) {
	const ExpectedRetryable = false
	tcs := []struct {
		InputCode string
	}{
		{
			InputCode: "00000",
		},
		{
			InputCode: "54000",
		},
		{
			InputCode: "54001",
		},
		{
			InputCode: "54011",
		},
		{
			InputCode: "54023",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(fmt.Sprintf("error_code_is_not_retryable_%s", tc.InputCode), func(t *testing.T) {
			t.Parallel()

			actualRetryable := IsRetryablePostgresError(&pq.Error{Code: pq.ErrorCode(tc.InputCode)})

			if diff := cmp.Diff(ExpectedRetryable, actualRetryable); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestIsNotRetryableErrorEndlessLoop(t *testing.T) {
	tcs := []struct {
		InputCode         error
		ExpectedRetryable bool
	}{
		{
			InputCode:         fmt.Errorf("foobar"),
			ExpectedRetryable: false,
		},
		{
			InputCode:         fmt.Errorf("foobar1: %w", &pq.Error{Code: pq.ErrorCode("54000")}),
			ExpectedRetryable: false,
		},
		{
			InputCode:         fmt.Errorf("other1: %w", fmt.Errorf("foobar: %w", &pq.Error{Code: pq.ErrorCode("54000")})),
			ExpectedRetryable: false,
		},
		{
			InputCode:         fmt.Errorf("foobar2: %w", &pq.Error{Code: pq.ErrorCode("40000")}),
			ExpectedRetryable: true,
		},
		{
			InputCode:         fmt.Errorf("other2: %w", fmt.Errorf("foobar: %w", &pq.Error{Code: pq.ErrorCode("40000")})),
			ExpectedRetryable: true,
		},
		{
			InputCode:         repository.GetCreateReleaseGeneralFailure(fmt.Errorf("other2: %w", fmt.Errorf("foobar: %w", &pq.Error{Code: pq.ErrorCode("23505")}))),
			ExpectedRetryable: true,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(fmt.Sprintf("endless_loop_check_%s", tc.InputCode.Error()), func(t *testing.T) {
			t.Parallel()

			actualRetryable := IsRetryablePostgresError(tc.InputCode)

			if diff := cmp.Diff(tc.ExpectedRetryable, actualRetryable); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
