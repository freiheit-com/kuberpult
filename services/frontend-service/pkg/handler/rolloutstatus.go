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
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	pgperrors "github.com/ProtonMail/go-crypto/openpgp/errors"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"go.uber.org/zap"
)

func (s Server) handleEnvironmentGroupRolloutStatus(w http.ResponseWriter, req *http.Request, environmentGroup string) {
	if s.RolloutClient == nil {
		http.Error(w, "not implemented", http.StatusNotImplemented)
		return
	}
	ctx := req.Context()
	if s.checkContentType(w, req) {
		return
	}

	var reqBody struct {
		Signature    string `json:"signature"`
		Team         string `json:"team"`
		WaitDuration string `json:"waitDuration"`
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
	var waitSeconds uint64
	if reqBody.WaitDuration != "" {
		duration, err := time.ParseDuration(reqBody.WaitDuration)
		if err != nil {

			http.Error(w, fmt.Sprintf("Invalid waitDuration: %s", reqBody.WaitDuration), http.StatusBadRequest)
			return
		}
		if duration > s.Config.MaxWaitDuration {
			http.Error(w, fmt.Sprintf("waitDuration is too high: %s - maximum is %s", reqBody.WaitDuration, s.Config.MaxWaitDuration), http.StatusBadRequest)
			return
		}
		waitSeconds = uint64(duration.Seconds())
		if waitSeconds == 0 {

			http.Error(w, fmt.Sprintf("waitDuration is shorter than one second: %s", reqBody.WaitDuration), http.StatusBadRequest)
			return
		}
	}
	res, err := s.RolloutClient.GetStatus(ctx, &api.GetStatusRequest{
		EnvironmentGroup: environmentGroup,
		Team:             reqBody.Team,
		WaitSeconds:      waitSeconds,
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
	w.Write(jsonResponse) //nolint:errcheck

}

func statusName(a api.RolloutStatus) string {
	switch a {
	case api.RolloutStatus_ROLLOUT_STATUS_SUCCESFUL:
		return "succesful"
	case api.RolloutStatus_ROLLOUT_STATUS_PROGRESSING:
		return "progressing"
	case api.RolloutStatus_ROLLOUT_STATUS_ERROR:
		return "error"
	case api.RolloutStatus_ROLLOUT_STATUS_PENDING:
		return "pending"
	case api.RolloutStatus_ROLLOUT_STATUS_UNHEALTHY:
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
