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
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

func prepareHttpRequest(parsedArgs *releaseParameters) (*http.Request, error) {
	url := "http://localhost:3000/release"

	form := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(form)
	
	if err := writer.WriteField("application", parsedArgs.Application); err != nil {
		return nil, fmt.Errorf("error writing application field, error: %w", err)
	}

	for environment, manifestPath := range parsedArgs.ManifestFiles {
		part, err := writer.CreateFormFile(fmt.Sprintf("manifests[%s]", environment), manifestPath)
		if err != nil {
			return nil, fmt.Errorf("error creating the form entry for environment %s with manifest file %s, error: %w", environment, manifestPath, err)
		}
		manifestFile, err := os.Open(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("error opening the manifest file %s for environment %s, error: %w", manifestPath, environment, err)
		}
		defer manifestFile.Close()

		_, err = io.Copy(part, manifestFile)
		if err != nil {
			return nil, fmt.Errorf("error writing the manifest file %s for environment %s into the form field, error: %w", manifestPath, environment, err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("error closing the writer, error: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, form)
	if err != nil {
		return nil, fmt.Errorf("error creating the HTTP request, error: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	return req, nil
}

func issueHttpRequest(parsedArgs *releaseParameters) error {
	req, err := prepareHttpRequest(parsedArgs)
	if err != nil {
		return fmt.Errorf("error while preparing HTTP request, error: %w", err)
	}

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