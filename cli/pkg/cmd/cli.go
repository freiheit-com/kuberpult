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
	"fmt"
	"log"
	"os"
)

type ReturnCode int

const (
	ReturnCodeSuccess          = 0 //Success
	ReturnCodeFailure          = 1 //Error on kuberpult interaction
	ReturnCodeInvalidArguments = 2 //Error on CLI usage
)

type kuberpultClientParameters struct {
	url         string
	retries     uint64
	authorName  *string
	authorEmail *string
	iapToken    *string
	dexToken    *string
}

func RunCLI() ReturnCode {
	kpClientParams, other, err := parseArgs(os.Args[1:])
	if err != nil {
		log.Printf("error while parsing command line arguments, error: %v", err)
		return ReturnCodeInvalidArguments
	}

	if len(other) == 0 {
		log.Println("a subcommand must be specified")
		return ReturnCodeInvalidArguments
	}

	subcommand := other[0]
	subflags := other[1:]

	if envVar, envVarExists := os.LookupEnv("KUBERPULT_IAP_TOKEN"); envVarExists {
		kpClientParams.iapToken = &envVar
	}

	if envVar, envVarExists := os.LookupEnv("KUBERPULT_DEX_ACCESS_TOKEN"); envVarExists {
		kpClientParams.dexToken = &envVar
	}
	switch subcommand {
	case "help":
		fmt.Println(helpMessage)
		return ReturnCodeSuccess
	case "release":
		return handleRelease(*kpClientParams, subflags)
	default:
		log.Printf("unknown subcommand %s\n", subcommand)
		return ReturnCodeInvalidArguments
	}
}
