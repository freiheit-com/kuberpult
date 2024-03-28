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

package service

import (
	"context"

	"testing"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository/testutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/ptr"
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
				EnvironmentGroup: ptr.FromString("development"),
			},
		},
		&rp.CreateEnvironment{
			Environment: "development-2",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Environment: "",
					Latest:      true,
				},
				EnvironmentGroup: ptr.FromString("development"),
			},
		},
		&rp.CreateEnvironment{
			Environment: "development-3",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Environment: "",
					Latest:      true,
				},
				EnvironmentGroup: ptr.FromString("development"),
			},
		},
		&rp.CreateEnvironment{
			Environment: "staging-1",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Environment: "development-1",
					Latest:      false,
				},
				EnvironmentGroup: ptr.FromString("staging"),
			},
		},
		&rp.CreateEnvironment{
			Environment: "staging-2",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Environment: "development-1", // CAREFUL, downstream from development-1, not development-2
					Latest:      false,
				},
				EnvironmentGroup: ptr.FromString("staging"),
			},
		},
		&rp.CreateEnvironment{
			Environment: "staging-3",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Environment: "development-3",
					Latest:      false,
				},
				EnvironmentGroup: ptr.FromString("staging"),
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
				},
				&rp.CreateApplicationVersion{
					Application: "potato-app",
					Manifests: map[string]string{
						"development-1": "",
						"staging-1":     "",
					},
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
				},
				&rp.CreateApplicationVersion{
					Application: "potato-app",
					Manifests: map[string]string{
						"development-1": "",
						"staging-1":     "",
					},
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
			repo, err := setupRepositoryTest(t)
			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}

			err = repo.Apply(testutil.MakeTestContext(), environmentSetup...)
			if err != nil {
				t.Fatalf("error during setup, error: %v", err)
			}
			err = repo.Apply(testutil.MakeTestContext(), tc.Setup...)
			if err != nil {
				t.Fatalf("error during setup, error: %v", err)
			}

			sv := &ReleaseTrainPrognosisServer{Repository: repo}
			resp, err := sv.GetReleaseTrainPrognosis(context.Background(), tc.Request)

			if status.Code(err) != tc.ExpectedError {
				t.Fatalf("expected error doesn't match actual error, expected %v, got code: %v, error: %v", tc.ExpectedError, status.Code(err), err)
			}
			if !proto.Equal(tc.ExpectedResponse, resp) {
				t.Fatalf("expected respones doesn't match actualy response, expected %v, got %v", tc.ExpectedResponse, resp)
			}
		})
	}
}
