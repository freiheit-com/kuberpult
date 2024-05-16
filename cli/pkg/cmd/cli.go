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

func RunCLI() {
	params, other, err := parseArgs(os.Args[1:])
	if err != nil {
		log.Fatalf("error while parsing command line arguments, error: %v", err)
	}

	if len(other) == 0 {
		log.Fatalf("a subcommand must be specified")
	}

	var iapToken *string
	if envVar, envVarExists := os.LookupEnv("KUBERPULT_IAP_TOKEN"); envVarExists {
		iapToken = &envVar
	}
	
	subcommand := other[0]
	subflags := other[1:]

	switch subcommand {
	case "help":
		fmt.Println(helpMessage)
	case "release":
		handleRelease(params.url, iapToken, subflags)
	default:
		log.Fatalf("unknown subcommand %s", subcommand)
	}
}
