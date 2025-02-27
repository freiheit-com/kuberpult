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

Copyright freiheit.com
*/
package environments

import (
	"flag"
	"fmt"
	"strings"

	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
)

type DeleteEnvironmentCommandLineArguments struct {
	environment cli_utils.RepeatedString
}

func argsValidDeleteEnvironment(cmdArgs *DeleteEnvironmentCommandLineArguments) (bool, string) {
	if len(cmdArgs.environment.Values) != 1 {
		return false, "the --environment arg must be set exactly once"
	}

	return true, ""
}

func readDeleteEnvironmentArgs(args []string) (*DeleteEnvironmentCommandLineArguments, error) {
	cmdArgs := DeleteEnvironmentCommandLineArguments{}

	fs := flag.NewFlagSet("flag set", flag.ContinueOnError)

	fs.Var(&cmdArgs.environment, "environment", "the environment you are trying to delete")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("error while parsing command line arguments, error: %w", err)
	}

	if len(fs.Args()) != 0 { // kuberpult-cli release does not accept any positional arguments, so this is an error
		return nil, fmt.Errorf("these arguments are not recognized: \"%v\"", strings.Join(fs.Args(), " "))
	}

	if ok, msg := argsValidDeleteEnvironment(&cmdArgs); !ok {
		return nil, fmt.Errorf(msg)
	}

	return &cmdArgs, nil
}

func convertToDeleteEnvironmentParams(cmdArgs DeleteEnvironmentCommandLineArguments) (*DeleteEnvironmentParameters, error) {
	if ok, msg := argsValidDeleteEnvironment(&cmdArgs); !ok {
		return nil, fmt.Errorf("the provided command line arguments structure is invalid, cause: %s", msg)
	}

	rp := DeleteEnvironmentParameters{
		Environment: cmdArgs.environment.Values[0],
	}

	return &rp, nil
}

func ParseArgsDeleteEnvironment(args []string) (*DeleteEnvironmentParameters, error) {
	cmdArgs, err := readDeleteEnvironmentArgs(args)
	if err != nil {
		return nil, fmt.Errorf("error while reading command line arguments for deleting an environment, error: %w", err)
	}
	rp, err := convertToDeleteEnvironmentParams(*cmdArgs)
	if err != nil {
		return nil, fmt.Errorf("error while creating parameters for deleting an environment, error: %w", err)
	}
	return rp, nil
}
