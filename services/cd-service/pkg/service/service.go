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

package service

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"reflect"
	"regexp"
	"strconv"

	"github.com/ProtonMail/go-crypto/openpgp"
	pgperrors "github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	xpath "github.com/freiheit-com/kuberpult/pkg/path"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/valid"
	"go.uber.org/zap"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

const (
	// This maximum in-memory multipart size.
	// It was chosen based on the assumption that we have < 10 envs with < 3MB manifests per env.
	MAXIMUM_MULTIPART_SIZE = 32 * 1024 * 1024 // = 32Mi
)

var (
	manifestFieldRx = regexp.MustCompile(`\Amanifests\[([^]]+)\]\z`)
	// matches hex strings with 7 - 40 chars
	commitIdRx = regexp.MustCompile(`\A[0-9a-f]{7,40}\z`)
	// parses anything that looks like "name <mail@host.com>"
	authorRx = regexp.MustCompile(`\A[^<\n]+( <[^@\n]+@[^>\n]+>)?\z`)
)

type Service struct {
	Repository repository.Repository
	KeyRing    openpgp.KeyRing
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	head, tail := xpath.Shift(r.URL.Path)
	switch head {
	case "health":
		s.ServeHTTPHealth(w, r)
	case "release":
		s.ServeHTTPRelease(tail, w, r)
	}
	return
}

func (s *Service) ServeHTTPHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	fmt.Fprintf(w, "ok\n")
}

func (s *Service) ServeHTTPRelease(tail string, w http.ResponseWriter, r *http.Request) {
	tf := repository.CreateApplicationVersion{
		Manifests: map[string]string{},
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
			fmt.Fprintf(w, "Invalid application name")
			return
		}
	}
	application := form.Value["application"][0]
	if !valid.ApplicationName(application) {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Invalid application name: '%s' - must match regexp '%s' and less than %d characters", application, `[a-z0-9]+(?:-[a-z0-9]+)*`, 40)
		return
	}
	tf.Application = application
	for k, v := range form.File {
		match := manifestFieldRx.FindStringSubmatch(k)
		if match != nil {
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
									w.WriteHeader(500)
									fmt.Fprintf(w, "Internal: Invalid Signature: %s", err)
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
						fmt.Fprintf(w, "Invalid signature")
						return
					}

				}

				// TODO(HVG): validate that the manifest is valid yaml
				tf.Manifests[environmentName] = string(content)
			}
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

	if source_commit_id, ok := form.Value["source_commit_id"]; ok {
		if len(source_commit_id) == 1 && isCommitId(source_commit_id[0]) {
			tf.SourceCommitId = source_commit_id[0]
		}
	}

	if source_author, ok := form.Value["source_author"]; ok {
		if len(source_author) == 1 && isAuthor(source_author[0]) {
			tf.SourceAuthor = source_author[0]
		}
	}

	if source_message, ok := form.Value["source_message"]; ok {
		if len(source_message) == 1 {
			tf.SourceMessage = source_message[0]
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

	if err := s.Repository.Apply(r.Context(), &tf); err != nil {
		if ierr, ok := err.(*repository.InternalError); ok {
			if span, ok := tracer.SpanFromContext(r.Context()); ok {
				span.SetTag(ext.ErrorType, fmt.Sprintf("%s", reflect.TypeOf(ierr)))
				span.SetTag(ext.ErrorMsg, fmt.Sprintf("%s", ierr))
			}
			logger.FromContext(r.Context()).Error("http.handle", zap.Error(ierr))
			w.WriteHeader(500)
			fmt.Fprintf(w, "internal: %s", err)
			return
		} else if errors.Is(err, repository.ErrReleaseAlreadyExist) {
			w.WriteHeader(200)
			fmt.Fprintf(w, "not updated")
			return
		} else {
			if span, ok := tracer.SpanFromContext(r.Context()); ok {
				span.SetTag(ext.ErrorType, fmt.Sprintf("%s", reflect.TypeOf(err)))
				span.SetTag(ext.ErrorMsg, fmt.Sprintf("%s", err))
			}
			logger.FromContext(r.Context()).Error("http.handle", zap.Error(err))
			w.WriteHeader(400)
			fmt.Fprintf(w, "internal: %s", err)
			return
		}
	} else {
		w.WriteHeader(201)
		fmt.Fprintf(w, "created")
	}
	return
}

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

var _ http.Handler = (*Service)(nil)
