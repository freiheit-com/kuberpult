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

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

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
	tr, err := v.tryGetVersion(environment, application)
	if err == nil {
		return tr, nil
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "GetVersionRequest")
	defer span.Finish()
	span.SetTag("GitRevision", revision)
	span.SetTag("Application", application)
	span.SetTag("Environment", environment)
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
func (v *versionClient) tryGetVersion(environment, application string) (*VersionInfo, error) {
	var appDetails *api.GetAppDetailsResponse
	entry, ok := v.cache.Get(application)
	if !ok {
		return nil, ErrNotFound
	}
	appDetails = entry.(*api.GetAppDetailsResponse)

	if deployment, exists := appDetails.Deployments[environment]; exists {
		deployedVersion := deployment.Version
		return &VersionInfo{
			Version:        deployedVersion,
			SourceCommitId: sourceCommitId(appDetails.Application.Releases, deployment),
			DeployedAt:     deployedAtFromApp(deployment),
		}, nil
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

func deployedAtFromApp(deployment *api.Deployment) time.Time {
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

func sourceCommitId(appReleases []*api.Release, deployment *api.Deployment) string {
	for _, rel := range appReleases {
		if rel.Version == deployment.Version {
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
	seenVersions := map[key]uint64{}
	environmentGroups := map[key]string{}
	teams := map[key]string{}
	return hr.Retry(ctx, func() error {
		client, err := v.overviewClient.StreamChangedApps(ctx, &api.GetChangedAppsRequest{})

		if err != nil {
			return fmt.Errorf("StreamChangedApps.connect: %w", err)
		}
		hr.ReportReady("consuming")
		appsToChange := map[string]*api.GetAppDetailsResponse{}
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

			ov, err := v.overviewClient.GetOverview(ctx, &api.GetOverviewRequest{
				GitRevision: "",
			})
			if err != nil {
				grpcErr := grpc.UnwrapGRPCStatus(err)
				if grpcErr != nil {
					if grpcErr.Code() == codes.Canceled {
						return nil
					}
				}
				return fmt.Errorf("overviewClient.GetOverview: %w", err)
			}
			l := logger.FromContext(ctx)

			l.Info("overview.get")

			overview := argo.ArgoOverview{
				Overview:   ov,
				AppDetails: nil,
			}

			fmt.Println("OUCH GOT HERE 0")
			for _, appDetailsResponse := range changedApps.ChangedApps {
				appName := appDetailsResponse.Application.Name
				appsToChange[appName] = appDetailsResponse
				v.cache.Add(appName, appDetailsResponse) // Update cache of app details
				fmt.Printf("OUCH dep number: %d\n", len(appDetailsResponse.Deployments))

				for key, _ := range seenVersions {
					if key.Application != appName {
						continue
					}

					var foundEnv *api.Environment = nil
					for _, envGroup := range overview.Overview.EnvironmentGroups {
						for _, env := range envGroup.Environments {
							if env.Name == key.Environment {
								foundEnv = env
								break
							}
						}
						if foundEnv != nil {
							break
						}
					}
					if foundEnv != nil {
						continue
					}

					delete(seenVersions, key)
					processor.ProcessKuberpultEvent(ctx, KuberpultEvent{
						IsProduction:     false,
						Application:      appName,
						Environment:      key.Environment,
						EnvironmentGroup: environmentGroups[key],
						Team:             teams[key],
						Version: &VersionInfo{
							Version:        0,
							SourceCommitId: "",
							DeployedAt:     time.Time{},
						},
					})
				}

				for _, envGroup := range overview.Overview.EnvironmentGroups {
					for _, env := range envGroup.Environments {
						argoAppKey := key{Environment: env.Name, Application: appName}
						fmt.Printf("OUCH %s, %s\n", env.Name, appName)
						seenVersion, ok := seenVersions[argoAppKey]
						deployment, deploymentExists := appDetailsResponse.Deployments[env.Name]

						fmt.Printf("OUCH tests: %t, %v\n", deploymentExists, deployment)

						if !deploymentExists || deployment == nil {
							fmt.Println("OUCH GOT HERE")
							// Check we knew a real version and deployment does not exist
							if ok && seenVersion > 0 {
								fmt.Println("OUCH DELETED")
								delete(seenVersions, argoAppKey)
								processor.ProcessKuberpultEvent(ctx, KuberpultEvent{
									IsProduction:     false,
									Application:      appName,
									Environment:      env.Name,
									EnvironmentGroup: envGroup.EnvironmentGroupName,
									Team:             appDetailsResponse.Application.Team,
									Version: &VersionInfo{
										Version:        0,
										SourceCommitId: "",
										DeployedAt:     time.Time{},
									},
								})
							}
							continue
						} else if deployment.Version == seenVersion {
							continue
						}

						seenVersions[argoAppKey] = deployment.Version
						dt := deployedAt(deployment)
						sc := sourceCommitId(appDetailsResponse.Application.Releases, deployment)
						tm := appDetailsResponse.Application.Team
						l.Info("version.process", zap.String("application", appName), zap.String("environment", env.Name), zap.Uint64("version", deployment.Version), zap.Time("deployedAt", dt))

						environmentGroups[argoAppKey] = envGroup.EnvironmentGroupName
						teams[argoAppKey] = tm
						processor.ProcessKuberpultEvent(ctx, KuberpultEvent{
							Application:      appName,
							Environment:      env.Name,
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
			}

			overview.AppDetails = appsToChange
			if err := v.ArgoProcessor.Push(ctx, &overview); err != nil {
				l.Sugar().Warnf("version.push failed: %v", err)
			} else {
				l.Info("version.push")
				appsToChange = make(map[string]*api.GetAppDetailsResponse)
			}
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
