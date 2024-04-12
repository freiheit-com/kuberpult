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
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
)

func TestReadArgs(t *testing.T) {
	type testCase struct {
		name             string
		args             []string
		expectedCmdArgs  *cmdArguments
		expectedErrorMsg string
	}

	tcs := []testCase{
		{
			name:             "nothing provided",
			args:             []string{},
			expectedErrorMsg: "the --application arg must be set exactly once",
		},
		{
			name: "only --application is properly provided but without --environment and --manifest",
			args: []string{"--application", "potato"},
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
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cmdArgs, err := readArgs(tc.args)
			if err != nil && err.Error() != tc.expectedErrorMsg {
				t.Fatalf("error messages mismatched, expected %s, received %s", tc.expectedErrorMsg, err.Error())
			}
			if err == nil && tc.expectedErrorMsg != "" {
				t.Fatalf("expected error %v, but no error was raised", tc.expectedErrorMsg)
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
			setup:   []fileCreation{},
			name:    "no enviornments and manifests",
			cmdArgs: []string{"--application", "potato"},
			expectedErrorMsg: "error while reading command line arguments, error: the args --enviornment and --manifest must be set at least once",
		},
		{
			setup: []fileCreation{
				{
					filename: "production-manifest.yaml",
					content: "some production manifest",
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
					content: "some development manifest",
				},
				{
					filename: "production-manifest.yaml",
					content: "some production manifest",
				},
			},
			name:    "with environment and manifest multiple times",
			cmdArgs: []string{"--application", "potato", "--environment", "development", "--manifest", "development-manifest.yaml", "--environment", "production", "--manifest", "production-manifest.yaml"},
			expectedParams: &ReleaseParameters{
				Application: "potato",
				Manifests: map[string]string{
					"development": "some development manifest",
					"production": "some production manifest",
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
			// test setup
			dir, err := os.MkdirTemp("", "kuberpult-cli-test-*")
			if err != nil {
				t.Fatalf("error encoutered while creating test directory, error: %v", err)
			}
			t.Cleanup(func() {
				os.RemoveAll(dir)
			})
			err = os.Chdir(dir)
			if err != nil {
				t.Fatalf("error encountered while changing working directory to %s, error: %v", dir, err)
			}
			
			for _, fc := range tc.setup {
				err = os.WriteFile(fc.filename, []byte(fc.content), 0664)
				if err != nil {
					t.Fatalf("error while creating file %s, error: %v", fc.filename, err)
				}
			}

			params, err := ParseArgs(tc.cmdArgs)
			// check errors
			if err != nil && err.Error() != tc.expectedErrorMsg {
				t.Fatalf("error messages mismatched, expected %s, received %s", tc.expectedErrorMsg, err.Error())
			}
			if err == nil && tc.expectedErrorMsg != "" {
				t.Fatalf("expected error %v, but no error was raised", tc.expectedErrorMsg)
			}

			// check result
			if !cmp.Equal(params, tc.expectedParams, cmp.AllowUnexported(cmdArguments{})) {
				t.Fatalf("expected args %v, got %v", tc.expectedParams, params)
			}
		})
	}
}
