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

Copyright freiheit.com
*/

package cli_utils

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	HttpDefaultTimeout = 180
)

func doRequest(request *http.Request) (*http.Response, []byte, error) {
	//exhaustruct:ignore
	client := &http.Client{
		Timeout: HttpDefaultTimeout * time.Second,
	}

	resp, err := client.Do(request)
	if err != nil {
		return nil, nil, fmt.Errorf("error issuing the HTTP request, error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to read response: %v with error: %w", resp, err)
	}

	return resp, body, nil
}

func IssueHttpRequest(req http.Request, retries uint64) error {
	var i uint64
	for i = 0; i < retries+1; i++ {
		response, body, err := doRequest(&req)
		if err != nil {
			log.Printf("error issuing http request: %v\n", err)
		} else if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
			log.Printf("Recieved response code %d - %s from Kuberpult\nResponse body:\n%s\n", response.StatusCode, http.StatusText(response.StatusCode), string(body))
		} else {
			log.Printf("Success: %d - %s\nResponse body:\n%s\n", response.StatusCode, http.StatusText(response.StatusCode), string(body))
			return nil
		}
		if i < retries {
			backoff := time.Duration(i+1) * time.Second
			log.Printf("Retrying in %v...\n", backoff)
			time.Sleep(backoff)
		}
	}
	return fmt.Errorf("could not perform a successful call to kuberpult")
}
