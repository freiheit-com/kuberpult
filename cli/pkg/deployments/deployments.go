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
	"fmt"
	"net/http"
	urllib "net/url"

	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
	kutil "github.com/freiheit-com/kuberpult/cli/pkg/kuberpult_utils"
)

type CommitDeploymentsParameters struct {
	CommitId string
	OutFile  string
}

func HandleCommitDeployments(requestParams kutil.RequestParameters, authParams kutil.AuthenticationParameters, params *CommitDeploymentsParameters) error {
	req, err := createHttpRequest(*requestParams.Url, authParams, params)
	if err != nil {
		return fmt.Errorf("error while preparing HTTP request, error: %w", err)
	}
	body, err := cli_utils.IssueHttpRequestWithBodyReturn(*req)
	if err != nil {
		return fmt.Errorf("error while issuing HTTP request, error: %v", err)
	}
	if params.OutFile != "" {
		err = cli_utils.WriteToFile(params.OutFile, body)
		if err != nil {
			return fmt.Errorf("error while writing to file, error: %v", err)
		}
	} else {
		fmt.Println(string(body))
	}

	return nil
}

func createHttpRequest(url string, authParams kutil.AuthenticationParameters, parameters *CommitDeploymentsParameters) (*http.Request, error) {
	urlStruct, err := urllib.Parse(url)
	if err != nil {
		return nil, fmt.Errorf("the provided url %s is invalid, error: %w", url, err)
	}

	path := "/api/commit-deployments/" + parameters.CommitId

	req, err := http.NewRequest(http.MethodGet, urlStruct.JoinPath(path).String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating the HTTP request, error: %w", err)
	}

	if authParams.IapToken != nil {
		req.Header.Add("Proxy-Authorization", "Bearer "+*authParams.IapToken)
	}

	if authParams.DexToken != nil {
		req.Header.Add("Authorization", "Bearer "+*authParams.DexToken)
	}

	return req, nil
}
