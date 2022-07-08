/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	xpath "github.com/freiheit-com/kuberpult/pkg/path"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/valid"
	"golang.org/x/crypto/openpgp"
	pgperrors "golang.org/x/crypto/openpgp/errors"
	"golang.org/x/sync/errgroup"
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
	ArgoCdHost string
	ArgoCdUser string
	ArgoCdPass string
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	head, tail := xpath.Shift(r.URL.Path)
	switch head {
	case "health":
		s.ServeHTTPHealth(w, r)
	case "release":
		s.ServeHTTPRelease(tail, w, r)
	case "sync":
		s.ServeHTTPSync(tail[1:], w, r)
	}
	return
}

func (s *Service) ServeHTTPHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	fmt.Fprintf(w, "ok\n")
}

func (s *Service) createTransformerFromRequest(r *http.Request) (repository.CreateApplicationVersion, string, int) {
	tf := repository.CreateApplicationVersion{
		Manifests: map[string]string{},
	}
	if err := r.ParseMultipartForm(MAXIMUM_MULTIPART_SIZE); err != nil {
		return tf, fmt.Sprintf("Invalid body: %s", err), 400
	}
	form := r.MultipartForm
	if len(form.Value["application"]) != 1 {
		if len(form.Value["application"]) > 1 {
			return tf, fmt.Sprintf("Please provide single application name"), 400
		} else {
			return tf, fmt.Sprintf("Invalid application name"), 400
		}
	}
	application := form.Value["application"][0]
	if !valid.ApplicationName(application) {
		return tf, fmt.Sprintf("Invalid application name"), 400
	}
	tf.Application = application
	for k, v := range form.File {
		match := manifestFieldRx.FindStringSubmatch(k)
		if match != nil {
			environmentName := match[1]
			if len(v) != 1 {
				return tf, fmt.Sprintf("multiple manifests submitted for %q", environmentName), 400
			}
			if content, err := readMultipartFile(v[0]); err != nil {
				return tf, fmt.Sprintf("Internal: %s", err), 500
			} else {
				if s.KeyRing != nil {
					validSignature := false
					for _, sig := range form.File[fmt.Sprintf("signatures[%s]", environmentName)] {
						if signature, err := readMultipartFile(sig); err != nil {
							return tf, fmt.Sprintf("Internal: %s", err), 500
						} else {
							if _, err := openpgp.CheckArmoredDetachedSignature(s.KeyRing, bytes.NewReader(content), bytes.NewReader(signature)); err != nil {
								if err != pgperrors.ErrUnknownIssuer {
									return tf, fmt.Sprintf("Internal: %s", err), 500
								}
							} else {
								validSignature = true
								break
							}
						}
					}
					if !validSignature {
						return tf, fmt.Sprintf("Invalid signature"), 400
					}

				}

				// TODO(HVG): validate that the manifest is valid yaml
				tf.Manifests[environmentName] = string(content)
			}
		}
	}
	if len(tf.Manifests) == 0 {
		return tf, fmt.Sprintf("No manifest files provided"), 400
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
				return tf, fmt.Sprintf("Invalid version: %s", err), 400
			}
			tf.Version = val
		}
	}
	return tf, "", 201
}

func (s *Service) ServeHTTPRelease(tail string, w http.ResponseWriter, r *http.Request) {
	tf, errorString, returnCode := s.createTransformerFromRequest(r)
	if returnCode != 201 {
		w.WriteHeader(returnCode)
		fmt.Fprintf(w, errorString)
		return
	}

	if err := s.Repository.Apply(r.Context(), &tf); err != nil {
		if _, ok := err.(*repository.InternalError); ok {
			w.WriteHeader(500)
			fmt.Fprintf(w, "internal: %s", err)
			return
		} else if errors.Is(err, repository.ErrReleaseAlreadyExist) {
			w.WriteHeader(200)
			fmt.Fprintf(w, "not updated")
			return
		} else {
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

func (s *Service) ServeHTTPSync(env string, w http.ResponseWriter, r *http.Request) {
	state := s.Repository.State()
	apps, err := state.GetEnvironmentApplications(env)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "unexpected error: cannot read apps in environment %v\n", env)
		return
	}

	g := new(errgroup.Group)
	for idx := range apps {
		argocd_app_name := env + "-" + apps[idx]
		g.Go(func() error {
			_, err := argocdSyncApp(argocd_app_name)
			return err
		})
	}
	err = g.Wait()
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprint(w, "cannot sync some apps\n")
		return
	}
	w.WriteHeader(200)
	fmt.Fprintf(w, "All apps synced in %v\n", env)
	return
}

func argocdSyncApp(name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "argocd", "app", "sync", name)
	_, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return "", wrapArgoError(err, name, "ArgoCD sync app timeout")
	}
	if err != nil {
		return "", wrapArgoError(err, name, fmt.Sprintf("Cannot sync app: %v\n", name))
	}
	return "", nil
}

func ArgocdLogin(host string, username string, password string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "argocd", "login", host, "--username", username, "--password", password, "--plaintext", "--logformat", "json")
	_, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return "", wrapArgoError(err, "login", "ArgoCD login timeout")
	}
	if err != nil {
		return "", wrapArgoError(err, "login", "Cannot login to ArgoCD")
	}
	return "", nil
}

func wrapArgoError(e error, app string, message string) error {
	return fmt.Errorf("%s '%s': %w", message, app, e)
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
