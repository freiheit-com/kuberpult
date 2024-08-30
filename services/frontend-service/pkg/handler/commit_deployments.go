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
	"encoding/json"
	"fmt"
	"net/http"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	xpath "github.com/freiheit-com/kuberpult/pkg/path"
	"go.uber.org/zap"
)

func (s Server) handleCommitDeployments(w http.ResponseWriter, r *http.Request, tail string) {
	commitHash, tail := xpath.Shift(tail)
	if commitHash == "" {
		http.Error(w, "missing commit hash", http.StatusBadRequest)
		return
	}
	if tail != "/" {
		http.Error(w, "invalid path", http.StatusNotFound)
		return
	}
	ctx := r.Context()
	resp, err := s.CommitDeploymentsClient.GetCommitDeploymentInfo(ctx, &api.GetCommitDeploymentInfoRequest{
		CommitId: commitHash,
	})
	if err != nil {
		logger.FromContext(ctx).Error("failed to get commit deployments from server", zap.Error(err))
		http.Error(w, fmt.Sprintf("failed to get commit deployments from server: %v", err), http.StatusInternalServerError)
		return
	}
	json, err := json.Marshal(resp.DeploymentStatus)
	if err != nil {
		logger.FromContext(ctx).Error("failed to get commit deployments from server: failed to marshal response", zap.Error(err))
		http.Error(w, fmt.Sprintf("failed to marshal response: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(json)
	_, _ = w.Write([]byte("\n"))
}
