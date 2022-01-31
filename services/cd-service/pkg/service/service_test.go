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
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path"
	"reflect"
	"testing"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"golang.org/x/crypto/openpgp"
)

func TestServeHttpSuccess(t *testing.T) {
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
		Name           string
		Application    string
		SourceCommitId string
		SourceAuthor   string
		SourceMessage  string
		Manifests      map[string]string
		Signatures     map[string]string
		KeyRing        openpgp.KeyRing
		ExpectedStatus int
		Tests          func(t *testing.T, repo repository.Repository, resp *http.Response, remoteDir string)
	}{
		{
			Name:           "It accepts a set of manifests",
			Application:    "demo",
			Manifests:      exampleManifests,
			ExpectedStatus: 204,
			Tests: func(t *testing.T, repo repository.Repository, resp *http.Response, remoteDir string) {
				head := repo.State()
				if apps, err := head.Applications(); err != nil {
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
			Name:           "It stores source information",
			Application:    "demo",
			Manifests:      exampleManifests,
			SourceAuthor:   "Älejandrø \"weirdma\" <alejandro@weirdma.il>",
			SourceMessage:  "Did something",
			SourceCommitId: "deadbeef",
			ExpectedStatus: 204,
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
			ExpectedStatus: 204,
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
			Name:           "Rejects missing signatures",
			Application:    "demo",
			Manifests:      exampleManifests,
			KeyRing:        exampleKeyRing,
			ExpectedStatus: 400,
			Tests:          func(t *testing.T, repo repository.Repository, resp *http.Response, remoteDir string) {},
		},
		{
			Name:           "Accepts valid signatures",
			Application:    "demo",
			Manifests:      exampleManifests,
			KeyRing:        exampleKeyRing,
			Signatures:     exampleSignatures,
			ExpectedStatus: 204,
			Tests:          func(t *testing.T, repo repository.Repository, resp *http.Response, remoteDir string) {},
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
				context.Background(),
				repository.Config{
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
				KeyRing:    tc.KeyRing,
			}
			// start server
			srv := httptest.NewServer(service)
			defer srv.Close()
			// first request
			var buf bytes.Buffer
			body := multipart.NewWriter(&buf)
			if err := body.WriteField("application", tc.Application); err != nil {
				t.Fatal(err)
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
			body.Close()

			if resp, err := http.Post(srv.URL+"/release", "multipart/form-data; boundary="+body.Boundary(), &buf); err != nil {
				t.Fatal(err)
			} else {
				if resp.StatusCode != tc.ExpectedStatus {
					t.Fatalf("expected http status %d, received %d", tc.ExpectedStatus, resp.StatusCode)
				}
				tc.Tests(t, repo, resp, remoteDir)
			}
		})
	}

}
