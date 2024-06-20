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

package cli_utils

import (
	"flag"
	"fmt"
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

func TestParseCommandLineArgsString(t *testing.T) {
	type testCase struct {
		name           string
		argNames       []string
		cmdArgs        []string
		expectedValues map[string]string
		expectedError  error
	}

	tcs := []testCase{
		{
			name: "one arg set",
			argNames: []string{
				"arg",
			},
			cmdArgs: []string{"--arg", "value"},
			expectedValues: map[string]string{
				"arg": "value",
			},
		},
		{
			name: "one arg set multiple time",
			argNames: []string{
				"arg",
			},
			cmdArgs: []string{"--arg", "value1", "--arg", "value2"},
			expectedValues: map[string]string{
				"arg": "value1,value2",
			},
		},
		{
			name: "arg specified but value missing",
			argNames: []string{
				"arg",
			},
			cmdArgs:        []string{"--arg"},
			expectedValues: map[string]string{},
			expectedError: errMatcher{
				msg: "flag needs an argument: -arg",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fs := flag.NewFlagSet("flag set", flag.ContinueOnError)
			rsVars := make(map[string]*RepeatedString)

			for _, argName := range tc.argNames {
				rsVars[argName] = &RepeatedString{}
				fs.Var(rsVars[argName], argName, fmt.Sprintf("usage for %s", argName))
			}

			err := fs.Parse(tc.cmdArgs)
			// check errors
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			for argName, expectedValue := range tc.expectedValues {
				value := rsVars[argName].String()
				if expectedValue != value {
					t.Fatalf("arg values mismatch for arg %s, expected %s, received %s", argName, expectedValue, value)
				}
			}
		})
	}
}

func TestParseCommandLineArgsInt(t *testing.T) {
	type testCase struct {
		name           string
		argNames       []string
		cmdArgs        []string
		expectedValues map[string]string
		expectedError  error
	}

	tcs := []testCase{
		{
			name: "one arg set",
			argNames: []string{
				"arg",
			},
			cmdArgs: []string{"--arg", "1234"},
			expectedValues: map[string]string{
				"arg": "1234",
			},
		},
		{
			name: "one arg set multiple time",
			argNames: []string{
				"arg",
			},
			cmdArgs: []string{"--arg", "1234", "--arg", "5678"},
			expectedValues: map[string]string{
				"arg": "1234,5678",
			},
		},
		{
			name: "arg specified but value missing",
			argNames: []string{
				"arg",
			},
			cmdArgs: []string{"--arg"},
			expectedError: errMatcher{
				msg: "flag needs an argument: -arg",
			},
		},
		{
			name: "arg specified but with invalid value",
			argNames: []string{
				"arg",
			},
			cmdArgs: []string{"--arg", "potato"},
			expectedError: errMatcher{
				msg: "invalid value \"potato\" for flag -arg: the provided value \"potato\" is not an integer",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fs := flag.NewFlagSet("flag set", flag.ContinueOnError)
			rsVars := make(map[string]*RepeatedInt)

			for _, argName := range tc.argNames {
				rsVars[argName] = &RepeatedInt{}
				fs.Var(rsVars[argName], argName, fmt.Sprintf("usage for %s", argName))
			}

			err := fs.Parse(tc.cmdArgs)
			// check errors
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			for argName, expectedValue := range tc.expectedValues {
				value := rsVars[argName].String()
				if expectedValue != value {
					t.Fatalf("arg values mismatch for arg %s, expected %s, received %s", argName, expectedValue, value)
				}
			}
		})
	}
}
