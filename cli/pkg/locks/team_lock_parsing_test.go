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

func TestReadArgsTeamLock(t *testing.T) {
	type testCase struct {
		name            string
		args            []string
		expectedCmdArgs *CreateTeamLockCommandLineArguments
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
			args: []string{"--environment", "development", "--potato", "tomato"},
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
			name: "lockID is not provided",
			args: []string{"--environment", "development", "--team", "my-team", "--message", "\"my message\""},
			expectedError: errMatcher{
				msg: "the --lockID arg must be set exactly once",
			},
		},
		{
			name: "environment is not provided",
			args: []string{"--team", "my-team", "--lockID", "my-lock", "--message", "\"my message\""},
			expectedError: errMatcher{
				msg: "the --environment arg must be set exactly once",
			},
		},
		{
			name: "application is not provided",
			args: []string{"--environment", "development", "--lockID", "my-lock", "--message", "\"my message\""},
			expectedError: errMatcher{
				msg: "the --team arg must be set exactly once",
			},
		},
		{
			name: "only --lockID is properly provided but without --environment",
			args: []string{"--lockID", "potato"},
			expectedError: errMatcher{
				msg: "the --environment arg must be set exactly once",
			},
		},
		{
			name: "environment, lockID, application and message are are specified",
			args: []string{"--environment", "development", "--team", "my-team", "--lockID", "my-lock", "--message", "\"my message\""},
			expectedCmdArgs: &CreateTeamLockCommandLineArguments{
				environment: cli_utils.RepeatedString{
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
				team: cli_utils.RepeatedString{
					Values: []string{
						"my-team",
					},
				},
			},
		},
		{
			name: "environment, lockID, team and ciLink are specified",
			args: []string{"--environment", "development", "--team", "my-team", "--lockID", "my-lock", "--ci_link", "https://localhost:8000"},
			expectedCmdArgs: &CreateTeamLockCommandLineArguments{
				environment: cli_utils.RepeatedString{
					Values: []string{
						"development",
					},
				},
				lockId: cli_utils.RepeatedString{
					Values: []string{
						"my-lock",
					},
				},
				team: cli_utils.RepeatedString{
					Values: []string{
						"my-team",
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
			args: []string{"--environment", "development", "--team", "my-team", "--lockID", "my-lock", "--ci_link", "https//localhost:8000"},
			expectedError: errMatcher{
				msg: "provided invalid --ci_link value 'https//localhost:8000'",
			},
		},
		{
			name: "--ci_link is specified twice",
			args: []string{"--environment", "development", "--team", "my-team", "--lockID", "my-lock", "--ci_link", "https://localhost:8000", "--ci_link", "https://localhost:8000"},
			expectedError: errMatcher{
				msg: "the --ci_link arg must be set at most once",
			},
		},
		{
			name: "--suggested_lifetime is invalid",
			args: []string{"--environment", "development", "--team", "my-team", "--lockID", "my-lock", "--suggested_lifetime", "12"},
			expectedError: errMatcher{
				msg: "provided invalid --suggested_lifetime value '12'",
			},
		},
		{
			name: "--suggested_lifetime is specified twice",
			args: []string{"--environment", "development", "--team", "my-team", "--lockID", "my-lock", "--suggested_lifetime", "6d", "--suggested_lifetime", "4h"},
			expectedError: errMatcher{
				msg: "the --suggested_lifetime arg must be set at most once",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmdArgs, err := readCreateTeamLockArgs(tc.args)
			// check errors
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(cmdArgs, tc.expectedCmdArgs, cmp.AllowUnexported(CreateTeamLockCommandLineArguments{})); diff != "" {
				t.Fatalf("expected args:\n  %v\n, got:\n  %v, diff:\n  %s\n", tc.expectedCmdArgs, cmdArgs, diff)
			}
		})
	}
}

func TestReadArgsDeleteTeamLock(t *testing.T) {
	type testCase struct {
		name            string
		args            []string
		expectedCmdArgs *DeleteTeamLockCommandLineArguments
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
			args: []string{"--environment", "development", "--potato", "tomato"},
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
			name: "lockID is not provided",
			args: []string{"--environment", "development", "--team", "my-team"},
			expectedError: errMatcher{
				msg: "the --lockID arg must be set exactly once",
			},
		},
		{
			name: "environment is not provided",
			args: []string{"--team", "my-team", "--lockID", "my-lock"},
			expectedError: errMatcher{
				msg: "the --environment arg must be set exactly once",
			},
		},
		{
			name: "application is not provided",
			args: []string{"--environment", "development", "--lockID", "my-lock"},
			expectedError: errMatcher{
				msg: "the --team arg must be set exactly once",
			},
		},
		{
			name: "only --lockID is properly provided but without --environment",
			args: []string{"--lockID", "potato"},
			expectedError: errMatcher{
				msg: "the --environment arg must be set exactly once",
			},
		},
		{
			name: "delete does not accept message",
			args: []string{"--environment", "development", "--team", "my-team", "--lockID", "my-lock", "--message", "message"},
			expectedError: errMatcher{
				msg: "error while parsing command line arguments, error: flag provided but not defined: -message",
			},
		},
		{
			name: "environment, lockID and application are are specified",
			args: []string{"--environment", "development", "--team", "my-team", "--lockID", "my-lock"},
			expectedCmdArgs: &DeleteTeamLockCommandLineArguments{
				environment: cli_utils.RepeatedString{
					Values: []string{
						"development",
					},
				},
				lockId: cli_utils.RepeatedString{
					Values: []string{
						"my-lock",
					},
				},
				team: cli_utils.RepeatedString{
					Values: []string{
						"my-team",
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmdArgs, err := readDeleteTeamLockArgs(tc.args)
			// check errors
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(cmdArgs, tc.expectedCmdArgs, cmp.AllowUnexported(DeleteTeamLockCommandLineArguments{})); diff != "" {
				t.Fatalf("expected args:\n  %v\n, got:\n  %v, diff:\n  %s\n", tc.expectedCmdArgs, cmdArgs, diff)
			}
		})
	}
}

func TestParseArgsCreateTeamLock(t *testing.T) {
	type testCase struct {
		name           string
		cmdArgs        []string
		expectedParams LockParameters
		expectedError  error
	}

	tcs := []testCase{
		{
			name:    "with environment and lockID and message",
			cmdArgs: []string{"--environment", "development", "--team", "my-team", "--lockID", "my-lock", "--message", "message"},
			expectedParams: &CreateTeamLockParameters{
				Environment:          "development",
				LockId:               "my-lock",
				Message:              "message",
				Team:                 "my-team",
				UseDexAuthentication: true,
			},
		},
		{
			name:    "with environment, app and lockID and no message",
			cmdArgs: []string{"--environment", "development", "--team", "my-team", "--lockID", "my-lock"},
			expectedParams: &CreateTeamLockParameters{
				Environment:          "development",
				LockId:               "my-lock",
				Message:              "",
				Team:                 "my-team",
				UseDexAuthentication: true,
			},
		},
		{
			name:    "with environment and lockID and multi word message message",
			cmdArgs: []string{"--environment", "development", "--team", "my-team", "--lockID", "my-lock", "--message", "this is a very long message"},
			expectedParams: &CreateTeamLockParameters{
				Environment:          "development",
				LockId:               "my-lock",
				Team:                 "my-team",
				Message:              "this is a very long message",
				UseDexAuthentication: true,
			},
		},
		{
			name:    "with environment, lockID, team and ciLink",
			cmdArgs: []string{"--environment", "development", "--team", "my-team", "--lockID", "my-lock", "--ci_link", "https://localhost:8000"},
			expectedParams: &CreateTeamLockParameters{
				Environment:          "development",
				LockId:               "my-lock",
				Team:                 "my-team",
				UseDexAuthentication: true,
				CiLink:               strPtr("https://localhost:8000"),
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			params, err := ParseArgsCreateTeamLock(tc.cmdArgs)
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

func TestParseArgsDeleteTeamLock(t *testing.T) {
	type testCase struct {
		name           string
		cmdArgs        []string
		expectedParams LockParameters
		expectedError  error
	}

	tcs := []testCase{
		{
			name:    "with environment and lockID and team",
			cmdArgs: []string{"--environment", "development", "--team", "my-team", "--lockID", "my-lock"},
			expectedParams: &DeleteTeamLockParameters{
				Environment:          "development",
				LockId:               "my-lock",
				Team:                 "my-team",
				UseDexAuthentication: true,
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			params, err := ParseArgsDeleteTeamLock(tc.cmdArgs)
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
