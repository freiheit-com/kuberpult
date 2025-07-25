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
	"context"
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
		defer func() { _ = file.Close() }()
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
	_, _ = w.Write(jsonBlob)     //nolint:errcheck
	_, _ = w.Write([]byte("\n")) //nolint:errcheck
}

func (s Server) HandleRelease(w http.ResponseWriter, r *http.Request, tail string) {
	ctx := r.Context()
	if tail != "/" {
		http.Error(w, fmt.Sprintf("Release does not accept additional path arguments, got: %s", tail), http.StatusNotFound)
		return
	}

	tf := api.CreateReleaseRequest{
		Environment:                    "",
		Application:                    "",
		Team:                           "",
		Version:                        0,
		SourceCommitId:                 "",
		SourceAuthor:                   "",
		SourceMessage:                  "",
		SourceRepoUrl:                  "",
		PreviousCommitId:               "",
		DisplayVersion:                 "",
		Manifests:                      map[string]string{},
		CiLink:                         "",
		IsPrepublish:                   false,
		DeployToDownstreamEnvironments: []string{},
		Revision:                       0,
	}
	if err := r.ParseMultipartForm(MAXIMUM_MULTIPART_SIZE); err != nil {
		w.WriteHeader(400)
		_, _ = fmt.Fprintf(w, "Invalid body: %s", err)
		return
	}
	form := r.MultipartForm
	if len(form.Value["application"]) != 1 {
		if len(form.Value["application"]) > 1 {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "Please provide single application name")
			return
		} else {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "Must provide application name")
			return
		}
	}
	application := form.Value["application"][0]
	if application == "" {
		w.WriteHeader(400)
		_, _ = fmt.Fprintf(w, "Invalid application name: '%s' - must not be empty", application)
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
			_, _ = fmt.Fprintf(w, "Invalid manifest form file: '%s'. Must match '%s'", k, manifestFieldRx)
			return
		}
		environmentName := match[1]
		if len(v) != 1 {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "multiple manifests submitted for %q", environmentName)
			return
		}
		if content, err := readMultipartFile(v[0]); err != nil {
			w.WriteHeader(500)
			_, _ = fmt.Fprintf(w, "Internal: %s", err)
			return
		} else {
			if s.KeyRing != nil {
				validSignature := false
				for _, sig := range form.File[fmt.Sprintf("signatures[%s]", environmentName)] {
					if signature, err := readMultipartFile(sig); err != nil {
						w.WriteHeader(500)
						_, _ = fmt.Fprintf(w, "Internal: %s", err)
						return
					} else {
						if _, err := openpgp.CheckArmoredDetachedSignature(s.KeyRing, bytes.NewReader(content), bytes.NewReader(signature), nil); err != nil {
							if err != pgperrors.ErrUnknownIssuer {
								w.WriteHeader(400)
								_, _ = fmt.Fprintf(w, "Internal: Invalid Signature for %s: %s", k, err.Error())
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
					_, _ = fmt.Fprintf(w, "signature is invalid or it was not found for environment %s", environmentName)
					return
				}

			}

			// TODO(HVG): validate that the manifest is valid yaml
			tf.Manifests[environmentName] = string(content)
		}

	}
	if len(tf.Manifests) == 0 {
		w.WriteHeader(400)
		_, _ = fmt.Fprintf(w, "No manifest files provided")
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
		} else {
			logger.FromContext(ctx).Sugar().Warnf("commit id not valid: '%s'", sourceCommitId)
		}
	} else {
		logger.FromContext(ctx).Sugar().Warnf("commit id not found: '%s'", sourceCommitId)
	}

	if previousCommitId, ok := form.Value["previous_commit_id"]; ok {
		if len(previousCommitId) != 1 {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "Invalid number of previous commit IDs provided. Expecting 1, got %d", len(previousCommitId))
			return
		}
		if !isCommitId(previousCommitId[0]) {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "Provided commit id (%s) is not valid.", previousCommitId[0])
			return
		}
		tf.PreviousCommitId = previousCommitId[0]
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
				_, _ = fmt.Fprintf(w, "Invalid version: %s", err)
				return
			}
			tf.Version = val
		}
	}

	if displayVersion, ok := form.Value["display_version"]; ok {
		if len(displayVersion) != 1 {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "Invalid number of display versions provided: %d, ", len(displayVersion))
			return
		}
		if len(displayVersion[0]) > 15 {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "DisplayVersion given should be <= 15 characters")
			return
		}
		tf.DisplayVersion = displayVersion[0]

	}
	if ciLink, ok := form.Value["ci_link"]; ok {
		if len(ciLink) != 1 {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "Invalid number of ci links provided: %d, ", len(ciLink))
			return
		}

		tf.CiLink = ciLink[0]
	}

	if revision, ok := form.Value["revision"]; ok { //Revision is an optional parameter
		if !s.Config.RevisionsEnabled {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "The release endpoint does not allow revisions (frontend.enabledRevisions = false).")
			return
		}

		if len(revision) == 1 {
			r, err := strconv.ParseUint(revision[0], 10, 64)
			if err != nil {
				w.WriteHeader(400)
				_, _ = fmt.Fprintf(w, "Invalid version: %s", err)
				return
			}
			tf.Revision = r
		} else {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "Invalid number of revisions provided: %d, ", len(revision))
			return
		}
	}

	response, err := s.BatchClient.ProcessBatch(ctx, &api.BatchRequest{Actions: []*api.BatchAction{
		{
			Action: &api.BatchAction_CreateRelease{
				CreateRelease: &tf,
			},
		}},
	})
	if err != nil {
		handleGRPCError(ctx, w, err)
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

	writeCorrespondingResponse(ctx, w, r, releaseResponse, err)
	logger.FromContext(ctx).Warn("warning: The /release endpoint will be deprecated in the future, use /api/release instead. Check https://github.com/freiheit-com/kuberpult/blob/main/docs/endpoint-release.md for more information.\n")
}

func (s Server) handleApiRelease(w http.ResponseWriter, r *http.Request, tail string) {
	ctx := r.Context()

	if tail != "/" {
		http.Error(w, fmt.Sprintf("Release does not accept additional path arguments, got: %s", tail), http.StatusNotFound)
		return
	}

	tf := api.CreateReleaseRequest{
		Environment:                    "",
		Application:                    "",
		Team:                           "",
		Version:                        0,
		SourceCommitId:                 "",
		SourceAuthor:                   "",
		SourceMessage:                  "",
		SourceRepoUrl:                  "",
		PreviousCommitId:               "",
		DisplayVersion:                 "",
		Manifests:                      map[string]string{},
		CiLink:                         "",
		IsPrepublish:                   false,
		DeployToDownstreamEnvironments: []string{},
		Revision:                       0,
	}
	if err := r.ParseMultipartForm(MAXIMUM_MULTIPART_SIZE); err != nil {
		w.WriteHeader(400)
		_, _ = fmt.Fprintf(w, "Invalid body: %s", err)
		return
	}
	form := r.MultipartForm
	if len(form.Value["application"]) != 1 {
		if len(form.Value["application"]) > 1 {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "Please provide single application name")
			return
		} else {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "Must provide application name")
			return
		}
	}
	application := form.Value["application"][0]
	if application == "" {
		w.WriteHeader(400)
		_, _ = fmt.Fprintf(w, "Invalid application name: '%s' - must not be empty", application)
		return
	}
	tf.Application = application
	for k, v := range form.File {
		match := manifestFieldRx.FindStringSubmatch(k)
		if match == nil {
			// it's not a manifest
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "Invalid manifest form file: '%s'. Must match '%s'", k, manifestFieldRx)
			return
		}
		environmentName := match[1]
		if len(v) != 1 {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "multiple manifests submitted for %q", environmentName)
			return
		}
		content, err := readMultipartFile(v[0])
		if err != nil {
			w.WriteHeader(500)
			_, _ = fmt.Fprintf(w, "Internal: %s", err)
			return
		}
		tf.Manifests[environmentName] = string(content)

	}
	if len(tf.Manifests) == 0 {
		w.WriteHeader(400)
		_, _ = fmt.Fprintf(w, "No manifest files provided")
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
		} else {
			logger.FromContext(ctx).Sugar().Warnf("commit id not valid: '%s'", sourceCommitId)
		}
	} else {
		logger.FromContext(ctx).Sugar().Warnf("commit id not found: '%s'", sourceCommitId)
	}

	if previousCommitId, ok := form.Value["previous_commit_id"]; ok {
		if len(previousCommitId) != 1 {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "Invalid number of previous commit IDs provided. Expecting 1, got %d", len(previousCommitId))
		}
		if !isCommitId(previousCommitId[0]) {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "Provided commit id (%s) is not valid.", previousCommitId[0])
		}
		tf.PreviousCommitId = previousCommitId[0]
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
				_, _ = fmt.Fprintf(w, "Invalid version: %s", err)
				return
			}
			tf.Version = val
		}
	}

	if displayVersion, ok := form.Value["display_version"]; ok {
		if len(displayVersion) != 1 {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "Invalid number of display versions provided: %d, ", len(displayVersion))
		}
		if len(displayVersion[0]) > 15 {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "DisplayVersion given should be <= 15 characters")
			return
		}
		tf.DisplayVersion = displayVersion[0]

	}
	if isPrepublish, ok := form.Value["is_prepublish"]; ok {
		if len(isPrepublish) != 1 {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "Invalid is_prepublish value")
			return
		}
		val, err := strconv.ParseBool(isPrepublish[0])
		if err != nil {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "Invalid is_prepublish value: %s", err)
			return
		}
		tf.IsPrepublish = val
	}
	if ciLink, ok := form.Value["ci_link"]; ok {
		if len(ciLink) != 1 {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "Invalid number of ci links provided: %d, ", len(ciLink))
			return
		}

		tf.CiLink = ciLink[0]
	}

	if revision, ok := form.Value["revision"]; ok { //Revision is an optional parameter
		if !s.Config.RevisionsEnabled {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "The release endpoint does not allow revisions (frontend.enabledRevisions = false).")
			return
		}

		if len(revision) == 1 {
			r, err := strconv.ParseUint(revision[0], 10, 64)
			if err != nil {
				w.WriteHeader(400)
				_, _ = fmt.Fprintf(w, "Invalid version: %s", err)
				return
			}
			tf.Revision = r
		} else {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "Invalid number of revisions provided: %d, ", len(revision))
			return
		}
	}

	if deployToDownstreamEnvironments, ok := form.Value["deploy_to_downstream_environments"]; ok {
		tf.DeployToDownstreamEnvironments = deployToDownstreamEnvironments
	}
	response, err := s.BatchClient.ProcessBatch(ctx, &api.BatchRequest{Actions: []*api.BatchAction{
		{
			Action: &api.BatchAction_CreateRelease{
				CreateRelease: &tf,
			},
		}},
	})
	if err != nil {
		handleGRPCError(ctx, w, err)
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
	writeCorrespondingResponse(ctx, w, r, releaseResponse, err)

}

func writeCorrespondingResponse(ctx context.Context, w http.ResponseWriter, r *http.Request, releaseResponse *api.CreateReleaseResponse, _ error) {
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
	case *api.CreateReleaseResponse_MissingManifest:
		{
			jsonBlob, err := json.Marshal(firstResponse)
			writeReleaseResponse(w, r, jsonBlob, err, http.StatusBadRequest)
		}
	case *api.CreateReleaseResponse_IsNoDownstream:
		{
			jsonBlob, err := json.Marshal(firstResponse)
			writeReleaseResponse(w, r, jsonBlob, err, http.StatusBadRequest)
		}
	default:
		{
			msg := "unknown response type"
			jsonBlob, err := json.Marshal(releaseResponse)
			logger.FromContext(ctx).Error(fmt.Sprintf("%s: %s, %s", msg, jsonBlob, err))
			writeReleaseResponse(w, r, []byte(fmt.Sprintf("%s: ,response: %s", msg, jsonBlob)), err, http.StatusInternalServerError)
		}
	}
}
