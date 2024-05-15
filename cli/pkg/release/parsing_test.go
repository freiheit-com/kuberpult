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
		expectedCmdArgs  *commandLineArguments
		expectedErrorMsg string
	}

	tcs := []testCase{
		{
			name:             "some unrecognized positional arguments",
			args:             []string{"--skip_signatures", "potato", "tomato"},
			expectedErrorMsg: "these arguments are not recognized: \"potato tomato\"",
		},
		{
			name:             "some flags that don't exist",
			args:             []string{"--skip_signatures", "--potato", "tomato"},
			expectedErrorMsg: "error while parsing command line arguments, error: flag provided but not defined: -potato",
		},
		{
			name:             "nothing provided",
			args:             []string{"--skip_signatures"},
			expectedErrorMsg: "the --application arg must be set exactly once",
		},
		{
			name:             "only --application is properly provided but without --environment and --manifest",
			args:             []string{"--skip_signatures", "--application", "potato"},
			expectedErrorMsg: "the args --enviornment and --manifest must be set at least once",
		},
		{
			name:             "--environment is specified without --manifest",
			args:             []string{"--skip_signatures", "--application", "potato", "--environment", "production"},
			expectedErrorMsg: "all --environment args must have a --manifest arg set immediately afterwards",
		},
		{
			name: "--environment and --manifest are specified",
			args: []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml"},
			expectedCmdArgs: &commandLineArguments{
				skipSignatures: true,
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
			name: "--environment and --manifest are specified twice",
			args: []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest1-file.yaml", "--environment", "development", "--manifest", "manifest2-file.yaml"},
			expectedCmdArgs: &commandLineArguments{
				skipSignatures: true,
				application: cli_utils.RepeatedString{
					Values: []string{
						"potato",
					},
				},
				environments: cli_utils.RepeatedString{
					Values: []string{
						"production",
						"development",
					},
				},
				manifests: cli_utils.RepeatedString{
					Values: []string{
						"manifest1-file.yaml",
						"manifest2-file.yaml",
					},
				},
			},
		},
		{
			name:             "--environment and --manifest are specified but with an extra --manifest",
			args:             []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--manifest", "something-else.yaml"},
			expectedErrorMsg: "all --manifest args must be set immediately after an --environment arg",
		},
		{
			name: "signatures not skipped, --environment and --manifest and --signature are specified",
			args: []string{"--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--signature", "signature-file.gpg"},
			expectedCmdArgs: &commandLineArguments{
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
				signatures: cli_utils.RepeatedString{
					Values: []string{
						"signature-file.gpg",
					},
				},
			},
		},
		{
			name:             "signatures skipped, --environment and --manifest and --signature are specified",
			args:             []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--signature", "signature-file.gpg"},
			expectedErrorMsg: "--signature args are not allowed when --skip_signatures is set",
		},
		{
			name: "signatures not skipped, --environment and --manifest and --signature are specified twice",
			args: []string{"--application", "potato", "--environment", "production", "--manifest", "manifest1-file.yaml", "--signature", "signature1-file.gpg", "--environment", "development", "--manifest", "manifest2-file.yaml", "--signature", "signature2-file.gpg"},
			expectedCmdArgs: &commandLineArguments{
				application: cli_utils.RepeatedString{
					Values: []string{
						"potato",
					},
				},
				environments: cli_utils.RepeatedString{
					Values: []string{
						"production",
						"development",
					},
				},
				manifests: cli_utils.RepeatedString{
					Values: []string{
						"manifest1-file.yaml",
						"manifest2-file.yaml",
					},
				},
				signatures: cli_utils.RepeatedString{
					Values: []string{
						"signature1-file.gpg",
						"signature2-file.gpg",
					},
				},
			},
		},
		{
			name:             "signatures not skipped, --environment and --manifest are specified without signature",
			args:             []string{"--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml"},
			expectedErrorMsg: "all --manifest args must have a --signature arg set immediately afterwards, unless --skip_signatures is set",
		},
		{
			name:             "signatures not skipped, --environment and --manifest is specified with multiple signatures",
			args:             []string{"--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--signature", "signature1-file.gpg", "--signature", "signature2-file.gpg"},
			expectedErrorMsg: "all --signature args must be set immediately after a --manifest arg",
		},
		{
			name: "--team is specified",
			args: []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team"},
			expectedCmdArgs: &commandLineArguments{
				skipSignatures: true,
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
			name:             "--team is specified twice",
			args:             []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--team", "tomato-team"},
			expectedErrorMsg: "the --team arg must be set at most once",
		},
		{
			name: "--source_commit_id is specified",
			args: []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--source_commit_id", "0123abcdef0123abcdef0123abcdef0123abcdef"},
			expectedCmdArgs: &commandLineArguments{
				skipSignatures: true,
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
				sourceCommitId: cli_utils.RepeatedString{
					Values: []string{
						"0123abcdef0123abcdef0123abcdef0123abcdef",
					},
				},
			},
		},
		{
			name:             "--source_commit_id is specified but has an invalid value",
			args:             []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--source_commit_id", "potato-commit"},
			expectedErrorMsg: "the --source_commit_id arg must be assigned a complete SHA1 commit hash in hexadecimal",
		},
		{
			name:             "--source_commit_id is specified twice",
			args:             []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--source_commit_id", "0123abcdef0123abcdef0123abcdef0123abcdef", "--source_commit_id", "0123abcdef0123abcdef0123abcdef0123abcdef"},
			expectedErrorMsg: "the --source_commit_id arg must be set at most once",
		},
		{
			name: "--previous_commit_id is specified",
			args: []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--source_commit_id", "0123abcdef0123abcdef0123abcdef0123abcdef", "--previous_commit_id", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			expectedCmdArgs: &commandLineArguments{
				skipSignatures: true,
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
				sourceCommitId: cli_utils.RepeatedString{
					Values: []string{
						"0123abcdef0123abcdef0123abcdef0123abcdef",
					},
				},
				previousCommitId: cli_utils.RepeatedString{
					Values: []string{
						"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					},
				},
			},
		},
		{
			name:             "--previous_commit_id is specified without --source_commit_id",
			args:             []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--previous_commit_id", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			expectedErrorMsg: "the --previous_commit_id arg can be set only if --source_commit_id is set",
		},
		{
			name:             "--previous_commit_id is specified twice",
			args:             []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--source_commit_id", "0123abcdef0123abcdef0123abcdef0123abcdef", "--previous_commit_id", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--previous_commit_id", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
			expectedErrorMsg: "the --previous_commit_id arg must be set at most once",
		},
		{
			name:             "--previous_commit_id is specified but with an invalid value",
			args:             []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--source_commit_id", "0123abcdef0123abcdef0123abcdef0123abcdef", "--previous_commit_id", "potato"},
			expectedErrorMsg: "the --previous_commit_id arg must be assigned a complete SHA1 commit hash in hexadecimal",
		},
		{
			name: "--source_author is specified",
			args: []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--source_commit_id", "0123abcdef0123abcdef0123abcdef0123abcdef", "--previous_commit_id", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--source_author", "potato@tomato.com"},
			expectedCmdArgs: &commandLineArguments{
				skipSignatures: true,
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
				sourceCommitId: cli_utils.RepeatedString{
					Values: []string{
						"0123abcdef0123abcdef0123abcdef0123abcdef",
					},
				},
				previousCommitId: cli_utils.RepeatedString{
					Values: []string{
						"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					},
				},
				sourceAuthor: cli_utils.RepeatedString{
					Values: []string{
						"potato@tomato.com",
					},
				},
			},
		},
		{
			name:             "--source_author is specified twice",
			args:             []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--source_commit_id", "0123abcdef0123abcdef0123abcdef0123abcdef", "--previous_commit_id", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--source_author", "potato@tomato.com", "--source_author", "foo@bar.com"},
			expectedErrorMsg: "the --source_author arg must be set at most once",
		},
		{
			name: "--source_message is specified",
			args: []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--source_commit_id", "0123abcdef0123abcdef0123abcdef0123abcdef", "--previous_commit_id", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--source_author", "potato@tomato.com", "--source_message", "test source message"},
			expectedCmdArgs: &commandLineArguments{
				skipSignatures: true,
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
				sourceCommitId: cli_utils.RepeatedString{
					Values: []string{
						"0123abcdef0123abcdef0123abcdef0123abcdef",
					},
				},
				previousCommitId: cli_utils.RepeatedString{
					Values: []string{
						"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					},
				},
				sourceAuthor: cli_utils.RepeatedString{
					Values: []string{
						"potato@tomato.com",
					},
				},
				sourceMessage: cli_utils.RepeatedString{
					Values: []string{
						"test source message",
					},
				},
			},
		},
		{
			name:             "--source_message is specified twice",
			args:             []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--source_commit_id", "0123abcdef0123abcdef0123abcdef0123abcdef", "--previous_commit_id", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--source_author", "potato@tomato.com", "--source_message", "test source message", "--source_message", "another test source message"},
			expectedErrorMsg: "the --source_message arg must be set at most once",
		},
		{
			name: "--version is specified",
			args: []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--source_commit_id", "0123abcdef0123abcdef0123abcdef0123abcdef", "--previous_commit_id", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--source_author", "potato@tomato.com", "--source_message", "test source message", "--version", "1234"},
			expectedCmdArgs: &commandLineArguments{
				skipSignatures: true,
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
				sourceCommitId: cli_utils.RepeatedString{
					Values: []string{
						"0123abcdef0123abcdef0123abcdef0123abcdef",
					},
				},
				previousCommitId: cli_utils.RepeatedString{
					Values: []string{
						"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					},
				},
				sourceAuthor: cli_utils.RepeatedString{
					Values: []string{
						"potato@tomato.com",
					},
				},
				sourceMessage: cli_utils.RepeatedString{
					Values: []string{
						"test source message",
					},
				},
				version: cli_utils.RepeatedString{
					Values: []string{
						"1234",
					},
				},
			},
		},
		{
			name:             "--version is specified twice",
			args:             []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--source_commit_id", "0123abcdef0123abcdef0123abcdef0123abcdef", "--previous_commit_id", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--source_author", "potato@tomato.com", "--source_message", "test source message", "--version", "123", "--version", "456"},
			expectedErrorMsg: "the --version arg must be set at most once",
		},
		{
			name:             "--version is set to non-integer value",
			args:             []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--source_commit_id", "0123abcdef0123abcdef0123abcdef0123abcdef", "--previous_commit_id", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--source_author", "potato@tomato.com", "--source_message", "test source message", "--version", "abc"},
			expectedErrorMsg: "the --version arg must be an integer value",
		},
		{
			name:             "--version is set to negative integer value",
			args:             []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--source_commit_id", "0123abcdef0123abcdef0123abcdef0123abcdef", "--previous_commit_id", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--source_author", "potato@tomato.com", "--source_message", "test source message", "--version", "-123"},
			expectedErrorMsg: "the --version arg value must be positive",
		},
		{
			name: "--display_version is specified",
			args: []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--source_commit_id", "0123abcdef0123abcdef0123abcdef0123abcdef", "--previous_commit_id", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--source_author", "potato@tomato.com", "--source_message", "test source message", "--version", "1234", "--display_version", "1.23.4"},
			expectedCmdArgs: &commandLineArguments{
				skipSignatures: true,
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
				sourceCommitId: cli_utils.RepeatedString{
					Values: []string{
						"0123abcdef0123abcdef0123abcdef0123abcdef",
					},
				},
				previousCommitId: cli_utils.RepeatedString{
					Values: []string{
						"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					},
				},
				sourceAuthor: cli_utils.RepeatedString{
					Values: []string{
						"potato@tomato.com",
					},
				},
				sourceMessage: cli_utils.RepeatedString{
					Values: []string{
						"test source message",
					},
				},
				version: cli_utils.RepeatedString{
					Values: []string{
						"1234",
					},
				},
				displayVersion: cli_utils.RepeatedString{
					Values: []string{
						"1.23.4",
					},
				},
			},
		},
		{
			name:             "--display_version is specified twice",
			args:             []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--source_commit_id", "0123abcdef0123abcdef0123abcdef0123abcdef", "--previous_commit_id", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--source_author", "potato@tomato.com", "--source_message", "test source message", "--version", "123", "--display_version", "1.23", "--display_version", "a.bc"},
			expectedErrorMsg: "the --display_version arg must be set at most once",
		},
		{
			name:             "--display_version is specified but is too long",
			args:             []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "manifest-file.yaml", "--team", "potato-team", "--source_commit_id", "0123abcdef0123abcdef0123abcdef0123abcdef", "--previous_commit_id", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--source_author", "potato@tomato.com", "--source_message", "test source message", "--version", "123", "--display_version", "loooooooooooooooooooooooooooooooooooooooooong"},
			expectedErrorMsg: "the --display_version arg must be at most 15 characters long",
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmdArgs, err := readArgs(tc.args)
			// check errors
			if diff := cmp.Diff(errMatcher{tc.expectedErrorMsg}, err, cmpopts.EquateErrors()); !(err == nil && tc.expectedErrorMsg == "") && diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			if !cmp.Equal(cmdArgs, tc.expectedCmdArgs, cmp.AllowUnexported(commandLineArguments{})) {
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
			cmdArgs:          []string{"--skip_signatures", "--application", "potato"},
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
			cmdArgs: []string{"--skip_signatures", "--application", "potato", "--environment", "production", "--manifest", "production-manifest.yaml"},
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
					filename: "production-manifest.yaml",
					content:  "some production manifest",
				},
				{
					filename: "production-signature.gpg",
					content:  "some production signature",
				},
			},
			name:    "with environment and manifest and signatures",
			cmdArgs: []string{"--application", "potato", "--environment", "production", "--manifest", "production-manifest.yaml", "--signature", "production-signature.gpg"},
			expectedParams: &ReleaseParameters{
				Application: "potato",
				Manifests: map[string]string{
					"production": "some production manifest",
				},
				Signatures: map[string][]byte{
					"production": []byte("some production signature"),
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
			cmdArgs: []string{"--skip_signatures", "--application", "potato", "--environment", "development", "--manifest", "development-manifest.yaml", "--environment", "production", "--manifest", "production-manifest.yaml"},
			expectedParams: &ReleaseParameters{
				Application: "potato",
				Manifests: map[string]string{
					"development": "some development manifest",
					"production":  "some production manifest",
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
				{
					filename: "development-signature.gpg",
					content:  "some development signature",
				},
				{
					filename: "production-signature.gpg",
					content:  "some production signature",
				},
			},
			name:    "with environment and manifest and signature multiple times",
			cmdArgs: []string{"--application", "potato", "--environment", "development", "--manifest", "development-manifest.yaml", "--signature", "development-signature.gpg", "--environment", "production", "--manifest", "production-manifest.yaml", "--signature", "production-signature.gpg"},
			expectedParams: &ReleaseParameters{
				Application: "potato",
				Manifests: map[string]string{
					"development": "some development manifest",
					"production":  "some production manifest",
				},
				Signatures: map[string][]byte{
					"development": []byte("some development signature"),
					"production":  []byte("some production signature"),
				},
			},
		},
		{
			name:             "some error occurs in argument parsing",
			cmdArgs:          []string{"--skip_signatures", "--application"},
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

			for i := range tc.setup {
				tc.setup[i].filename = filepath.Join(dir, tc.setup[i].filename)

			}
			for i, arg := range tc.cmdArgs {
				if arg == "--manifest" {
					tc.cmdArgs[i+1] = filepath.Join(dir, tc.cmdArgs[i+1])
				}
				if arg == "--signature" {
					tc.cmdArgs[i+1] = filepath.Join(dir, tc.cmdArgs[i+1])
				}
			}

			for _, fc := range tc.setup {
				err = os.WriteFile(fc.filename, []byte(fc.content), 0664)
				if err != nil {
					t.Fatalf("error while creating file %s, error: %v", fc.filename, err)
				}
			}

			params, err := ParseArgs(tc.cmdArgs)
			// check errors
			if diff := cmp.Diff(errMatcher{tc.expectedErrorMsg}, err, cmpopts.EquateErrors()); !(err == nil && tc.expectedErrorMsg == "") && diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			// check result
			if !cmp.Equal(params, tc.expectedParams, cmp.AllowUnexported(commandLineArguments{})) {
				t.Fatalf("expected args %v, got %v", tc.expectedParams, params)
			}
		})
	}
}
