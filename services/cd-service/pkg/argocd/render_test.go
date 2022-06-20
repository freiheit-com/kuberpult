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
	"testing"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/argocd/v1alpha1"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/google/go-cmp/cmp"
	godebug "github.com/kylelemons/godebug/diff"
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
    syncOptions:
    - ApplyOutOfSyncOnly=true
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
    syncOptions:
    - ApplyOutOfSyncOnly=true
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
				syncOptions = []string{"ApplyOutOfSyncOnly=true"}
			)

			actualResult, err := RenderApp(GitUrl, gitBranch, annotations, env, appData, destination, ignoreDifferences, syncOptions)
			if err != nil {
				t.Fatal(err)
			}
			if actualResult != tc.ExpectedResult {
				t.Fatalf("unexpected argocd manifest:\ndiff:\n%s\n\n", godebug.Diff(tc.ExpectedResult, actualResult))
			}
		})
	}
}

func TestRenderV1Alpha1(t *testing.T) {
	tests := []struct {
		name    string
		config  config.EnvironmentConfig
		want    string
		wantErr bool
	}{
		{
			name: "without sync window",
			config: config.EnvironmentConfig{
				ArgoCd: &config.EnvironmentConfigArgoCd{
					SyncWindows: nil,
				},
			},
			want: `apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: test-env
spec:
  description: test-env
  destinations:
  - {}
  sourceRepos:
  - '*'
`,
		},
		{
			name: "with sync window without apps",
			config: config.EnvironmentConfig{
				ArgoCd: &config.EnvironmentConfigArgoCd{
					SyncWindows: []config.ArgoCdSyncWindow{
						{
							Schedule: "not a valid crontab entry",
							Duration: "invalid duration",
							Kind:     "neither deny nor allow",
							Apps:     nil,
						},
					},
				},
			},
			want: `apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: test-env
spec:
  description: test-env
  destinations:
  - {}
  sourceRepos:
  - '*'
  syncWindows:
  - applications:
    - '*'
    clusters:
    - '*'
    duration: invalid duration
    kind: neither deny nor allow
    manualSync: true
    namespaces:
    - '*'
    schedule: not a valid crontab entry
`,
		},
		{
			name: "with sync window with apps",
			config: config.EnvironmentConfig{
				ArgoCd: &config.EnvironmentConfigArgoCd{
					SyncWindows: []config.ArgoCdSyncWindow{
						{
							Schedule: "not a valid crontab entry",
							Duration: "invalid duration",
							Kind:     "neither deny nor allow",
							Apps: []string{
								"app*",
							},
						},
					},
				},
			},
			want: `apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: test-env
spec:
  description: test-env
  destinations:
  - {}
  sourceRepos:
  - '*'
  syncWindows:
  - applications:
    - app*
    clusters:
    - '*'
    duration: invalid duration
    kind: neither deny nor allow
    manualSync: true
    namespaces:
    - '*'
    schedule: not a valid crontab entry
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const (
				gitUrl    = "https://git.example.com/"
				gitBranch = "branch-name"
				env       = "test-env"
			)
			got, err := RenderV1Alpha1(gitUrl, gitBranch, tt.config, env, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, string(got)); d != "" {
				t.Errorf("mismatch: %s", d)
			}
		})
	}
}
