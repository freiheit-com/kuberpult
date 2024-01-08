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

package versions

import (
	"context"
	"fmt"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	argoApp "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/argo"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/argocd/v1alpha1"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/cmd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/v2/util/grpc"
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"k8s.io/utils/lru"
)

// This is a the user that the rollout service uses to query the versions.
// It is not written to the repository.
var RolloutServiceUser auth.User = auth.User{
	Email: "kuberpult-rollout-service@local",
	Name:  "kuberpult-rollout-service",
}

type VersionClient interface {
	GetVersion(ctx context.Context, revision, environment, application string) (*VersionInfo, error)
	ConsumeEvents(ctx context.Context, processor VersionEventProcessor, hr *setup.HealthReporter) error
}

type versionClient struct {
	overviewClient        api.OverviewServiceClient
	versionClient         api.VersionServiceClient
	cache                 *lru.Cache
	manageArgoAppsEnabled bool
	manageArgoAppsFilter  string
	ArgoProcessor         argo.ArgoAppProcessor
}

type VersionInfo struct {
	Version        uint64
	SourceCommitId string
	DeployedAt     time.Time
}

type Job struct {
	Name        string
	Content     string
	Application *v1alpha1.Application
}

func (v *VersionInfo) Equal(w *VersionInfo) bool {
	if v == nil {
		return w == nil
	}
	if w == nil {
		return false
	}
	return v.Version == w.Version
}

var ErrNotFound error = fmt.Errorf("not found")
var ZeroVersion VersionInfo

// GetVersion implements VersionClient
func (v *versionClient) GetVersion(ctx context.Context, revision, environment, application string) (*VersionInfo, error) {
	ctx = auth.WriteUserToGrpcContext(ctx, RolloutServiceUser)
	tr, err := v.tryGetVersion(ctx, revision, environment, application)
	if err == nil {
		return tr, nil
	}
	info, err := v.versionClient.GetVersion(ctx, &api.GetVersionRequest{
		GitRevision: revision,
		Environment: environment,
		Application: application,
	})
	if err != nil {
		return nil, err
	}
	return &VersionInfo{
		Version:        info.Version,
		SourceCommitId: info.SourceCommitId,
		DeployedAt:     info.DeployedAt.AsTime(),
	}, nil
}

// Tries getting the version from cache
func (v *versionClient) tryGetVersion(ctx context.Context, revision, environment, application string) (*VersionInfo, error) {
	var overview *api.GetOverviewResponse
	entry, ok := v.cache.Get(revision)
	if !ok {
		return nil, ErrNotFound
	}
	overview = entry.(*api.GetOverviewResponse)
	for _, group := range overview.GetEnvironmentGroups() {
		for _, env := range group.GetEnvironments() {
			if env.Name == environment {
				app := env.Applications[application]
				if app == nil {
					return &ZeroVersion, nil
				}
				return &VersionInfo{
					Version:        app.Version,
					SourceCommitId: sourceCommitId(overview, app),
					DeployedAt:     deployedAt(app),
				}, nil
			}
		}
	}
	return &ZeroVersion, nil
}

func deployedAt(app *api.Environment_Application) time.Time {
	if app.DeploymentMetaData == nil {
		return time.Time{}
	}
	deployTime := app.DeploymentMetaData.DeployTime
	if deployTime != "" {
		dt, err := strconv.ParseInt(deployTime, 10, 64)
		if err != nil {
			return time.Time{}
		}
		return time.Unix(dt, 0).UTC()
	}
	return time.Time{}
}

func team(overview *api.GetOverviewResponse, app string) string {
	a := overview.Applications[app]
	if a == nil {
		return ""
	}
	return a.Team
}

func sourceCommitId(overview *api.GetOverviewResponse, app *api.Environment_Application) string {
	a := overview.Applications[app.Name]
	if a == nil {
		return ""
	}
	for _, rel := range a.Releases {
		if rel.Version == app.Version {
			return rel.SourceCommitId
		}
	}
	return ""
}

type KuberpultEvent struct {
	Environment      string
	Application      string
	EnvironmentGroup string
	IsProduction     bool
	Team             string
	Version          *VersionInfo
}

type VersionEventProcessor interface {
	ProcessKuberpultEvent(ctx context.Context, ev KuberpultEvent)
}

type key struct {
	Environment string
	Application string
}

func (v *versionClient) ConsumeEvents(ctx context.Context, processor VersionEventProcessor, hr *setup.HealthReporter) error {
	ctx = auth.WriteUserToGrpcContext(ctx, RolloutServiceUser)
	versions := map[key]uint64{}
	environmentGroups := map[key]string{}
	teams := map[key]string{}
	return hr.Retry(ctx, func() error {
		client, err := v.overviewClient.StreamOverview(ctx, &api.GetOverviewRequest{})
		if err != nil {
			return fmt.Errorf("overview.connect: %w", err)
		}
		hr.ReportReady("consuming")
		for {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			overview, err := client.Recv()
			if err != nil {
				grpcErr := grpc.UnwrapGRPCStatus(err)
				if grpcErr != nil {
					if grpcErr.Code() == codes.Canceled {
						return nil
					}
				}
				return fmt.Errorf("overview.recv: %w", err)
			}
			l := logger.FromContext(ctx).With(zap.String("git.revision", overview.GitRevision))
			v.cache.Add(overview.GitRevision, overview)
			l.Info("overview.get")
			seen := make(map[key]uint64, len(versions))
			for _, envGroup := range overview.EnvironmentGroups {
				for _, env := range envGroup.Environments {
					for _, app := range env.Applications {
						dt := deployedAt(app)
						sc := sourceCommitId(overview, app)
						tm := team(overview, app.Name)

						l.Info("version.process", zap.String("application", app.Name), zap.String("environment", env.Name), zap.Uint64("version", app.Version), zap.Time("deployedAt", dt))
						k := key{env.Name, app.Name}
						seen[k] = app.Version
						environmentGroups[k] = envGroup.EnvironmentGroupName
						teams[k] = tm
						if versions[k] == app.Version {
							continue
						}

						if v.manageArgoAppsEnabled && len(v.manageArgoAppsFilter) > 0 && strings.Contains(v.manageArgoAppsFilter, app.Name) {
							manifestPath := path.Join("environments", env.Name, "applications", app.Name, "manifests", "manifests.yaml")
							l.Info(manifestPath)

							var annotations map[string]string
							var labels map[string]string

							for k, v := range env.Config.Argocd.ApplicationAnnotations {
								annotations[k] = v
							}
							annotations["com.freiheit.kuberpult/team"] = tm
							annotations["com.freiheit.kuberpult/application"] = app.Name
							annotations["com.freiheit.kuberpult/environment"] = env.Name
							annotations["com.freiheit.kuberpult/self-managed"] = "true"
							annotations["com.freiheit.kuberpult/self-managed-filter"] = v.manageArgoAppsFilter
							// This annotation is so that argoCd does not invalidate *everything* in the whole repo when receiving a git webhook.
							// It has to start with a "/" to be absolute to the git repo.
							// See https://argo-cd.readthedocs.io/en/stable/operator-manual/high_availability/#webhook-and-manifest-paths-annotation
							annotations["argocd.argoproj.io/manifest-generate-paths"] = "/" + manifestPath
							labels["com.freiheit.kuberpult/team"] = tm

							applicationNs := ""

							if env.Config.Argocd.Destination.Namespace != nil {
								applicationNs = *env.Config.Argocd.Destination.Namespace
							} else if env.Config.Argocd.Destination.ApplicationNamespace != nil {
								applicationNs = *env.Config.Argocd.Destination.ApplicationNamespace
							}

							applicationDestination := v1alpha1.ApplicationDestination{
								Name:      env.Config.Argocd.Destination.Name,
								Namespace: applicationNs,
								Server:    env.Config.Argocd.Destination.Server,
							}

							syncWindows := v1alpha1.SyncWindows{}

							ignoreDifferences := make([]v1alpha1.ResourceIgnoreDifferences, len(env.Config.Argocd.IgnoreDifferences))
							for index, value := range env.Config.Argocd.IgnoreDifferences {
								difference := v1alpha1.ResourceIgnoreDifferences{
									Group:                 value.Group,
									Kind:                  value.Kind,
									Name:                  value.Name,
									Namespace:             value.Namespace,
									JSONPointers:          value.JsonPointers,
									JqPathExpressions:     value.JqPathExpressions,
									ManagedFieldsManagers: value.ManagedFieldsManagers,
								}
								ignoreDifferences[index] = difference
							}

							for _, w := range env.Config.Argocd.SyncWindows {
								apps := []string{"*"}
								if len(w.Applications) > 0 {
									apps = w.Applications
								}
								syncWindows = append(syncWindows, &v1alpha1.SyncWindow{
									Applications: apps,
									Schedule:     w.Schedule,
									Duration:     w.Duration,
									Kind:         w.Kind,
									ManualSync:   true,
								})
							}

							deployApp := &v1alpha1.Application{
								ObjectMeta: v1alpha1.ObjectMeta{
									Name:        app.Name,
									Annotations: annotations,
									Labels:      labels,
									Finalizers:  calculateFinalizers(),
								},
								Spec: v1alpha1.ApplicationSpec{
									Project: env.Name,
									Source: v1alpha1.ApplicationSource{
										RepoURL:        overview.ManifestRepoUrl,
										Path:           manifestPath,
										TargetRevision: overview.Branch,
									},
									Destination: applicationDestination,
									SyncPolicy: &v1alpha1.SyncPolicy{
										Automated: &v1alpha1.SyncPolicyAutomated{
											Prune:    false,
											SelfHeal: false,
											// We always allow empty, because it makes it easier to delete apps/environments
											AllowEmpty: true,
										},
										SyncOptions: env.Config.Argocd.SyncOptions,
									},
									IgnoreDifferences: ignoreDifferences,
								},
							}

							argoApplication := convertApplication(deployApp)

							v.ArgoProcessor.Push(overview, argoApplication)

						} else {
							processor.ProcessKuberpultEvent(ctx, KuberpultEvent{
								Application:      app.Name,
								Environment:      env.Name,
								EnvironmentGroup: envGroup.EnvironmentGroupName,
								Team:             tm,
								IsProduction:     env.Priority == api.Priority_PROD,
								Version: &VersionInfo{
									Version:        app.Version,
									SourceCommitId: sc,
									DeployedAt:     dt,
								},
							})
						}
					}
				}
			}
			// Send events with version 0 for deleted applications so that we can react
			// to apps getting deleted.
			for k := range versions {
				if seen[k] == 0 {
					processor.ProcessKuberpultEvent(ctx, KuberpultEvent{
						Application:      k.Application,
						Environment:      k.Environment,
						EnvironmentGroup: environmentGroups[k],
						Team:             teams[k],
						Version:          &VersionInfo{},
					})
				}
			}
			versions = seen
		}
	})
}

func New(oclient api.OverviewServiceClient, vclient api.VersionServiceClient, appClient application.ApplicationServiceClient, config cmd.Config) VersionClient {
	result := &versionClient{
		cache:                 lru.New(20),
		overviewClient:        oclient,
		versionClient:         vclient,
		manageArgoAppsEnabled: config.ManageArgoApplicationEnabled,
		manageArgoAppsFilter:  config.ManageArgoApplicationFilter,
		ArgoProcessor: argo.ArgoAppProcessor{
			ApplicationClient: appClient,
			//TODO: [LS] Need to add the health reporter
		},
	}
	return result
}

func calculateFinalizers() []string {
	return []string{
		"resources-finalizer.argocd.argoproj.io",
	}
}

func convertApplication(app *v1alpha1.Application) *argoApp.Application {
	convertedApp := &argoApp.Application{
		TypeMeta: app.TypeMeta,
		Spec: argoApp.ApplicationSpec{
			Source: &argoApp.ApplicationSource{
				RepoURL:        app.Spec.Source.RepoURL,
				Path:           app.Spec.Source.Path,
				TargetRevision: app.Spec.Source.TargetRevision,
			},
			Destination: argoApp.ApplicationDestination{
				Server:    app.Spec.Destination.Server,
				Namespace: app.Spec.Destination.Namespace,
				Name:      app.Spec.Destination.Name,
			},
			Project: app.Spec.Project,
			SyncPolicy: &argoApp.SyncPolicy{
				Automated: &argoApp.SyncPolicyAutomated{
					AllowEmpty: app.Spec.SyncPolicy.Automated.AllowEmpty,
					Prune:      app.Spec.SyncPolicy.Automated.Prune,
					SelfHeal:   app.Spec.SyncPolicy.Automated.SelfHeal,
				},
				SyncOptions: (argoApp.SyncOptions)(app.Spec.SyncPolicy.SyncOptions),
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        app.ObjectMeta.Name,
			Annotations: app.ObjectMeta.Annotations,
			Labels:      app.ObjectMeta.Labels,
			Finalizers:  app.ObjectMeta.Finalizers,
		},
		Operation: &argoApp.Operation{},
	}

	return convertedApp
}
