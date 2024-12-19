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

package migrations

import (
	"fmt"
	"google.golang.org/protobuf/testing/protocmp"
	"testing"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/google/go-cmp/cmp"
)

func TestParseKuberpultVersion(t *testing.T) {
	type TestCase struct {
		name                  string
		kuberpultVersionInput string
		expectedVersion       *api.KuberpultVersion
		expectedError         error
	}

	tcs := []TestCase{
		{
			name:                  "should read middle part for pr version",
			kuberpultVersionInput: "pr-v11.6.10-7-g08f811e8",
			expectedVersion:       CreateKuberpultVersion(11, 6, 10),
			expectedError:         nil,
		},
		{
			name:                  "should read middle part for main version",
			kuberpultVersionInput: "main-v12.1.2-7-g08f811e8",
			expectedVersion:       CreateKuberpultVersion(12, 1, 2),
			expectedError:         nil,
		},
		{
			name:                  "invalid number of dashes",
			kuberpultVersionInput: "main-main-v12.1.2-7-g08f811e8",
			expectedVersion:       nil,
			expectedError:         fmt.Errorf("invalid version, expected 0 or 3 dashes"),
		},
		{
			name:                  "0 dashes also works",
			kuberpultVersionInput: "v12.1.2",
			expectedVersion:       CreateKuberpultVersion(12, 1, 2),
			expectedError:         nil,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			actualVersion, actualErr := ParseKuberpultVersion(tc.kuberpultVersionInput)

			if diff := cmp.Diff(tc.expectedVersion, actualVersion, protocmp.Transform()); diff != "" {
				t.Errorf("version mismatch (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.expectedError, actualErr, protocmp.Transform()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
