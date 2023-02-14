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
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/crypto/openpgp"
	pgperrors "golang.org/x/crypto/openpgp/errors"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (s Server) handleCreateEnvironment(w http.ResponseWriter, req *http.Request, environment, tail string) {

	if tail != "/" {
		http.Error(w, fmt.Sprintf("Create Environment does not accept additional path arguments, got: '%s'", tail), http.StatusNotFound)
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
	fmt.Println(s.EnvironmentClient)
	fmt.Println("METHOD", s.EnvironmentClient.CreateEnvironment)
	_, err := s.EnvironmentClient.CreateEnvironment(req.Context(), &emptypb.Empty{})
	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
