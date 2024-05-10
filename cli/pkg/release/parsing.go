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
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
)

// a simple container for the command line args, not meant for anything except the use of flag.Parse
// unless you're working on the readArgs and parseArgs functions, you probably don't need this type, see releaseParameters instead
type cmdArguments struct {
	application    cli_utils.RepeatedString // code-simplifying hack: we use RepeatingString for application even though it's not meant to be repeated so that we can raise and error when it's repeated more or less than once
	environments   cli_utils.RepeatedString
	manifests      cli_utils.RepeatedString
	team           cli_utils.RepeatedString // same hack as application field here
	sourceCommitId cli_utils.RepeatedString // same hack as application field here
}

// checks whether every --environment arg is matched with a --manifest arg
func environmentManifestsPaired(args []string) (result bool, message string) {
	for i, arg := range args {
		if arg == "--environment" {
			nextIndex := i + 2
			if nextIndex >= len(args) || args[nextIndex] != "--manifest" {
				return false, "all --environment args must have a --manifest arg set immediately afterwards"
			}
		}
		prevIndex := i - 2
		if arg == "--manifest" {
			if prevIndex < 0 || args[prevIndex] != "--environment" {
				return false, "all --manifest args must be set immediately after an --environment arg"
			}
		}
	}
	return true, ""
}

// checks if a string is a hexadecimal SHA1 hash, used for validating a commit ID
func isHexSHA1(s string) bool {
	regex := regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
	return regex.MatchString(s)
}

func parsedArgsValid(cmdArgs *cmdArguments) (result bool, message string) {
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
		if !isHexSHA1(cmdArgs.sourceCommitId.Values[0]) {
			return false, "the --source_commit_id arg must be assigned a complete SHA1 commit hash in hexadecimal"
		}
	}

	return true, ""
}

func parseArgs(args []string) (*cmdArguments, error) {
	cmdArgs := cmdArguments{}

	fs := flag.NewFlagSet("flag set", flag.ContinueOnError)

	fs.Var(&cmdArgs.application, "application", "the name of the application to deploy (must be set exactly once)")
	fs.Var(&cmdArgs.environments, "environment", "an environment to deploy to (must have --manifest set immediately afterwards)")
	fs.Var(&cmdArgs.manifests, "manifest", "the name of the file containing manifests to be deployed (must be set immediately after --environment)")
	fs.Var(&cmdArgs.team, "team", "the name of the team to which this release belongs (must not be set more than once)")
	fs.Var(&cmdArgs.sourceCommitId, "source_commit_id", "the SHA1 hash of the source commit (must not be set more than once)")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("error while parsing command line arguments, error: %w", err)
	}

	if len(fs.Args()) != 0 { // kuberpult-cli release does not accept any positional arguments, so this is an error
		return nil, fmt.Errorf("these arguments are not recognized: \"%v\"", strings.Join(fs.Args(), " "))
	}

	if ok, msg := parsedArgsValid(&cmdArgs); !ok {
		return nil, fmt.Errorf(msg)
	}

	if ok, msg := environmentManifestsPaired(args); !ok {
		return nil, fmt.Errorf(msg)
	}

	return &cmdArgs, nil
}

func ProcessArgs(args []string) (*ReleaseParameters, error) {
	cmdArgs, err := parseArgs(args)
	if err != nil {
		return nil, fmt.Errorf("error while reading command line arguments, error: %w", err)
	}

	rp := ReleaseParameters{}
	rp.Application = cmdArgs.application.Values[0]
	rp.Manifests = make(map[string]string)
	for i := range cmdArgs.environments.Values {
		manifestFile := cmdArgs.manifests.Values[i]
		environemnt := cmdArgs.environments.Values[i]

		manifestBytes, err := os.ReadFile(manifestFile)
		if err != nil {
			return nil, fmt.Errorf("error while reading the manifest file %s, error: %w", manifestFile, err)
		}
		rp.Manifests[environemnt] = string(manifestBytes)
	}
	if len(cmdArgs.team.Values) == 1 {
		rp.Team = &cmdArgs.team.Values[0]
	}
	if len(cmdArgs.sourceCommitId.Values) == 1 {
		rp.SourceCommitId = &cmdArgs.sourceCommitId.Values[0]
	}

	return &rp, nil
}
