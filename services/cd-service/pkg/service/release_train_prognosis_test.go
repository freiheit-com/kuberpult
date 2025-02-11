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

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/google/go-cmp/cmp"

	"testing"

	"github.com/freiheit-com/kuberpult/pkg/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/conversion"
	rp "github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

func TestReleaseTrainPrognosis(t *testing.T) {
	environmentSetup := []rp.Transformer{
		&rp.CreateEnvironment{
			Environment: "development-1",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Environment: "",
					Latest:      true,
				},
				EnvironmentGroup: conversion.FromString("development"),
			},
		},
		&rp.CreateEnvironment{
			Environment: "development-2",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Environment: "",
					Latest:      true,
				},
				EnvironmentGroup: conversion.FromString("development"),
			},
		},
		&rp.CreateEnvironment{
			Environment: "development-3",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Environment: "",
					Latest:      true,
				},
				EnvironmentGroup: conversion.FromString("development"),
			},
		},
		&rp.CreateEnvironment{
			Environment: "staging-1",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Environment: "development-1",
					Latest:      false,
				},
				EnvironmentGroup: conversion.FromString("staging"),
			},
		},
		&rp.CreateEnvironment{
			Environment: "staging-2",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Environment: "development-1", // CAREFUL, downstream from development-1, not development-2
					Latest:      false,
				},
				EnvironmentGroup: conversion.FromString("staging"),
			},
		},
		&rp.CreateEnvironment{
			Environment: "staging-3",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Environment: "development-3",
					Latest:      false,
				},
				EnvironmentGroup: conversion.FromString("staging"),
			},
		},
	}
	type TestCase struct {
		Name             string
		Setup            []rp.Transformer
		Request          *api.ReleaseTrainRequest
		ExpectedResponse *api.GetReleaseTrainPrognosisResponse
		ExpectedError    codes.Code
	}

	tcs := []TestCase{
		{
			Name:  "error with release train missing environment",
			Setup: []rp.Transformer{},
			Request: &api.ReleaseTrainRequest{
				Target: "non-existent environment",
			},
			ExpectedResponse: nil,
			ExpectedError:    codes.InvalidArgument,
		},
		{
			Name: "some environment is skipped",
			Setup: []rp.Transformer{
				&rp.CreateEnvironmentLock{
					Environment: "staging-1",
					LockId:      "staging-1-lock",
				},
			},
			Request: &api.ReleaseTrainRequest{
				Target: "staging",
			},
			ExpectedResponse: &api.GetReleaseTrainPrognosisResponse{
				EnvsPrognoses: map[string]*api.ReleaseTrainEnvPrognosis{
					"staging-1": &api.ReleaseTrainEnvPrognosis{
						Outcome: &api.ReleaseTrainEnvPrognosis_SkipCause{
							SkipCause: api.ReleaseTrainEnvSkipCause_ENV_IS_LOCKED,
						},
						Locks: []*api.Lock{
							{
								LockId:    "staging-1-lock",
								CreatedAt: timestamppb.Now(),
								CreatedBy: &api.Actor{
									Email: "testmail@example.com",
									Name:  "test tester",
								},
								Message: "",
							},
						},
					},
					"staging-2": &api.ReleaseTrainEnvPrognosis{
						Outcome: &api.ReleaseTrainEnvPrognosis_AppsPrognoses{
							AppsPrognoses: &api.ReleaseTrainEnvPrognosis_AppsPrognosesWrapper{
								Prognoses: map[string]*api.ReleaseTrainAppPrognosis{},
							},
						},
					},
					"staging-3": &api.ReleaseTrainEnvPrognosis{
						Outcome: &api.ReleaseTrainEnvPrognosis_AppsPrognoses{
							AppsPrognoses: &api.ReleaseTrainEnvPrognosis_AppsPrognosesWrapper{
								Prognoses: map[string]*api.ReleaseTrainAppPrognosis{},
							},
						},
					},
				},
			},
			ExpectedError: codes.OK,
		},
		{
			Name: "some application is skipped",
			Setup: []rp.Transformer{
				&rp.CreateApplicationVersion{
					Application: "potato-app",
					Manifests: map[string]string{
						"development-1": "",
						"staging-1":     "",
					},
					Version: 1,
				},
				&rp.CreateApplicationVersion{
					Application: "potato-app",
					Manifests: map[string]string{
						"development-1": "",
						"staging-1":     "",
					},
					Version: 2,
				},
				&rp.DeployApplicationVersion{
					Environment: "development-1",
					Application: "potato-app",
					Version:     2,
				},
				&rp.DeployApplicationVersion{
					Environment: "staging-1",
					Application: "potato-app",
					Version:     1,
				},
				&rp.CreateEnvironmentApplicationLock{
					Environment: "staging-1",
					Application: "potato-app",
					LockId:      "staging-1-potato-app-lock",
				},
			},
			Request: &api.ReleaseTrainRequest{
				Target: "staging",
			},
			ExpectedResponse: &api.GetReleaseTrainPrognosisResponse{
				EnvsPrognoses: map[string]*api.ReleaseTrainEnvPrognosis{
					"staging-1": &api.ReleaseTrainEnvPrognosis{
						Outcome: &api.ReleaseTrainEnvPrognosis_AppsPrognoses{
							AppsPrognoses: &api.ReleaseTrainEnvPrognosis_AppsPrognosesWrapper{
								Prognoses: map[string]*api.ReleaseTrainAppPrognosis{
									"potato-app": &api.ReleaseTrainAppPrognosis{
										Outcome: &api.ReleaseTrainAppPrognosis_SkipCause{
											SkipCause: api.ReleaseTrainAppSkipCause_APP_IS_LOCKED,
										},
										Locks: []*api.Lock{
											{
												LockId:    "staging-1-potato-app-lock",
												CreatedAt: timestamppb.Now(),
												CreatedBy: &api.Actor{
													Email: "testmail@example.com",
													Name:  "test tester",
												},
												Message: "",
											},
										},
									},
								},
							},
						},
					},
					"staging-2": &api.ReleaseTrainEnvPrognosis{
						Outcome: &api.ReleaseTrainEnvPrognosis_AppsPrognoses{
							AppsPrognoses: &api.ReleaseTrainEnvPrognosis_AppsPrognosesWrapper{
								Prognoses: map[string]*api.ReleaseTrainAppPrognosis{
									"potato-app": &api.ReleaseTrainAppPrognosis{
										Outcome: &api.ReleaseTrainAppPrognosis_SkipCause{
											SkipCause: api.ReleaseTrainAppSkipCause_APP_DOES_NOT_EXIST_IN_ENV,
										},
									},
								},
							},
						},
					},
					"staging-3": &api.ReleaseTrainEnvPrognosis{
						Outcome: &api.ReleaseTrainEnvPrognosis_AppsPrognoses{
							AppsPrognoses: &api.ReleaseTrainEnvPrognosis_AppsPrognosesWrapper{
								Prognoses: map[string]*api.ReleaseTrainAppPrognosis{},
							},
						},
					},
				},
			},
			ExpectedError: codes.OK,
		},
		{
			Name: "some application is skipped because of team lock",
			Setup: []rp.Transformer{
				&rp.CreateApplicationVersion{
					Application: "potato-app",
					Manifests: map[string]string{
						"development-1": "",
						"staging-1":     "",
					},
					Team:    "sre-team",
					Version: 1,
				},
				&rp.CreateApplicationVersion{
					Application: "potato-app",
					Manifests: map[string]string{
						"development-1": "",
						"staging-1":     "",
					},
					Team:    "sre-team",
					Version: 2,
				},
				&rp.DeployApplicationVersion{
					Environment: "development-1",
					Application: "potato-app",
					Version:     2,
				},
				&rp.DeployApplicationVersion{
					Environment: "staging-1",
					Application: "potato-app",
					Version:     1,
				},
				&rp.CreateEnvironmentTeamLock{
					Environment: "staging-1",
					Team:        "sre-team",
					LockId:      "staging-1-sre-team-lock",
				},
			},
			Request: &api.ReleaseTrainRequest{
				Target: "staging",
			},
			ExpectedResponse: &api.GetReleaseTrainPrognosisResponse{
				EnvsPrognoses: map[string]*api.ReleaseTrainEnvPrognosis{
					"staging-1": &api.ReleaseTrainEnvPrognosis{
						Outcome: &api.ReleaseTrainEnvPrognosis_AppsPrognoses{
							AppsPrognoses: &api.ReleaseTrainEnvPrognosis_AppsPrognosesWrapper{
								Prognoses: map[string]*api.ReleaseTrainAppPrognosis{
									"potato-app": &api.ReleaseTrainAppPrognosis{
										Outcome: &api.ReleaseTrainAppPrognosis_SkipCause{
											SkipCause: api.ReleaseTrainAppSkipCause_TEAM_IS_LOCKED,
										},
										Locks: []*api.Lock{
											{
												LockId:    "staging-1-sre-team-lock",
												CreatedAt: timestamppb.Now(),
												CreatedBy: &api.Actor{
													Email: "testmail@example.com",
													Name:  "test tester",
												},
												Message: "",
											},
										},
									},
								},
							},
						},
					},
					"staging-2": &api.ReleaseTrainEnvPrognosis{
						Outcome: &api.ReleaseTrainEnvPrognosis_AppsPrognoses{
							AppsPrognoses: &api.ReleaseTrainEnvPrognosis_AppsPrognosesWrapper{
								Prognoses: map[string]*api.ReleaseTrainAppPrognosis{
									"potato-app": &api.ReleaseTrainAppPrognosis{
										Outcome: &api.ReleaseTrainAppPrognosis_SkipCause{
											SkipCause: api.ReleaseTrainAppSkipCause_APP_DOES_NOT_EXIST_IN_ENV,
										},
									},
								},
							},
						},
					},
					"staging-3": &api.ReleaseTrainEnvPrognosis{
						Outcome: &api.ReleaseTrainEnvPrognosis_AppsPrognoses{
							AppsPrognoses: &api.ReleaseTrainEnvPrognosis_AppsPrognosesWrapper{
								Prognoses: map[string]*api.ReleaseTrainAppPrognosis{},
							},
						},
					},
				},
			},
			ExpectedError: codes.OK,
		},
		{
			Name: "proper release train",
			Setup: []rp.Transformer{
				&rp.CreateApplicationVersion{
					Application: "potato-app",
					Manifests: map[string]string{
						"development-1": "",
						"staging-1":     "",
					},
					Version: 1,
				},
				&rp.CreateApplicationVersion{
					Application: "potato-app",
					Manifests: map[string]string{
						"development-1": "",
						"staging-1":     "",
					},
					Version: 2,
				},
				&rp.DeployApplicationVersion{
					Environment: "development-1",
					Application: "potato-app",
					Version:     2,
				},
				&rp.DeployApplicationVersion{
					Environment: "staging-1",
					Application: "potato-app",
					Version:     1,
				},
			},
			Request: &api.ReleaseTrainRequest{
				Target: "staging",
			},
			ExpectedResponse: &api.GetReleaseTrainPrognosisResponse{
				EnvsPrognoses: map[string]*api.ReleaseTrainEnvPrognosis{
					"staging-1": &api.ReleaseTrainEnvPrognosis{
						Outcome: &api.ReleaseTrainEnvPrognosis_AppsPrognoses{
							AppsPrognoses: &api.ReleaseTrainEnvPrognosis_AppsPrognosesWrapper{
								Prognoses: map[string]*api.ReleaseTrainAppPrognosis{
									"potato-app": &api.ReleaseTrainAppPrognosis{
										Outcome: &api.ReleaseTrainAppPrognosis_DeployedVersion{
											DeployedVersion: 2,
										},
									},
								},
							},
						},
					},
					"staging-2": &api.ReleaseTrainEnvPrognosis{
						Outcome: &api.ReleaseTrainEnvPrognosis_AppsPrognoses{
							AppsPrognoses: &api.ReleaseTrainEnvPrognosis_AppsPrognosesWrapper{
								Prognoses: map[string]*api.ReleaseTrainAppPrognosis{
									"potato-app": &api.ReleaseTrainAppPrognosis{
										Outcome: &api.ReleaseTrainAppPrognosis_SkipCause{
											SkipCause: api.ReleaseTrainAppSkipCause_APP_DOES_NOT_EXIST_IN_ENV,
										},
									},
								},
							},
						},
					},
					"staging-3": &api.ReleaseTrainEnvPrognosis{
						Outcome: &api.ReleaseTrainEnvPrognosis_AppsPrognoses{
							AppsPrognoses: &api.ReleaseTrainEnvPrognosis_AppsPrognosesWrapper{
								Prognoses: map[string]*api.ReleaseTrainAppPrognosis{},
							},
						},
					},
				},
			},
			ExpectedError: codes.OK,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			migrationsPath, err := testutil.CreateMigrationsPath(4)
			if err != nil {
				t.Fatal(err)
			}
			dbConfig := &db.DBConfig{
				DriverName:     "sqlite3",
				MigrationsPath: migrationsPath,
				WriteEslOnly:   false,
			}
			repo, err := setupRepositoryTestWithDB(t, dbConfig)
			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}
			ctx := testutil.MakeTestContext()

			err = repo.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err2 := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, environmentSetup...)
				if err2 != nil {
					return err2
				}
				_, _, _, err2 = repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Setup...)
				if err2 != nil {
					return err2
				}

				return nil
			})
			if err != nil {
				t.Fatalf("error during setup, error: %v", err)
			}

			sv := &ReleaseTrainPrognosisServer{Repository: repo}
			resp, err := sv.GetReleaseTrainPrognosis(context.Background(), tc.Request)

			if status.Code(err) != tc.ExpectedError {
				t.Fatalf("expected error doesn't match actual error, expected %v, got code: %v, error: %v", tc.ExpectedError, status.Code(err), err)
			}
			if diff := cmp.Diff(tc.ExpectedResponse, resp, protocmp.Transform(), protocmp.IgnoreFields(&api.Lock{}, "created_at")); diff != "" {
				t.Fatalf("expected response mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
