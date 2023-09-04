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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ProtonMail/go-crypto/openpgp"
	pgperrors "github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	xpath "github.com/freiheit-com/kuberpult/pkg/path"
	"io"
	"net/http"
	"strings"
)

func (s Server) handleEnvironmentGroupLocks(w http.ResponseWriter, req *http.Request, environmentGroup, tail string) {
	lockID, tail := xpath.Shift(tail)
	if lockID == "" {
		http.Error(w, "missing ID for env group lock", http.StatusNotFound)
		return
	}
	if tail != "/" {
		http.Error(w, fmt.Sprintf("group locks does not accept additional path arguments after the lock ID, got: '%s'", tail), http.StatusNotFound)
		return
	}

	switch req.Method {
	case http.MethodPut:
		s.handlePutEnvironmentGroupLock(w, req, environmentGroup, lockID)
	case http.MethodDelete:
		s.handleDeleteEnvironmentGroupLock(w, req, environmentGroup, lockID)
	default:
		http.Error(w, fmt.Sprintf("unsupported method '%s'", req.Method), http.StatusMethodNotAllowed)
	}
}

func (s Server) handleEnvironmentLocks(w http.ResponseWriter, req *http.Request, environment, tail string) {
	lockID, tail := xpath.Shift(tail)
	if lockID == "" {
		http.Error(w, "missing ID for env lock", http.StatusNotFound)
		return
	}
	if tail != "/" {
		http.Error(w, fmt.Sprintf("env locks does not accept additional path arguments after the lock ID, got: '%s'", tail), http.StatusNotFound)
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
	if s.checkContentType(w, req) {
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

	if s.AzureAuth {
		signature := body.Signature
		if len(signature) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Missing signature in request body"))
			return
		}

		if _, err := openpgp.CheckArmoredDetachedSignature(s.KeyRing, strings.NewReader(environment+lockID), strings.NewReader(signature), nil); err != nil {
			if err != pgperrors.ErrUnknownIssuer {
				w.WriteHeader(500)
				fmt.Fprintf(w, "Internal: Invalid Signature: %s", err)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, "Invalid signature")
			return
		}
	}

	_, err := s.BatchClient.ProcessBatch(req.Context(), &api.BatchRequest{Actions: []*api.BatchAction{
		{Action: &api.BatchAction_CreateEnvironmentLock{
			CreateEnvironmentLock: &api.CreateEnvironmentLockRequest{
				Environment: environment,
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

func (s Server) handleDeleteEnvironmentLock(w http.ResponseWriter, req *http.Request, environment, lockID string) {
	if s.AzureAuth {
		if req.Body == nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "missing request body")
			return
		}
		signature, err := io.ReadAll(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Can't read request body %s", err)
			return
		}

		if len(signature) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Missing signature in request body"))
			return
		}

		if _, err := openpgp.CheckArmoredDetachedSignature(s.KeyRing, strings.NewReader(environment+lockID), bytes.NewReader(signature), nil); err != nil {
			if err != pgperrors.ErrUnknownIssuer {
				w.WriteHeader(500)
				fmt.Fprintf(w, "Internal: Invalid Signature: %s", err)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, "Invalid signature")
			return
		}
	}
	_, err := s.BatchClient.ProcessBatch(req.Context(), &api.BatchRequest{Actions: []*api.BatchAction{
		{Action: &api.BatchAction_DeleteEnvironmentLock{
			DeleteEnvironmentLock: &api.DeleteEnvironmentLockRequest{
				Environment: environment,
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

func (s Server) handlePutEnvironmentGroupLock(w http.ResponseWriter, req *http.Request, environmentGroup, lockID string) {
	if s.checkContentType(w, req) {
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

	signature := body.Signature
	if len(signature) == 0 && s.AzureAuth {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Missing signature in request body - this is required with AzureAuth enabled"))
		return
	}

	if len(signature) > 0 {
		if s.KeyRing == nil {
			http.Error(w, "key ring is not configured", http.StatusNotFound)
			return
		}
		if _, err := openpgp.CheckArmoredDetachedSignature(s.KeyRing, strings.NewReader(environmentGroup+lockID), strings.NewReader(signature), nil); err != nil {
			if err != pgperrors.ErrUnknownIssuer {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(w, "Internal: Invalid Signature: %s", err)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, "Invalid signature")
			return
		}
	}

	response, err := s.BatchClient.ProcessBatch(req.Context(), &api.BatchRequest{Actions: []*api.BatchAction{
		{Action: &api.BatchAction_CreateEnvironmentGroupLock{
			CreateEnvironmentGroupLock: &api.CreateEnvironmentGroupLockRequest{
				EnvironmentGroup: environmentGroup,
				LockId:           lockID,
				Message:          body.Message,
			},
		}},
	}})

	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}

	jsonResponse, err := json.Marshal(response.Results[0])
	if err != nil {
		return
	}
	w.WriteHeader(http.StatusCreated)
	w.Write(jsonResponse)
}

func (s Server) checkContentType(w http.ResponseWriter, req *http.Request) bool {
	contentType := req.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, fmt.Sprintf("body must be application/json, got: '%s'", contentType), http.StatusUnsupportedMediaType)
		return true
	}
	return false
}

func (s Server) handleDeleteEnvironmentGroupLock(w http.ResponseWriter, req *http.Request, environmentGroup, lockID string) {
	if req.Body == nil && s.AzureAuth {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "missing request body")
		return
	}
	if req.Body != nil {
		if s.checkContentType(w, req) {
			return
		}
		signature, err := io.ReadAll(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Can't read request body %s", err)
			return
		}
		if len(signature) == 0 && s.AzureAuth {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Missing signature in request body"))
			return
		}
		if len(signature) > 0 {
			if s.KeyRing == nil {
				logger.FromContext(req.Context()).Warn("NO KEYRING. signature: " + string(signature))
				http.Error(w, "key ring is not configured", http.StatusNotFound)
				return
			}
			if _, err := openpgp.CheckArmoredDetachedSignature(s.KeyRing, strings.NewReader(environmentGroup+lockID), bytes.NewReader(signature), nil); err != nil {
				if err != pgperrors.ErrUnknownIssuer {
					w.WriteHeader(500)
					fmt.Fprintf(w, "Internal: Invalid Signature: %s", err)
					return
				}
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(w, "Invalid signature")
				return
			}
		}
	}
	response, err := s.BatchClient.ProcessBatch(req.Context(), &api.BatchRequest{Actions: []*api.BatchAction{
		{Action: &api.BatchAction_DeleteEnvironmentGroupLock{
			DeleteEnvironmentGroupLock: &api.DeleteEnvironmentGroupLockRequest{
				EnvironmentGroup: environmentGroup,
				LockId:           lockID,
			},
		}},
	}})

	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}

	if response == nil || len(response.Results) == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("cd-service did not return a result"))
		return
	}
	jsonResponse, err := json.Marshal(response.Results[0])
	if err != nil {
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}
