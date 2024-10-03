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

package releasetrain

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	urllib "net/url"

	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
	kutil "github.com/freiheit-com/kuberpult/cli/pkg/kuberpult_utils"
)

type ReleaseTrainJsonData struct {
	CiLink string `json:"ciLink,omitempty"`
}

type ReleaseTrainParameters struct {
	TargetEnvironment    string
	Team                 *string
	CiLink               *string
	UseDexAuthentication bool
}

func HandleReleaseTrain(requestParams kutil.RequestParameters, authParams kutil.AuthenticationParameters, params ReleaseTrainParameters) error {
	req, err := createHttpRequest(*requestParams.Url, authParams, params)
	if err != nil {
		return fmt.Errorf("error while preparing HTTP request, error: %w", err)
	}
	if err := cli_utils.IssueHttpRequest(*req, requestParams.Retries); err != nil {
		return fmt.Errorf("error while issuing HTTP request, error: %v", err)
	}
	return nil
}

func createHttpRequest(url string, authParams kutil.AuthenticationParameters, parameters ReleaseTrainParameters) (*http.Request, error) {
	urlStruct, err := urllib.Parse(url)
	if err != nil {
		return nil, fmt.Errorf("the provided url %s is invalid, error: %w", url, err)
	}

	prefix := "environments"

	if parameters.UseDexAuthentication {
		prefix = "api/environments"
	}

	path := fmt.Sprintf("%s/%s/releasetrain", prefix, parameters.TargetEnvironment)

	if parameters.Team != nil {
		values := urlStruct.Query()
		values.Add("team", *parameters.Team)
		urlStruct.RawQuery = values.Encode()
	}

	var jsonData []byte
	if parameters.CiLink != nil {
		d := &ReleaseTrainJsonData{
			CiLink: *parameters.CiLink,
		}

		jsonData, err = json.Marshal(d)
		if err != nil {
			return nil, fmt.Errorf("Could not EnvironmentLockParameters data to json: %w\n", err)
		}
	}

	req, err := http.NewRequest(http.MethodPut, urlStruct.JoinPath(path).String(), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating the HTTP request, error: %w", err)
	}

	if authParams.IapToken != nil {
		req.Header.Add("Proxy-Authorization", "Bearer "+*authParams.IapToken)
	}

	if authParams.DexToken != nil {
		req.Header.Add("Authorization", "Bearer "+*authParams.DexToken)
	}

	if authParams.AuthorName != nil {
		req.Header.Add("author-name", base64.StdEncoding.EncodeToString([]byte(*authParams.AuthorName)))
	}

	if authParams.AuthorEmail != nil {
		req.Header.Add("author-email", base64.StdEncoding.EncodeToString([]byte(*authParams.AuthorEmail)))
	}

	return req, nil
}
