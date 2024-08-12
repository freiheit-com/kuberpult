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

func ptrStr(s string) *string {
	return &s
}

func TestParseArgs(t *testing.T) {
	type TestCase struct {
		name           string
		cmdArgs        string
		expectedParams *kuberpultClientParameters
		expectedOther  []string
		expectedError  error
	}

	tcs := []TestCase{
		{
			name:    "nothing provided",
			cmdArgs: "",
			expectedError: errMatcher{
				msg: "error while creating kuberpult client parameters, error: the --url arg must be set exactly once",
			},
		},
		{
			name:    "something other than --url is provided",
			cmdArgs: "--potato tomato",
			expectedError: errMatcher{
				msg: "error while parsing command line arguments, error: error while reading command line arguments, error: flag provided but not defined: -potato",
			},
		},
		{
			name:    "only --url is provided",
			cmdArgs: "--url something.somewhere",
			expectedParams: &kuberpultClientParameters{
				url:     "something.somewhere",
				retries: DefaultRetries,
			},
		},
		{
			name:    "--url is provided along with some other stuff",
			cmdArgs: "--url something.somewhere --potato tomato",
			expectedError: errMatcher{
				msg: "error while parsing command line arguments, error: error while reading command line arguments, error: flag provided but not defined: -potato",
			},
		},
		{
			name:    "--url is provided twice",
			cmdArgs: "--url something.somewhere --url somethingelse.somewhere",
			expectedError: errMatcher{
				msg: "error while creating kuberpult client parameters, error: the --url arg must be set exactly once",
			},
		},
		{
			name:    "--url is provided with some tail",
			cmdArgs: "--url something.somewhere potato --tomato",
			expectedParams: &kuberpultClientParameters{
				url:     "something.somewhere",
				retries: DefaultRetries,
			},
			expectedOther: []string{"potato", "--tomato"},
		},
		{
			name:    "--url and --author_name are provided with some tail",
			cmdArgs: "--url something.somewhere --author_name john subcommand --arg1 val1 etc etc",
			expectedParams: &kuberpultClientParameters{
				url:        "something.somewhere",
				authorName: ptrStr("john"),
				retries:    DefaultRetries,
			},
			expectedOther: []string{"subcommand", "--arg1", "val1", "etc", "etc"},
		},
		{
			name:    "--author_name is provided twice",
			cmdArgs: "--url something.somewhere --author_name john --author_name joseph subcommand --arg1 val1 etc etc",
			expectedError: errMatcher{
				msg: "error while creating kuberpult client parameters, error: the --author_name arg must be set at most once",
			},
		},
		{
			name:    "--url and --author_email are provided with some tail",
			cmdArgs: "--url something.somewhere --author_email john subcommand --arg1 val1 etc etc",
			expectedParams: &kuberpultClientParameters{
				url:         "something.somewhere",
				authorEmail: ptrStr("john"),
				retries:     DefaultRetries,
			},
			expectedOther: []string{"subcommand", "--arg1", "val1", "etc", "etc"},
		},
		{
			name:    "--author_email is provided twice",
			cmdArgs: "--url something.somewhere --author_email john --author_email joseph subcommand --arg1 val1 etc etc",
			expectedError: errMatcher{
				msg: "error while creating kuberpult client parameters, error: the --author_email arg must be set at most once",
			},
		},
		{
			name:    "default retries",
			cmdArgs: "--url something.somewhere potato --tomato",
			expectedParams: &kuberpultClientParameters{
				url:     "something.somewhere",
				retries: DefaultRetries,
			},
			expectedOther: []string{"potato", "--tomato"},
		},
		{
			name:    "default retries",
			cmdArgs: "--url something.somewhere --retries 10 potato --tomato",
			expectedParams: &kuberpultClientParameters{
				url:     "something.somewhere",
				retries: 10,
			},
			expectedOther: []string{"potato", "--tomato"},
		},
		{
			name:    "--retries is negative",
			cmdArgs: "--url something.somewhere --retries -1 --author_email john  subcommand --arg1 val1 etc etc",
			expectedError: errMatcher{
				msg: "error while creating kuberpult client parameters, error: --retries arg value must be positive",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			params, other, err := parseArgs(strings.Split(tc.cmdArgs, " "))
			// check errors
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(params, tc.expectedParams, cmp.AllowUnexported(kuberpultClientParameters{})); diff != "" {
				t.Fatalf("expected args:\n  %v\ngot:\n  %v\ndiff:\n  %s\n", tc.expectedParams, params, diff)
			}

			if diff := cmp.Diff(other, tc.expectedOther, cmpopts.EquateEmpty()); diff != "" {
				t.Fatalf("expected other args:\n  %v\ngot:\n  %v\ndiff:\n  %s\n", tc.expectedOther, other, diff)
			}
		})
	}
}
