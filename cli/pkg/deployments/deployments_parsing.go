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

package deployments

import (
	"flag"
	"fmt"
	"strings"
)

func ParseArgsCommitDeployments(args []string) (*CommitDeploymentsParameters, error) {
	cmdArgs := CommitDeploymentsParameters{
		CommitId: "",
		OutFile:  "",
	}

	fs := flag.NewFlagSet("flag set", flag.ContinueOnError)
	fs.StringVar(&cmdArgs.CommitId, "commit", "", "the commit you want to get the deployments for")
	fs.StringVar(&cmdArgs.OutFile, "out", "", "the file to write the deployments to (default is stdout)")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("error while parsing command line arguments, error: %w", err)
	}

	if len(fs.Args()) != 0 { // kuberpult-cli release does not accept any positional arguments, so this is an error
		return nil, fmt.Errorf("these arguments are not recognized: \"%v\"", strings.Join(fs.Args(), " "))
	}
	if cmdArgs.CommitId == "" {
		return nil, fmt.Errorf("the commit hash must be set with the --commit flag")
	}
	if !validateCommitHash(cmdArgs.CommitId) {
		return nil, fmt.Errorf("the commit hash: %s is invalid", cmdArgs.CommitId)
	}

	return &cmdArgs, nil
}

func validateCommitHash(commit string) bool {
	if len(commit) != 40 {
		return false
	}
	commit = strings.ToLower(commit)
	for _, c := range commit {
		if c < '0' || c > 'f' {
			return false
		}
	}
	return true
}
