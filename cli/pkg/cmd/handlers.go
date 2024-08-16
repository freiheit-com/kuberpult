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
	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
	"github.com/freiheit-com/kuberpult/cli/pkg/locks"
	"log"

	kutil "github.com/freiheit-com/kuberpult/cli/pkg/kuberpult_utils"
	rl "github.com/freiheit-com/kuberpult/cli/pkg/release"
)

func handleRelease(kpClientParams kuberpultClientParameters, args []string) ReturnCode {
	parsedArgs, err := rl.ParseArgs(args)

	if err != nil {
		log.Printf("error while parsing command line args, error: %v", err)
		return ReturnCodeInvalidArguments
	}

	authParams := kutil.AuthenticationParameters{
		IapToken:    kpClientParams.iapToken,
		DexToken:    kpClientParams.dexToken,
		AuthorName:  kpClientParams.authorName,
		AuthorEmail: kpClientParams.authorEmail,
	}

	requestParameters := kutil.RequestParameters{
		Url:         &kpClientParams.url,
		Retries:     kpClientParams.retries,
		HttpTimeout: cli_utils.HttpDefaultTimeout,
	}

	if err = rl.Release(requestParameters, authParams, *parsedArgs); err != nil {
		log.Printf("error on release, error: %v", err)
		return ReturnCodeFailure
	}
	return ReturnCodeSuccess
}

func handleCreateEnvLock(kpClientParams kuberpultClientParameters, args []string) ReturnCode {
	parsedArgs, err := locks.ParseArgsCreateEnvironmentLock(args)

	if err != nil {
		log.Printf("error while parsing command line args, error: %v", err)
		return ReturnCodeInvalidArguments
	}
	return handleCreateLock(kpClientParams, parsedArgs)
}

func handleCreateAppLock(kpClientParams kuberpultClientParameters, args []string) ReturnCode {
	parsedArgs, err := locks.ParseArgsCreateAppLock(args)

	if err != nil {
		log.Printf("error while parsing command line args, error: %v", err)
		return ReturnCodeInvalidArguments
	}
	return handleCreateLock(kpClientParams, parsedArgs)
}

func handleCreateTeamLock(kpClientParams kuberpultClientParameters, args []string) ReturnCode {
	parsedArgs, err := locks.ParseArgsCreateTeamLock(args)

	if err != nil {
		log.Printf("error while parsing command line args, error: %v", err)
		return ReturnCodeInvalidArguments
	}
	return handleCreateLock(kpClientParams, parsedArgs)
}

func handleCreateLock(kpClientParams kuberpultClientParameters, parsedArgs locks.LockParameters) ReturnCode {
	authParams := kutil.AuthenticationParameters{
		IapToken:    kpClientParams.iapToken,
		DexToken:    kpClientParams.dexToken,
		AuthorName:  kpClientParams.authorName,
		AuthorEmail: kpClientParams.authorEmail,
	}

	requestParameters := kutil.RequestParameters{
		Url:         &kpClientParams.url,
		Retries:     kpClientParams.retries,
		HttpTimeout: cli_utils.HttpDefaultTimeout,
	}

	if err := locks.CreateLock(requestParameters, authParams, parsedArgs); err != nil {
		log.Printf("error creating lock, error: %v", err)
		return ReturnCodeFailure
	}
	return ReturnCodeSuccess
}
