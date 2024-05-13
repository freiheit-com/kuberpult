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
	"strconv"
	"strings"

	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
)

// a simple container for the command line args, not meant for anything except the use of flag.Parse
// unless you're working on the readArgs and parseArgs functions, you probably don't need this type, see releaseParameters instead
type cmdArguments struct {
	application      cli_utils.RepeatedString // code-simplifying hack: we use RepeatingString for application even though it's not meant to be repeated so that we can raise and error when it's repeated more or less than once
	environments     cli_utils.RepeatedString
	manifests        cli_utils.RepeatedString
	team             cli_utils.RepeatedString // same hack as application field here
	sourceCommitId   cli_utils.RepeatedString // same hack as application field here
	previousCommitId cli_utils.RepeatedString // same hack as application field here
	sourceAuthor     cli_utils.RepeatedString // same hack as application field here
	sourceMessage    cli_utils.RepeatedString // same hack as application field here
	version          cli_utils.RepeatedString // same hack as application field here
	displayVersion   cli_utils.RepeatedString // same hack as application field here
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

var commitIDRegex = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

func isCommitID(s string) bool {
	return commitIDRegex.MatchString(s)
}

var authorIDRegex = regexp.MustCompile(`^[^<\n]+( <[^@\n]+@[^>\n]+>)?$`)

func isAuthorID(s string) bool {
	return authorIDRegex.MatchString(s)
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
		if val, err := strconv.Atoi(cmdArgs.version.Values[0]); err != nil {
			return false, "the --version arg must be an integer value"
		} else {
			if val <= 0 {
				return false, "the --version arg value must be positive"
			}
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
	fs.Var(&cmdArgs.previousCommitId, "previous_commit_id", "the SHA1 hash of the previous commit (must not be set more than once and can only be set when source_commit_id is set)")
	fs.Var(&cmdArgs.sourceAuthor, "source_author", "the souce author (must not be set more than once)")
	fs.Var(&cmdArgs.sourceMessage, "source_message", "the source commit message (must not be set more than once)")
	fs.Var(&cmdArgs.version, "version", "the release version (must be a positive integer)")
	fs.Var(&cmdArgs.displayVersion, "display_version", "display version (must be a string between 1 and characters long)")

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
		version, _ := strconv.Atoi(cmdArgs.version.Values[0])
		version64 := uint64(version)
		rp.Version = &version64
	}
	if len(cmdArgs.displayVersion.Values) == 1 {
		rp.DisplayVersion = &cmdArgs.displayVersion.Values[0]
	}

	return &rp, nil
}
