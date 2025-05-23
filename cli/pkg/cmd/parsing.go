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
	"errors"
	"flag"
	"fmt"

	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
)

const DefaultRetries = 3

type commandLineArguments struct {
	url         cli_utils.RepeatedString
	authorEmail cli_utils.RepeatedString
	authorName  cli_utils.RepeatedString
	retries     cli_utils.RepeatedInt
	timeout     cli_utils.RepeatedInt
}

func readArgs(args []string) (*commandLineArguments, []string, error) {
	cmdArgs := commandLineArguments{} //exhaustruct:ignore

	fs := flag.NewFlagSet("top level", flag.ContinueOnError)

	fs.Var(&cmdArgs.url, "url", "the URL of the Kuberpult instance (must be set exactly once)")
	fs.Var(&cmdArgs.authorName, "author_name", "the name of the git author who eventually will write to the manifest repo (must be set at most once)")
	fs.Var(&cmdArgs.authorEmail, "author_email", "the email of the git author who eventially will write to the manifest repo (must be set at most once)")
	fs.Var(&cmdArgs.retries, "retries", "number of times the cli will retry http requests to kuberpult upon failure (must be set at most once) - default=3")
	fs.Var(&cmdArgs.timeout, "timeout", "requests timeout seconds (must be set at most once) - default=180")

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
	if len(cmdArgs.authorName.Values) > 1 {
		return false, "the --author_name arg must be set at most once"
	}
	if len(cmdArgs.authorEmail.Values) > 1 {
		return false, "the --author_email arg must be set at most once"
	}
	if len(cmdArgs.retries.Values) > 1 {
		return false, "the --retries arg must be set at most once"
	}
	if len(cmdArgs.timeout.Values) > 1 {
		return false, "the --timeout arg must be set at most once"
	}

	return true, ""
}

func convertToParams(cmdArgs *commandLineArguments) (*kuberpultClientParameters, error) {
	if ok, msg := argsValid(cmdArgs); !ok {
		return nil, errors.New(msg)
	}

	params := kuberpultClientParameters{} //exhaustruct:ignore

	params.url = cmdArgs.url.Values[0]

	if len(cmdArgs.authorName.Values) == 1 {
		params.authorName = &cmdArgs.authorName.Values[0]
	}

	if len(cmdArgs.authorEmail.Values) == 1 {
		params.authorEmail = &cmdArgs.authorEmail.Values[0]
	}

	if len(cmdArgs.retries.Values) == 1 {
		if cmdArgs.retries.Values[0] <= 0 {
			return nil, fmt.Errorf("--retries arg value must be positive")
		}
		params.retries = uint64(cmdArgs.retries.Values[0])
	} else {
		params.retries = DefaultRetries
	}
	if len(cmdArgs.timeout.Values) == 1 {
		if cmdArgs.timeout.Values[0] <= 0 {
			return nil, fmt.Errorf("--timeout arg value must be positive")
		}
		params.timeout = uint64(cmdArgs.timeout.Values[0])
	} else {
		params.timeout = cli_utils.HttpDefaultTimeout
	}

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
