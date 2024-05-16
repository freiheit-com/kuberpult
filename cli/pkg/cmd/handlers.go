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
	"log"
	"os"

	rl "github.com/freiheit-com/kuberpult/cli/pkg/release"
)

func handleRelease(url string, args []string) {
	parsedArgs, err := rl.ParseArgs(args)

	if err != nil {
		log.Fatalf("error while parsing command line args, error: %v", err)
	}

	if IAPToken, envVarExists := os.LookupEnv("KUBERPULT_IAP_TOKEN"); envVarExists {
		parsedArgs.IAPToken = &IAPToken
	}

	if err = rl.Release(url, parsedArgs); err != nil {
		log.Fatalf("error on release, error: %v", err)
	}
}
