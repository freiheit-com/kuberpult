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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	pgperrors "github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"go.uber.org/zap"
)

func (s *Server) handleEnvironmentGroupRolloutStatus(w http.ResponseWriter, req *http.Request, environmentGroup string) {
	if s.RolloutClient == nil {
		http.Error(w, "not implemented", http.StatusNotImplemented)
		return
	}
	ctx := req.Context()
	if s.checkContentType(w, req) {
		return
	}

	var reqBody struct {
		Signature string `json:"signature"`
	}
	err := json.NewDecoder(req.Body).Decode(&reqBody)
	if err != nil {
		http.Error(w, "invalid json in request", http.StatusBadRequest)
		return
	}

	signature := reqBody.Signature
	if len(signature) == 0 && s.AzureAuth {
		http.Error(w, "Missing signature in request body - this is required with AzureAuth enabled", http.StatusBadRequest)
		return
	}

	if len(signature) > 0 {
		if s.KeyRing == nil {
			http.Error(w, "key ring is not configured", http.StatusNotFound)
			return
		}
		if _, err := openpgp.CheckArmoredDetachedSignature(s.KeyRing, strings.NewReader(environmentGroup), strings.NewReader(signature), nil); err != nil {
			if err != pgperrors.ErrUnknownIssuer {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(w, "Internal: Invalid Signature: %s", err)
				return
			}
			http.Error(w, "Invalid Signature", http.StatusUnauthorized)
			return
		}
	}
	res, err := s.RolloutClient.GetStatus(ctx, &api.GetStatusRequest{
		EnvironmentGroup: environmentGroup,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Internal error: %s", err), http.StatusInternalServerError)
		logger.FromContext(ctx).Error("rollout", zap.Error(err))
		return
	}

	resBody := struct {
		Status       string       `json:"status"`
		Applications []rolloutApp `json:"applications"`
	}{
		Status:       statusName(res.Status),
		Applications: transformApps(res.Applications),
	}
	jsonResponse, err := json.Marshal(resBody)
	if err != nil {
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)

}

func statusName(a api.RolloutStatus) string {
	switch a {
	case api.RolloutStatus_RolloutStatusSuccesful:
		return "succesful"
	case api.RolloutStatus_RolloutStatusProgressing:
		return "progressing"
	case api.RolloutStatus_RolloutStatusError:
		return "error"
	case api.RolloutStatus_RolloutStatusPending:
		return "pending"
	case api.RolloutStatus_RolloutStatusUnhealthy:
		return "unhealthy"
	}
	return "unknown"
}

type rolloutApp struct {
	Application string `json:"application"`
	Environment string `json:"environment"`
	Status      string `json:"status"`
}

func transformApps(apps []*api.GetStatusResponse_ApplicationStatus) []rolloutApp {
	result := make([]rolloutApp, 0, len(apps))
	for _, app := range apps {
		result = append(result, rolloutApp{
			Application: app.Application,
			Environment: app.Environment,
			Status:      statusName(app.RolloutStatus),
		})
	}
	return result
}
