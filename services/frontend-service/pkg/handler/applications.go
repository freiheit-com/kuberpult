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

	"github.com/freiheit-com/kuberpult/pkg/api"
	xpath "github.com/freiheit-com/kuberpult/pkg/path"
)

func (s Server) handleApplications(w http.ResponseWriter, req *http.Request, environment, tail string) {
	application, tail := xpath.Shift(tail)
	if application == "" {
		http.Error(w, "missing application ID", http.StatusNotFound)
		return
	}

	function, tail := xpath.Shift(tail)
	switch function {
	case "locks":
		s.handleApplicationLocks(w, req, environment, application, tail)
	default:
		http.Error(w, fmt.Sprintf("unknown function '%s'", function), http.StatusNotFound)
	}
}

func (s Server) handleApplicationLocks(w http.ResponseWriter, req *http.Request, environment, application, tail string) {
	lockID, tail := xpath.Shift(tail)
	if lockID == "" {
		http.Error(w, "missing lock ID", http.StatusNotFound)
		return
	}
	if tail != "/" {
		http.Error(w, fmt.Sprintf("locks does not accept additional path arguments after the lock ID, got: %s", tail), http.StatusNotFound)
		return
	}

	switch req.Method {
	case http.MethodPut:
		s.handlePutApplicationLock(w, req, environment, application, lockID)
	case http.MethodDelete:
		s.handleDeleteApplicationLock(w, req, environment, application, lockID)
	default:
		http.Error(w, fmt.Sprintf("unsupported method '%s'", req.Method), http.StatusMethodNotAllowed)
	}
}

func (s Server) handlePutApplicationLock(w http.ResponseWriter, req *http.Request, environment, application, lockID string) {
	contentType := req.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, fmt.Sprintf("body must be application/json, got: '%s'", contentType), http.StatusUnsupportedMediaType)
		return
	}

	var body putLockRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err := s.LockClient.CreateEnvironmentApplicationLock(req.Context(), &api.CreateEnvironmentApplicationLockRequest{
		Environment: environment,
		Application: application,
		LockId:      lockID,
		Message:     body.Message,
	})
	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s Server) handleDeleteApplicationLock(w http.ResponseWriter, req *http.Request, environment, application, lockID string) {
	_, err := s.LockClient.DeleteEnvironmentApplicationLock(req.Context(), &api.DeleteEnvironmentApplicationLockRequest{
		Environment: environment,
		Application: application,
		LockId:      lockID,
	})
	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
