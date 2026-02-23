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
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/logger"
)

type ProcessDelayRestResponse struct {
	UnprocessedEvents uint64  `json:"unprocessed_events"`
	DelaySeconds      float64 `json:"delay_seconds"`
}

// handleProcessDelay returns the number of events that the manifest-export has not processed yet.
// It is intended as an extended "health check" for our stress-tests.
func (s Server) handleProcessDelay(ctx context.Context, w http.ResponseWriter, r *http.Request, tail string) {
	if tail != "/" {
		http.Error(w, "invalid path", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	resp, err := s.ManifestRepoGitClient.GetGitSyncStatus(ctx, &api.GetGitSyncStatusRequest{})
	if err != nil {
		msg := "failed to get commit deployments from server"
		logger.FromContext(ctx).Error(msg, zap.Error(err))
		http.Error(w, fmt.Sprintf("%s: %v", msg, err), http.StatusInternalServerError)
		return
	}
	jsonResponse := ProcessDelayRestResponse{
		UnprocessedEvents: resp.ProcessDelayEvents,
		DelaySeconds:      resp.ProcessDelaySeconds,
	}
	jsonResponseBytes, err := json.Marshal(jsonResponse)
	if err != nil {
		msg := "failed to get commit deployments from server: failed to marshal response"
		logger.FromContext(ctx).Error(msg, zap.Error(err))
		http.Error(w, fmt.Sprintf("%s: %v", msg, err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(jsonResponseBytes)
	_, _ = w.Write([]byte("\n"))
}
