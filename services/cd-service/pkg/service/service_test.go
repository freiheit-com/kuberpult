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
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/testutil"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path"
	"reflect"
	"testing"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/ProtonMail/go-crypto/openpgp"
)

func TestServeHttp(t *testing.T) {
	exampleManifests :=
		map[string]string{
			"development": "---\nkind: Test",
		}
	exampleKey, err := openpgp.NewEntity("Test", "", "test@example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	exampleKeyRing := openpgp.EntityList{exampleKey}
	exampleSignatures :=
		map[string]string{}

	signatureBuffer := bytes.Buffer{}
	err = openpgp.ArmoredDetachSign(&signatureBuffer, exampleKey, bytes.NewReader([]byte(exampleManifests["development"])), nil)
	if err != nil {
		t.Fatal(err)
	}
	exampleSignatures["development"] = signatureBuffer.String()

	t.Logf("signatures: %#v\n", exampleSignatures)

	tcs := []struct {
		Name             string
		Application      string
		SourceCommitId   string
		SourceAuthor     string
		SourceMessage    string
		Version          string
		Manifests        map[string]string
		Signatures       map[string]string
		AdditionalFields map[string]string
		AdditionalFiles  map[string]string
		KeyRing          openpgp.KeyRing
		Setup            []repository.Transformer
		ExpectedStatus   int
		ExpectedError    string
		Tests            func(t *testing.T, repo repository.Repository, resp *http.Response, remoteDir string)
	}{
		{
			Name:           "It accepts a set of manifests",
			Application:    "demo",
			Manifests:      exampleManifests,
			ExpectedStatus: 201,
			Tests: func(t *testing.T, repo repository.Repository, resp *http.Response, remoteDir string) {
				head := repo.State()
				if apps, err := head.GetApplications(); err != nil {
					t.Fatal(err)
				} else if !reflect.DeepEqual(apps, []string{"demo"}) {
					t.Fatalf("expected applications to be exactly 'demo', got %q", apps)
				}

				if releases, err := head.Releases("demo"); err != nil {
					t.Fatal(err)
				} else if !reflect.DeepEqual(releases, []uint64{1}) {
					t.Fatalf("expected releases to be just 1, got %q", releases)
				}

				if manifests, err := head.ReleaseManifests("demo", 1); err != nil {
					t.Fatal(err)
				} else if !reflect.DeepEqual(manifests, exampleManifests) {
					t.Fatalf("expected manifest to be the same as the example manifests, got %#v", manifests)
				}

				expectedMsg := "Author: defaultUser <local.user@freiheit.com>\n" +
					"Committer: kuberpult <kuberpult@freiheit.com>\n" +
					"created version 1 of \"demo\"\n\n"
				cmd := exec.Command("git", "--git-dir="+remoteDir, "log", "--format=Author: %an <%ae>%nCommitter: %cn <%ce>%n%B", "-n", "1", "HEAD")
				if out, err := cmd.Output(); err != nil {
					t.Fatal(err)
				} else {
					if string(out) != expectedMsg {
						t.Fatalf("unexpected output: '%s', expected: '%s'", out, expectedMsg)
					}
				}
			},
		},
		{
			Name:           "Proper error when no application provided",
			ExpectedStatus: 400,
			ExpectedError:  "Invalid application name",
		},
		{
			Name:           "Proper error when multiple application provided",
			ExpectedStatus: 400,
			Application:    "demo",
			AdditionalFields: map[string]string{
				"application": "demo",
			},
			ExpectedError: "Please provide single application name",
		},
		{
			Name:           "Proper error when long application name provided",
			Application:    "demoWithTooManyCharactersInItsNameToBeValid",
			ExpectedStatus: 400,
			ExpectedError:  "Invalid application name: 'demoWithTooManyCharactersInItsNameToBeValid' - must match regexp '[a-z0-9]+(?:-[a-z0-9]+)*' and less than 40 characters",
		},
		{
			Name:           "Proper error when invalid application name provided",
			Application:    "invalidCharactersInName?",
			ExpectedStatus: 400,
			ExpectedError:  "Invalid application name: 'invalidCharactersInName?' - must match regexp '[a-z0-9]+(?:-[a-z0-9]+)*' and less than 40 characters",
		},
		{
			Name:           "Proper error when no manifests provided",
			ExpectedStatus: 400,
			Application:    "demo",
			ExpectedError:  "No manifest files provided",
		},
		{
			Name:           "Proper error when multiple manifests provided",
			ExpectedStatus: 400,
			Application:    "demo",
			Manifests:      exampleManifests,
			AdditionalFiles: map[string]string{
				"manifests[development]": "content",
			},
			ExpectedError: `multiple manifests submitted for "development"`,
		},
		{
			Name:           "Proper error when no signature provided",
			ExpectedStatus: 400,
			Application:    "demo",
			Manifests:      exampleManifests,
			KeyRing:        exampleKeyRing,
			ExpectedError:  "Invalid signature",
		},
		{
			Name:           "Proper error when invalid signature provided",
			ExpectedStatus: 500,
			Application:    "demo",
			Manifests:      exampleManifests,
			KeyRing:        exampleKeyRing,
			Signatures: map[string]string{
				"development": "invalid sign",
			},
			ExpectedError: "Internal: Invalid Signature: EOF",
		},
		{
			Name:           "It stores source information",
			Application:    "demo",
			Manifests:      exampleManifests,
			SourceAuthor:   "Älejandrø \"weirdma\" <alejandro@weirdma.il>",
			SourceMessage:  "Did something",
			SourceCommitId: "deadbeef",
			ExpectedStatus: 201,
			Tests: func(t *testing.T, repo repository.Repository, resp *http.Response, remoteDir string) {
				head := repo.State()
				rel, err := head.GetApplicationRelease("demo", 1)
				if err != nil {
					t.Fatal(err)
				}
				if rel.SourceAuthor != "Älejandrø \"weirdma\" <alejandro@weirdma.il>" {
					t.Errorf("unexpected source author: expected \"Älejandrø \\\"weirdma\\\" <alejandro@weirdma.il>\", actual %q", rel.SourceAuthor)
				}
				if rel.SourceCommitId != "deadbeef" {
					t.Errorf("unexpected source commit id: expected \"deadbeef\", actual %q", rel.SourceCommitId)
				}
				if rel.SourceMessage != "Did something" {
					t.Errorf("unexpected source message: expected \"Did something\", actual %q", rel.SourceMessage)
				}
			},
		},
		{
			Name:           "It ignores broken source information",
			Application:    "demo",
			Manifests:      exampleManifests,
			SourceAuthor:   "Not an email\nbut a multiline\ntext",
			SourceCommitId: "Not hex",
			ExpectedStatus: 201,
			Tests: func(t *testing.T, repo repository.Repository, resp *http.Response, remoteDir string) {
				head := repo.State()
				rel, err := head.GetApplicationRelease("demo", 1)
				if err != nil {
					t.Fatal(err)
				}
				if rel.SourceAuthor != "" {
					t.Errorf("unexpected source author: expected \"\", actual %q", rel.SourceAuthor)
				}
				if rel.SourceCommitId != "" {
					t.Errorf("unexpected source commit id: expected \"\", actual %q", rel.SourceCommitId)
				}
				if rel.SourceMessage != "" {
					t.Errorf("unexpected source message: expected \"\", actual %q", rel.SourceMessage)
				}
			},
		},
		{
			Name:           "Accepts valid signatures",
			Application:    "demo",
			Manifests:      exampleManifests,
			KeyRing:        exampleKeyRing,
			Signatures:     exampleSignatures,
			ExpectedStatus: 201,
			Tests:          func(t *testing.T, repo repository.Repository, resp *http.Response, remoteDir string) {},
		},
		{
			Name:           "It accepts a version",
			Application:    "demo",
			Manifests:      exampleManifests,
			Version:        "42",
			ExpectedStatus: 201,
			Tests: func(t *testing.T, repo repository.Repository, resp *http.Response, remoteDir string) {
				head := repo.State()

				if releases, err := head.Releases("demo"); err != nil {
					t.Fatal(err)
				} else if !reflect.DeepEqual(releases, []uint64{42}) {
					t.Fatalf("expected releases to be just 42, got %q", releases)
				}
			},
		},
		{
			Name:           "It accepts a duplicate version",
			Application:    "demo",
			Manifests:      exampleManifests,
			Version:        "42",
			ExpectedStatus: 200,
			Setup: []repository.Transformer{
				&repository.CreateApplicationVersion{
					Application: "demo",
					Version:     42,
					Manifests:   exampleManifests,
				},
			},
			Tests: func(t *testing.T, repo repository.Repository, resp *http.Response, remoteDir string) {
				head := repo.State()

				if releases, err := head.Releases("demo"); err != nil {
					t.Fatal(err)
				} else if !reflect.DeepEqual(releases, []uint64{42}) {
					t.Fatalf("expected releases to be just 42, got %q", releases)
				}
			},
		},
		{
			Name:           "It rejects non numeric versions",
			Application:    "demo",
			Manifests:      exampleManifests,
			Version:        "foo",
			ExpectedStatus: 400,
			ExpectedError:  `Invalid version: strconv.ParseUint: parsing "foo": invalid syntax`,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			// setup repository
			dir := t.TempDir()
			remoteDir := path.Join(dir, "remote")
			localDir := path.Join(dir, "local")
			cmd := exec.Command("git", "init", "--bare", remoteDir)
			cmd.Start()
			cmd.Wait()
			repo, err := repository.New(
				testutil.MakeTestContext(),
				repository.RepositoryConfig{
					URL:            remoteDir,
					Path:           localDir,
					CommitterEmail: "kuberpult@freiheit.com",
					CommitterName:  "kuberpult",
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if tc.Setup != nil {
				err := repo.Apply(testutil.MakeTestContext(), tc.Setup...)
				if err != nil {
					t.Fatal(err)
				}
			}
			// setup service
			service := &Service{
				Repository: repo,
				KeyRing:    tc.KeyRing,
			}
			// start server
			srv := httptest.NewServer(service)
			defer srv.Close()
			// first request
			var buf bytes.Buffer
			body := multipart.NewWriter(&buf)
			if tc.Application != "" {
				if err := body.WriteField("application", tc.Application); err != nil {
					t.Fatal(err)
				}
			}
			if len(tc.AdditionalFields) > 0 {
				for k, v := range tc.AdditionalFields {
					if err := body.WriteField(k, v); err != nil {
						t.Fatal(err)
					}
				}
			}
			if len(tc.AdditionalFiles) > 0 {
				for k, v := range tc.AdditionalFiles {
					if w, err := body.CreateFormFile(k, "doesntmatter"); err != nil {
						t.Fatal(err)
					} else {
						fmt.Fprint(w, v)
					}
				}
			}

			for k, v := range tc.Manifests {
				if w, err := body.CreateFormFile("manifests["+k+"]", "doesntmatter"); err != nil {
					t.Fatal(err)
				} else {
					fmt.Fprint(w, v)
				}
			}
			for k, v := range tc.Signatures {
				if w, err := body.CreateFormFile("signatures["+k+"]", "doesntmatter"); err != nil {
					t.Fatal(err)
				} else {
					fmt.Fprint(w, v)
				}
			}
			if tc.SourceAuthor != "" {
				if err := body.WriteField("source_author", tc.SourceAuthor); err != nil {
					t.Fatal(err)
				}
			}
			if tc.SourceCommitId != "" {
				if err := body.WriteField("source_commit_id", tc.SourceCommitId); err != nil {
					t.Fatal(err)
				}
			}
			if tc.SourceMessage != "" {
				if err := body.WriteField("source_message", tc.SourceMessage); err != nil {
					t.Fatal(err)
				}
			}
			if tc.Version != "" {
				if err := body.WriteField("version", tc.Version); err != nil {
					t.Fatal(err)
				}
			}
			body.Close()

			request, err := http.NewRequest("POST", srv.URL+"/release", &buf)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set(auth.HeaderUserEmail, auth.Encode64("local.user@freiheit.com"))
			request.Header.Set(auth.HeaderUserName, auth.Encode64("defaultUser"))
			request.Header.Set("content-type", "multipart/form-data; boundary="+body.Boundary())
			resp, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			} else {
				if resp.StatusCode != tc.ExpectedStatus {
					t.Fatalf("expected http status %d, received %d", tc.ExpectedStatus, resp.StatusCode)
				}
				if len(tc.ExpectedError) > 0 {
					bodyBytes, err := io.ReadAll(resp.Body)
					if err != nil {
						t.Fatal(err)
					}
					bodyString := string(bodyBytes)
					if bodyString != tc.ExpectedError {
						t.Fatalf(`expected http body "%s", received "%s"`, tc.ExpectedError, bodyString)
					}

				}
				if tc.Tests != nil {
					tc.Tests(t, repo, resp, remoteDir)
				}
			}
		})
	}

}

func TestServeHttpEmptyBody(t *testing.T) {
	exampleKey, err := openpgp.NewEntity("Test", "", "test@example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	exampleKeyRing := openpgp.EntityList{exampleKey}
	tcs := []struct {
		Name           string
		ExpectedStatus int
		ExpectedError  string
		FormMetaData   string
	}{{
		Name:           "Error when no boundary provided",
		ExpectedStatus: 400,
		ExpectedError:  "Invalid body: no multipart boundary param in Content-Type",
		FormMetaData:   "multipart/form-data;",
	}, {
		Name:           "Error when no content provided",
		ExpectedStatus: 400,
		ExpectedError:  "Invalid body: multipart: NextPart: EOF",
		FormMetaData:   "multipart/form-data;boundary=nonExistantBoundary;",
	}}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			// setup repository
			dir := t.TempDir()
			remoteDir := path.Join(dir, "remote")
			localDir := path.Join(dir, "local")
			cmd := exec.Command("git", "init", "--bare", remoteDir)
			cmd.Start()
			cmd.Wait()
			repo, err := repository.New(
				testutil.MakeTestContext(),
				repository.RepositoryConfig{
					URL:            remoteDir,
					Path:           localDir,
					CommitterEmail: "kuberpult@freiheit.com",
					CommitterName:  "kuberpult",
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			// setup service
			service := &Service{
				Repository: repo,
				KeyRing:    exampleKeyRing,
			}
			// start server
			srv := httptest.NewServer(service)
			defer srv.Close()
			// first request
			var buf bytes.Buffer
			body := multipart.NewWriter(&buf)
			body.Close()

			if resp, err := http.Post(srv.URL+"/release", tc.FormMetaData, &buf); err != nil {
				t.Fatal(err)
			} else {
				if resp.StatusCode != tc.ExpectedStatus {
					t.Fatalf("expected http status %d, received %d", tc.ExpectedStatus, resp.StatusCode)
				}
				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}
				bodyString := string(bodyBytes)
				if bodyString != tc.ExpectedError {
					t.Fatalf(`expected http body "%s", received "%s"`, tc.ExpectedError, bodyString)
				}
			}
		})
	}
}
