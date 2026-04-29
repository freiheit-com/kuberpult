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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/util/grpc"
	"github.com/freiheit-com/kuberpult/pkg/logging"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"k8s.io/utils/lru"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/argo"
)

// This is a the user that the rollout service uses to query the versions.
// It is not written to the repository.
var RolloutServiceUser auth.User = auth.User{
	DexAuthContext: nil,
	Email:          "kuberpult-rollout-service@local",
	Name:           "kuberpult-rollout-service",
}

type VersionClient interface {
	GetVersion(ctx context.Context, revision, environment, app string) (*VersionInfo, error)
	ConsumeEvents(ctx context.Context, processor VersionEventProcessor, hr *setup.HealthReporter) error
	GetArgoProcessor() *argo.ArgoAppProcessor
}

type versionClient struct {
	overviewClient api.OverviewServiceClient
	versionClient  api.VersionServiceClient
	cache          *lru.Cache
	ArgoProcessor  argo.ArgoAppProcessor
	db             db.DBHandler

	experimentalBracketsClusters []string
}

type VersionInfo struct {
	Version        types.RolloutAppBracketVersion
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
func (v *versionClient) GetVersion(ctx context.Context, revision, environment, app string) (*VersionInfo, error) {
	// use db access see cd-service/pkg/services/version
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "GetVersion")
	defer span.Finish()
	span.SetTag("GitRevision", revision)
	span.SetTag("Environment", environment)
	span.SetTag("Application", app)
	logging.Warn(ctx, "getversion called", zap.String("env", environment), zap.String("app", app))

	if slices.Contains(v.experimentalBracketsClusters, environment) {
		result, err := v.getBracketVersion(ctx, revision, environment, types.ArgoBracketName(app))
		if err != nil {
			return nil, onErr(err)
		}
		return result, nil
	}

	releaseVersion, err := strconv.ParseUint(revision, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("could not parse GitRevision '%s' for app '%s' in env '%s': %w",
			revision, app, environment, err)
	}
	return db.WithTransactionT[VersionInfo](&v.db, ctx, 1, true, func(ctx context.Context, tx *sql.Tx) (*VersionInfo, error) {
		deployment, err := v.db.DBSelectSpecificDeploymentHistory(ctx, tx, types.AppName(app), environment, releaseVersion)
		if err != nil || deployment == nil {
			return nil, onErr(fmt.Errorf("no deployment found for env='%s' and app='%s': %w", environment, app, err))
		}
		release, err := v.db.DBSelectReleaseByVersion(ctx, tx, types.AppName(app), types.ReleaseNumbers{Version: &releaseVersion, Revision: 0}, true)
		if err != nil {
			return nil, onErr(fmt.Errorf("could not get release of app %s: %v", app, err))
		}
		if release == nil {
			return nil, onErr(fmt.Errorf("no release found for env='%s' and app='%s'", environment, app))
		}
		return &VersionInfo{
			Version:        types.RolloutAppBracketVersionFromUint64(releaseVersion),
			DeployedAt:     deployment.Created,
			SourceCommitId: release.Metadata.SourceCommitId,
		}, nil
	})
}

func (v *versionClient) getBracketVersion(ctx context.Context, revision, environment string, bracketName types.ArgoBracketName) (*VersionInfo, error) {
	return db.WithTransactionT[VersionInfo](&v.db, ctx, 1, true, func(ctx context.Context, tx *sql.Tx) (*VersionInfo, error) {
		bracketRow, err := db.DBSelectBracketHistoryLatest(ctx, &v.db, tx)
		if err != nil {
			return nil, fmt.Errorf("getBracketVersion: could not get bracket history for bracket '%s': %w", bracketName, err)
		}
		if bracketRow == nil {
			return nil, fmt.Errorf("getBracketVersion: no bracket history found for bracket '%s'", bracketName)
		}
		appNames := bracketRow.AllBracketsJsonBlob.BracketMap[bracketName]
		sortedAppNames := make(db.AppNames, len(appNames))
		copy(sortedAppNames, appNames)
		slices.SortFunc(sortedAppNames, func(a, b types.AppName) int { return strings.Compare(string(a), string(b)) })

		versionStrs := strings.Split(revision, ":")

		var latestTime time.Time
		var latestSourceCommitId string

		for i, appName := range sortedAppNames {
			if i >= len(versionStrs) {
				break
			}
			releaseVersion, err := strconv.ParseUint(versionStrs[i], 10, 64)
			if err != nil || releaseVersion == 0 {
				continue
			}
			deployment, err := v.db.DBSelectSpecificDeploymentHistory(ctx, tx, appName, environment, releaseVersion)
			if err != nil || deployment == nil {
				continue
			}
			if deployment.Created.After(latestTime) {
				latestTime = deployment.Created
				release, err := v.db.DBSelectReleaseByVersion(ctx, tx, appName, types.ReleaseNumbers{Version: &releaseVersion, Revision: 0}, true)
				if err == nil && release != nil {
					latestSourceCommitId = release.Metadata.SourceCommitId
				}
			}
		}

		return &VersionInfo{
			Version:        types.RolloutAppBracketVersion(revision),
			DeployedAt:     latestTime,
			SourceCommitId: latestSourceCommitId,
		}, nil
	})
}

func deployedAt(deployment *api.Deployment) time.Time {
	if deployment.DeploymentMetaData == nil {
		return time.Time{}
	}
	deployTime := deployment.DeploymentMetaData.DeployTime
	if deployTime != nil {
		return deployTime.AsTime()
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
	Environment       string
	ParentEnvironment string
	Application       string
	EnvironmentGroup  string
	IsProduction      bool
	Team              string
	Version           *VersionInfo
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
	seenVersions := map[key]types.RolloutAppBracketVersion{}
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

			l.Info("overview.get",
				zap.Int("changedApps", len(changedApps.ChangedApps)),
				zap.Int("brackets", len(changedApps.ChangedBrackets)),
			)

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
						if hasVersion && types.RolloutAppBracketVersionFromUint64(deployment.Version) == seenVersion {
							continue
						}

						seenVersions[argoAppKey] = types.RolloutAppBracketVersionFromUint64(deployment.Version)
						environmentGroups[argoAppKey] = envGroup.EnvironmentGroupName
						teams[argoAppKey] = appDetailsResponse.Application.Team

						dt := deployedAt(deployment)
						sc := sourceCommitId(appDetailsResponse.Application.Releases, deployment)
						l.Info("version.process", zap.String("application", appName), zap.String("environment", env.Name), zap.Uint64("version", deployment.Version), zap.Time("deployedAt", dt), zap.String("commitid", sc))

						clusters := childEnvironments(env)
						for _, cluster := range clusters {
							processor.ProcessKuberpultEvent(ctx, KuberpultEvent{
								Application:       appName,
								Environment:       cluster,
								ParentEnvironment: env.Name,
								EnvironmentGroup:  envGroup.EnvironmentGroupName,
								Team:              appDetailsResponse.Application.Team,
								IsProduction:      (envGroup.Priority == api.Priority_PROD || envGroup.Priority == api.Priority_CANARY),
								Version: &VersionInfo{
									Version:        types.RolloutAppBracketVersionFromUint64(deployment.Version),
									SourceCommitId: sc,
									DeployedAt:     dt,
								},
							})
						}
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
							Version:        types.RolloutAppBracketVersion(""),
							SourceCommitId: "",
							DeployedAt:     time.Time{},
						},
					})
					delete(seenVersions, deletedArgoAppKey)
					delete(environmentGroups, deletedArgoAppKey)
					delete(teams, deletedArgoAppKey)
				}
			}

			var bracketHistoryRow *db.BracketRow
			if len(changedApps.ChangedBrackets) > 0 {
				bracketHistoryRow, err = db.WithTransactionT[db.BracketRow](&v.db, ctx, 1, true,
					func(ctx context.Context, tx *sql.Tx) (*db.BracketRow, error) {
						return db.DBSelectBracketHistoryLatest(ctx, &v.db, tx)
					})
				if err != nil {
					return fmt.Errorf("consumeEvents could not get bracket history: %w", err)
				}
			}

			for _, bracketDetails := range changedApps.ChangedBrackets {
				bracketName := bracketDetails.BracketName
				for envName, bracketDeployment := range bracketDetails.Deployments {
					if !slices.Contains(v.experimentalBracketsClusters, envName) {
						logging.Warn(ctx, "env not in bracketclusters", zap.String("env", envName), zap.Strings("bracketClusters", v.experimentalBracketsClusters))
						continue
					}
					bracketKey := key{Environment: envName, Application: bracketName}
					seenVersion, hasVersion := seenVersions[bracketKey]
					bracketVersion := types.RolloutAppBracketVersion(bracketDeployment.Version)
					if hasVersion && bracketVersion == seenVersion {
						continue
					}
					seenVersions[bracketKey] = bracketVersion

					var dt time.Time
					if bracketDeployment.DeployedAt != nil {
						dt = bracketDeployment.DeployedAt.AsTime()
					}

					isProduction := false
					envGroup := ""
					for _, eg := range ov.EnvironmentGroups {
						for _, env := range eg.Environments {
							if env.Name == envName {
								isProduction = eg.Priority == api.Priority_PROD || eg.Priority == api.Priority_CANARY
								envGroup = eg.EnvironmentGroupName
							}
						}
					}

					l.Info("version.process.bracket", zap.String("bracket", bracketName), zap.String("environment", envName), zap.String("version", bracketDeployment.Version))
					processor.ProcessKuberpultEvent(ctx, KuberpultEvent{
						Application:       bracketName,
						Environment:       envName,
						ParentEnvironment: envName,
						EnvironmentGroup:  envGroup,
						IsProduction:      isProduction,
						Team:              "",
						Version: &VersionInfo{
							Version:        bracketVersion,
							SourceCommitId: bracketDeployment.SourceCommitId,
							DeployedAt:     dt,
						},
					})
					if bracketHistoryRow != nil {
						v.addBracketAppsToChange(appsToChange, bracketHistoryRow, types.ArgoBracketName(bracketName), envName, bracketDeployment.Version)
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

func New(oclient api.OverviewServiceClient, vclient api.VersionServiceClient, appClient application.ApplicationServiceClient, manageArgoApplicationEnabled, kuberpultMetricsEnabled, argoAppsMetricsEnabled bool, manageArgoApplicationFilter []string, dbHandler db.DBHandler, triggerChannelSize, argoAppsChannelSize int, ddMetrics statsd.ClientInterface, experimentalBracketsClusters []string) VersionClient {
	result := &versionClient{
		cache:          lru.New(20),
		overviewClient: oclient,
		versionClient:  vclient,
		ArgoProcessor:  argo.New(appClient, manageArgoApplicationEnabled, kuberpultMetricsEnabled, argoAppsMetricsEnabled, manageArgoApplicationFilter, triggerChannelSize, argoAppsChannelSize, ddMetrics),
		db:             dbHandler,

		experimentalBracketsClusters: experimentalBracketsClusters,
	}
	return result
}

func (v *versionClient) GetArgoProcessor() *argo.ArgoAppProcessor {
	return &v.ArgoProcessor
}

func (v *versionClient) addBracketAppsToChange(
	appsToChange map[string]*api.GetAppDetailsResponse,
	bracketRow *db.BracketRow,
	bracketName types.ArgoBracketName,
	envName string,
	versionStr string,
) {
	appNames := bracketRow.AllBracketsJsonBlob.BracketMap[bracketName]
	sortedAppNames := make(db.AppNames, len(appNames))
	copy(sortedAppNames, appNames)
	slices.SortFunc(sortedAppNames, func(a, b types.AppName) int {
		return strings.Compare(string(a), string(b))
	})
	versionStrs := strings.Split(versionStr, ":")
	for i, appName := range sortedAppNames {
		if i >= len(versionStrs) {
			break
		}
		releaseVersion, err := strconv.ParseUint(versionStrs[i], 10, 64)
		if err != nil || releaseVersion == 0 {
			continue
		}
		v.addAppDeploymentToChange(appsToChange, string(appName), envName, releaseVersion)
	}
}

func (v *versionClient) addAppDeploymentToChange(
	appsToChange map[string]*api.GetAppDetailsResponse,
	appName string,
	envName string,
	version uint64,
) {
	//exhaustruct:ignore
	newDep := &api.Deployment{Version: version}

	mergeDeployments := func(base map[string]*api.Deployment) map[string]*api.Deployment {
		result := make(map[string]*api.Deployment, len(base)+1)
		for k, dep := range base {
			result[k] = dep
		}
		result[envName] = newDep
		return result
	}

	if existing, ok := appsToChange[appName]; ok {
		//exhaustruct:ignore
		appsToChange[appName] = &api.GetAppDetailsResponse{
			Application: existing.Application,
			Deployments: mergeDeployments(existing.Deployments),
		}
	} else if cached, ok := v.cache.Get(appName); ok {
		original := cached.(*api.GetAppDetailsResponse)
		//exhaustruct:ignore
		appsToChange[appName] = &api.GetAppDetailsResponse{
			Application: original.Application,
			Deployments: mergeDeployments(original.Deployments),
		}
	} else {
		//exhaustruct:ignore
		appsToChange[appName] = &api.GetAppDetailsResponse{
			//exhaustruct:ignore
			Application: &api.Application{Name: appName},
			Deployments: map[string]*api.Deployment{envName: newDep},
		}
	}
}

func childEnvironments(env *api.Environment) []string {
	if env.Config == nil || env.Config.ArgoConfigs == nil || env.Config.ArgoConfigs.CommonEnvPrefix == "" {
		return []string{env.Name}
	}
	result := make([]string, 0, len(env.Config.ArgoConfigs.Configs))
	for _, config := range env.Config.ArgoConfigs.Configs {
		result = append(result, fmt.Sprintf("%s-%s-%s", env.Config.ArgoConfigs.CommonEnvPrefix, env.Name, config.ConcreteEnvName))
	}
	return result
}
