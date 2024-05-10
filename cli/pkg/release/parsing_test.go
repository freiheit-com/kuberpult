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

package release

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
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

func TestReadArgs(t *testing.T) {
	type testCase struct {
		name             string
		args             []string
		expectedCmdArgs  *cmdArguments
		expectedErrorMsg string
	}

	tcs := []testCase{
		{
			name: "some unrecognized positional arguments",
			args: []string{"potato", "tomato"},
			expectedErrorMsg: "these arguments are not recognized: \"potato tomato\"",
		},
		{
			name: "some flags that don't exist",
			args: []string{"--potato", "tomato"},
			expectedErrorMsg: "error while parsing command line arguments, error: flag provided but not defined: -potato",
		},
		{
			name:             "nothing provided",
			args:             []string{},
			expectedErrorMsg: "the --application arg must be set exactly once",
		},
		{
			name:             "only --application is properly provided but without --environment and --manifest",
			args:             []string{"--application", "potato"},
			expectedErrorMsg: "the args --enviornment and --manifest must be set at least once",
		},
		{
			name:             "--application has some improper value",
			args:             []string{"--application", "something,not,allowed"},
			expectedErrorMsg: "error while parsing command line arguments, error: invalid value \"something,not,allowed\" for flag -application: the string \"something,not,allowed\" may not be used as a flag value, all values must match the regex ^[a-zA-Z0-9_\\./-]+$",
		},
		{
			name:             "--environment is specified without --manifest",
			args:             []string{"--application", "potato", "--environment", "production"},
			expectedErrorMsg: "all --environment args must have a --manifest arg set immediately afterwards",
		},
		{
			name: "--environment and --manifest are specified",
			args: []string{"--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml"},
			expectedCmdArgs: &cmdArguments{
				application: cli_utils.RepeatedString{
					Values: []string{
						"potato",
					},
				},
				environments: cli_utils.RepeatedString{
					Values: []string{
						"production",
					},
				},
				manifests: cli_utils.RepeatedString{
					Values: []string{
						"manifest-file.yaml",
					},
				},
			},
		},
		{
			name:             "--environment and --manifest are specified but with an extra --manifest",
			args:             []string{"--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--manifest", "something-else.yaml"},
			expectedErrorMsg: "all --manifest args must be set immediately after an --environment arg",
		},
		{
			name: "--team is specified",
			args: []string{"--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team"},
			expectedCmdArgs: &cmdArguments{
				application: cli_utils.RepeatedString{
					Values: []string{
						"potato",
					},
				},
				environments: cli_utils.RepeatedString{
					Values: []string{
						"production",
					},
				},
				manifests: cli_utils.RepeatedString{
					Values: []string{
						"manifest-file.yaml",
					},
				},
				team: cli_utils.RepeatedString{
					Values: []string{
						"potato-team",
					},
				},
			},
		},
		{
			name: "--team is specified twice",
			args: []string{"--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--team", "tomato-team"},
			expectedErrorMsg: "the --team arg must be set at most once",
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			
			cmdArgs, err := parseArgs(tc.args)
			// check errors
			if diff := cmp.Diff(errMatcher{tc.expectedErrorMsg}, err, cmpopts.EquateErrors()); !(err == nil && tc.expectedErrorMsg == "") && diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			if !cmp.Equal(cmdArgs, tc.expectedCmdArgs, cmp.AllowUnexported(cmdArguments{})) {
				t.Fatalf("expected args %v, got %v", tc.expectedCmdArgs, cmdArgs)
			}
		})
	}
}

func TestParseArgs(t *testing.T) {
	type fileCreation struct {
		filename string
		content  string
	}
	type testCase struct {
		setup            []fileCreation
		name             string
		cmdArgs          []string
		expectedParams   *ReleaseParameters
		expectedErrorMsg string
	}

	tcs := []testCase{
		{
			setup:            []fileCreation{},
			name:             "no enviornments and manifests",
			cmdArgs:          []string{"--application", "potato"},
			expectedErrorMsg: "error while reading command line arguments, error: the args --enviornment and --manifest must be set at least once",
		},
		{
			setup: []fileCreation{
				{
					filename: "production-manifest.yaml",
					content:  "some production manifest",
				},
			},
			name:    "with environment and manifest",
			cmdArgs: []string{"--application", "potato", "--environment", "production", "--manifest", "production-manifest.yaml"},
			expectedParams: &ReleaseParameters{
				Application: "potato",
				Manifests: map[string]string{
					"production": "some production manifest",
				},
			},
		},
		{
			setup: []fileCreation{
				{
					filename: "development-manifest.yaml",
					content:  "some development manifest",
				},
				{
					filename: "production-manifest.yaml",
					content:  "some production manifest",
				},
			},
			name:    "with environment and manifest multiple times",
			cmdArgs: []string{"--application", "potato", "--environment", "development", "--manifest", "development-manifest.yaml", "--environment", "production", "--manifest", "production-manifest.yaml"},
			expectedParams: &ReleaseParameters{
				Application: "potato",
				Manifests: map[string]string{
					"development": "some development manifest",
					"production":  "some production manifest",
				},
			},
		},
		{
			name:             "some error occurs in argument parsing",
			cmdArgs:          []string{"--application"},
			expectedParams:   nil,
			expectedErrorMsg: "error while reading command line arguments, error: error while parsing command line arguments, error: flag needs an argument: -application",
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			
			// test setup
			dir, err := os.MkdirTemp("", "kuberpult-cli-test-*")
			if err != nil {
				t.Fatalf("error encoutered while creating test directory, error: %v", err)
			}
			t.Cleanup(func() {
				os.RemoveAll(dir)
			})

			for i, _ := range tc.setup {
				tc.setup[i].filename = filepath.Join(dir, tc.setup[i].filename)

			}
			for i, arg := range tc.cmdArgs {
				if arg == "--manifest" {
					tc.cmdArgs[i+1] = filepath.Join(dir, tc.cmdArgs[i+1])
				}
			}

			for _, fc := range tc.setup {
				err = os.WriteFile(fc.filename, []byte(fc.content), 0664)
				if err != nil {
					t.Fatalf("error while creating file %s, error: %v", fc.filename, err)
				}
			}

			params, err := ProcessArgs(tc.cmdArgs)
			// check errors
			if diff := cmp.Diff(errMatcher{tc.expectedErrorMsg}, err, cmpopts.EquateErrors()); !(err == nil && tc.expectedErrorMsg == "") && diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			// check result
			if !cmp.Equal(params, tc.expectedParams, cmp.AllowUnexported(cmdArguments{})) {
				t.Fatalf("expected args %v, got %v", tc.expectedParams, params)
			}
		})
	}
}
