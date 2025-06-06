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
	"errors"
	"flag"
	"fmt"
	"net/url"
	"strings"

	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
)

type CreateTeamLockCommandLineArguments struct {
	environment       cli_utils.RepeatedString
	lockId            cli_utils.RepeatedString
	message           cli_utils.RepeatedString
	team              cli_utils.RepeatedString
	ciLink            cli_utils.RepeatedString
	suggestedLifetime cli_utils.RepeatedString
}

func argsValidCreateTeamLock(cmdArgs *CreateTeamLockCommandLineArguments) (result bool, errorMessage string) {
	if len(cmdArgs.lockId.Values) != 1 {
		return false, "the --lockID arg must be set exactly once"
	}
	if len(cmdArgs.environment.Values) != 1 {
		return false, "the --environment arg must be set exactly once"
	}
	if len(cmdArgs.team.Values) != 1 {
		return false, "the --team arg must be set exactly once"
	}
	if len(cmdArgs.message.Values) > 1 {
		return false, "the --message arg must be set at most once"
	}
	if len(cmdArgs.ciLink.Values) > 1 {
		return false, "the --ci_link arg must be set at most once"
	} else if len(cmdArgs.ciLink.Values) == 1 {
		_, err := url.ParseRequestURI(cmdArgs.ciLink.Values[0])
		if err != nil {
			return false, fmt.Sprintf("provided invalid --ci_link value '%s'", cmdArgs.ciLink.Values[0])
		}
	}
	if len(cmdArgs.suggestedLifetime.Values) > 1 {
		return false, "the --suggested_lifetime arg must be set at most once"
	} else if len(cmdArgs.suggestedLifetime.Values) == 1 {
		if !cli_utils.IsValidLifeTime(cmdArgs.suggestedLifetime.Values[0]) {
			return false, fmt.Sprintf("provided invalid --suggested_lifetime value '%s'", cmdArgs.suggestedLifetime.Values[0])
		}
	}

	return true, ""
}

func readCreateTeamLockArgs(args []string) (*CreateTeamLockCommandLineArguments, error) {
	cmdArgs := CreateTeamLockCommandLineArguments{} //exhaustruct:ignore

	fs := flag.NewFlagSet("flag set", flag.ContinueOnError)

	fs.Var(&cmdArgs.lockId, "lockID", "the ID of the lock you are trying to create")
	fs.Var(&cmdArgs.environment, "environment", "the environment to lock")
	fs.Var(&cmdArgs.message, "message", "lock message")
	fs.Var(&cmdArgs.team, "team", "team to lock")
	fs.Var(&cmdArgs.ciLink, "ci_link", "the link to the CI run that created this lock")
	fs.Var(&cmdArgs.suggestedLifetime, "suggested_lifetime", "the suggested lifetime for the lock e.g. 4h, 2d")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("error while parsing command line arguments, error: %w", err)
	}

	if len(fs.Args()) != 0 { // kuberpult-cli release does not accept any positional arguments, so this is an error
		return nil, fmt.Errorf("these arguments are not recognized: \"%v\"", strings.Join(fs.Args(), " "))
	}

	if ok, msg := argsValidCreateTeamLock(&cmdArgs); !ok {
		return nil, errors.New(msg)
	}

	return &cmdArgs, nil
}

// converts the intermediate representation of the command line flags into the final structure containing parameters for the release endpoint
func convertToCreateTeamLockParams(cmdArgs CreateTeamLockCommandLineArguments) (LockParameters, error) {
	if ok, msg := argsValidCreateTeamLock(&cmdArgs); !ok {
		// this should never happen, as the validation is already peformed by the readArgs function
		return nil, fmt.Errorf("the provided command line arguments structure is invalid, cause: %s", msg)
	}

	rp := CreateTeamLockParameters{
		LockId:               cmdArgs.lockId.Values[0],
		Environment:          cmdArgs.environment.Values[0],
		Team:                 cmdArgs.team.Values[0],
		UseDexAuthentication: true, //For now there is no ambiguity as to which endpoint to use
		Message:              "",
		CiLink:               nil,
		SuggestedLifeTime:    nil,
	}
	if len(cmdArgs.message.Values) != 0 {
		rp.Message = cmdArgs.message.Values[0]
	}
	if len(cmdArgs.ciLink.Values) == 1 {
		rp.CiLink = &cmdArgs.ciLink.Values[0]
	}
	if len(cmdArgs.suggestedLifetime.Values) == 1 {
		rp.SuggestedLifeTime = &cmdArgs.suggestedLifetime.Values[0]
	}
	return &rp, nil
}

func ParseArgsCreateTeamLock(args []string) (LockParameters, error) {
	cmdArgs, err := readCreateTeamLockArgs(args)
	if err != nil {
		return nil, fmt.Errorf("error while reading command line arguments for team lock, error: %w", err)
	}
	rp, err := convertToCreateTeamLockParams(*cmdArgs)
	if err != nil {
		return nil, fmt.Errorf("error while creating parameters for team lock, error: %w", err)
	}
	return rp, nil
}

type DeleteTeamLockCommandLineArguments struct {
	environment cli_utils.RepeatedString
	lockId      cli_utils.RepeatedString
	team        cli_utils.RepeatedString
}

func argsValidDeleteTeamLock(cmdArgs *DeleteTeamLockCommandLineArguments) (result bool, errorMessage string) {
	if len(cmdArgs.lockId.Values) != 1 {
		return false, "the --lockID arg must be set exactly once"
	}
	if len(cmdArgs.environment.Values) != 1 {
		return false, "the --environment arg must be set exactly once"
	}
	if len(cmdArgs.team.Values) != 1 {
		return false, "the --team arg must be set exactly once"
	}

	return true, ""
}

func readDeleteTeamLockArgs(args []string) (*DeleteTeamLockCommandLineArguments, error) {
	cmdArgs := DeleteTeamLockCommandLineArguments{} //exhaustruct:ignore

	fs := flag.NewFlagSet("flag set", flag.ContinueOnError)

	fs.Var(&cmdArgs.lockId, "lockID", "the ID of the lock you are trying to delete")
	fs.Var(&cmdArgs.environment, "environment", "the environment of the lock you are trying to delete")
	fs.Var(&cmdArgs.team, "team", "the team of the lock you are trying to delete")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("error while parsing command line arguments, error: %w", err)
	}

	if len(fs.Args()) != 0 { // kuberpult-cli release does not accept any positional arguments, so this is an error
		return nil, fmt.Errorf("these arguments are not recognized: \"%v\"", strings.Join(fs.Args(), " "))
	}

	if ok, msg := argsValidDeleteTeamLock(&cmdArgs); !ok {
		return nil, errors.New(msg)
	}

	return &cmdArgs, nil
}

// converts the intermediate representation of the command line flags into the final structure containing parameters for the release endpoint
func convertToDeleteTeamLockParams(cmdArgs DeleteTeamLockCommandLineArguments) (LockParameters, error) {
	if ok, msg := argsValidDeleteTeamLock(&cmdArgs); !ok {
		// this should never happen, as the validation is already peformed by the readArgs function
		return nil, fmt.Errorf("the provided command line arguments structure is invalid, cause: %s", msg)
	}

	rp := DeleteTeamLockParameters{
		LockId:               cmdArgs.lockId.Values[0],
		Environment:          cmdArgs.environment.Values[0],
		Team:                 cmdArgs.team.Values[0],
		UseDexAuthentication: true,
	}
	return &rp, nil
}

func ParseArgsDeleteTeamLock(args []string) (LockParameters, error) {
	cmdArgs, err := readDeleteTeamLockArgs(args)
	if err != nil {
		return nil, fmt.Errorf("error while reading command line arguments for team lock, error: %w", err)
	}
	rp, err := convertToDeleteTeamLockParams(*cmdArgs)
	if err != nil {
		return nil, fmt.Errorf("error while creating parameters for deleting team lock, error: %w", err)
	}
	return rp, nil
}
