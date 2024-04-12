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

Copyright 2023 freiheit.com*/

package cli_utils

import (
	"flag"
	"fmt"
	"testing"
)

func TestIsWellBehaved(t *testing.T) {
	type testCase struct {
		name string
		str  string
		res  bool
	}

	tcs := []testCase{
		{
			name: "okay string",
			str:  "totally-okay",
			res:  true,
		},
		{
			name: "comma",
			str:  "bad,comma",
			res:  false,
		},
		{
			name: "space",
			str:  "bad space",
			res:  false,
		},
		{
			name: "dot",
			str:  "something.yaml",
			res:  true,
		},
		{
			name: "slashes",
			str:  "path/to/something",
			res:  true,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			res := isWellBehavedString(tc.str)
			if res != tc.res {
				t.Fatalf("incorrect result for string %s, expected %v, got %v", tc.str, tc.res, res)
			}
		})
	}
}

func TestParseCommandLineArgs(t *testing.T) {
	type testCase struct {
		name             string
		argNames         []string
		cmdArgs          []string
		expectedValues   map[string]string
		expectedErrorMsg string // should not be flakey
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
			cmdArgs:          []string{"--arg"},
			expectedValues:   map[string]string{},
			expectedErrorMsg: "flag needs an argument: -arg",
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			fs := flag.NewFlagSet("flag set", flag.ContinueOnError)
			rsVars := make(map[string]*RepeatedString)

			for _, argName := range tc.argNames {
				rsVars[argName] = &RepeatedString{}
				fs.Var(rsVars[argName], argName, fmt.Sprintf("usage for %s", argName))
			}

			err := fs.Parse(tc.cmdArgs)
			if err != nil && err.Error() != tc.expectedErrorMsg {
				t.Fatalf("error messages mismatched, expected %s, received %s", tc.expectedErrorMsg, err.Error())
			}
			if err == nil && tc.expectedErrorMsg != "" {
				t.Fatalf("expected error %v, but no error was raised", tc.expectedErrorMsg)
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
