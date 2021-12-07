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
package argocd

import (
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/argocd/v1alpha1"
	godebug "github.com/kylelemons/godebug/diff"
	"testing"
)

func TestRender(t *testing.T) {
	tcs := []struct {
		Name              string
		IsUndeployVersion bool
		ExpectedResult    string
	}{
		{
			Name:              "deploy",
			IsUndeployVersion: false,
			ExpectedResult: `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: dev-app1
spec:
  destination: {}
  project: dev
  source:
    path: environments/dev/applications/app1/manifests
    repoURL: example.com/github
    targetRevision: main
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
`,
		},
		{
			Name:              "undeploy",
			IsUndeployVersion: true,
			ExpectedResult: `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  finalizers:
  - resources-finalizer.argocd.argoproj.io
  name: dev-app1
spec:
  destination: {}
  project: dev
  source:
    path: environments/dev/applications/app1/manifests
    repoURL: example.com/github
    targetRevision: main
  syncPolicy:
    automated:
      allowEmpty: true
      prune: true
      selfHeal: true
`,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			var (
				annotations       = map[string]string{}
				ignoreDifferences = []v1alpha1.ResourceIgnoreDifferences{}
				destination       = v1alpha1.ApplicationDestination{}
				GitUrl            = "example.com/github"
				gitBranch         = "main"
				env               = "dev"
				appData           = AppData{
					AppName:           "app1",
					IsUndeployVersion: tc.IsUndeployVersion,
				}
			)

			actualResult, err := RenderApp(GitUrl, gitBranch, annotations, env, appData, destination, ignoreDifferences)
			if err != nil {
				t.Fatal(err)
			}
			if actualResult != tc.ExpectedResult {
				t.Fatalf("unexpected argocd manifest:\ndiff:\n%s\n\n", godebug.Diff(tc.ExpectedResult, actualResult))
			}
		})
	}
}
