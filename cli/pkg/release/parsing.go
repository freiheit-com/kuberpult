/*
This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright 2023 freiheit.com
*/

package release

import (
	"flag"
	"fmt"
	"os"

	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
)

// a simple container for the command line args, not meant for anything except the use of flag.Parse
// unless you're working on the readArgs and parseArgs functions, you probably don't need this type, see releaseParameters instead
type cmdArguments struct {
	application   cli_utils.RepeatedString
	environments  cli_utils.RepeatedString
	manifestFiles cli_utils.RepeatedString
}

func readArgs(args []string) (*cmdArguments, error) {
	cmdArgs := cmdArguments{}

	fs := flag.NewFlagSet("flag set", flag.ExitOnError)

	fs.Var(&cmdArgs.application, "application", "the name of the application to deploy (must be set exactly once)")
	fs.Var(&cmdArgs.environments, "environment", "an environment to deploy to (must have --manifest set immediately afterwards)")
	fs.Var(&cmdArgs.manifestFiles, "manifest", "the name of the file containing manifests to be deployed (must be set immediately after --environment)")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("error while parsing command line arguments, error: %w", err)
	}

	if len(cmdArgs.application.Values) != 1 {
		return nil, fmt.Errorf("the --application arg must be set exactly once")
	}

	for i, arg := range args {
		if arg == "--environment" {
			if i+2 >= len(args) || args[i+2] != "--manifest" {
				return nil, fmt.Errorf("all --environment args must have a --manifest arg set immediately afterwards")
			}
		}
		if arg == "--manifest" {
			if i-2 < 0 || args[i-2] != "--environment" {
				return nil, fmt.Errorf("all --manifest args must be set immediately after an --environment arg")
			}
		}
	}

	if len(cmdArgs.environments.Values) != len(cmdArgs.manifestFiles.Values) {
		return nil, fmt.Errorf("the args --environment and --manifest must be set an equal number of times")
	}

	for _, manifestFile := range cmdArgs.manifestFiles.Values {
		if _, err := os.Stat(manifestFile); err != nil {
			return nil, fmt.Errorf("error while running stat on file %s, error: %w", manifestFile, err)
		}
	}
	return &cmdArgs, nil
}

func ParseArgs(args []string) (*ReleaseParameters, error) {
	cmdArgs, err := readArgs(args)
	if err != nil {
		return nil, fmt.Errorf("error while reading command line arguments, error: %w", err)
	}

	rp := ReleaseParameters{}
	rp.Application = cmdArgs.application.Values[0]
	rp.ManifestFiles = make(map[string]string)
	for i := range cmdArgs.environments.Values {
		manifestFile := cmdArgs.manifestFiles.Values[i]
		environemnt := cmdArgs.environments.Values[i]

		rp.ManifestFiles[environemnt] = manifestFile
	}

	return &rp, nil
}
