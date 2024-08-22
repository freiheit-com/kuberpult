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
	"strings"

	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
)

type CreateAppLockCommandLineArguments struct {
	environment cli_utils.RepeatedString
	lockId      cli_utils.RepeatedString
	message     cli_utils.RepeatedString
	application cli_utils.RepeatedString
}

func argsValidCreateAppLock(cmdArgs *CreateAppLockCommandLineArguments) (result bool, errorMessage string) {
	if len(cmdArgs.lockId.Values) != 1 {
		return false, "the --lockID arg must be set exactly once"
	}
	if len(cmdArgs.environment.Values) != 1 {
		return false, "the --environment arg must be set exactly once"
	}
	if len(cmdArgs.application.Values) != 1 {
		return false, "the --application arg must be set exactly once"
	}
	if len(cmdArgs.message.Values) > 1 {
		return false, "the --message arg must be set at most once"
	}

	return true, ""
}

func readCreateAppLockArgs(args []string) (*CreateAppLockCommandLineArguments, error) {
	cmdArgs := CreateAppLockCommandLineArguments{} //exhaustruct:ignore

	fs := flag.NewFlagSet("flag set", flag.ContinueOnError)

	fs.Var(&cmdArgs.lockId, "lockID", "the ID of the lock you are trying to create")
	fs.Var(&cmdArgs.environment, "environment", "the environment to lock")
	fs.Var(&cmdArgs.message, "message", "lock message")
	fs.Var(&cmdArgs.application, "application", "application to lock")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("error while parsing command line arguments, error: %w", err)
	}

	if len(fs.Args()) != 0 { // kuberpult-cli release does not accept any positional arguments, so this is an error
		return nil, fmt.Errorf("these arguments are not recognized: \"%v\"", strings.Join(fs.Args(), " "))
	}

	if ok, msg := argsValidCreateAppLock(&cmdArgs); !ok {
		return nil, fmt.Errorf(msg)
	}

	return &cmdArgs, nil
}

func convertToCreateAppLockParams(cmdArgs CreateAppLockCommandLineArguments) (LockParameters, error) {
	if ok, msg := argsValidCreateAppLock(&cmdArgs); !ok {
		return nil, fmt.Errorf("the provided command line arguments structure is invalid, cause: %s", msg)
	}

	rp := CreateAppLockParameters{
		LockId:               cmdArgs.lockId.Values[0],
		Environment:          cmdArgs.environment.Values[0],
		Application:          cmdArgs.application.Values[0],
		Message:              "",
		UseDexAuthentication: false, //For now there is no ambiguity as to which endpoint to use
	}
	if len(cmdArgs.message.Values) != 0 {
		rp.Message = cmdArgs.message.Values[0]
	}
	return &rp, nil
}

func ParseArgsCreateAppLock(args []string) (LockParameters, error) {
	cmdArgs, err := readCreateAppLockArgs(args)
	if err != nil {
		return nil, fmt.Errorf("error while reading command line arguments for creating an app lock, error: %w", err)
	}
	rp, err := convertToCreateAppLockParams(*cmdArgs)
	if err != nil {
		return nil, fmt.Errorf("error while creating parameters for creating an application lock, error: %w", err)
	}
	return rp, nil
}

type DeleteAppLockCommandLineArguments struct {
	environment cli_utils.RepeatedString
	lockId      cli_utils.RepeatedString
	application cli_utils.RepeatedString
}

func argsValidDeleteAppLock(cmdArgs *DeleteAppLockCommandLineArguments) (result bool, errorMessage string) {
	if len(cmdArgs.lockId.Values) != 1 {
		return false, "the --lockID arg must be set exactly once"
	}
	if len(cmdArgs.environment.Values) != 1 {
		return false, "the --environment arg must be set exactly once"
	}
	if len(cmdArgs.application.Values) != 1 {
		return false, "the --application arg must be set exactly once"
	}

	return true, ""
}

func readDeleteAppLockArgs(args []string) (*DeleteAppLockCommandLineArguments, error) {
	cmdArgs := DeleteAppLockCommandLineArguments{} //exhaustruct:ignore

	fs := flag.NewFlagSet("flag set", flag.ContinueOnError)

	fs.Var(&cmdArgs.lockId, "lockID", "the ID of the lock you are trying to delete")
	fs.Var(&cmdArgs.environment, "environment", "the environment of the lock you are trying to delete")
	fs.Var(&cmdArgs.application, "application", "the application of the lock you are trying to delete")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("error while parsing command line arguments, error: %w", err)
	}

	if len(fs.Args()) != 0 { // kuberpult-cli release does not accept any positional arguments, so this is an error
		return nil, fmt.Errorf("these arguments are not recognized: \"%v\"", strings.Join(fs.Args(), " "))
	}

	if ok, msg := argsValidDeleteAppLock(&cmdArgs); !ok {
		return nil, fmt.Errorf(msg)
	}

	return &cmdArgs, nil
}

func convertToDeleteAppLockParams(cmdArgs DeleteAppLockCommandLineArguments) (LockParameters, error) {
	if ok, msg := argsValidDeleteAppLock(&cmdArgs); !ok {
		return nil, fmt.Errorf("the provided command line arguments structure is invalid, cause: %s", msg)
	}

	rp := DeleteAppLockParameters{
		LockId:               cmdArgs.lockId.Values[0],
		Environment:          cmdArgs.environment.Values[0],
		Application:          cmdArgs.application.Values[0],
		UseDexAuthentication: false,
	}
	return &rp, nil
}

func ParseArgsDeleteAppLock(args []string) (LockParameters, error) {
	cmdArgs, err := readDeleteAppLockArgs(args)
	if err != nil {
		return nil, fmt.Errorf("error while reading command line arguments for deleting an app lock, error: %w", err)
	}
	rp, err := convertToDeleteAppLockParams(*cmdArgs)
	if err != nil {
		return nil, fmt.Errorf("error while creating parameters for deleting an application lock, error: %w", err)
	}
	return rp, nil
}
