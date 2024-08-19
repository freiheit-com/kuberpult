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

package locks

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
	kutil "github.com/freiheit-com/kuberpult/cli/pkg/kuberpult_utils"
	"net/http"
	urllib "net/url"
)

type LockParameters interface {
	FillHttpInfo() (*HttpInfo, error)
}

type CreateEnvironmentLockParameters struct {
	Environment          string
	LockId               string
	Message              string
	HttpMethod           string
	UseDexAuthentication bool
}

type DeleteEnvironmentLockParameters struct {
	Environment          string
	LockId               string
	UseDexAuthentication bool
}

type CreateAppLockParameters struct {
	Environment          string
	LockId               string
	Message              string
	Application          string
	HttpMethod           string
	UseDexAuthentication bool
}
type DeleteAppLockParameters struct {
	Environment          string
	LockId               string
	Application          string
	UseDexAuthentication bool
}

type CreateTeamLockParameters struct {
	Environment          string
	LockId               string
	Message              string
	Team                 string
	HttpMethod           string
	UseDexAuthentication bool
}

type DeleteTeamLockParameters struct {
	Environment          string
	LockId               string
	Team                 string
	UseDexAuthentication bool
}

type EnvironmentGroupLockParameters struct {
	EnvironmentGroup     string
	LockId               string
	Message              string
	HttpMethod           string
	UseDexAuthentication bool
}

type LockJsonData struct {
	Message string `json:"message"`
}

type HttpInfo struct {
	jsonData    []byte
	ContentType string
	HttpMethod  string
	RestPath    string
}

func HandleLockRequest(requestParams kutil.RequestParameters, authParams kutil.AuthenticationParameters, params LockParameters) error {
	data, err := params.FillHttpInfo()
	if err != nil {
		return fmt.Errorf("error while preparing HTTP request. Could not fill form error: %w", err)
	}
	req, err := createHttpRequest(*requestParams.Url, authParams, data)

	if err != nil {
		return fmt.Errorf("error while preparing HTTP request, error: %w", err)
	}
	if err := cli_utils.IssueHttpRequest(*req, requestParams.Retries); err != nil {
		return fmt.Errorf("error while issuing HTTP request, error: %v", err)
	}
	return nil
}

func (e *CreateEnvironmentLockParameters) FillHttpInfo() (*HttpInfo, error) {
	d := LockJsonData{
		Message: e.Message,
	}
	var jsonData, err = json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("Could not EnvironmentLockParameters data to json: %w\n", err)
	}
	prefix := "environments"
	if e.UseDexAuthentication {
		prefix = "api/environments"
	}
	return &HttpInfo{
		jsonData:    jsonData,
		ContentType: "application/json",
		HttpMethod:  http.MethodPut,
		RestPath:    fmt.Sprintf("%s/%s/locks/%s", prefix, e.Environment, e.LockId),
	}, nil
}

func (e *DeleteEnvironmentLockParameters) FillHttpInfo() (*HttpInfo, error) {
	prefix := "environments"
	if e.UseDexAuthentication {
		prefix = "api/environments"
	}
	return &HttpInfo{
		jsonData:    []byte{},
		ContentType: "application/json",
		HttpMethod:  http.MethodDelete,
		RestPath:    fmt.Sprintf("%s/%s/locks/%s", prefix, e.Environment, e.LockId),
	}, nil
}

func (e *CreateAppLockParameters) FillHttpInfo() (*HttpInfo, error) {
	d := LockJsonData{
		Message: e.Message,
	}
	var jsonData, err = json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("Could not marshal CreateAppLockParameters data to json: %w\n", err)
	}
	prefix := "environments"
	if e.UseDexAuthentication {
		prefix = "api/environments"
	}
	return &HttpInfo{
		jsonData:    jsonData,
		ContentType: "application/json",
		HttpMethod:  http.MethodPut,
		RestPath:    fmt.Sprintf("%s/%s/applications/%s/locks/%s", prefix, e.Environment, e.Application, e.LockId),
	}, nil
}

func (e *DeleteAppLockParameters) FillHttpInfo() (*HttpInfo, error) {
	prefix := "environments"
	if e.UseDexAuthentication {
		prefix = "api/environments"
	}
	return &HttpInfo{
		jsonData:    []byte{},
		ContentType: "application/json",
		HttpMethod:  http.MethodDelete,
		RestPath:    fmt.Sprintf("%s/%s/applications/%s/locks/%s", prefix, e.Environment, e.Application, e.LockId),
	}, nil
}

func (e *CreateTeamLockParameters) FillHttpInfo() (*HttpInfo, error) {
	d := LockJsonData{
		Message: e.Message,
	}
	var jsonData, err = json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("Could not marshal CreateTeamLockParameters data to json: %w\n", err)
	}
	prefix := "environments"
	if e.UseDexAuthentication {
		prefix = "api/environments"
	}
	return &HttpInfo{
		jsonData:    jsonData,
		ContentType: "application/json",
		HttpMethod:  http.MethodPut,
		RestPath:    fmt.Sprintf("%s/%s/lock/team/%s/%s", prefix, e.Environment, e.Team, e.LockId),
	}, nil
}

func (e *DeleteTeamLockParameters) FillHttpInfo() (*HttpInfo, error) {
	prefix := "environments"
	if e.UseDexAuthentication {
		prefix = "api/environments"
	}
	return &HttpInfo{
		jsonData:    []byte{},
		ContentType: "application/json",
		HttpMethod:  http.MethodDelete,
		RestPath:    fmt.Sprintf("%s/%s/lock/team/%s/%s", prefix, e.Environment, e.Team, e.LockId),
	}, nil
}

func (e *EnvironmentGroupLockParameters) FillHttpInfo() (*HttpInfo, error) {
	d := LockJsonData{
		Message: e.Message,
	}
	var jsonData, err = json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("Could not marshal EnvironmentGroupLockParameters data to json: %w\n", err)
	}
	prefix := "environment-groups"
	if e.UseDexAuthentication {
		prefix = "api/environment-groups"
	}
	return &HttpInfo{
		jsonData:    jsonData,
		ContentType: "application/json",
		HttpMethod:  http.MethodPut,
		RestPath:    fmt.Sprintf("%s/%s/locks/%s", prefix, e.EnvironmentGroup, e.LockId),
	}, nil
}

func createHttpRequest(url string, authParams kutil.AuthenticationParameters, requestInfo *HttpInfo) (*http.Request, error) {
	urlStruct, err := urllib.Parse(url)
	if err != nil {
		return nil, fmt.Errorf("the provided url %s is invalid, error: %w", url, err)
	}

	req, err := http.NewRequest(requestInfo.HttpMethod, urlStruct.JoinPath(requestInfo.RestPath).String(), bytes.NewBuffer(requestInfo.jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating the HTTP request, error: %w", err)
	}
	req.Header.Set("Content-Type", requestInfo.ContentType)

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
