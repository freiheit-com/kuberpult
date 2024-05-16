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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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

func TestParseArgs(t *testing.T) {
	type TestCase struct {
		name             string
		cmdArgs          string
		expectedParams   *kuberpultClientParameters
		expectedOther    []string
		expectedErrorMsg string
	}

	tcs := []TestCase{
		{
			name:             "nothing provided",
			cmdArgs:          "",
			expectedErrorMsg: "error while creating kuberpult client parameters, error: the --url arg must be set exactly once",
		},
		{
			name:             "something other than --url is provided",
			cmdArgs:          "--potato tomato",
			expectedErrorMsg: "error while parsing command line arguments, error: error while reading command line arguments, error: flag provided but not defined: -potato",
		},
		{
			name:    "only --url is provided",
			cmdArgs: "--url something.somewhere",
			expectedParams: &kuberpultClientParameters{
				url: "something.somewhere",
			},
		},
		{
			name: "--url is provided along with some other stuff",
			cmdArgs: "--url something.somewhere --potato tomato",
			expectedErrorMsg: "error while parsing command line arguments, error: error while reading command line arguments, error: flag provided but not defined: -potato",
		},
		{
			name: "--url is provided with some tail",
			cmdArgs: "--url something.somewhere potato --tomato",
			expectedParams: &kuberpultClientParameters{
				url: "something.somewhere",
			},
			expectedOther: []string{"potato", "--tomato"},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			params, other, err := parseArgs(strings.Split(tc.cmdArgs, " "))
			// check errors
			if diff := cmp.Diff(errMatcher{tc.expectedErrorMsg}, err, cmpopts.EquateErrors()); !(err == nil && tc.expectedErrorMsg == "") && diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			if !cmp.Equal(params, tc.expectedParams, cmp.AllowUnexported(kuberpultClientParameters{})) {
				t.Fatalf("expected args %v, got %v", tc.expectedParams, params)
			}

			if !cmp.Equal(other, tc.expectedOther, cmpopts.EquateEmpty()) {
				t.Fatalf("expected other args %v, got %v", tc.expectedOther, other)
			}
		})
	}
}
