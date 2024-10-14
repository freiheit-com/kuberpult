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

package versions

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/argo"

	"github.com/argoproj/argo-cd/v2/util/grpc"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
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
	DexAuthContext: nil,
	Email:          "kuberpult-rollout-service@local",
	Name:           "kuberpult-rollout-service",
}

type VersionClient interface {
	GetVersion(ctx context.Context, revision, environment, application string) (*VersionInfo, error)
	ConsumeEvents(ctx context.Context, processor VersionEventProcessor, hr *setup.HealthReporter) error
	GetArgoProcessor() *argo.ArgoAppProcessor
}

type versionClient struct {
	overviewClient api.OverviewServiceClient
	versionClient  api.VersionServiceClient
	cache          *lru.Cache
	ArgoProcessor  argo.ArgoAppProcessor
}

type VersionInfo struct {
	Version        uint64
	SourceCommitId string
	DeployedAt     time.Time
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
					SourceCommitId: sourceCommitIdFromOverview(overview, app),
					DeployedAt:     deployedAtFromApp(app),
				}, nil
			}
		}
	}
	return &ZeroVersion, nil
}

func deployedAt(deployment *api.Deployment) time.Time {
	if deployment.DeploymentMetaData == nil {
		return time.Time{}
	}
	deployTime := deployment.DeploymentMetaData.DeployTime
	if deployTime != "" {
		dt, err := strconv.ParseInt(deployTime, 10, 64)
		if err != nil {
			return time.Time{}
		}
		return time.Unix(dt, 0).UTC()
	}
	return time.Time{}
}

func deployedAtFromApp(app *api.Environment_Application) time.Time {
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

func sourceCommitId(appReleases []*api.Release, deployment *api.Deployment) string {
	for _, rel := range appReleases {
		if rel.Version == deployment.Version {
			return rel.SourceCommitId
		}
	}
	return ""
}
func sourceCommitIdFromOverview(overview *api.GetOverviewResponse, app *api.Environment_Application) string {
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
		client, err := v.overviewClient.StreamChangedApps(ctx, &api.GetChangedAppsRequest{})

		if err != nil {
			return fmt.Errorf("StreamChangedApps.connect: %w", err)
		}
		hr.ReportReady("consuming")
		for {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			changedApps, err := client.Recv()
			if err != nil {
				grpcErr := grpc.UnwrapGRPCStatus(err)
				if grpcErr != nil {
					if grpcErr.Code() == codes.Canceled {
						return nil
					}
				}
				return fmt.Errorf("changedApps.recv: %w", err)
			}
			fmt.Println(">>>>>>>>>>>>>>")
			fmt.Println(changedApps)
			ov, err := v.overviewClient.GetOverview(ctx, &api.GetOverviewRequest{
				GitRevision: "", //TODO: Overview will get smaller in the future, for now there is redundant data between appdetails and overview
			})
			fmt.Println(ov)
			l := logger.FromContext(ctx)
			v.cache.Add(ov.GitRevision, ov)
			l.Info("overview.get")
			seen := make(map[key]uint64, len(versions))

			overview := argo.ArgoOverview{
				Overview:   ov,
				AppDetails: make(map[string]*api.GetAppDetailsResponse),
			}
			for _, appName := range changedApps.ChangedApps {
				appDetailsResponse, err := v.overviewClient.GetAppDetails(ctx, &api.GetAppDetailsRequest{
					AppName: appName,
				})
				fmt.Println(appDetailsResponse)
				if err != nil {
					grpcErr := grpc.UnwrapGRPCStatus(err)
					if grpcErr != nil {
						if grpcErr.Code() == codes.Canceled {
							return nil
						}
					}
					return fmt.Errorf("changedApps.recv: %w", err)
				}

				overview.AppDetails[appName] = appDetailsResponse

				app := appDetailsResponse.Application
				for env, deployment := range appDetailsResponse.Deployments {
					fmt.Println("deployment")
					dt := deployedAt(deployment)
					sc := sourceCommitId(appDetailsResponse.Application.Releases, deployment)
					tm := appDetailsResponse.Application.Team

					foundEnv := false
					var envGroup *api.EnvironmentGroup
					for _, currEnvGroup := range overview.Overview.EnvironmentGroups {
						for _, currEnv := range currEnvGroup.Environments {
							fmt.Println(currEnv.Name)
							if currEnv.Name == env {
								foundEnv = true
								envGroup = currEnvGroup
							}
						}
					}

					if !foundEnv {
						fmt.Println("!exists")
						return fmt.Errorf("getAppDetails returned information regarding a deployment for app %s on env %s, but did not provide any environment group information about this environment", appName, env)
					}

					l.Info("version.process", zap.String("application", app.Name), zap.String("environment", env), zap.Uint64("version", deployment.Version), zap.Time("deployedAt", dt))
					k := key{env, appName}
					seen[k] = deployment.Version
					environmentGroups[k] = envGroup.EnvironmentGroupName
					teams[k] = tm
					fmt.Println(versions[k])
					fmt.Println(deployment.Version)
					if versions[k] == deployment.Version {
						fmt.Println("Already seen, skipping event")
						continue
					}
					fmt.Println("New event")
					processor.ProcessKuberpultEvent(ctx, KuberpultEvent{
						Application:      appName,
						Environment:      env,
						EnvironmentGroup: envGroup.EnvironmentGroupName,
						Team:             tm,
						IsProduction:     (envGroup.Priority == api.Priority_PROD || envGroup.Priority == api.Priority_CANARY),
						Version: &VersionInfo{
							Version:        deployment.Version,
							SourceCommitId: sc,
							DeployedAt:     dt,
						},
					})
				}

			}

			l.Info("version.push")
			v.ArgoProcessor.Push(ctx, &overview)
			// Send events with version 0 for deleted applications so that we can react
			// to apps getting deleted.
			for k := range versions {
				if seen[k] == 0 {
					processor.ProcessKuberpultEvent(ctx, KuberpultEvent{
						IsProduction:     false,
						Application:      k.Application,
						Environment:      k.Environment,
						EnvironmentGroup: environmentGroups[k],
						Team:             teams[k],
						Version: &VersionInfo{
							Version:        0,
							SourceCommitId: "",
							DeployedAt:     time.Time{},
						},
					})
				}
			}
			versions = seen
		}
	})
}

func New(oclient api.OverviewServiceClient, vclient api.VersionServiceClient, appClient application.ApplicationServiceClient, manageArgoApplicationEnabled bool, manageArgoApplicationFilter []string) VersionClient {
	result := &versionClient{
		cache:          lru.New(20),
		overviewClient: oclient,
		versionClient:  vclient,
		ArgoProcessor:  argo.New(appClient, manageArgoApplicationEnabled, manageArgoApplicationFilter),
	}
	return result
}

func (v *versionClient) GetArgoProcessor() *argo.ArgoAppProcessor {
	return &v.ArgoProcessor
}
