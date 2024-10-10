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

package releasetrain

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

func TestReadArgsReleaseTrain(t *testing.T) {
	type testCase struct {
		name            string
		args            []string
		expectedCmdArgs *ReleaseTrainCommandLineArguments
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
			args: []string{"--target-environment", "development", "--potato", "tomato"},
			expectedError: errMatcher{
				msg: "error while parsing command line arguments, error: flag provided but not defined: -potato",
			},
		},
		{
			name: "nothing provided",
			args: []string{},
			expectedError: errMatcher{
				msg: "the --target-environment arg must be set exactly once",
			},
		},
		{
			name: "only target environment is specified",
			args: []string{"--target-environment", "development"},
			expectedCmdArgs: &ReleaseTrainCommandLineArguments{
				targetEnvironment: cli_utils.RepeatedString{
					Values: []string{
						"development",
					},
				},
				useDexAuthentication: false,
			},
		},
		{
			name: "target environment and team are specified",
			args: []string{"--target-environment", "development", "--team", "test-team"},
			expectedCmdArgs: &ReleaseTrainCommandLineArguments{
				targetEnvironment: cli_utils.RepeatedString{
					Values: []string{
						"development",
					},
				},
				team: cli_utils.RepeatedString{
					Values: []string{
						"test-team",
					},
				},
				useDexAuthentication: false,
			},
		},
		{
			name: "target environment, team and use dex are specified",
			args: []string{"--target-environment", "development", "--team", "test-team", "--use_dex_auth"},
			expectedCmdArgs: &ReleaseTrainCommandLineArguments{
				targetEnvironment: cli_utils.RepeatedString{
					Values: []string{
						"development",
					},
				},
				team: cli_utils.RepeatedString{
					Values: []string{
						"test-team",
					},
				},
				useDexAuthentication: true,
			},
		},
		{
			name: "ci_link is specified",
			args: []string{"--target-environment", "development", "--team", "test-team", "--ci_link", "https://localhost:8000"},
			expectedCmdArgs: &ReleaseTrainCommandLineArguments{
				targetEnvironment: cli_utils.RepeatedString{
					Values: []string{
						"development",
					},
				},
				team: cli_utils.RepeatedString{
					Values: []string{
						"test-team",
					},
				},
				ciLink: cli_utils.RepeatedString{
					Values: []string{
						"https://localhost:8000",
					},
				},
				useDexAuthentication: false,
			},
		},
		{
			name: "--ci_link is invalid url",
			args: []string{"--target-environment", "development", "--team", "test-team", "--ci_link", "https//localhost:8000"},
			expectedError: errMatcher{
				msg: "provided invalid --ci_link value 'https//localhost:8000'",
			},
		},
		{
			name: "--ci_link is specified twice",
			args: []string{"--target-environment", "development", "--team", "test-team", "--ci_link", "https//localhost:8000", "--ci_link", "https//localhost:8000"},
			expectedError: errMatcher{
				msg: "the --ci_link arg must be set at most once",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmdArgs, err := readArgs(tc.args)
			// check errors
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(cmdArgs, tc.expectedCmdArgs, cmp.AllowUnexported(ReleaseTrainCommandLineArguments{})); diff != "" {
				t.Fatalf("expected args:\n  %v\n, got:\n  %v, diff:\n  %s\n", tc.expectedCmdArgs, cmdArgs, diff)
			}
		})
	}
}

func TestParseArgsReleaseTrainLock(t *testing.T) {
	type testCase struct {
		name           string
		cmdArgs        []string
		expectedParams ReleaseTrainParameters
		expectedError  error
	}

	tcs := []testCase{
		{
			name:    "target only",
			cmdArgs: []string{"--target-environment", "development"},
			expectedParams: ReleaseTrainParameters{
				Team:                 nil,
				TargetEnvironment:    "development",
				UseDexAuthentication: false,
			},
		},
		{
			name:    "target and team",
			cmdArgs: []string{"--target-environment", "development", "--team", "my-team"},
			expectedParams: ReleaseTrainParameters{
				Team:                 makeStringPointer("my-team"),
				TargetEnvironment:    "development",
				UseDexAuthentication: false,
			},
		},
		{
			name:    "target, team and ciLink",
			cmdArgs: []string{"--target-environment", "development", "--team", "my-team", "--ci_link", "https://localhost:8000"},
			expectedParams: ReleaseTrainParameters{
				Team:                 makeStringPointer("my-team"),
				TargetEnvironment:    "development",
				UseDexAuthentication: false,
				CiLink:               makeStringPointer("https://localhost:8000"),
			},
		},
		{
			name:    "target, team and use dex auth",
			cmdArgs: []string{"--target-environment", "development", "--team", "my-team", "--use_dex_auth"},
			expectedParams: ReleaseTrainParameters{
				Team:                 makeStringPointer("my-team"),
				TargetEnvironment:    "development",
				UseDexAuthentication: true,
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			params, err := ParseArgsReleaseTrain(tc.cmdArgs)
			// check errors
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			// check result
			if diff := cmp.Diff(&tc.expectedParams, params); diff != "" {
				t.Fatalf("expected args:\n  %v\n, got:\n  %v\n, diff:\n  %s\n", tc.expectedParams, params, diff)
			}
		})
	}
}

func makeStringPointer(str string) *string {
	var ret string
	ret = str
	return &ret
}
