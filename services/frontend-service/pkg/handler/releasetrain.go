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

func (s Server) handleReleaseTrain(w http.ResponseWriter, req *http.Request, environment, tail string) {
	if req.Method != http.MethodPut {
		http.Error(w, fmt.Sprintf("releasetrain only accepts method PUT, got: '%s'", req.Method), http.StatusMethodNotAllowed)
		return
	}
	if tail != "/" {
		http.Error(w, fmt.Sprintf("releasetrain does not accept additional path arguments, got: '%s'", tail), http.StatusNotFound)
		return
	}
	queryParams := req.URL.Query()
	teamParam := queryParams.Get("team")

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
	response, err := s.DeployClient.ReleaseTrain(req.Context(), &api.ReleaseTrainRequest{
		Environment: environment,
		Team:        teamParam,
	})
	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}
	json, err := json.Marshal(response)
	if err != nil {
		return
	}
	w.Write(json)
}
