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

package locks

import (
	"flag"
	"fmt"
	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
	"strings"
)

type CreateEnvLockCommandLineArguments struct {
	environment          cli_utils.RepeatedString
	lockId               cli_utils.RepeatedString
	message              cli_utils.RepeatedString
	useDexAuthentication bool
}

func argsValidCreateEnvLock(cmdArgs *CreateEnvLockCommandLineArguments) (result bool, errorMessage string) {
	if len(cmdArgs.lockId.Values) != 1 {
		return false, "the --lockID arg must be set exactly once"
	}
	if len(cmdArgs.environment.Values) != 1 {
		return false, "the --environment arg must be set exactly once"
	}
	if len(cmdArgs.message.Values) > 1 {
		return false, "the --message arg must be set at most once"
	}

	return true, ""
}

// takes the raw command line flags and converts them to an intermediate represnetations for easy validation
func readCreateEnvLockArgs(args []string) (*CreateEnvLockCommandLineArguments, error) {
	cmdArgs := CreateEnvLockCommandLineArguments{} //exhaustruct:ignore

	fs := flag.NewFlagSet("flag set", flag.ContinueOnError)

	fs.Var(&cmdArgs.environment, "environment", "the environment to lock")
	fs.Var(&cmdArgs.lockId, "lockID", "the ID of the lock you are trying to create")
	fs.Var(&cmdArgs.message, "message", "lock message")
	fs.BoolVar(&cmdArgs.useDexAuthentication, "use_dex_auth", false, "use /api/* endpoint, if set to true, dex must be enabled and dex token must be provided otherwise the request will be denied")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("error while parsing command line arguments, error: %w", err)
	}

	if len(fs.Args()) != 0 { // kuberpult-cli release does not accept any positional arguments, so this is an error
		return nil, fmt.Errorf("these arguments are not recognized: \"%v\"", strings.Join(fs.Args(), " "))
	}

	if ok, msg := argsValidCreateEnvLock(&cmdArgs); !ok {
		return nil, fmt.Errorf(msg)
	}

	return &cmdArgs, nil
}

// converts the intermediate representation of the command line flags into the final structure containing parameters for the create lock endpoint
func convertToCreateEnvironmentLockParams(cmdArgs CreateEnvLockCommandLineArguments) (LockParameters, error) {
	if ok, msg := argsValidCreateEnvLock(&cmdArgs); !ok {
		// this should never happen, as the validation is already performed by the readArgs function
		return nil, fmt.Errorf("the provided command line arguments structure is invalid, cause: %s", msg)
	}

	rp := EnvironmentLockParameters{} //exhaustruct:ignore
	rp.LockId = cmdArgs.lockId.Values[0]
	rp.Environment = cmdArgs.environment.Values[0]
	if len(cmdArgs.message.Values) != 0 {
		rp.Message = cmdArgs.message.Values[0]
	}
	rp.UseDexAuthentication = cmdArgs.useDexAuthentication
	return &rp, nil
}

func ParseArgsCreateEnvironmentLock(args []string) (LockParameters, error) {
	cmdArgs, err := readCreateEnvLockArgs(args)
	if err != nil {
		return nil, fmt.Errorf("error while reading command line arguments, error: %w", err)
	}
	rp, err := convertToCreateEnvironmentLockParams(*cmdArgs)
	if err != nil {
		return nil, fmt.Errorf("error while creating /release endpoint params, error: %w", err)
	}

	return rp, nil
}
