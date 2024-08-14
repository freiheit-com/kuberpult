package locks

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	kutil "github.com/freiheit-com/kuberpult/cli/pkg/kuberpult_utils"
	"io"
	"log"
	"net/http"
	urllib "net/url"
	"time"
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

type TeamLockParameters struct {
	Environment          string
	LockId               string
	Message              string
	Team                 string
	UseDexAuthentication bool
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
	if err := issueHttpRequest(*req, requestParams.Retries); err != nil {
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
	type data struct {
		Message string `json:"message"`
	}
	d := data{
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
	type data struct {
		Message string `json:"message"`
	}
	d := data{
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

func (e *TeamLockParameters) GetPath() string {
	prefix := "environments"
	if e.UseDexAuthentication {
		prefix = "api/environments"
	}

	return fmt.Sprintf("%s/%s/lock/team/%s/%s", prefix, e.Environment, e.Team, e.LockId)
}

func (e *TeamLockParameters) FillForm() (*HttpFormDataInfo, error) {
	type data struct {
		Message string `json:"message"`
	}
	d := data{
		Message: e.Message,
	}
	var jsonData, err = json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("Could not marshal TeamLockParameters data to json: %w\n", err)
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

func doRequest(request *http.Request) (*http.Response, []byte, error) {
	//exhaustruct:ignore
	client := &http.Client{
		Timeout: 10 * time.Second,
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

func issueHttpRequest(req http.Request, retries uint64) error {
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
