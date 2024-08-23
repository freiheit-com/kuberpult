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
	"flag"
	"fmt"
	"strings"

	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
)

type ReleaseTrainCommandLineArguments struct {
	targetEnvironment    cli_utils.RepeatedString
	team                 cli_utils.RepeatedString
	useDexAuthentication bool
}

func releaseTrainArgsValid(cmdArgs *ReleaseTrainCommandLineArguments) (result bool, errorMessage string) {
	if len(cmdArgs.targetEnvironment.Values) != 1 {
		return false, "the --target-environment arg must be set exactly once"
	}

	if len(cmdArgs.team.Values) > 1 {
		return false, "the --team arg must be set at most once"
	}

	return true, ""
}

func readArgs(args []string) (*ReleaseTrainCommandLineArguments, error) {
	cmdArgs := ReleaseTrainCommandLineArguments{} //exhaustruct:ignore

	fs := flag.NewFlagSet("flag set", flag.ContinueOnError)

	fs.Var(&cmdArgs.targetEnvironment, "target-environment", "the name of the environment to target with the release train (must be set exactly once)")
	fs.Var(&cmdArgs.team, "team", "the target team. Only specified teams services will be taken into account when conducting the release train")
	fs.BoolVar(&cmdArgs.useDexAuthentication, "use_dex_auth", false, "if set to true, the /api/* endpoint will be used. Dex must be enabled on the server side and a dex token must be provided, otherwise the request will be denied")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("error while parsing command line arguments, error: %w", err)
	}

	if len(fs.Args()) != 0 { // kuberpult-cli release does not accept any positional arguments, so this is an error
		return nil, fmt.Errorf("these arguments are not recognized: \"%v\"", strings.Join(fs.Args(), " "))
	}

	if ok, msg := releaseTrainArgsValid(&cmdArgs); !ok {
		return nil, fmt.Errorf(msg)
	}

	return &cmdArgs, nil
}

func convertToParams(cmdArgs ReleaseTrainCommandLineArguments) (*ReleaseTrainParameters, error) {
	if ok, msg := releaseTrainArgsValid(&cmdArgs); !ok {
		return nil, fmt.Errorf("the provided command line arguments structure is invalid, cause: %s", msg)
	}

	rp := ReleaseTrainParameters{
		Team:                 nil,
		TargetEnvironment:    cmdArgs.targetEnvironment.Values[0],
		UseDexAuthentication: cmdArgs.useDexAuthentication,
	}

	if len(cmdArgs.team.Values) == 1 {
		rp.Team = &cmdArgs.team.Values[0]
	}
	return &rp, nil
}

func ParseArgsReleaseTrain(args []string) (*ReleaseTrainParameters, error) {
	cmdArgs, err := readArgs(args)
	if err != nil {
		return nil, fmt.Errorf("error while reading command line arguments for running a release train error: %w", err)
	}
	rp, err := convertToParams(*cmdArgs)
	if err != nil {
		return nil, fmt.Errorf("error while creating endpoint params for running a release train, error: %w", err)
	}

	return rp, nil
}
