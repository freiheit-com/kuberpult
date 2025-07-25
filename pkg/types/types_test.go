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

package types

import (
	"fmt"
	errs "github.com/freiheit-com/kuberpult/pkg/errorMatcher"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"testing"
)

func TestReleaseNumberParsing(t *testing.T) {
	tcs := []struct {
		Name            string
		Version         string
		ExpectedVersion ReleaseNumbers
		expectedError   error
	}{
		{
			Name:            "Accepts old version types",
			Version:         "1",
			ExpectedVersion: MakeReleaseNumberVersion(1),
			expectedError:   nil,
		},
		{
			Name:            "Accepts version with revision",
			Version:         "1.100",
			ExpectedVersion: MakeReleaseNumbers(1, 100),
			expectedError:   nil,
		},
		{
			Name:    "Rejects partial revision",
			Version: "1.something",
			expectedError: errs.ErrMatcher{
				Message: fmt.Sprintf("error generating release number. %s is not a valid revision", "something"),
			},
		},
		{
			Name:    "Rejects partial version",
			Version: "something.1",
			expectedError: errs.ErrMatcher{
				Message: fmt.Sprintf("error generating release number. %s is not a valid version", "something"),
			},
		},
		{
			Name:    "Rejects invalid revision",
			Version: "1.100.1",
			expectedError: errs.ErrMatcher{
				Message: fmt.Sprintf("error generating release number. %s is not a valid release number", "1.100.1"),
			},
		},
		{
			Name:    "Rejects something that is not a revision",
			Version: "this is not a valid revision",
			expectedError: errs.ErrMatcher{
				Message: fmt.Sprintf("error generating release number. %s is not a valid release number", "this is not a valid revision"),
			},
		},
		{
			Name:    "Rejects something that is not a revision",
			Version: "something.something",
			expectedError: errs.ErrMatcher{
				Message: fmt.Sprintf("error generating release number. %s is not a valid version", "something"),
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			parsedVersion, err := MakeReleaseNumberFromString(tc.Version)
			if tc.expectedError != nil {
				if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
			} else {
				if !Equal(parsedVersion, tc.ExpectedVersion) {
					t.Fatalf("Version: %v does not match expected version: %v\n", parsedVersion, tc.ExpectedVersion)
				}
			}

		})
	}

}
