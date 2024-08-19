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

func TestReadArgsAppLock(t *testing.T) {
	type testCase struct {
		name            string
		args            []string
		expectedCmdArgs *CreateAppLockCommandLineArguments
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
			args: []string{"--environment", "development", "--application", "my-app", "--message", "\"my message\""},
			expectedError: errMatcher{
				msg: "the --lockID arg must be set exactly once",
			},
		},
		{
			name: "environment is not provided",
			args: []string{"--application", "my-app", "--lockID", "my-lock", "--message", "\"my message\""},
			expectedError: errMatcher{
				msg: "the --environment arg must be set exactly once",
			},
		},
		{
			name: "application is not provided",
			args: []string{"--environment", "development", "--lockID", "my-lock", "--message", "\"my message\""},
			expectedError: errMatcher{
				msg: "the --application arg must be set exactly once",
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
			args: []string{"--environment", "development", "--application", "my-app", "--lockID", "my-lock", "--message", "\"my message\""},
			expectedCmdArgs: &CreateAppLockCommandLineArguments{
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
				application: cli_utils.RepeatedString{
					Values: []string{
						"my-app",
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmdArgs, err := readCreateAppLockArgs(tc.args)
			// check errors
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(cmdArgs, tc.expectedCmdArgs, cmp.AllowUnexported(CreateAppLockCommandLineArguments{})); diff != "" {
				t.Fatalf("expected args:\n  %v\n, got:\n  %v, diff:\n  %s\n", tc.expectedCmdArgs, cmdArgs, diff)
			}
		})
	}
}

func TestReadArgsDeleteAppLock(t *testing.T) {
	type testCase struct {
		name            string
		args            []string
		expectedCmdArgs *DeleteAppLockCommandLineArguments
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
			args: []string{"--environment", "development", "--application", "my-app"},
			expectedError: errMatcher{
				msg: "the --lockID arg must be set exactly once",
			},
		},
		{
			name: "environment is not provided",
			args: []string{"--application", "my-app", "--lockID", "my-lock"},
			expectedError: errMatcher{
				msg: "the --environment arg must be set exactly once",
			},
		},
		{
			name: "application is not provided",
			args: []string{"--environment", "development", "--lockID", "my-lock"},
			expectedError: errMatcher{
				msg: "the --application arg must be set exactly once",
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
			name: "message is not accepted by delete",
			args: []string{"--environment", "development", "--application", "my-app", "--lockID", "my-lock", "--message", "\"my message\""},
			expectedError: errMatcher{
				msg: "error while parsing command line arguments, error: flag provided but not defined: -message",
			},
		},
		{
			name: "environment, lockID, application and message are are specified",
			args: []string{"--environment", "development", "--application", "my-app", "--lockID", "my-lock"},
			expectedCmdArgs: &DeleteAppLockCommandLineArguments{
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
				application: cli_utils.RepeatedString{
					Values: []string{
						"my-app",
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmdArgs, err := readDeleteAppLockArgs(tc.args)
			// check errors
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(cmdArgs, tc.expectedCmdArgs, cmp.AllowUnexported(DeleteAppLockCommandLineArguments{})); diff != "" {
				t.Fatalf("expected args:\n  %v\n, got:\n  %v, diff:\n  %s\n", tc.expectedCmdArgs, cmdArgs, diff)
			}
		})
	}
}

func TestParseArgsCreateAppLock(t *testing.T) {
	type testCase struct {
		name           string
		cmdArgs        []string
		expectedParams LockParameters
		expectedError  error
	}

	tcs := []testCase{
		{
			name:    "with environment and lockID and message",
			cmdArgs: []string{"--environment", "development", "--application", "my-app", "--lockID", "my-lock", "--message", "message"},
			expectedParams: &CreateAppLockParameters{
				Environment: "development",
				LockId:      "my-lock",
				Message:     "message",
				Application: "my-app",
			},
		},
		{
			name:    "with environment, app and lockID and no message",
			cmdArgs: []string{"--environment", "development", "--application", "my-app", "--lockID", "my-lock"},
			expectedParams: &CreateAppLockParameters{
				Environment: "development",
				LockId:      "my-lock",
				Message:     "",
				Application: "my-app",
			},
		},

		{
			name:    "with environment and lockID and multi word message message",
			cmdArgs: []string{"--environment", "development", "--application", "my-app", "--lockID", "my-lock", "--message", "this is a very long message"},
			expectedParams: &CreateAppLockParameters{
				Environment: "development",
				LockId:      "my-lock",
				Application: "my-app",
				Message:     "this is a very long message",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			params, err := ParseArgsCreateAppLock(tc.cmdArgs)
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

func TestParseArgsDeleteAppLock(t *testing.T) {
	type testCase struct {
		name           string
		cmdArgs        []string
		expectedParams LockParameters
		expectedError  error
	}

	tcs := []testCase{
		{
			name:    "with environment, lockID and app",
			cmdArgs: []string{"--environment", "development", "--application", "my-app", "--lockID", "my-lock"},
			expectedParams: &DeleteAppLockParameters{
				Environment: "development",
				LockId:      "my-lock",
				Application: "my-app",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			params, err := ParseArgsDeleteAppLock(tc.cmdArgs)
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
