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
				EnableSqlite:         false,
				ReleaseVersionsLimit: minReleaseVersionsLimit,
			},
			expectedError: nil,
		},
		{
			name: "versions limit equals the maximum allowed",
			config: Config{
				EnableSqlite:         false,
				ReleaseVersionsLimit: maxReleaseVersionsLimit,
			},
			expectedError: nil,
		},
		{
			name: "default versions limit",
			config: Config{
				EnableSqlite:         false,
				ReleaseVersionsLimit: 20,
			},
			expectedError: nil,
		},
		{
			name: "versions limit below minimum",
			config: Config{
				EnableSqlite:         false,
				ReleaseVersionsLimit: 3,
			},
			expectedError: releaseVersionsLimitError{limit: 3},
		},
		{
			name: "versions limit above maximum",
			config: Config{
				EnableSqlite:         false,
				ReleaseVersionsLimit: 45,
			},
			expectedError: releaseVersionsLimitError{limit: 45},
		},
		{
			name: "sqlite enabled with versions limit within range",
			config: Config{
				EnableSqlite:         true,
				ReleaseVersionsLimit: 20,
			},
			expectedError: nil,
		},
		{
			name: "sqlite enabled with versions limit out of range",
			config: Config{
				EnableSqlite:         true,
				ReleaseVersionsLimit: 45,
			},
			expectedError: nil,
		},
	} {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := checkReleaseVersionLimit(tc.config)
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
