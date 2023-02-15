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
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"golang.org/x/crypto/openpgp"
	pgperrors "golang.org/x/crypto/openpgp/errors"
)

const (
	// This maximum in-memory multipart size.
	// It was chosen based on the assumption that we have < 10 envs with < 3MB manifests per env.
	MAXIMUM_MULTIPART_SIZE = 32 * 1024 * 1024 // = 32Mi
)

func (s Server) handleCreateEnvironment(w http.ResponseWriter, req *http.Request, environment, tail string) {

	if tail != "/" {
		http.Error(w, fmt.Sprintf("Create Environment does not accept additional path arguments, got: '%s'", tail), http.StatusNotFound)
		return
	}
	if err := req.ParseMultipartForm(MAXIMUM_MULTIPART_SIZE); err != nil {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Invalid body: %s", err)
		return
	}
	
	form := req.MultipartForm
	envConfig := api.EnvironmentConfig{}

	if config, ok := form.Value["config"]; ok {
		err := json.Unmarshal([]byte(config[0]), &envConfig)
		if err != nil {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Invalid body: %s", err)
			return
		}
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
			w.Write([]byte("Missing signature in request body"))
			return
		}

		if _, err := openpgp.CheckArmoredDetachedSignature(s.KeyRing, strings.NewReader(environment), bytes.NewReader(signature)); err != nil {
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

	_, err := s.EnvironmentClient.CreateEnvironment(req.Context(), &api.CreateEnvironmentRequest{
		Environment: environment,
		Config:      &envConfig,
	})

	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
