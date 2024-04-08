/*
This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright 2023 freiheit.com
*/
package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/freiheit-com/kuberpult/cli/pkg/release"
)

func RunCLI() {
	if len(os.Args) < 2 {
		log.Fatalf("a subcommand must be specified, run \"kuberpult-client help\" for more information")
		os.Exit(1)
	}

	subcommand := os.Args[1]
	flags := os.Args[2:]

	switch subcommand {
	case "help":
		fmt.Println(helpMessage)
	case "release":
		release.Handle(flags)
	default:
		log.Fatalf("unknown subcommand %s", subcommand)
	}
}
