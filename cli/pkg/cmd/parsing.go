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

package cmd

import (
	"flag"
	"fmt"
	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
)

type commandLineArguments struct {
	url cli_utils.RepeatedString
}

type kuberpultClientParameters struct {
	url string
}

func readArgs(args []string) (*commandLineArguments, []string, error) {
	cmdArgs := commandLineArguments{}
	
	fs := flag.NewFlagSet("top level", flag.ExitOnError)

	fs.Var(&cmdArgs.url, "url", "the URL of the Kuberpult instance (must be set exactly once)")

	if err := fs.Parse(args); err != nil {
		return nil, nil, fmt.Errorf("error while reading command line arguments, error: %w", err)
	}

	other := fs.Args()

	return &cmdArgs, other, nil
}

func argsValid(cmdArgs *commandLineArguments) (bool, string) {
	if len(cmdArgs.url.Values) != 1 {
		return false, "the --url arg must be set exactly once"
	}
	return true, ""
}

func convertToParams(cmdArgs *commandLineArguments) (*kuberpultClientParameters, error) {
	if ok, msg := argsValid(cmdArgs); !ok {
		return nil, fmt.Errorf(msg)
	}

	params := kuberpultClientParameters{}
	
	params.url = cmdArgs.url.Values[0]
	
	return &params, nil
}

func parseArgs(args []string) (*kuberpultClientParameters, []string, error) {
	cmdArgs, other, err := readArgs(args)
	if err != nil {
		return nil, nil, fmt.Errorf("error while parsing command line arguments, error: %w", err)
	}

	params, err := convertToParams(cmdArgs)
	if err != nil {
		return nil, nil, fmt.Errorf("error while creating kuberpult client parameters, error: %w", err)
	}

	return params, other, nil
}