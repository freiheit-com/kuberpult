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

package errors

import (
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"testing"
)

func TestUnwrapUntilRetryError(t *testing.T) {
	tcs := []struct {
		InputError error
		Expected   *RetryError
	}{
		{
			InputError: fmt.Errorf("errorf"),
			Expected:   nil,
		},
		{
			InputError: errors.New("new"),
			Expected:   nil,
		},
		{
			InputError: RetryGitRepo(fmt.Errorf("original")),
			Expected:   RetryGitRepo(fmt.Errorf("original")),
		},
		{
			InputError: fmt.Errorf("foo: %w", RetryGitRepo(fmt.Errorf("original"))),
			Expected:   RetryGitRepo(fmt.Errorf("original")),
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(fmt.Sprintf("error_retryable_%s", tc.InputError.Error()), func(t *testing.T) {
			t.Parallel()

			actualRetryable := UnwrapUntilRetryError(tc.InputError)

			if diff := cmp.Diff(tc.Expected, actualRetryable); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
