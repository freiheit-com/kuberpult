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
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/argocd/v1alpha1"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
)

type ApiVersion string

const V1Alpha1 ApiVersion = "v1alpha1"

var ApiVersions []ApiVersion = []ApiVersion{V1Alpha1}

func Render(gitUrl string, gitBranch string, config config.EnvironmentConfig, env string, apps []string) (map[ApiVersion][]byte, error) {
	if config.ArgoCd == nil {
		return nil, fmt.Errorf("no ArgoCd configured for environment %s", env)
	}
	result := map[ApiVersion][]byte{}
	if content, err := RenderV1Alpha1(gitUrl, gitBranch, config, env, apps); err != nil {
		return nil, err
	} else {
		result[V1Alpha1] = content
	}
	return result, nil
}

func RenderV1Alpha1(gitUrl string, gitBranch string, config config.EnvironmentConfig, env string, apps []string) ([]byte, error) {
	destination := v1alpha1.ApplicationDestination{
		Name:      config.ArgoCd.Destination.Name,
		Namespace: config.ArgoCd.Destination.Namespace,
		Server:    config.ArgoCd.Destination.Server,
	}
	buf := []string{}
	syncWindows := v1alpha1.SyncWindows{}
	for _, w := range config.ArgoCd.SyncWindows {
		syncWindows = append(syncWindows, &v1alpha1.SyncWindow{
			Applications: []string{"*"},
			Clusters:     []string{"*"},
			Namespaces:   []string{"*"},
			Schedule:     w.Schedule,
			Duration:     w.Duration,
			Kind:         w.Kind,
			ManualSync:   true,
		})
	}
	accessEntries := []v1alpha1.AccessEntry{}
	for _, w := range config.ArgoCd.ClusterResourceWhitelist {
		accessEntries = append(accessEntries, v1alpha1.AccessEntry{
			Kind:  w.Kind,
			Group: w.Group,
		})
	}
	project := v1alpha1.AppProject{
		TypeMeta: v1alpha1.AppProjectTypeMeta,
		ObjectMeta: v1alpha1.ObjectMeta{
			Name: env,
		},
		Spec: v1alpha1.AppProjectSpec{
			Description:              env,
			SourceRepos:              []string{"*"},
			Destinations:             []v1alpha1.ApplicationDestination{destination},
			SyncWindows:              syncWindows,
			ClusterResourceWhitelist: accessEntries,
		},
	}
	if content, err := yaml.Marshal(&project); err != nil {
		return nil, err
	} else {
		buf = append(buf, string(content))
	}
	ignoreDifferences := make([]v1alpha1.ResourceIgnoreDifferences, len(config.ArgoCd.IgnoreDifferences))
	for index, value := range config.ArgoCd.IgnoreDifferences {
		ignoreDifferences[index] = v1alpha1.ResourceIgnoreDifferences(value)
	}
	sort.Strings(apps)
	for _, name := range apps {
		app := v1alpha1.Application{
			TypeMeta: v1alpha1.ApplicationTypeMeta,
			ObjectMeta: v1alpha1.ObjectMeta{
				Name:        fmt.Sprintf("%s-%s", env, name),
				Annotations: config.ArgoCd.ApplicationAnnotations,
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: env,
				Source: v1alpha1.ApplicationSource{
					RepoURL:        gitUrl,
					Path:           filepath.Join("environments", env, "applications", name, "manifests"),
					TargetRevision: gitBranch,
				},
				Destination: destination,
				SyncPolicy: &v1alpha1.SyncPolicy{
					Automated: &v1alpha1.SyncPolicyAutomated{
						Prune:    true,
						SelfHeal: true,
					},
				},
				IgnoreDifferences: ignoreDifferences,
			},
		}
		if content, err := yaml.Marshal(&app); err != nil {
			return nil, err
		} else {
			buf = append(buf, string(content))
		}
	}
	return ([]byte)(strings.Join(buf, "---\n")), nil
}
