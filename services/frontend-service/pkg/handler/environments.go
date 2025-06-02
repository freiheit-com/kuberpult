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
	"bytes"
	"fmt"
	"github.com/gogo/protobuf/jsonpb"

	"github.com/ProtonMail/go-crypto/openpgp"
	pgperrors "github.com/ProtonMail/go-crypto/openpgp/errors"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"io"
	"net/http"
	"strings"
)

const (
	MAXIMUM_MULTIPART_SIZE = 12 * 1024 * 1024 // = 12Mi
)

func (s Server) handleCreateEnvironment(w http.ResponseWriter, req *http.Request, environment, tail string) {

	if tail != "/" {
		http.Error(w, fmt.Sprintf("Create Environment does not accept additional path arguments, got: '%s'", tail), http.StatusNotFound)
		return
	}
	if err := req.ParseMultipartForm(MAXIMUM_MULTIPART_SIZE); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Invalid body: %s", err)
		return
	}
	envConfig, errCode, errMessage := s.validateCreateEnvironmentRequest(req)
	if envConfig == nil {
		w.WriteHeader(errCode)
		fmt.Fprint(w, errMessage) //nolint:errcheck
		return
	}
	_, err := s.BatchClient.ProcessBatch(req.Context(),
		&api.BatchRequest{Actions: []*api.BatchAction{
			{Action: &api.BatchAction_CreateEnvironment{
				CreateEnvironment: &api.CreateEnvironmentRequest{
					Environment: environment,
					Config:      envConfig,
				}}},
		},
		})

	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s Server) handleApiCreateEnvironment(w http.ResponseWriter, req *http.Request, environment, tail string) {

	if tail != "/" {
		http.Error(w, fmt.Sprintf("Create Environment does not accept additional path arguments, got: '%s'", tail), http.StatusNotFound)
		return
	}
	if err := req.ParseMultipartForm(MAXIMUM_MULTIPART_SIZE); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Invalid body: %s", err)
		return
	}
	envConfig, errCode, message := s.validateCreateEnvironmentRequest(req)
	if envConfig == nil {
		w.WriteHeader(errCode)
		fmt.Fprint(w, message) //nolint:errcheck
		return
	}
	if envConfig.Argocd != nil { //ArgoCd field is not supported in API version of create Environment
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("ArgoCd field is not supported")) //nolint:errcheck
		return
	}
	_, err := s.BatchClient.ProcessBatch(req.Context(),
		&api.BatchRequest{Actions: []*api.BatchAction{
			{Action: &api.BatchAction_CreateEnvironment{
				CreateEnvironment: &api.CreateEnvironmentRequest{
					Environment: environment,
					Config:      envConfig,
				}}},
		},
		})

	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s Server) validateCreateEnvironmentRequest(req *http.Request) (*api.EnvironmentConfig, int, string) {
	form := req.MultipartForm
	//exhaustruct:ignore
	envConfig := api.EnvironmentConfig{}

	config, ok := form.Value["config"]
	if !ok {
		return nil, http.StatusBadRequest, "Missing config in request body"
	}
	err := jsonpb.UnmarshalString(config[0], &envConfig)
	if err != nil {

		return nil, http.StatusBadRequest, fmt.Sprintf("Invalid body: %s", err)
	}

	if signature, ok := form.Value["signature"]; ok {
		if _, err := openpgp.CheckArmoredDetachedSignature(s.KeyRing, bytes.NewReader([]byte(config[0])), bytes.NewReader([]byte(signature[0])), nil); err != nil {
			if err != pgperrors.ErrUnknownIssuer {
				return nil, http.StatusInternalServerError, fmt.Sprintf("Internal: Invalid Signature: %s", err)
			}
			return nil, http.StatusUnauthorized, "Invalid signature"
		}
	} else if s.AzureAuth {
		return nil, http.StatusBadRequest, "Missing signature in request body"
	}
	return &envConfig, -1, ""
}

func (s Server) handleDeleteEnvironment(w http.ResponseWriter, req *http.Request, environment, tail string) {
	if tail != "/" {
		http.Error(w, fmt.Sprintf("Delete Environment does not accept additional path arguments, got: '%s'", tail), http.StatusNotFound)
		return
	}
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
			w.Write([]byte("Missing signature in request body")) //nolint:errcheck
			return
		}

		if _, err := openpgp.CheckArmoredDetachedSignature(s.KeyRing, strings.NewReader(environment), bytes.NewReader(signature), nil); err != nil {
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
	_, err := s.BatchClient.ProcessBatch(req.Context(),
		&api.BatchRequest{Actions: []*api.BatchAction{
			{Action: &api.BatchAction_DeleteEnvironment{
				DeleteEnvironment: &api.DeleteEnvironmentRequest{
					Environment: environment,
				}}},
		},
		})

	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
