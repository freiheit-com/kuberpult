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

package cmd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestCheckReleaseVersionLimit(t *testing.T) {
	for _, test := range []struct {
		name          string
		config        Config
		expectedError error
	}{
		{
			name: "versions limit equals the minimum allowed",
			config: Config{
				ReleaseVersionsLimit: minReleaseVersionsLimit,
			},
			expectedError: nil,
		},
		{
			name: "versions limit equals the maximum allowed",
			config: Config{
				ReleaseVersionsLimit: maxReleaseVersionsLimit,
			},
			expectedError: nil,
		},
		{
			name: "default versions limit",
			config: Config{
				ReleaseVersionsLimit: 20,
			},
			expectedError: nil,
		},
		{
			name: "versions limit below minimum",
			config: Config{
				ReleaseVersionsLimit: 3,
			},
			expectedError: releaseVersionsLimitError{limit: 3},
		},
		{
			name: "versions limit above maximum",
			config: Config{
				ReleaseVersionsLimit: 45,
			},
			expectedError: releaseVersionsLimitError{limit: 45},
		},
	} {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := checkReleaseVersionLimit(tc.config.ReleaseVersionsLimit)
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
