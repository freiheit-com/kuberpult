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
	"net/http"
	"strings"
)

type CreateEnvLockCommandLineArguments struct {
	environment cli_utils.RepeatedString
	lockId      cli_utils.RepeatedString
	message     cli_utils.RepeatedString
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

	rp := CreateEnvironmentLockParameters{
		LockId:               cmdArgs.lockId.Values[0],
		Environment:          cmdArgs.environment.Values[0],
		UseDexAuthentication: false, //For now there is no ambiguity as to which endpoint to use
		Message:              "",
		HttpMethod:           http.MethodPut,
	}
	if len(cmdArgs.message.Values) != 0 {
		rp.Message = cmdArgs.message.Values[0]
	}
	return &rp, nil
}

func ParseArgsCreateEnvironmentLock(args []string) (LockParameters, error) {
	cmdArgs, err := readCreateEnvLockArgs(args)
	if err != nil {
		return nil, fmt.Errorf("error while reading command line arguments for env locks, error: %w", err)
	}
	rp, err := convertToCreateEnvironmentLockParams(*cmdArgs)
	if err != nil {
		return nil, fmt.Errorf("error while creating parameters for creating an environment lock, error: %w", err)
	}

	return rp, nil
}

type DeleteEnvLockCommandLineArguments struct {
	environment cli_utils.RepeatedString
	lockId      cli_utils.RepeatedString
}

func argsValidDeleteEnvLock(cmdArgs *DeleteEnvLockCommandLineArguments) (result bool, errorMessage string) {
	if len(cmdArgs.lockId.Values) != 1 {
		return false, "the --lockID arg must be set exactly once"
	}
	if len(cmdArgs.environment.Values) != 1 {
		return false, "the --environment arg must be set exactly once"
	}

	return true, ""
}

// takes the raw command line flags and converts them to an intermediate represnetations for easy validation
func readDeleteEnvLockArgs(args []string) (*DeleteEnvLockCommandLineArguments, error) {
	cmdArgs := DeleteEnvLockCommandLineArguments{} //exhaustruct:ignore

	fs := flag.NewFlagSet("flag set", flag.ContinueOnError)

	fs.Var(&cmdArgs.environment, "environment", "the environment to delete the lock for")
	fs.Var(&cmdArgs.lockId, "lockID", "the ID of the lock you are trying to delete")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("error while parsing command line arguments, error: %w", err)
	}

	if len(fs.Args()) != 0 { // kuberpult-cli release does not accept any positional arguments, so this is an error
		return nil, fmt.Errorf("these arguments are not recognized: \"%v\"", strings.Join(fs.Args(), " "))
	}

	if ok, msg := argsValidDeleteEnvLock(&cmdArgs); !ok {
		return nil, fmt.Errorf(msg)
	}

	return &cmdArgs, nil
}

// converts the intermediate representation of the command line flags into the final structure containing parameters for the create lock endpoint
func convertToDeleteEnvironmentLockParams(cmdArgs DeleteEnvLockCommandLineArguments) (LockParameters, error) {
	if ok, msg := argsValidDeleteEnvLock(&cmdArgs); !ok {
		// this should never happen, as the validation is already performed by the readArgs function
		return nil, fmt.Errorf("the provided command line arguments structure is invalid, cause: %s", msg)
	}
	return &DeleteEnvironmentLockParameters{
		LockId:               cmdArgs.lockId.Values[0],
		Environment:          cmdArgs.environment.Values[0],
		UseDexAuthentication: false,
	}, nil
}

func ParseArgsDeleteEnvironmentLock(args []string) (LockParameters, error) {
	cmdArgs, err := readDeleteEnvLockArgs(args)
	if err != nil {
		return nil, fmt.Errorf("error while reading command line arguments for deleting env locks, error: %w", err)
	}
	rp, err := convertToDeleteEnvironmentLockParams(*cmdArgs)
	if err != nil {
		return nil, fmt.Errorf("error while creating parameters for deleting an environment lock, error: %w", err)
	}
	return rp, nil
}
