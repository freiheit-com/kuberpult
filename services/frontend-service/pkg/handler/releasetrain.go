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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	pgperrors "github.com/ProtonMail/go-crypto/openpgp/errors"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
)

func (s Server) handleReleaseTrainExecution(w http.ResponseWriter, req *http.Request, target string) {
	if req.Method != http.MethodPut {
		http.Error(w, fmt.Sprintf("releasetrain only accepts method PUT, got: '%s'", req.Method), http.StatusMethodNotAllowed)
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
			w.Write([]byte("Missing signature in request body")) //nolint:errcheck
			return
		}

		if _, err := openpgp.CheckArmoredDetachedSignature(s.KeyRing, strings.NewReader(target), bytes.NewReader(signature), nil); err != nil {
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

	tf := &api.ReleaseTrainRequest{
		CommitHash: "",
		Target:     target,
		Team:       teamParam,
		TargetType: api.ReleaseTrainRequest_UNKNOWN,
		CiLink:     "",
	}

	type releaseTrainBody struct {
		CiLink string `json:"ciLink,omitempty"`
	}
	var body releaseTrainBody
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		decodeError := err.Error()
		if errors.Is(err, io.EOF) {
			tf.CiLink = "" //If no body, CI link is empty
		} else {
			http.Error(w, decodeError, http.StatusBadRequest)
			return
		}
	} else {
		tf.CiLink = body.CiLink
	}

	response, err := s.BatchClient.ProcessBatch(req.Context(),
		&api.BatchRequest{Actions: []*api.BatchAction{
			{
				Action: &api.BatchAction_ReleaseTrain{
					ReleaseTrain: tf,
				},
			},
		},
		})
	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}
	json, err := json.Marshal(response.Results[0].GetReleaseTrain())
	if err != nil {
		return
	}
	w.Write(json) //nolint:errcheck
}
func (s Server) handleAPIReleaseTrainExecution(w http.ResponseWriter, req *http.Request, target string, TargetType api.ReleaseTrainRequest_TargetType) {
	if req.Method != http.MethodPut {
		http.Error(w, fmt.Sprintf("releasetrain only accepts method PUT, got: '%s'", req.Method), http.StatusMethodNotAllowed)
		return
	}
	queryParams := req.URL.Query()
	teamParam := queryParams.Get("team")

	response, err := s.BatchClient.ProcessBatch(req.Context(),
		&api.BatchRequest{Actions: []*api.BatchAction{
			{Action: &api.BatchAction_ReleaseTrain{
				ReleaseTrain: &api.ReleaseTrainRequest{
					CommitHash: "",
					Target:     target,
					Team:       teamParam,
					TargetType: TargetType,
				}}},
		},
		})
	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}
	json, err := json.Marshal(response.Results[0].GetReleaseTrain())
	if err != nil {
		return
	}
	w.Write(json) //nolint:errcheck
}

func (s Server) handleReleaseTrainPrognosis(w http.ResponseWriter, req *http.Request, target string) {
	if req.Method != http.MethodGet {
		http.Error(w, fmt.Sprintf("releasetrain prognosis only accepts method GET, got: '%s'", req.Method), http.StatusMethodNotAllowed)
		return
	}

	queryParams := req.URL.Query()
	teamParam := queryParams.Get("team")

	response, err := s.ReleaseTrainPrognosisClient.GetReleaseTrainPrognosis(req.Context(), &api.ReleaseTrainRequest{
		Target:     target,
		CommitHash: "",
		CiLink:     "",
		Team:       teamParam,
		TargetType: api.ReleaseTrainRequest_UNKNOWN,
	})

	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}
	json, err := json.Marshal(response.EnvsPrognoses)
	if err != nil {
		w.Write([]byte(fmt.Sprintf("error while serializing response, error: %v", err.Error()))) //nolint:errcheck
		return
	}
	w.Write(json) //nolint:errcheck
}

func (s Server) handleReleaseTrain(w http.ResponseWriter, req *http.Request, target, tail string) {
	switch tail {
	case "/":
		s.handleReleaseTrainExecution(w, req, target)
	default:
		http.Error(w, fmt.Sprintf("release trains must be invoked via /releasetrain, but it was invoked via /releasetrain%s", tail), http.StatusNotFound)
		return
	}
}

func (s Server) handleApiEnvironmentReleaseTrain(w http.ResponseWriter, req *http.Request, target, tail string) {
	switch tail {
	case "/":
		s.handleAPIReleaseTrainExecution(w, req, target, api.ReleaseTrainRequest_ENVIRONMENT)
	case "/prognosis":
		s.handleReleaseTrainPrognosis(w, req, target)
	default:
		http.Error(w, fmt.Sprintf("release trains must be invoked via /releasetrain/prognosis, but it was invoked via /releasetrain%s", tail), http.StatusNotFound)
		return
	}
}

func (s Server) handleApiEnvironmentGroupReleaseTrain(w http.ResponseWriter, req *http.Request, target, tail string) {
	switch tail {
	case "/":
		s.handleAPIReleaseTrainExecution(w, req, target, api.ReleaseTrainRequest_ENVIRONMENTGROUP)
	default:
		http.Error(w, fmt.Sprintf("release trains must be invoked via /releasetrain, but it was invoked via /releasetrain%s", tail), http.StatusNotFound)
		return
	}
}
