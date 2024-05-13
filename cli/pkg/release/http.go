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

package release

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
)

func prepareHttpRequest(url string, parsedArgs *ReleaseParameters) (*http.Request, error) {
	form := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(form)
	
	if err := writer.WriteField("application", parsedArgs.Application); err != nil {
		return nil, fmt.Errorf("error writing application field, error: %w", err)
	}

	for environment, manifest := range parsedArgs.Manifests {
		part, err := writer.CreateFormFile(fmt.Sprintf("manifests[%s]", environment), fmt.Sprintf("%s-manifest", environment))
		if err != nil {
			writer.Close()
			return nil, fmt.Errorf("error creating the form entry for environment %s with manifest file %s, error: %w", environment, manifest, err)
		}
		_, err = part.Write([]byte(manifest))
		if err != nil {
			writer.Close()
			return nil, fmt.Errorf("error writing the form entry for environment %s with manifest file %s, error: %w", environment, manifest, err)
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

func issueHttpRequest(url string, parsedArgs *ReleaseParameters) error {
	req, err := prepareHttpRequest(url, parsedArgs)
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
		return fmt.Errorf("response was not OK or Accepted, response code: %v", resp.StatusCode)
	}

	return nil
}
