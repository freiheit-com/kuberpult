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
		name            string
		args            []string
		expectedCmdArgs *CreateEnvLockCommandLineArguments
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
			name: "only --environment is properly provided but without --lockID",
			args: []string{"--environment", "potato"},
			expectedError: errMatcher{
				msg: "the --lockID arg must be set exactly once",
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
			name: "--environment, lockID and message are are specified",
			args: []string{"--environment", "development", "--lockID", "my-lock", "--message", "\"my message\""},
			expectedCmdArgs: &CreateEnvLockCommandLineArguments{
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
			},
		},
		{
			name: "--environment, lockID are are specified, but not the message",
			args: []string{"--environment", "development", "--lockID", "my-lock"},
			expectedCmdArgs: &CreateEnvLockCommandLineArguments{
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
				message: cli_utils.RepeatedString{},
			},
		},
		{
			name: "environment, lockID and ciLink are specified",
			args: []string{"--environment", "development", "--lockID", "my-lock", "--ci_link", "https://localhost:8000"},
			expectedCmdArgs: &CreateEnvLockCommandLineArguments{
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
				ciLink: cli_utils.RepeatedString{
					Values: []string{
						"https://localhost:8000",
					},
				},
			},
		},
		{
			name: "--ci_link is invalid url",
			args: []string{"--environment", "development", "--lockID", "my-lock", "--ci_link", "https//localhost:8000"},
			expectedError: errMatcher{
				msg: "provided invalid --ci_link value 'https//localhost:8000'",
			},
		},
		{
			name: "--ci_link is specified twice",
			args: []string{"--environment", "development", "--lockID", "my-lock", "--ci_link", "https://localhost:8000", "--ci_link", "https://localhost:8000"},
			expectedError: errMatcher{
				msg: "the --ci_link arg must be set at most once",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmdArgs, err := readCreateEnvLockArgs(tc.args)
			// check errors
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(cmdArgs, tc.expectedCmdArgs, cmp.AllowUnexported(CreateEnvLockCommandLineArguments{})); diff != "" {
				t.Fatalf("expected args:\n  %v\n, got:\n  %v, diff:\n  %s\n", tc.expectedCmdArgs, cmdArgs, diff)
			}
		})
	}
}

func TestReadDeleteEnvLockArgs(t *testing.T) {
	type testCase struct {
		name            string
		args            []string
		expectedCmdArgs *DeleteEnvLockCommandLineArguments
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
			name: "only --environment is properly provided but without --lockID",
			args: []string{"--environment", "potato"},
			expectedError: errMatcher{
				msg: "the --lockID arg must be set exactly once",
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
			name: "--message is not accepted",
			args: []string{"--environment", "development", "--lockID", "my-lock", "--message", "\"my message\""},
			expectedError: errMatcher{
				msg: "error while parsing command line arguments, error: flag provided but not defined: -message",
			},
		},
		{
			name: "--environment, lockID are are specified, but not the message",
			args: []string{"--environment", "development", "--lockID", "my-lock"},
			expectedCmdArgs: &DeleteEnvLockCommandLineArguments{
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
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmdArgs, err := readDeleteEnvLockArgs(tc.args)
			// check errors
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(cmdArgs, tc.expectedCmdArgs, cmp.AllowUnexported(DeleteEnvLockCommandLineArguments{})); diff != "" {
				t.Fatalf("expected args:\n  %v\n, got:\n  %v, diff:\n  %s\n", tc.expectedCmdArgs, cmdArgs, diff)
			}
		})
	}
}

func TestParseArgs(t *testing.T) {
	type testCase struct {
		name           string
		cmdArgs        []string
		expectedParams LockParameters
		expectedError  error
	}

	tcs := []testCase{
		{
			name:    "with environment and lockID and message",
			cmdArgs: []string{"--environment", "development", "--lockID", "my-lock", "--message", "message"},
			expectedParams: &CreateEnvironmentLockParameters{
				Environment: "development",
				LockId:      "my-lock",
				Message:     "message",
			},
		},
		{
			name:    "with environment and lockID and no message",
			cmdArgs: []string{"--environment", "development", "--lockID", "my-lock"},
			expectedParams: &CreateEnvironmentLockParameters{
				Environment: "development",
				LockId:      "my-lock",
				Message:     "",
			},
		},

		{
			name:    "with environment and lockID and multi word message message",
			cmdArgs: []string{"--environment", "development", "--lockID", "my-lock", "--message", "this is a very long message"},
			expectedParams: &CreateEnvironmentLockParameters{
				Environment: "development",
				LockId:      "my-lock",
				Message:     "this is a very long message",
			},
		},
		{
			name:    "with environment, lockID and ciLink",
			cmdArgs: []string{"--environment", "development", "--lockID", "my-lock", "--ci_link", "https://localhost:8000"},
			expectedParams: &CreateEnvironmentLockParameters{
				Environment: "development",
				LockId:      "my-lock",
				CiLink:      strPtr("https://localhost:8000"),
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			params, err := ParseArgsCreateEnvironmentLock(tc.cmdArgs)
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

func TestParseArgsDeleteEnvLock(t *testing.T) {
	type testCase struct {
		name           string
		cmdArgs        []string
		expectedParams LockParameters
		expectedError  error
	}

	tcs := []testCase{
		{
			name:    "with environment and lockID and message",
			cmdArgs: []string{"--environment", "development", "--lockID", "my-lock"},
			expectedParams: &DeleteEnvironmentLockParameters{
				Environment:          "development",
				LockId:               "my-lock",
				UseDexAuthentication: false,
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			params, err := ParseArgsDeleteEnvironmentLock(tc.cmdArgs)
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
