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

package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/publicapi"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/freiheit-com/kuberpult/pkg/auth"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	xpath "github.com/freiheit-com/kuberpult/pkg/path"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/config"
)

type Server struct {
	BatchClient                 api.BatchServiceClient
	RolloutClient               api.RolloutServiceClient
	VersionClient               api.VersionServiceClient
	ReleaseTrainPrognosisClient api.ReleaseTrainPrognosisServiceClient
	CommitDeploymentsClient     api.CommitDeploymentServiceClient
	ManifestRepoGitClient       api.GitServiceClient
	//
	Config    config.ServerConfig
	KeyRing   openpgp.KeyRing
	AzureAuth bool
	User      auth.User
}

func (s Server) Handle(w http.ResponseWriter, req *http.Request) {
	group, tail := xpath.Shift(req.URL.Path)
	switch group {
	case "environments":
		s.HandleEnvironments(w, req, tail)
	case "environment-groups":
		s.HandleEnvironmentGroups(w, req, tail)
	case "release":
		s.HandleRelease(w, req, tail)
	default:
		http.Error(w, fmt.Sprintf("unknown endpoint '%s'", group), http.StatusNotFound)
	}
}

func (s Server) HandleAPI(w http.ResponseWriter, req *http.Request) {
	group, tail := xpath.Shift(req.URL.Path)
	if group != "api" {
		http.Error(w, fmt.Sprintf("unknown endpoint '%s'", group), http.StatusNotFound)
	}

	group, tail = xpath.Shift(tail)
	switch group {
	case "application":
		s.handleApiApplication(w, req, tail)
	case "environments":
		s.handleApiEnvironments(w, req, tail)
	case "environment-groups":
		s.handleApiEnvironmentGroups(w, req, tail)
	case "release":
		s.handleApiRelease(w, req, tail)
	case "commit-deployments":
		s.handleCommitDeployments(req.Context(), w, req, tail)
	default:
		http.Error(w, fmt.Sprintf("unknown endpoint 'api/%s'", group), http.StatusNotFound)
	}
}

func toBasicAuth(username, password string) string {
	creds := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(creds))
}

type DexResponse struct {
	AccessToken     string `json:"access_token"`
	IssuedTokenType string `json:"issued_token_type"`
	TokenType       string `json:"token_type"`
	ExpiresIn       int    `json:"expires_in"`
}

func (s Server) HandleDex(w http.ResponseWriter, r *http.Request, client *auth.DexAppClient) {
	group, _ := xpath.Shift(r.URL.Path)
	if group != "token" {
		http.Error(w, fmt.Sprintf("unknown endpoint '%s'", group), http.StatusNotFound)
	}

	err := r.ParseForm()

	if err != nil {
		http.Error(w, fmt.Sprintf("Could not parse form. Error: %s", err), http.StatusBadRequest)
	}

	subjectToken := r.Form["subject_token"]

	if len(subjectToken) == 0 {
		http.Error(w, "/token endpoint needs a subject_token.", http.StatusBadRequest)
	}

	data := url.Values{}
	data.Set("connector_id", "google")
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	data.Set("scope", "openid email profile offline_access")
	data.Set("requested_token_type", "urn:ietf:params:oauth:token-type:access_token")
	data.Set("subject_token", r.Form["subject_token"][0])
	data.Set("subject_token_type", "urn:ietf:params:oauth:token-type:access_token")

	//exhaustruct:ignore
	httpClient := &http.Client{}
	dexContactUrl := ""
	if client.UseClusterInternalCommunication {
		dexContactUrl = client.DexServiceURL
	} else {
		dexContactUrl = client.BaseURL
	}

	req, err := http.NewRequest("POST", dexContactUrl+"/dex/token", strings.NewReader(data.Encode()))

	if err != nil {
		http.Error(w, fmt.Sprintf("Not able to construct http request to dex error: %s", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Basic "+toBasicAuth(client.ClientID, client.ClientSecret))

	dexResponse, err := httpClient.Do(req)
	defer func() { _ = dexResponse.Body.Close() }()

	if err != nil {
		http.Error(w, fmt.Sprintf("Error when contacting dex. error: %s", err), http.StatusBadGateway)
		return
	}

	if dexResponse.StatusCode == http.StatusOK {
		//exhaustruct:ignore
		var resp = DexResponse{}
		err = json.NewDecoder(dexResponse.Body).Decode(&resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(resp.AccessToken))
	} else {
		var v []byte
		_, err := dexResponse.Body.Read(v)
		if err != nil {
			return
		}
		http.Error(w, fmt.Sprintf("Dex returned an error: %+v. %s", dexResponse.Status, string(v)), http.StatusBadGateway)
	}
}

type PublicApiServer struct {
	S Server
}

func (s *PublicApiServer) GetPublicApiSchemaOptions(w http.ResponseWriter, _ *http.Request) {
	s.setCorsHeaders(w)
	w.WriteHeader(http.StatusOK)
}

func (s *PublicApiServer) setCorsHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")                                       // Or "application/json"
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, PUT, PATCH, OPTIONS") // Or "application/json"
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, api_key, Authorization")   // Or "application/json"
}

var _ publicapi.ServerInterface = &PublicApiServer{} //exhaustruct:ignore

func (s *PublicApiServer) GetPublicApiSchema(w http.ResponseWriter, _ *http.Request) {
	specFile := "api.yaml" // Or openapi.json
	currentDir, err := os.Getwd()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading current dir: %v", err), http.StatusInternalServerError)
		return
	}

	dir := currentDir + "/kp/kuberpult/pkg/publicapi/"
	content, err := os.ReadFile(dir + specFile)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading OpenAPI spec: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/yaml") // Or "application/json"
	s.setCorsHeaders(w)

	w.WriteHeader(http.StatusOK)
	_, err = w.Write(content)
	if err != nil {
		fmt.Printf("Error writing response: %v", err)
	}
}

func (s *PublicApiServer) GetCommitDeployments(w http.ResponseWriter, r *http.Request, commitHash string) {
	s.setCorsHeaders(w)
	ctx := r.Context()
	ctx = auth.WriteUserToGrpcContext(ctx, auth.User{
		Email:          s.S.User.Email,
		Name:           s.S.User.Name,
		DexAuthContext: nil,
	})

	s.S.handleCommitDeployments(ctx, w, r, commitHash)
}
