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
	"fmt"

	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
	kutil "github.com/freiheit-com/kuberpult/cli/pkg/kuberpult_utils"
)

// a representation of the parameters of the /release endpoint
type ReleaseParameters struct {
	Application          string
	Manifests            map[string][]byte // key is name of the environment and value is the manifest file name
	Signatures           map[string][]byte // key is name of the environment and value is the signature file name
	Team                 *string
	SourceCommitId       *string
	PreviousCommitId     *string
	SourceAuthor         *string
	SourceMessage        *string
	Version              *uint64
	Revision             *uint64
	DisplayVersion       *string
	CiLink               *string
	UseDexAuthentication bool
	IsPrepublish         bool
	DryRun               bool
}

// Release calls the release endpoint with the specified parameters
// this function might be used in the future for programmatic interaction with Kuberpult, hence its separation
func Release(requestParams kutil.RequestParameters, authParams kutil.AuthenticationParameters, params ReleaseParameters) error {
	req, err := prepareHttpReleaseRequest(*requestParams.Url, authParams, params)
	if err != nil {
		return fmt.Errorf("error while preparing HTTP request, error: %w", err)
	}
	if err := cli_utils.IssueHttpRequest(*req, requestParams.Retries, requestParams.HttpTimeout); err != nil {
		return fmt.Errorf("error while issuing HTTP request, error: %v", err)
	}
	return nil
}

// GetManifests calls the API to retrieve current manifests
func GetManifests(requestParams kutil.RequestParameters, authParams kutil.AuthenticationParameters, params ReleaseParameters) (string, error) {
	req, err := prepareHttpManifestRequest(*requestParams.Url, authParams, params)
	if err != nil {
		return "", fmt.Errorf("error while preparing HTTP request, error: %w", err)
	}
	body, notFound, err := cli_utils.IssueHttpRequestWithBodyReturnAllowNotFound(*req, requestParams.HttpTimeout)
	if notFound {
		return "", nil
	}
	if err != nil || body == nil {
		return "", fmt.Errorf("error while issuing HTTP request, error: %v", err)
	}
	return string(body), nil
}
