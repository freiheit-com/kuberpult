/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/freiheit-com/kuberpult/pkg/api"
	xpath "github.com/freiheit-com/kuberpult/pkg/path"
)

func (s Server) handleEnvironmentLocks(w http.ResponseWriter, req *http.Request, environment, tail string) {
	lockID, tail := xpath.Shift(tail)
	if lockID == "" {
		http.Error(w, "missing lock ID", http.StatusNotFound)
		return
	}
	if tail != "/" {
		http.Error(w, fmt.Sprintf("locks does not accept additional path arguments after the lock ID, got: '%s'", tail), http.StatusNotFound)
		return
	}

	switch req.Method {
	case http.MethodPut:
		s.handlePutEnvironmentLock(w, req, environment, lockID)
	case http.MethodDelete:
		s.handleDeleteEnvironmentLock(w, req, environment, lockID)
	default:
		http.Error(w, fmt.Sprintf("unsupported method '%s'", req.Method), http.StatusMethodNotAllowed)
	}
}

func (s Server) handlePutEnvironmentLock(w http.ResponseWriter, req *http.Request, environment, lockID string) {
	contentType := req.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, fmt.Sprintf("body must be application/json, got: '%s'", contentType), http.StatusUnsupportedMediaType)
		return
	}

	var body putLockRequest
	invalidMessage := "Please provide lock message in body"
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		decodeError := err.Error()
		if errors.Is(err, io.EOF) {
			decodeError = invalidMessage
		}
		http.Error(w, decodeError, http.StatusBadRequest)
		return
	}

	if len(body.Message) == 0 {
		http.Error(w, invalidMessage, http.StatusBadRequest)
		return
	}
	_, err := s.LockClient.CreateEnvironmentLock(req.Context(), &api.CreateEnvironmentLockRequest{
		Environment: environment,
		LockId:      lockID,
		Message:     body.Message,
	})
	if err != nil {
		handleGRPCErrgor(req.Context(), w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s Server) handleDeleteEnvironmentLock(w http.ResponseWriter, req *http.Request, environment, lockID string) {
	_, err := s.LockClient.DeleteEnvironmentLock(req.Context(), &api.DeleteEnvironmentLockRequest{
		Environment: environment,
		LockId:      lockID,
	})
	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
