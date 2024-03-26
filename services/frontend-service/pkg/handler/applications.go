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

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/logger"
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

	_, err := s.BatchClient.ProcessBatch(req.Context(), &api.BatchRequest{Actions: []*api.BatchAction{
		{Action: &api.BatchAction_CreateEnvironmentApplicationLock{
			CreateEnvironmentApplicationLock: &api.CreateEnvironmentApplicationLockRequest{
				Environment: environment,
				Application: application,
				LockId:      lockID,
				Message:     body.Message,
			},
		}},
	}})
	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s Server) handleDeleteApplicationLock(w http.ResponseWriter, req *http.Request, environment, application, lockID string) {
	_, err := s.BatchClient.ProcessBatch(req.Context(), &api.BatchRequest{Actions: []*api.BatchAction{
		{Action: &api.BatchAction_DeleteEnvironmentApplicationLock{
			DeleteEnvironmentApplicationLock: &api.DeleteEnvironmentApplicationLockRequest{
				Environment: environment,
				Application: application,
				LockId:      lockID,
			},
		}},
	}})

	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

type ApplicationID = string

func (s Server) handleApiApplication(w http.ResponseWriter, req *http.Request, tail string) {
	applicationID, tail := xpath.Shift(tail)
	if applicationID == "" {
		http.Error(w, "missing application ID", http.StatusNotFound)
		return
	}
	applicationID = ApplicationID(applicationID)

	group, tail := xpath.Shift(tail)
	switch group {
	case "release":
		s.handleApplicationRelease(w, req, tail, applicationID)
	default:
		http.Error(w, fmt.Sprintf("unknown endpoint 'api/application/%s/%s'", applicationID, group), http.StatusNotFound)
	}
}

func (s Server) handleApplicationRelease(w http.ResponseWriter, req *http.Request, tail string, applicationID ApplicationID) {
	releaseNum, tail := xpath.Shift(tail)

	if releaseNum == "" {
		http.Error(w, "missing release number", http.StatusNotFound)
		return
	}

	group, _ := xpath.Shift(tail)
	switch group {
	case "manifests":
		s.handleApplicationReleaseManifests(w, req, applicationID, releaseNum)
	default:
		http.Error(w, fmt.Sprintf("unknown endpoint 'api/application/%s/%s'", releaseNum, group), http.StatusNotFound)
	}
}

func (s Server) handleApplicationReleaseManifests(w http.ResponseWriter, req *http.Request, applicationID ApplicationID, releaseNum string) {
	resp, err := s.VersionClient.GetManifests(req.Context(), &api.GetManifestsRequest{
		Application: string(applicationID),
		Release:     releaseNum,
	})
	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}
	encoded, err := json.Marshal(resp)
	if err != nil {
		logger.FromContext(req.Context()).Error("GetManifests: encoding response")
		http.Error(w, "GetManifests: encoding response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(encoded)
	if err != nil {
		logger.FromContext(req.Context()).Error("GetManifests: writing response")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
