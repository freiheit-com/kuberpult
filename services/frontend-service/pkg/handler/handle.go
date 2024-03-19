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

Copyright 2023 freiheit.com*/

package handler

import (
	"fmt"
	"net/http"

	"github.com/ProtonMail/go-crypto/openpgp"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	xpath "github.com/freiheit-com/kuberpult/pkg/path"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/config"
)

type Server struct {
	BatchClient                 api.BatchServiceClient
	RolloutClient               api.RolloutServiceClient
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
