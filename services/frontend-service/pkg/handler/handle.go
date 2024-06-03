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
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/ProtonMail/go-crypto/openpgp"

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

func (s Server) HandleDex(w http.ResponseWriter, r *http.Request) {
	group, tail := xpath.Shift(r.URL.Path)
	if group != "dex" {
		http.Error(w, fmt.Sprintf("unknown endpoint '%s'", group), http.StatusNotFound)
	}

	if tail != "" {
		http.Error(w, fmt.Sprintf("unknown endpoint '%s'", group), http.StatusNotFound)
	}
	err := r.ParseForm()

	if err != nil {
		http.Error(w, fmt.Sprintf("Dex error: %s\n", err), http.StatusNotImplemented)
	}

	log.Println("r.PostForm", r.PostForm)
	log.Println("r.Form", r.Form)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println("r.Body", string(body))

	values, err := url.ParseQuery(string(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Error(w, fmt.Sprintf("Dex endpoint under construction: %s\n", values), http.StatusNotImplemented)
}
