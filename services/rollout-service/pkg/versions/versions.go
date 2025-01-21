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
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/argo"

	"github.com/argoproj/argo-cd/v2/util/grpc"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/db"
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
	db             db.DBHandler
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
	// use db access see cd-service/pkg/services/version
	span, ctx := tracer.StartSpanFromContext(ctx, "GetVersion")
	defer span.Finish()
	span.SetTag("GitRevision", revision)
	span.SetTag("Environment", environment)
	span.SetTag("Application", application)

	releaseVersion, err := strconv.ParseUint(revision, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("could not parse GitRevision '%s' for app '%s' in env '%s': %w",
			revision, application, environment, err)
	}
	return db.WithTransactionT[VersionInfo](&v.db, ctx, 1, true, func(ctx context.Context, tx *sql.Tx) (*VersionInfo, error) {
		deployment, err := v.db.DBSelectSpecificDeployment(ctx, tx, environment, application, releaseVersion)
		if err != nil || deployment == nil {
			return nil, fmt.Errorf("no deployment found for env='%s' and app='%s': %w", environment, application, err)
		}
		release, err := v.db.DBSelectReleaseByVersion(ctx, tx, application, releaseVersion, true)
		if err != nil {
			return nil, fmt.Errorf("could not get release of app %s: %v", application, err)
		}
		if release == nil {
			return nil, fmt.Errorf("no release found for env='%s' and app='%s'", environment, application)
		}
		return &VersionInfo{
			Version:        releaseVersion,
			DeployedAt:     deployment.Created,
			SourceCommitId: release.Metadata.SourceCommitId,
		}, nil
	})
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

			for _, appDetailsResponse := range changedApps.ChangedApps {
				appName := appDetailsResponse.Application.Name
				appsToChange[appName] = appDetailsResponse
				v.cache.Add(appName, appDetailsResponse) // Update cache of app details

				appSeenVersions := make(map[string]struct{})
				for key := range seenVersions {
					if key.Application != appName {
						continue
					}

					appSeenVersions[key.Environment] = struct{}{}
				}

				for _, envGroup := range overview.Overview.EnvironmentGroups {
					for _, env := range envGroup.Environments {
						argoAppKey := key{Environment: env.Name, Application: appName}
						seenVersion, hasVersion := seenVersions[argoAppKey]
						deployment, deploymentExists := appDetailsResponse.Deployments[env.Name]

						if !deploymentExists || deployment == nil {
							continue
						}

						// Deployment exists, do not delete it
						delete(appSeenVersions, env.Name)
						if hasVersion && deployment.Version == seenVersion {
							continue
						}

						seenVersions[argoAppKey] = deployment.Version
						environmentGroups[argoAppKey] = envGroup.EnvironmentGroupName
						teams[argoAppKey] = appDetailsResponse.Application.Team

						dt := deployedAt(deployment)
						sc := sourceCommitId(appDetailsResponse.Application.Releases, deployment)
						l.Info("version.process", zap.String("application", appName), zap.String("environment", env.Name), zap.Uint64("version", deployment.Version), zap.Time("deployedAt", dt))

						processor.ProcessKuberpultEvent(ctx, KuberpultEvent{
							Application:      appName,
							Environment:      env.Name,
							EnvironmentGroup: envGroup.EnvironmentGroupName,
							Team:             appDetailsResponse.Application.Team,
							IsProduction:     (envGroup.Priority == api.Priority_PROD || envGroup.Priority == api.Priority_CANARY),
							Version: &VersionInfo{
								Version:        deployment.Version,
								SourceCommitId: sc,
								DeployedAt:     dt,
							},
						})
					}
				}
				// Delete all environments that we track but we did not see
				for missingEnvironment := range appSeenVersions {
					deletedArgoAppKey := key{Environment: missingEnvironment, Application: appName}

					processor.ProcessKuberpultEvent(ctx, KuberpultEvent{
						IsProduction:     false,
						Application:      appName,
						Environment:      missingEnvironment,
						EnvironmentGroup: environmentGroups[deletedArgoAppKey],
						Team:             teams[deletedArgoAppKey],
						Version: &VersionInfo{
							Version:        0,
							SourceCommitId: "",
							DeployedAt:     time.Time{},
						},
					})
					delete(seenVersions, deletedArgoAppKey)
					delete(environmentGroups, deletedArgoAppKey)
					delete(teams, deletedArgoAppKey)
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

func New(oclient api.OverviewServiceClient, vclient api.VersionServiceClient, appClient application.ApplicationServiceClient, manageArgoApplicationEnabled bool, manageArgoApplicationFilter []string, db db.DBHandler) VersionClient {
	result := &versionClient{
		cache:          lru.New(20),
		overviewClient: oclient,
		versionClient:  vclient,
		ArgoProcessor:  argo.New(appClient, manageArgoApplicationEnabled, manageArgoApplicationFilter),
		db:             db,
	}
	return result
}

func (v *versionClient) GetArgoProcessor() *argo.ArgoAppProcessor {
	return &v.ArgoProcessor
}
