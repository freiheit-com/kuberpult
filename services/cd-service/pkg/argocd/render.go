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

package argocd

import (
	"fmt"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/argocd/v1alpha1"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
)

type ApiVersion string

const V1Alpha1 ApiVersion = "v1alpha1"

type AppData struct {
	AppName  string
	TeamName string
}

var ApiVersions []ApiVersion = []ApiVersion{V1Alpha1}

func Render(gitUrl string, gitBranch string, config config.EnvironmentConfig, env string, appsData []AppData) (map[ApiVersion][]byte, error) {
	if config.ArgoCd == nil {
		return nil, fmt.Errorf("no ArgoCd configured for environment %s", env)
	}
	result := map[ApiVersion][]byte{}
	if content, err := RenderV1Alpha1(gitUrl, gitBranch, config, env, appsData); err != nil {
		return nil, err
	} else {
		result[V1Alpha1] = content
	}
	return result, nil
}

func RenderV1Alpha1(gitUrl string, gitBranch string, config config.EnvironmentConfig, env string, appsData []AppData) ([]byte, error) {
	applicationNs := ""
	if config.ArgoCd.Destination.Namespace != nil {
		applicationNs = *config.ArgoCd.Destination.Namespace
	} else if config.ArgoCd.Destination.ApplicationNamespace != nil {
		applicationNs = *config.ArgoCd.Destination.ApplicationNamespace
	}
	applicationDestination := v1alpha1.ApplicationDestination{
		Name:      config.ArgoCd.Destination.Name,
		Namespace: applicationNs,
		Server:    config.ArgoCd.Destination.Server,
	}
	buf := []string{}
	syncWindows := v1alpha1.SyncWindows{}
	for _, w := range config.ArgoCd.SyncWindows {
		apps := []string{"*"}
		if len(w.Apps) > 0 {
			apps = w.Apps
		}
		syncWindows = append(syncWindows, &v1alpha1.SyncWindow{
			Applications: apps,
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

	appProjectNs := ""
	appProjectDestination := applicationDestination
	if config.ArgoCd.Destination.Namespace != nil {
		appProjectNs = *config.ArgoCd.Destination.Namespace
	} else if config.ArgoCd.Destination.AppProjectNamespace != nil {
		appProjectNs = *config.ArgoCd.Destination.AppProjectNamespace
	}
	appProjectDestination.Namespace = appProjectNs

	project := v1alpha1.AppProject{
		TypeMeta: v1alpha1.AppProjectTypeMeta,
		ObjectMeta: v1alpha1.ObjectMeta{
			Name: env,
		},
		Spec: v1alpha1.AppProjectSpec{
			Description:              env,
			SourceRepos:              []string{"*"},
			Destinations:             []v1alpha1.ApplicationDestination{appProjectDestination},
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
	syncOptions := config.ArgoCd.SyncOptions
	for _, appData := range appsData {
		appManifest, err := RenderAppEnv(gitUrl, gitBranch, config.ArgoCd.ApplicationAnnotations, env, appData, applicationDestination, ignoreDifferences, syncOptions)
		if err != nil {
			return nil, err
		}
		buf = append(buf, appManifest)
	}
	return ([]byte)(strings.Join(buf, "---\n")), nil
}

func RenderAppEnv(gitUrl string, gitBranch string, applicationAnnotations map[string]string, env string, appData AppData, destination v1alpha1.ApplicationDestination, ignoreDifferences []v1alpha1.ResourceIgnoreDifferences, syncOptions v1alpha1.SyncOptions) (string, error) {
	name := appData.AppName
	annotations := map[string]string{}
	labels := map[string]string{}
	for k, v := range applicationAnnotations {
		annotations[k] = v
	}
	annotations["com.freiheit.kuberpult/team"] = appData.TeamName
	annotations["com.freiheit.kuberpult/application"] = name
	annotations["com.freiheit.kuberpult/environment"] = env
	labels["com.freiheit.kuberpult/team"] = appData.TeamName
	app := v1alpha1.Application{
		TypeMeta: v1alpha1.ApplicationTypeMeta,
		ObjectMeta: v1alpha1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s", env, name),
			Annotations: annotations,
			Labels:      labels,
			Finalizers:  calculateFinalizers(),
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
					// We always allow empty, because it makes it easier to delete apps/environments
					AllowEmpty: true,
				},
				SyncOptions: syncOptions,
			},
			IgnoreDifferences: ignoreDifferences,
		},
	}
	if content, err := yaml.Marshal(&app); err != nil {
		return "", err
	} else {
		return string(content), nil
	}
}

func calculateFinalizers() []string {
	return []string{
		"resources-finalizer.argocd.argoproj.io",
	}
}
