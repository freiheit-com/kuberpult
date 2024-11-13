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
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"testing"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/google/go-cmp/cmp"
)

// Used to compare two error message strings, needed because errors.Is(fmt.Errorf(text),fmt.Errorf(text)) == false
type errMatcher struct {
	msg string
}

func (e errMatcher) Error() string {
	return e.msg
}

func (e errMatcher) Is(err error) bool {
	return e.Error() == err.Error()
}

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
			name:                  "should read first part for kind version",
			kuberpultVersionInput: "v11.10.0-7-g1a2fd8d0",
			expectedVersion:       CreateKuberpultVersion(11, 10, 0),
			expectedError:         nil,
		},
		{
			name:                  "just another kind version",
			kuberpultVersionInput: "v1.13.2-8-g0a1fd1d1",
			expectedVersion:       CreateKuberpultVersion(1, 13, 2),
			expectedError:         nil,
		},
		{
			name:                  "invalid number of dashes",
			kuberpultVersionInput: "main-main-v12.1.2-7-g08f811e8",
			expectedVersion:       nil,
			expectedError:         errMatcher{msg: "invalid version, expected 0, 2, or 3 dashes, but got main-main-v12.1.2-7-g08f811e8"},
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

			if diff := cmp.Diff(tc.expectedError, actualErr, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
