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

package argocd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"sigs.k8s.io/yaml"

	"github.com/freiheit-com/kuberpult/pkg/argocd/v1alpha1"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/types"
)

type ApiVersion string

const V1Alpha1 ApiVersion = "v1alpha1"

type AppData struct {
	AppName  string
	TeamName string
}

type EnvironmentInfo struct {
	ArgoCDConfig          *config.EnvironmentConfigArgoCd
	CommonPrefix          string
	ParentEnvironmentName types.EnvName
	IsAAEnv               bool
}

func (e *EnvironmentInfo) GetFullyQualifiedName() string {
	if e.IsAAEnv {
		return e.CommonPrefix + "-" + string(e.ParentEnvironmentName) + "-" + e.ArgoCDConfig.ConcreteEnvName
	}
	return string(e.ParentEnvironmentName)
}

func Render(ctx context.Context, gitUrl string, gitBranch string, info *EnvironmentInfo, appsData []AppData) (map[ApiVersion][]byte, error) {
	span, _ := tracer.StartSpanFromContext(ctx, "Render")
	defer span.Finish()
	if info.ArgoCDConfig == nil {
		return nil, fmt.Errorf("no ArgoCd configured for environment %s", info.GetFullyQualifiedName())
	}
	result := map[ApiVersion][]byte{}
	if content, err := RenderV1Alpha1(gitUrl, gitBranch, info, appsData); err != nil {
		return nil, err
	} else {
		result[V1Alpha1] = content
	}
	return result, nil
}

func RenderV1Alpha1(gitUrl string, gitBranch string, info *EnvironmentInfo, appsData []AppData) ([]byte, error) {
	applicationNs := ""
	config := info.ArgoCDConfig
	if config.Destination.Namespace != nil {
		applicationNs = *config.Destination.Namespace
	} else if config.Destination.ApplicationNamespace != nil {
		applicationNs = *config.Destination.ApplicationNamespace
	}
	applicationDestination := v1alpha1.ApplicationDestination{
		Name:      config.Destination.Name,
		Namespace: applicationNs,
		Server:    config.Destination.Server,
	}
	buf := []string{}
	syncWindows := v1alpha1.SyncWindows{}
	for _, w := range config.SyncWindows {
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
	for _, w := range config.ClusterResourceWhitelist {
		accessEntries = append(accessEntries, v1alpha1.AccessEntry{
			Kind:  w.Kind,
			Group: w.Group,
		})
	}

	appProjectNs := ""
	appProjectDestination := applicationDestination
	if config.Destination.Namespace != nil {
		appProjectNs = *config.Destination.Namespace
	} else if config.Destination.AppProjectNamespace != nil {
		appProjectNs = *config.Destination.AppProjectNamespace
	}
	appProjectDestination.Namespace = appProjectNs

	project := v1alpha1.AppProject{
		TypeMeta: v1alpha1.AppProjectTypeMeta,
		ObjectMeta: v1alpha1.ObjectMeta{
			Annotations: nil,
			Labels:      nil,
			Finalizers:  nil,
			Name:        info.GetFullyQualifiedName(),
		},
		Spec: v1alpha1.AppProjectSpec{
			Description:              info.GetFullyQualifiedName(),
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
	ignoreDifferences := make([]v1alpha1.ResourceIgnoreDifferences, len(config.IgnoreDifferences))
	for index, value := range config.IgnoreDifferences {
		ignoreDifferences[index] = v1alpha1.ResourceIgnoreDifferences(value)
	}
	syncOptions := config.SyncOptions
	for _, appData := range appsData {
		appManifest, err := RenderAppEnv(gitUrl, gitBranch, config.ApplicationAnnotations, info, appData, applicationDestination, ignoreDifferences, syncOptions)
		if err != nil {
			return nil, err
		}
		buf = append(buf, appManifest)
	}
	return ([]byte)(strings.Join(buf, "---\n")), nil
}

func RenderAppEnv(gitUrl string, gitBranch string, applicationAnnotations map[string]string, info *EnvironmentInfo, appData AppData, destination v1alpha1.ApplicationDestination, ignoreDifferences []v1alpha1.ResourceIgnoreDifferences, syncOptions v1alpha1.SyncOptions) (string, error) {
	name := appData.AppName
	annotations := map[string]string{}
	labels := map[string]string{}
	manifestPath := filepath.Join("environments", string(info.ParentEnvironmentName), "applications", name, "manifests")
	for k, v := range applicationAnnotations {
		annotations[k] = v
	}
	annotations["com.freiheit.kuberpult/team"] = appData.TeamName
	annotations["com.freiheit.kuberpult/application"] = name
	annotations["com.freiheit.kuberpult/environment"] = info.GetFullyQualifiedName()
	annotations["com.freiheit.kuberpult/aa-parent-environment"] = string(info.ParentEnvironmentName)
	// This annotation is so that argoCd does not invalidate *everything* in the whole repo when receiving a git webhook.
	// It has to start with a "/" to be absolute to the git repo.
	// See https://argo-cd.readthedocs.io/en/stable/operator-manual/high_availability/#webhook-and-manifest-paths-annotation
	annotations["argocd.argoproj.io/manifest-generate-paths"] = "/" + manifestPath
	labels["com.freiheit.kuberpult/team"] = appData.TeamName
	app := v1alpha1.Application{
		TypeMeta: v1alpha1.ApplicationTypeMeta,
		ObjectMeta: v1alpha1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s", info.GetFullyQualifiedName(), name),
			Annotations: annotations,
			Labels:      labels,
			Finalizers:  calculateFinalizers(),
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: info.GetFullyQualifiedName(),
			Source: v1alpha1.ApplicationSource{
				RepoURL:        gitUrl,
				Path:           manifestPath,
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
