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
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	pgperrors "github.com/ProtonMail/go-crypto/openpgp/errors"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	manifestFieldRx = regexp.MustCompile(`\Amanifests\[([^]]+)\]\z`)
	// matches hex strings with 7 - 40 chars
	commitIdRx = regexp.MustCompile(`\A[0-9a-f]{7,40}\z`)
	// parses anything that looks like "name <mail@host.com>"
	authorRx = regexp.MustCompile(`\A[^<\n]+( <[^@\n]+@[^>\n]+>)?\z`)
)

func readMultipartFile(hdr *multipart.FileHeader) ([]byte, error) {
	if file, err := hdr.Open(); err != nil {
		return nil, err
	} else {
		defer file.Close()
		if content, err := io.ReadAll(file); err != nil {
			return nil, err
		} else {
			return content, nil
		}
	}
}

func isCommitId(value string) bool {
	return commitIdRx.MatchString(value)
}

func isAuthor(value string) bool {
	return authorRx.MatchString(value)
}

func writeReleaseResponse(w http.ResponseWriter, r *http.Request, jsonBlob []byte, err error, status int) {
	ctx := r.Context()
	if err != nil {
		logger.FromContext(ctx).Error(fmt.Sprintf("error in json.Marshal of /release: %s", err.Error()))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	w.Write(jsonBlob)     //nolint:errcheck
	w.Write([]byte("\n")) //nolint:errcheck
}

func (s Server) HandleRelease(w http.ResponseWriter, r *http.Request, tail string) {
	ctx := r.Context()
	if tail != "/" {
		http.Error(w, fmt.Sprintf("Release does not accept additional path arguments, got: %s", tail), http.StatusNotFound)
		return
	}

	tf := api.CreateReleaseRequest{
		Environment:    "",
		Application:    "",
		Team:           "",
		Version:        0,
		SourceCommitId: "",
		SourceAuthor:   "",
		SourceMessage:  "",
		SourceRepoUrl:  "",
		DisplayVersion: "",
		Manifests:      map[string]string{},
	}
	if err := r.ParseMultipartForm(MAXIMUM_MULTIPART_SIZE); err != nil {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Invalid body: %s", err)
		return
	}
	form := r.MultipartForm
	if len(form.Value["application"]) != 1 {
		if len(form.Value["application"]) > 1 {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Please provide single application name")
			return
		} else {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Must provide application name")
			return
		}
	}
	application := form.Value["application"][0]
	if application == "" {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Invalid application name: '%s' - must not be empty", application)
		return
	}
	tf.Application = application
	for k, v := range form.File {
		match := manifestFieldRx.FindStringSubmatch(k)
		if match == nil {
			if strings.Contains(k, "signatures") {
				// signatures are allowed
				continue
			}
			// it's neither a manifest nor a signature, that's an error
			w.WriteHeader(400)
			fmt.Fprintf(w, "Invalid manifest form file: '%s'. Must match '%s'", k, manifestFieldRx)
			return
		}
		environmentName := match[1]
		if len(v) != 1 {
			w.WriteHeader(400)
			fmt.Fprintf(w, "multiple manifests submitted for %q", environmentName)
			return
		}
		if content, err := readMultipartFile(v[0]); err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal: %s", err)
			return
		} else {
			if s.KeyRing != nil {
				validSignature := false
				for _, sig := range form.File[fmt.Sprintf("signatures[%s]", environmentName)] {
					if signature, err := readMultipartFile(sig); err != nil {
						w.WriteHeader(500)
						fmt.Fprintf(w, "Internal: %s", err)
						return
					} else {
						if _, err := openpgp.CheckArmoredDetachedSignature(s.KeyRing, bytes.NewReader(content), bytes.NewReader(signature), nil); err != nil {
							if err != pgperrors.ErrUnknownIssuer {
								w.WriteHeader(400)
								fmt.Fprintf(w, "Internal: Invalid Signature for %s: %s", k, err.Error())
								return
							}
						} else {
							validSignature = true
							break
						}
					}
				}
				if !validSignature {
					w.WriteHeader(400)
					fmt.Fprintf(w, "signature not found or invalid for %s", environmentName)
					return
				}

			}

			// TODO(HVG): validate that the manifest is valid yaml
			tf.Manifests[environmentName] = string(content)
		}

	}
	if len(tf.Manifests) == 0 {
		w.WriteHeader(400)
		fmt.Fprintf(w, "No manifest files provided")
		return
	}

	if team, ok := form.Value["team"]; ok {
		if len(team) == 1 {
			tf.Team = team[0]
		}
	}

	if sourceCommitId, ok := form.Value["source_commit_id"]; ok {
		if len(sourceCommitId) == 1 && isCommitId(sourceCommitId[0]) {
			tf.SourceCommitId = sourceCommitId[0]
		}
	}

	if sourceAuthor, ok := form.Value["source_author"]; ok {
		if len(sourceAuthor) == 1 && isAuthor(sourceAuthor[0]) {
			tf.SourceAuthor = sourceAuthor[0]
		}
	}

	if sourceMessage, ok := form.Value["source_message"]; ok {
		if len(sourceMessage) == 1 {
			tf.SourceMessage = sourceMessage[0]
		}
	}
	if version, ok := form.Value["version"]; ok {
		if len(version) == 1 {
			val, err := strconv.ParseUint(version[0], 10, 64)
			if err != nil {
				w.WriteHeader(400)
				fmt.Fprintf(w, "Invalid version: %s", err)
				return
			}
			tf.Version = val
		}
	}

	if displayVersion, ok := form.Value["display_version"]; ok {
		if len(displayVersion) != 1 {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Invalid number of display versions provided: %d, ", len(displayVersion))
		}
		if len(displayVersion[0]) > 15 {
			w.WriteHeader(400)
			fmt.Fprintf(w, "DisplayVersion given should be <= 15 characters")
			return
		}
		tf.DisplayVersion = displayVersion[0]

	}

	response, err := s.BatchClient.ProcessBatch(ctx, &api.BatchRequest{Actions: []*api.BatchAction{
		{
			Action: &api.BatchAction_CreateRelease{
				CreateRelease: &tf,
			},
		}},
	})
	if err != nil {
		s, ok := status.FromError(err)
		if ok && s.Code() == codes.InvalidArgument {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(response.Results) != 1 {
		msg := "mismatching response length"
		logger.FromContext(ctx).Error(fmt.Sprintf("error in parsing response of /release: %s", msg))
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	releaseResponse := response.Results[0].GetCreateReleaseResponse()
	if releaseResponse == nil {
		msg := "mismatching response length"
		logger.FromContext(ctx).Error(fmt.Sprintf("error in parsing response of /release: %s", msg))
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	switch firstResponse := releaseResponse.Response.(type) {
	case *api.CreateReleaseResponse_Success:
		{
			jsonBlob, err := json.Marshal(firstResponse)
			writeReleaseResponse(w, r, jsonBlob, err, http.StatusCreated)
		}
	case *api.CreateReleaseResponse_AlreadyExistsSame:
		{
			jsonBlob, err := json.Marshal(firstResponse)
			writeReleaseResponse(w, r, jsonBlob, err, http.StatusOK)
		}
	case *api.CreateReleaseResponse_AlreadyExistsDifferent:
		{
			jsonBlob, err := json.Marshal(firstResponse)
			writeReleaseResponse(w, r, jsonBlob, err, http.StatusConflict)
		}
	case *api.CreateReleaseResponse_GeneralFailure:
		{
			jsonBlob, err := json.Marshal(firstResponse)
			writeReleaseResponse(w, r, jsonBlob, err, http.StatusInternalServerError)
		}
	case *api.CreateReleaseResponse_TooOld:
		{
			jsonBlob, err := json.Marshal(firstResponse)
			writeReleaseResponse(w, r, jsonBlob, err, http.StatusBadRequest)
		}
	case *api.CreateReleaseResponse_TooLong:
		{
			jsonBlob, err := json.Marshal(firstResponse)
			writeReleaseResponse(w, r, jsonBlob, err, http.StatusBadRequest)
		}
	default:
		{
			msg := "unknown response type in /release"
			jsonBlob, err := json.Marshal(releaseResponse)
			jsonBlobRequest, _ := json.Marshal(&tf)
			logger.FromContext(ctx).Error(fmt.Sprintf("%s: %s, %s", msg, jsonBlob, err))
			writeReleaseResponse(w, r, []byte(fmt.Sprintf("%s: request: %s, response: %s", msg, jsonBlobRequest, jsonBlob)), err, http.StatusInternalServerError)
		}
	}
}
