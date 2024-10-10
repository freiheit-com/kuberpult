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

package locks

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
)

func TestReadGroupLockArgs(t *testing.T) {
	type testCase struct {
		name            string
		args            []string
		expectedCmdArgs *CreateEnvGroupLockCommandLineArguments
		expectedError   error
	}

	tcs := []testCase{
		{
			name: "some unrecognized positional arguments",
			args: []string{"potato", "tomato"},
			expectedError: errMatcher{
				msg: "these arguments are not recognized: \"potato tomato\"",
			},
		},
		{
			name: "some flags that don't exist",
			args: []string{"--environment-group", "development", "--potato", "tomato"},
			expectedError: errMatcher{
				msg: "error while parsing command line arguments, error: flag provided but not defined: -potato",
			},
		},
		{
			name: "nothing provided",
			args: []string{},
			expectedError: errMatcher{
				msg: "the --lockID arg must be set exactly once",
			},
		},
		{
			name: "only --environment-group is properly provided but without --lockID",
			args: []string{"--environment-group", "potato"},
			expectedError: errMatcher{
				msg: "the --lockID arg must be set exactly once",
			},
		},
		{
			name: "only --lockID is properly provided but without --environment-group",
			args: []string{"--lockID", "potato"},
			expectedError: errMatcher{
				msg: "the --environment-group arg must be set exactly once",
			},
		},
		{
			name: "--environment-group, lockID and message are are specified",
			args: []string{"--environment-group", "development", "--lockID", "my-lock", "--message", "\"my message\""},
			expectedCmdArgs: &CreateEnvGroupLockCommandLineArguments{
				environmentGroup: cli_utils.RepeatedString{
					Values: []string{
						"development",
					},
				},
				lockId: cli_utils.RepeatedString{
					Values: []string{
						"my-lock",
					},
				},
				message: cli_utils.RepeatedString{
					Values: []string{
						"\"my message\"",
					},
				},
			},
		},
		{
			name: "--environment-group, lockID are are specified, but not the message",
			args: []string{"--environment-group", "development", "--lockID", "my-lock"},
			expectedCmdArgs: &CreateEnvGroupLockCommandLineArguments{
				environmentGroup: cli_utils.RepeatedString{
					Values: []string{
						"development",
					},
				},
				lockId: cli_utils.RepeatedString{
					Values: []string{
						"my-lock",
					},
				},
				message: cli_utils.RepeatedString{},
			},
		},
		{
			name: "environment group, lockID and ciLink are specified",
			args: []string{"--environment-group", "development", "--lockID", "my-lock", "--ci_link", "https://localhost:8000"},
			expectedCmdArgs: &CreateEnvGroupLockCommandLineArguments{
				environmentGroup: cli_utils.RepeatedString{
					Values: []string{
						"development",
					},
				},
				lockId: cli_utils.RepeatedString{
					Values: []string{
						"my-lock",
					},
				},
				ciLink: cli_utils.RepeatedString{
					Values: []string{
						"https://localhost:8000",
					},
				},
			},
		},
		{
			name: "--ci_link is invalid url",
			args: []string{"--environment-group", "development", "--lockID", "my-lock", "--ci_link", "https//localhost:8000"},
			expectedError: errMatcher{
				msg: "provided invalid --ci_link value 'https//localhost:8000'",
			},
		},
		{
			name: "--ci_link is specified twice",
			args: []string{"--environment-group", "development", "--lockID", "my-lock", "--ci_link", "https//localhost:8000", "--ci_link", "https//localhost:8000"},
			expectedError: errMatcher{
				msg: "the --ci_link arg must be set at most once",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmdArgs, err := readCreateGroupLockArgs(tc.args)
			// check errors
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(cmdArgs, tc.expectedCmdArgs, cmp.AllowUnexported(CreateEnvGroupLockCommandLineArguments{})); diff != "" {
				t.Fatalf("expected args:\n  %v\n, got:\n  %v, diff:\n  %s\n", tc.expectedCmdArgs, cmdArgs, diff)
			}
		})
	}
}

func TestReadDeleteGroupLockArgs(t *testing.T) {
	type testCase struct {
		name            string
		args            []string
		expectedCmdArgs *DeleteEnvGroupLockCommandLineArguments
		expectedError   error
	}

	tcs := []testCase{
		{
			name: "some unrecognized positional arguments",
			args: []string{"potato", "tomato"},
			expectedError: errMatcher{
				msg: "these arguments are not recognized: \"potato tomato\"",
			},
		},
		{
			name: "some flags that don't exist",
			args: []string{"--environment-group", "development", "--potato", "tomato"},
			expectedError: errMatcher{
				msg: "error while parsing command line arguments, error: flag provided but not defined: -potato",
			},
		},
		{
			name: "nothing provided",
			args: []string{},
			expectedError: errMatcher{
				msg: "the --lockID arg must be set exactly once",
			},
		},
		{
			name: "only --environment-group is properly provided but without --lockID",
			args: []string{"--environment-group", "potato"},
			expectedError: errMatcher{
				msg: "the --lockID arg must be set exactly once",
			},
		},
		{
			name: "only --lockID is properly provided but without --environment-group",
			args: []string{"--lockID", "potato"},
			expectedError: errMatcher{
				msg: "the --environment-group arg must be set exactly once",
			},
		},
		{
			name: "--environment-group and lockID",
			args: []string{"--environment-group", "development", "--lockID", "my-lock"},
			expectedCmdArgs: &DeleteEnvGroupLockCommandLineArguments{
				environmentGroup: cli_utils.RepeatedString{
					Values: []string{
						"development",
					},
				},
				lockId: cli_utils.RepeatedString{
					Values: []string{
						"my-lock",
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmdArgs, err := readDeleteGroupLockArgs(tc.args)
			// check errors
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(cmdArgs, tc.expectedCmdArgs, cmp.AllowUnexported(DeleteEnvGroupLockCommandLineArguments{})); diff != "" {
				t.Fatalf("expected args:\n  %v\n, got:\n  %v, diff:\n  %s\n", tc.expectedCmdArgs, cmdArgs, diff)
			}
		})
	}
}

func TestParseEnvGroupArgs(t *testing.T) {
	type testCase struct {
		name           string
		cmdArgs        []string
		expectedParams LockParameters
		expectedError  error
	}

	tcs := []testCase{
		{
			name:    "with environment and lockID and message",
			cmdArgs: []string{"--environment-group", "development", "--lockID", "my-lock", "--message", "message"},
			expectedParams: &CreateEnvironmentGroupLockParameters{
				EnvironmentGroup: "development",
				LockId:           "my-lock",
				Message:          "message",
			},
		},
		{
			name:    "with environment and lockID and no message",
			cmdArgs: []string{"--environment-group", "development", "--lockID", "my-lock"},
			expectedParams: &CreateEnvironmentGroupLockParameters{
				EnvironmentGroup: "development",
				LockId:           "my-lock",
				Message:          "",
			},
		},

		{
			name:    "with environment and lockID and multi word message message",
			cmdArgs: []string{"--environment-group", "development", "--lockID", "my-lock", "--message", "this is a very long message"},
			expectedParams: &CreateEnvironmentGroupLockParameters{
				EnvironmentGroup: "development",
				LockId:           "my-lock",
				Message:          "this is a very long message",
			},
		},
		{
			name:    "with environment, lockID and ciLink",
			cmdArgs: []string{"--environment-group", "development", "--lockID", "my-lock", "--ci_link", "https://localhost:8000"},
			expectedParams: &CreateEnvironmentGroupLockParameters{
				EnvironmentGroup: "development",
				LockId:           "my-lock",
				CiLink:           strPtr("https://localhost:8000"),
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			params, err := ParseArgsCreateGroupLock(tc.cmdArgs)
			// check errors
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			// check result
			if diff := cmp.Diff(tc.expectedParams, params); diff != "" {
				t.Fatalf("expected args:\n  %v\n, got:\n  %v\n, diff:\n  %s\n", tc.expectedParams, params, diff)
			}
		})
	}
}

func TestParseDeleteEnvGroupArgs(t *testing.T) {
	type testCase struct {
		name           string
		cmdArgs        []string
		expectedParams LockParameters
		expectedError  error
	}

	tcs := []testCase{
		{
			name:    "with environment groups and lockID",
			cmdArgs: []string{"--environment-group", "development", "--lockID", "my-lock"},
			expectedParams: &DeleteEnvironmentGroupLockParameters{
				EnvironmentGroup: "development",
				LockId:           "my-lock",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			params, err := ParseArgsDeleteGroupLock(tc.cmdArgs)
			// check errors
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			// check result
			if diff := cmp.Diff(tc.expectedParams, params); diff != "" {
				t.Fatalf("expected args:\n  %v\n, got:\n  %v\n, diff:\n  %s\n", tc.expectedParams, params, diff)
			}
		})
	}
}
