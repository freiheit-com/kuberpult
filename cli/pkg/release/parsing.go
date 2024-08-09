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

package release

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
)

// a simple container for the command line args, not meant for anything except the use of flag.Parse
// unless you're working on the readArgs and parseArgs functions, you probably don't need this type, see ReleaseParameters instead
type commandLineArguments struct {
	// Technically `application` may not be set multiple times. However, the Go flag package doesn't offer a simple way of checking whether a flag was set multiple times.
	// Therefore, we set the value of `application` to RepeatedString and later we check if it's been set more than once and raise an error accordingly.
	// The same trick is used for all fields other than environments, manifests, and signatures.
	application          cli_utils.RepeatedString
	environments         cli_utils.RepeatedString
	manifests            cli_utils.RepeatedString
	team                 cli_utils.RepeatedString
	sourceCommitId       cli_utils.RepeatedString
	previousCommitId     cli_utils.RepeatedString
	sourceAuthor         cli_utils.RepeatedString
	sourceMessage        cli_utils.RepeatedString
	version              cli_utils.RepeatedInt
	displayVersion       cli_utils.RepeatedString
	skipSignatures       bool
	signatures           cli_utils.RepeatedString
	useDexAuthentication bool
	retries              cli_utils.RepeatedInt
}

// checks whether every --environment arg is matched with a --manifest arg
func environmentsManifestsPaired(args []string) (result bool, errorMessage string) {
	for i, arg := range args {
		if arg == "--environment" {
			nextIndex := i + 2
			if nextIndex >= len(args) || args[nextIndex] != "--manifest" {
				return false, "all --environment args must have a --manifest arg set immediately afterwards"
			}
		}
		if arg == "--manifest" {
			prevIndex := i - 2
			if prevIndex < 0 || args[prevIndex] != "--environment" {
				return false, "all --manifest args must be set immediately after an --environment arg"
			}
		}
	}
	return true, ""
}

// checks whether every --manifest arg is matched with a --signature arg
func manifestsSignaturesPaired(args []string) (result bool, errorMessage string) {
	for i, arg := range args {
		if arg == "--manifest" {
			nextIndex := i + 2
			if nextIndex >= len(args) || args[nextIndex] != "--signature" {
				return false, "all --manifest args must have a --signature arg set immediately afterwards, unless --skip_signatures is set"
			}
		}
		if arg == "--signature" {
			prevIndex := i - 2
			if prevIndex < 0 || args[prevIndex] != "--manifest" {
				return false, "all --signature args must be set immediately after a --manifest arg"
			}
		}
	}
	return true, ""
}

func argsValid(cmdArgs *commandLineArguments) (result bool, errorMessage string) {
	var commitIDRegex = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
	isCommitID := func(s string) bool {
		return commitIDRegex.MatchString(s)
	}

	// this regex is identical to the regex from services/frontend-service/pkg/handler/release.go
	var authorIDRegex = regexp.MustCompile(`^[^<\n]+( <[^@\n]+@[^>\n]+>)?$`)
	isAuthorID := func(s string) bool {
		return authorIDRegex.MatchString(s)
	}

	if len(cmdArgs.application.Values) != 1 {
		return false, "the --application arg must be set exactly once"
	}

	if len(cmdArgs.environments.Values) == 0 {
		return false, "the args --enviornment and --manifest must be set at least once"
	}

	if len(cmdArgs.team.Values) > 1 {
		return false, "the --team arg must be set at most once"
	}

	if len(cmdArgs.sourceCommitId.Values) > 1 {
		return false, "the --source_commit_id arg must be set at most once"
	}

	if len(cmdArgs.sourceCommitId.Values) == 1 {
		if !isCommitID(cmdArgs.sourceCommitId.Values[0]) {
			return false, "the --source_commit_id arg must be assigned a complete SHA1 commit hash in hexadecimal"
		}
	}

	if len(cmdArgs.previousCommitId.Values) > 1 {
		return false, "the --previous_commit_id arg must be set at most once"
	}

	if len(cmdArgs.previousCommitId.Values) == 1 {
		if len(cmdArgs.sourceCommitId.Values) != 1 { // not a requirement from the API, but it is a reasonable restriction to make
			return false, "the --previous_commit_id arg can be set only if --source_commit_id is set"
		}
		if !isCommitID(cmdArgs.previousCommitId.Values[0]) {
			return false, "the --previous_commit_id arg must be assigned a complete SHA1 commit hash in hexadecimal"
		}
	}

	if len(cmdArgs.sourceAuthor.Values) > 1 {
		return false, "the --source_author arg must be set at most once"
	}

	if len(cmdArgs.sourceAuthor.Values) == 1 {
		if !isAuthorID(cmdArgs.sourceAuthor.Values[0]) {
			return false, fmt.Sprintf("the --source_author must be assigned a proper author identifier, matching the regex %s", authorIDRegex)
		}
	}

	if len(cmdArgs.sourceMessage.Values) > 1 {
		return false, "the --source_message arg must be set at most once"
	}

	if len(cmdArgs.version.Values) > 1 {
		return false, "the --version arg must be set at most once"
	}

	if len(cmdArgs.version.Values) == 1 {
		if cmdArgs.version.Values[0] <= 0 {
			return false, "the --version arg value must be positive"
		}
	}

	if len(cmdArgs.displayVersion.Values) > 1 {
		return false, "the --display_version arg must be set at most once"
	}

	if len(cmdArgs.displayVersion.Values) == 1 {
		if len(cmdArgs.displayVersion.Values[0]) > 15 {
			return false, "the --display_version arg must be at most 15 characters long"
		}
	}

	if cmdArgs.skipSignatures || cmdArgs.useDexAuthentication {
		if len(cmdArgs.signatures.Values) > 0 {
			return false, "--signature args are not allowed when --skip_signatures or use_dex_auth are set"
		}
	}

	return true, ""
}

// takes the raw command line flags and converts them to an intermediate represnetations for easy validation
func readArgs(args []string) (*commandLineArguments, error) {
	cmdArgs := commandLineArguments{} //exhaustruct:ignore

	fs := flag.NewFlagSet("flag set", flag.ContinueOnError)

	fs.Var(&cmdArgs.application, "application", "the name of the application to deploy (must be set exactly once)")
	fs.Var(&cmdArgs.environments, "environment", "an environment to deploy to (must have --manifest set immediately afterwards)")
	fs.Var(&cmdArgs.manifests, "manifest", "the name of the file containing manifests to be deployed (must be set immediately after --environment)")
	fs.Var(&cmdArgs.team, "team", "the name of the team to which this release belongs (must not be set more than once)")
	fs.Var(&cmdArgs.sourceCommitId, "source_commit_id", "the SHA1 hash of the source commit (must not be set more than once)")
	fs.Var(&cmdArgs.previousCommitId, "previous_commit_id", "the SHA1 hash of the previous commit (must not be set more than once and can only be set when source_commit_id is set)")
	fs.Var(&cmdArgs.sourceAuthor, "source_author", "the souce author (must not be set more than once)")
	fs.Var(&cmdArgs.sourceMessage, "source_message", "the source commit message (must not be set more than once)")
	fs.Var(&cmdArgs.version, "version", "the release version (must be a positive integer)")
	fs.Var(&cmdArgs.displayVersion, "display_version", "display version (must be a string between 1 and characters long)")
	fs.BoolVar(&cmdArgs.skipSignatures, "skip_signatures", false, "if set to true, then the command line does not accept the --signature args")
	fs.Var(&cmdArgs.signatures, "signature", "the name of the file containing the signature of the manifest to be deployed (must be set immediately after --manifest)")
	fs.BoolVar(&cmdArgs.useDexAuthentication, "use_dex_auth", false, "use /api/release endpoint, if set to true, dex must be enabled and dex token must be provided otherwise the request will be denied")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("error while parsing command line arguments, error: %w", err)
	}

	if len(fs.Args()) != 0 { // kuberpult-cli release does not accept any positional arguments, so this is an error
		return nil, fmt.Errorf("these arguments are not recognized: \"%v\"", strings.Join(fs.Args(), " "))
	}

	if ok, msg := environmentsManifestsPaired(args); !ok {
		return nil, fmt.Errorf(msg)
	}

	if !cmdArgs.skipSignatures && !cmdArgs.useDexAuthentication {
		if ok, msg := manifestsSignaturesPaired(args); !ok {
			return nil, fmt.Errorf(msg)
		}
	}

	if ok, msg := argsValid(&cmdArgs); !ok {
		return nil, fmt.Errorf(msg)
	}

	return &cmdArgs, nil
}

// converts the intermediate representation of the command line flags into the final structure containing parameters for the release endpoint
func convertToParams(cmdArgs commandLineArguments) (*ReleaseParameters, error) {
	if ok, msg := argsValid(&cmdArgs); !ok {
		// this should never happen, as the validation is already peformed by the readArgs function
		return nil, fmt.Errorf("the provided command line arguments structure is invalid, cause: %s", msg)
	}

	rp := ReleaseParameters{} //exhaustruct:ignore
	rp.Manifests = make(map[string][]byte)
	if !cmdArgs.skipSignatures && !cmdArgs.useDexAuthentication {
		rp.Signatures = make(map[string][]byte)
	}

	rp.Application = cmdArgs.application.Values[0]
	if len(cmdArgs.team.Values) == 1 {
		rp.Team = &cmdArgs.team.Values[0]
	}
	if len(cmdArgs.sourceCommitId.Values) == 1 {
		rp.SourceCommitId = &cmdArgs.sourceCommitId.Values[0]
	}
	if len(cmdArgs.previousCommitId.Values) == 1 {
		rp.PreviousCommitId = &cmdArgs.previousCommitId.Values[0]
	}
	if len(cmdArgs.sourceAuthor.Values) == 1 {
		rp.SourceAuthor = &cmdArgs.sourceAuthor.Values[0]
	}
	if len(cmdArgs.sourceMessage.Values) == 1 {
		rp.SourceMessage = &cmdArgs.sourceMessage.Values[0]
	}
	if len(cmdArgs.version.Values) == 1 {
		version := cmdArgs.version.Values[0]
		version64 := uint64(version)
		rp.Version = &version64
	}
	if len(cmdArgs.displayVersion.Values) == 1 {
		rp.DisplayVersion = &cmdArgs.displayVersion.Values[0]
	}
	for i := range cmdArgs.environments.Values {
		manifestFile := cmdArgs.manifests.Values[i]
		environment := cmdArgs.environments.Values[i]

		manifestBytes, err := os.ReadFile(manifestFile)
		if err != nil {
			return nil, fmt.Errorf("error while reading the manifest file %s, error: %w", manifestFile, err)
		}
		rp.Manifests[environment] = manifestBytes

		// signatures are not allowed when using authentication
		if !cmdArgs.skipSignatures && !cmdArgs.useDexAuthentication {
			signatureFile := cmdArgs.signatures.Values[i]

			signatureBytes, err := os.ReadFile(signatureFile)
			if err != nil {
				return nil, fmt.Errorf("error while reading the signature file %s, error: %w", signatureFile, err)
			}
			rp.Signatures[environment] = signatureBytes
		}
	}
	rp.UseDexAuthentication = cmdArgs.useDexAuthentication
	return &rp, nil
}

// parses the command line flags provided to the release subcommand (not including the release subcommand itself) into a struct that can be passed to the Release function
func ParseArgs(args []string) (*ReleaseParameters, error) {
	cmdArgs, err := readArgs(args)
	if err != nil {
		return nil, fmt.Errorf("error while reading command line arguments, error: %w", err)
	}
	rp, err := convertToParams(*cmdArgs)
	if err != nil {
		return nil, fmt.Errorf("error while creating /release endpoint params, error: %w", err)
	}

	return rp, nil
}
