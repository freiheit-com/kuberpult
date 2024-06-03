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
	"fmt"
	"github.com/ProtonMail/go-crypto/openpgp"
	"log"
	"net/http"
	"net/url"
	"strings"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	xpath "github.com/freiheit-com/kuberpult/pkg/path"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/config"
)

type Server struct {
	BatchClient                 api.BatchServiceClient
	RolloutClient               api.RolloutServiceClient
	VersionClient               api.VersionServiceClient
	ReleaseTrainPrognosisClient api.ReleaseTrainPrognosisServiceClient
	Config                      config.ServerConfig
	KeyRing                     openpgp.KeyRing
	AzureAuth                   bool
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
	default:
		http.Error(w, fmt.Sprintf("unknown endpoint 'api/%s'", group), http.StatusNotFound)
	}
}

func basicAuth(username, password string) string {
	creds := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(creds))
}

func (s Server) HandleDex(w http.ResponseWriter, r *http.Request, clientID, clientSecret, dexUrl string) {
	group, _ := xpath.Shift(r.URL.Path)
	if group != "token" {
		http.Error(w, fmt.Sprintf("unknown endpoint '%s'", group), http.StatusNotFound)
	}

	err := r.ParseForm()

	if err != nil {
		http.Error(w, fmt.Sprintf("Dex error: %s\n", err), http.StatusNotImplemented)
	}

	log.Println("r.PostForm", r.PostForm)
	log.Println("r.Form", r.Form)

	data := url.Values{}
	data.Set("connector_id", "google")
	data.Set("grant_type", "urn:ietf:params:oauth:token-type:token-exchange")
	data.Set("connector_id", "google")
	data.Set("scope", "offline_access")
	data.Set("requested_token_type", "urn:ietf:params:oauth:token-type:access_token")
	data.Set("subject_token", r.Form["subject_token"][0])
	data.Set("subject_token_type", "urn:ietf:params:oauth:token-type:access_token")

	httpClient := &http.Client{}
	fmt.Printf("Dex URL: %s\n", dexUrl)
	req, err := http.NewRequest("POST", "/dex/token", strings.NewReader(data.Encode()))

	if err != nil {
		http.Error(w, fmt.Sprintf("Not able to construct http request to dex error: %s\n", err), http.StatusInternalServerError)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Basic "+basicAuth(clientID, clientSecret))

	res, err := httpClient.Do(req)

	if err != nil {
		http.Error(w, fmt.Sprintf("Error when contacting dex. error: %s\n", err), http.StatusInternalServerError)
	}

	http.Error(w, fmt.Sprintf("Dex worked: %w. %s\n", res.Status, res.Body), http.StatusOK)
}
