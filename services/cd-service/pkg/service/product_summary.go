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

package service

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/mapper"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

type ProductSummaryServer struct {
	State *repository.State
}

func (s *ProductSummaryServer) GetProductSummary(ctx context.Context, in *api.GetProductSummaryRequest) (*api.GetProductSummaryResponse, error) {
	if in.Environment == nil && in.EnvironmentGroup == nil {
		return nil, fmt.Errorf("must have an environment or environmentGroup to get the product summary for")
	}
	if in.Environment != nil && in.EnvironmentGroup != nil {
		if *in.Environment != "" && *in.EnvironmentGroup != "" {
			return nil, fmt.Errorf("can not have both an environment and environmentGroup to get the product summary for")
		}
	}
	if in.ManifestRepoCommitHash == "" {
		return nil, fmt.Errorf("must have a commit to get the product summary for")
	}
	var summaryFromEnv []api.ProductSummary
	state := s.State
	dbHandler := state.DBHandler
	response, err := db.WithTransactionT[api.GetProductSummaryResponse](dbHandler, ctx, db.DefaultNumRetries, true, func(ctx context.Context, transaction *sql.Tx) (*api.GetProductSummaryResponse, error) {
		//Translate a manifest repo commit hash into a DB transaction timestamp.
		ts, err := dbHandler.DBReadCommitHashTransactionTimestamp(ctx, transaction, in.ManifestRepoCommitHash)
		if err != nil {
			return nil, fmt.Errorf("unable to get manifest repo timestamp that corresponds to provided commit Hash %v", err)
		}

		if ts == nil {
			return nil, fmt.Errorf("could not find timestamp that corresponds to the given commit hash '%s'", in.ManifestRepoCommitHash)
		}
		if in.Environment != nil && *in.Environment != "" {
			//Single environment
			allAppsForEnv, err := state.GetEnvironmentApplicationsAtTimestamp(ctx, transaction, types.EnvName(*in.Environment), *ts)
			if err != nil {
				return nil, fmt.Errorf("unable to get applications for environment '%s': %v", *in.Environment, err)
			}
			if len(allAppsForEnv) == 0 {
				return &api.GetProductSummaryResponse{
					ProductSummary: nil,
				}, nil
			}
			for _, currentApp := range allAppsForEnv {
				currentAppDeployments, err := state.GetAllDeploymentsForAppFromDBAtTimestamp(ctx, transaction, currentApp, *ts)
				if err != nil {
					return nil, fmt.Errorf("unable to get GetAllDeploymentsForAppAtTimestamp  %v", err)
				}

				if version, ok := currentAppDeployments[types.EnvName(*in.Environment)]; ok {
					summaryFromEnv = append(summaryFromEnv, api.ProductSummary{
						CommitId:       "",
						DisplayVersion: "",
						Team:           "",
						App:            string(currentApp),
						Version:        strconv.FormatInt(int64(*version.Version), 10),
						Revision:       strconv.FormatInt(int64(version.Revision), 10),
						Environment:    *in.Environment,
					})

				}
			}
			if len(summaryFromEnv) == 0 {
				return &api.GetProductSummaryResponse{
					ProductSummary: nil,
				}, nil
			}
			sort.Slice(summaryFromEnv, func(i, j int) bool {
				a := summaryFromEnv[i].App
				b := summaryFromEnv[j].App
				return a < b
			})
		} else {
			//Environment Group
			var environmentGroups []*api.EnvironmentGroup

			envs, err := state.GetAllEnvironmentConfigs(ctx, transaction)
			if err != nil {
				return nil, fmt.Errorf("could not get environment configs %v", err)
			} else {
				environmentGroups = mapper.MapEnvironmentsToGroups(envs)
			}
			for _, envGroup := range environmentGroups {
				if *in.EnvironmentGroup == envGroup.EnvironmentGroupName {
					for _, env := range envGroup.Environments {
						envName := types.EnvName(env.Name)
						allAppsForEnv, err := state.GetEnvironmentApplicationsAtTimestamp(ctx, transaction, envName, *ts)
						if err != nil {
							return nil, fmt.Errorf("unable to get all applications for environment '%s': %v", envName, err)
						}
						if len(allAppsForEnv) == 0 {
							return &api.GetProductSummaryResponse{
								ProductSummary: nil,
							}, nil
						}
						for _, currentApp := range allAppsForEnv {

							currentAppDeployments, err := state.GetAllDeploymentsForAppFromDBAtTimestamp(ctx, transaction, currentApp, *ts)
							if err != nil {
								return nil, fmt.Errorf("unable to get GetAllDeploymentsForAppAtTimestamp  %v", err)
							}
							if version, ok := currentAppDeployments[envName]; ok {
								summaryFromEnv = append(summaryFromEnv, api.ProductSummary{
									CommitId:       "",
									DisplayVersion: "",
									Team:           "",
									App:            string(currentApp),
									Version:        strconv.FormatInt(int64(*version.Version), 10),
									Revision:       strconv.FormatInt(int64(version.Revision), 10),
									Environment:    string(envName),
								})

							}
						}
					}
				}
			}
			if len(summaryFromEnv) == 0 {
				return &api.GetProductSummaryResponse{
					ProductSummary: nil,
				}, nil
			}
			sort.Slice(summaryFromEnv, func(i, j int) bool {
				a := summaryFromEnv[i].App
				b := summaryFromEnv[j].App
				return a < b
			})
		}

		var productVersion []*api.ProductSummary
		for _, row := range summaryFromEnv { //nolint: govet
			v, err := strconv.ParseUint(row.Version, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("could not parse version to integer %s: %v", row.Version, err)
			}
			r, err := strconv.ParseUint(row.Revision, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("could not parse version to integer %s: %v", row.Revision, err)
			}
			release, err := dbHandler.DBSelectReleaseByVersionAtTimestamp(ctx, transaction, types.AppName(row.App), types.MakeReleaseNumbers(v, r), false, *ts)
			if err != nil {
				return nil, fmt.Errorf("error getting release for version")
			}
			team, err := state.GetApplicationTeamOwnerAtTimestamp(ctx, transaction, types.AppName(row.App), *ts)
			if err != nil {
				return nil, fmt.Errorf("could not find app %s: %v", row.App, err)
			}
			productVersion = append(productVersion, &api.ProductSummary{App: row.App, Version: row.Version, Revision: row.Revision, CommitId: release.Metadata.SourceCommitId, DisplayVersion: release.Metadata.DisplayVersion, Environment: row.Environment, Team: team})
		}
		return &api.GetProductSummaryResponse{ProductSummary: productVersion}, nil
	})
	return response, err
}
