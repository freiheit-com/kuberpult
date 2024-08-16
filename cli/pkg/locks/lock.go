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
	GetPath() string
	FillForm() (*HttpFormDataInfo, error)
}

type EnvironmentLockParameters struct {
	Environment          string
	LockId               string
	Message              string
	UseDexAuthentication bool
}

type AppLockParameters struct {
	Environment          string
	LockId               string
	Message              string
	Application          string
	UseDexAuthentication bool
}

type LockJsonData struct {
	Message string `json:"message"`
}

type HttpFormDataInfo struct {
	jsonData    []byte
	ContentType string
}

func CreateLock(requestParams kutil.RequestParameters, authParams kutil.AuthenticationParameters, params LockParameters) error {
	path := params.GetPath()
	data, err := params.FillForm()
	if err != nil {
		return fmt.Errorf("error while preparing HTTP request. Could not fill form error: %w", err)
	}
	req, err := createHttpRequest(*requestParams.Url, path, authParams, data)

	if err != nil {
		return fmt.Errorf("error while preparing HTTP request, error: %w", err)
	}
	if err := cli_utils.IssueHttpRequest(*req, requestParams.Retries); err != nil {
		return fmt.Errorf("error while issuing HTTP request, error: %v", err)
	}
	return nil
}

func (e *EnvironmentLockParameters) GetPath() string {
	prefix := "environments"
	if e.UseDexAuthentication {
		prefix = "api/environments"
	}

	return fmt.Sprintf("%s/%s/locks/%s", prefix, e.Environment, e.LockId)
}

func (e *EnvironmentLockParameters) FillForm() (*HttpFormDataInfo, error) {
	d := LockJsonData{
		Message: e.Message,
	}
	var jsonData, err = json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("Could not EnvironmentLockParameters data to json: %w\n", err)
	}
	return &HttpFormDataInfo{
		jsonData:    jsonData,
		ContentType: "application/json",
	}, nil
}

func (e *AppLockParameters) GetPath() string {
	prefix := "environments"
	if e.UseDexAuthentication {
		prefix = "api/environments"
	}

	return fmt.Sprintf("%s/%s/applications/%s/locks/%s", prefix, e.Environment, e.Application, e.LockId)
}

func (e *AppLockParameters) FillForm() (*HttpFormDataInfo, error) {
	d := LockJsonData{
		Message: e.Message,
	}
	var jsonData, err = json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("Could not marshal AppLockParameters data to json: %w\n", err)
	}
	return &HttpFormDataInfo{
		jsonData:    jsonData,
		ContentType: "application/json",
	}, nil
}

func createHttpRequest(url string, path string, authParams kutil.AuthenticationParameters, requestInfo *HttpFormDataInfo) (*http.Request, error) {
	urlStruct, err := urllib.Parse(url)
	if err != nil {
		return nil, fmt.Errorf("the provided url %s is invalid, error: %w", url, err)
	}

	req, err := http.NewRequest(http.MethodPut, urlStruct.JoinPath(path).String(), bytes.NewBuffer(requestInfo.jsonData))
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
