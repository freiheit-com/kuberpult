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

package release

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"

	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
)

type cmdArguments struct {
	application   cli_utils.RepeatedString
	environments  cli_utils.RepeatedString
	manifestFiles cli_utils.RepeatedString
}

func readArgs(args []string) (*cmdArguments, error) {
	cmdArgs := cmdArguments{}

	fs := flag.NewFlagSet("flag set", flag.ExitOnError)

	fs.Var(&cmdArgs.application, "application", "the name of the application to deploy (must be set exactly once)")
	fs.Var(&cmdArgs.environments, "environment", "an environment to deploy to (must have --manifest set immediately afterwards)")
	fs.Var(&cmdArgs.manifestFiles, "manifest", "the name of the file containing manifests to be deployed (must be set immediately after --environment)")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("error while parsing command line arguments, error: %w", err)
	}

	if len(cmdArgs.application.Values) != 1 {
		return nil, fmt.Errorf("the --application arg must be set exactly once")
	}

	for i, arg := range args {
		if arg == "--environment" {
			if i+2 >= len(args) || args[i+2] != "--manifest" {
				return nil, fmt.Errorf("all --environment args must have a --manifest arg set immediately afterwards")
			}
		}
		if arg == "--manifest" {
			if i-2 < 0 || args[i-2] != "--environment" {
				return nil, fmt.Errorf("all --manifest args must be set immediately after an --environment arg")
			}
		}
	}

	if len(cmdArgs.environments.Values) != len(cmdArgs.manifestFiles.Values) {
		return nil, fmt.Errorf("the args --environment and --manifest must be set an equal number of times")
	}

	for _, manifestFile := range cmdArgs.manifestFiles.Values {
		if _, err := os.Stat(manifestFile); err != nil {
			return nil, fmt.Errorf("error while running stat on file %s, error: %w", manifestFile, err)
		}
	}
	return &cmdArgs, nil
}

type releaseParameters struct {
	Application string
	ManifestFiles   map[string]string
}

func parseArgs(args []string) (*releaseParameters, error) {
	cmdArgs, err := readArgs(args)
	if err != nil {
		return nil, fmt.Errorf("error while reading command line arguments, error: %w", err)
	}

	rp := releaseParameters{}
	rp.Application = cmdArgs.application.Values[0]
	rp.ManifestFiles = make(map[string]string)
	for i := range cmdArgs.environments.Values {
		manifestFile := cmdArgs.manifestFiles.Values[i]
		environemnt := cmdArgs.environments.Values[i]

		rp.ManifestFiles[environemnt] = manifestFile
	}

	return &rp, nil
}

func issueHttpRequest(parsedArgs *releaseParameters) error {
	url := "http://localhost:3000/release"

	form := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(form)
	
	if err := writer.WriteField("application", parsedArgs.Application); err != nil {
		return fmt.Errorf("error writing application field, error: %w", err)
	}

	for environment, manifestPath := range parsedArgs.ManifestFiles {
		part, err := writer.CreateFormFile(fmt.Sprintf("manifests[%s]", environment), manifestPath)
		if err != nil {
			return fmt.Errorf("error creating the form entry for environment %s with manifest file %s, error: %w", environment, manifestPath, err)
		}
		manifestFile, err := os.Open(manifestPath)
		if err != nil {
			return fmt.Errorf("error opening the manifest file %s for environment %s, error: %w", manifestPath, environment, err)
		}
		defer manifestFile.Close()

		_, err = io.Copy(part, manifestFile)
		if err != nil {
			return fmt.Errorf("error writing the manifest file %s for environment %s into the form field, error: %w", manifestPath, environment, err)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("error closing the writer, error: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, form)
	if err != nil {
		return fmt.Errorf("error creating the HTTP request, error: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error issuing the HTTP request, error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("response was not OK or Accepted, response: %v", resp)
	}

	return nil
}

func Handle(args []string) {
	parsedArgs, err := parseArgs(args)

	if err != nil {
		log.Fatalf("error while parsing command line args, error: %v", err)
	}
	
	if err = issueHttpRequest(parsedArgs); err != nil {
		log.Fatalf("error while issuing HTTP request, error: %v", err)
	}
}
