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
	"os/exec"
	"path"
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/conversion"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

// Used to compare two error message strings, needed because errors.Is(fmt.Errorf(text),fmt.Errorf(text)) == false
type errMatcher struct {
	msg string
}

func (e errMatcher) Error() string {
	return e.msg
}

func (e errMatcher) Is(err error) bool {
	return e.Error() == err.Error()
}

func getBatchActions() []*api.BatchAction {
	opDeploy := &api.BatchAction_Deploy{
		Deploy: &api.DeployRequest{
			Environment:  "production",
			Application:  "test",
			Version:      1,
			LockBehavior: api.LockBehavior_FAIL,
		},
	}
	opCreateEnvLock := &api.BatchAction_CreateEnvironmentLock{
		CreateEnvironmentLock: &api.CreateEnvironmentLockRequest{
			Environment: "production",
			LockId:      "envlock",
			Message:     "please",
		},
	}
	opCreateAppLock := &api.BatchAction_CreateEnvironmentApplicationLock{
		CreateEnvironmentApplicationLock: &api.CreateEnvironmentApplicationLockRequest{
			Environment: "production",
			Application: "test",
			LockId:      "applock",
			Message:     "please",
		},
	}
	opDeleteEnvLock := &api.BatchAction_DeleteEnvironmentLock{ // this deletes the existing lock in the transformers
		DeleteEnvironmentLock: &api.DeleteEnvironmentLockRequest{
			Environment: "production",
			LockId:      "1234",
		},
	}
	opDeleteAppLock := &api.BatchAction_DeleteEnvironmentApplicationLock{ // this deletes the existing lock in the transformers
		DeleteEnvironmentApplicationLock: &api.DeleteEnvironmentApplicationLockRequest{
			Environment: "production",
			Application: "test",
			LockId:      "5678",
		},
	}
	opCreateTeamLock := &api.BatchAction_CreateEnvironmentTeamLock{ // this deletes the existing lock in the transformers
		CreateEnvironmentTeamLock: &api.CreateEnvironmentTeamLockRequest{
			Environment: "production",
			Team:        "test-team",
			LockId:      "teamlock",
			Message:     "Test Create a Team lock",
		},
	}
	opDeleteTeamLock := &api.BatchAction_DeleteEnvironmentTeamLock{ // this deletes the existing lock in the transformers
		DeleteEnvironmentTeamLock: &api.DeleteEnvironmentTeamLockRequest{
			Environment: "production",
			Team:        "test-team",
			LockId:      "91011",
		},
	}
	ops := []*api.BatchAction{ // it works through the batch in order
		{Action: opDeleteEnvLock},
		{Action: opDeleteAppLock},
		{Action: opDeleteTeamLock},
		{Action: opDeploy},
		{Action: opCreateEnvLock},
		{Action: opCreateAppLock},
		{Action: opCreateTeamLock},
	}
	return ops
}

func getNBatchActions(N int) []*api.BatchAction {
	var ops []*api.BatchAction
	for i := 1; i <= N; i++ {
		deploy := api.DeployRequest{
			Environment:  "production",
			Application:  "test",
			Version:      1,
			LockBehavior: api.LockBehavior_FAIL,
		}
		if i%2 == 0 {
			deploy.Version = 2
		}
		ops = append(ops, &api.BatchAction{
			Action: &api.BatchAction_Deploy{
				Deploy: &deploy,
			},
		})
	}
	return ops
}

func TestBatchServiceWorks(t *testing.T) {
	const prod = "production"
	var lifeTime2d = "2d"
	var lifeTime4h = "4h"
	var lifeTime3w = "3w"
	tcs := []struct {
		Name          string
		Batch         []*api.BatchAction
		Setup         []repository.Transformer
		svc           *BatchServer
		expectedError error
	}{
		{
			Name: "5 sample actions",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: prod,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						prod: "manifest",
					},
					Team:    "test-team",
					Version: 1,
				},
				&repository.CreateEnvironmentLock{ // will be deleted by the batch actions
					Environment: prod,
					LockId:      "1234",
					Message:     "EnvLock",
				},
				&repository.CreateEnvironmentApplicationLock{ // will be deleted by the batch actions
					Environment:       prod,
					Application:       "test",
					LockId:            "5678",
					Message:           "AppLock",
					SuggestedLifeTime: &lifeTime3w,
				},
				&repository.CreateEnvironmentTeamLock{ // will be deleted by the batch actions
					Environment: prod,
					Team:        "test-team",
					LockId:      "91011",
					Message:     "TeamLock",
				},
			},

			Batch: getBatchActions(),
			svc:   &BatchServer{},
		},
		{
			Name: "testing Dex setup with permissions",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "production",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "manifest",
					},
					Team:    "test-team",
					Version: 1,
				},
				&repository.CreateEnvironmentLock{
					Environment:       "production",
					LockId:            "1234",
					Message:           "EnvLock",
					SuggestedLifeTime: &lifeTime4h,
				},
				&repository.CreateEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					LockId:      "5678",
					Message:     "no message",
				},
				&repository.CreateEnvironmentTeamLock{ // will be deleted by the batch actions
					Environment:       prod,
					Team:              "test-team",
					LockId:            "91011",
					Message:           "TeamLock",
					SuggestedLifeTime: &lifeTime2d,
				},
			},
			Batch: getBatchActions(),
			svc: &BatchServer{
				RBACConfig: auth.RBACConfig{
					DexEnabled: true,
					Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,DeployRelease,production:production,*,allow": {Role: "Developer"},
						"p,role:developer,CreateLock,production:production,*,allow":    {Role: "Developer"},
						"p,role:developer,DeleteLock,production:production,*,allow":    {Role: "Developer"},
					},
					},
					Team: &auth.RBACTeams{Permissions: map[string][]string{
						"testmail@example.com": []string{"test-team"},
					}},
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			repo, err := setupRepositoryTestWithDB(t)
			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}
			ctx := testutil.MakeTestContext()
			user, err := auth.ReadUserFromContext(ctx)
			if err != nil {
				t.Fatalf("error reading user from context")
			}
			user.DexAuthContext = &auth.DexAuthContext{
				Role: []string{"developer"},
			}
			ctx = auth.WriteUserToContext(ctx, *user)

			_ = repo.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err2 := repo.ApplyTransformersInternal(ctx, transaction, tc.Setup...)
				if err2 != nil && err2.TransformerError != nil {
					t.Fatalf("error applying transformers: %v", err2.TransformerError)
				}
				return nil
			})
			_ = repo.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {

				tc.svc.Repository = repo
				resp, err := tc.svc.ProcessBatch(
					ctx,
					&api.BatchRequest{
						Actions: tc.Batch,
					},
				)
				if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				if tc.expectedError != nil {
					return nil
				}
				if len(resp.Results) != len(tc.Batch) {
					t.Errorf("got wrong number of batch results, expected %d but got %d", len(tc.Batch), len(resp.Results))
				}
				return nil
			})
			_ = repo.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				// check deployment version
				{
					version, err := tc.svc.Repository.State().GetEnvironmentApplicationVersion(ctx, transaction, "production", "test")
					if err != nil {
						t.Fatal(err)
					}
					if version == nil {
						t.Errorf("unexpected version: expected 1, actual: %d", version)
					}
					if *version != 1 {
						t.Errorf("unexpected version: expected 1, actual: %d", *version)
					}
				}
				// check that the envlock was created/deleted
				{
					envLocks, err := tc.svc.Repository.State().GetEnvironmentLocks(ctx, transaction, "production")
					if err != nil {
						t.Fatal(err)
					}
					lock, exists := envLocks["envlock"]
					if !exists {
						t.Error("lock was not created")
					}
					if lock.Message != "please" {
						t.Errorf("unexpected lock message: expected \"please\", actual: %q", lock.Message)
					}
					_, exists = envLocks["1234"]
					if exists {
						t.Error("lock was not deleted")
					}
				}
				// check that the applock was created/deleted
				{
					appLocks, err := tc.svc.Repository.State().GetEnvironmentApplicationLocks(ctx, transaction, "production", "test")
					if err != nil {
						t.Fatal(err)
					}
					lock, exists := appLocks["applock"]
					if !exists {
						t.Error("lock was not created")
					}
					if lock.Message != "please" {
						t.Errorf("unexpected lock message: expected \"please\", actual: %q", lock.Message)
					}
					_, exists = appLocks["5678"]
					if exists {
						t.Error("lock was not deleted")
					}
				}
				//Check that Team lock was created
				{
					teamLocks, err := tc.svc.Repository.State().GetEnvironmentTeamLocks(ctx, transaction, "production", "test-team")
					if err != nil {
						t.Fatal(err)
					}
					lock, exists := teamLocks["teamlock"]
					if !exists {
						t.Error("Team lock was not created")
					}
					if lock.Message != "Test Create a Team lock" {
						t.Errorf("unexpected lock message: expected \"please\", actual: %q", lock.Message)
					}
					_, exists = teamLocks["91011"]
					if exists {
						t.Error("lock was not deleted")
					}
				}
				return nil
			})

		})
	}
}

func TestBatchServiceFails(t *testing.T) {
	tcs := []struct {
		Name               string
		Batch              []*api.BatchAction
		Setup              []repository.Transformer
		context            context.Context
		svc                *BatchServer
		expectedError      error
		expectedSetupError error
	}{
		{
			Name: "testing Dex setup without permissions",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "production",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "manifest",
					},
					Version: 1,
				},
				&repository.CreateEnvironmentLock{ // will be deleted by the batch actions
					Environment:    "production",
					LockId:         "1234",
					Message:        "EnvLock",
					Authentication: repository.Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true}},
				},
			},
			Batch:              []*api.BatchAction{},
			context:            testutil.MakeTestContextDexEnabled(),
			svc:                &BatchServer{},
			expectedSetupError: errMatcher{"the desired action can not be performed because Dex is enabled without any RBAC policies"},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			repo, err := setupRepositoryTestWithDB(t)
			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}
			ctx := testutil.MakeTestContext()
			errSetupObserved := false
			_ = repo.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err2 := repo.ApplyTransformersInternal(ctx, transaction, tc.Setup...)
				if err2 != nil && err2.TransformerError != nil {
					if diff := cmp.Diff(tc.expectedSetupError, err2.TransformerError, cmpopts.EquateErrors()); diff != "" {
						t.Fatalf("error during setup mismatch (-want, +got):\n%s", diff)
					} else {
						errSetupObserved = true
					}
				}
				return nil
			})
			if tc.expectedSetupError != nil && !errSetupObserved {
				// ensure we fail on unobserved error
				t.Errorf("did not oberve error during setup: %s", tc.expectedSetupError.Error())
			}

			tc.svc.Repository = repo
			resp, err := tc.svc.ProcessBatch(
				tc.context,
				&api.BatchRequest{
					Actions: tc.Batch,
				},
			)
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			if len(resp.Results) != len(tc.Batch) {
				t.Errorf("got wrong number of batch results, expected %d but got %d", len(tc.Batch), len(resp.Results))
			}
		})
	}
}

func TestBatchServiceErrors(t *testing.T) {
	tcs := []struct {
		Name             string
		Batch            []*api.BatchAction
		Setup            []repository.Transformer
		ExpectedResponse *api.BatchResponse
		ExpectedError    error
	}{
		{
			// tests that in ProcessBatch, transformer errors are returned without wrapping them in a
			// not so helpful "internal error"
			Name:  "forwards transformers error to caller: cannot open manifest",
			Setup: []repository.Transformer{},
			Batch: []*api.BatchAction{
				{
					Action: &api.BatchAction_Deploy{
						Deploy: &api.DeployRequest{
							Environment:  "dev",
							Application:  "myapp",
							Version:      666,
							LockBehavior: 0,
						},
					},
				}},
			ExpectedResponse: nil,
			ExpectedError: &repository.TransformerBatchApplyError{
				Index:            0,
				TransformerError: errMatcher{"error at index 0 of transformer batch: could not find version 666 for app myapp"},
			},
		},
		{
			Name:  "create release endpoint fails app validity check",
			Setup: []repository.Transformer{},
			Batch: []*api.BatchAction{
				{
					Action: &api.BatchAction_CreateRelease{
						CreateRelease: &api.CreateReleaseRequest{
							Environment:    "dev",
							Application:    "myappIsWayTooLongDontYouThink",
							Team:           "team1",
							Manifests:      nil,
							Version:        666,
							SourceCommitId: "1",
							SourceAuthor:   "2",
							SourceMessage:  "3",
							SourceRepoUrl:  "4",
						},
					},
				},
			},
			ExpectedResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					{
						Result: &api.BatchResult_CreateReleaseResponse{
							CreateReleaseResponse: &api.CreateReleaseResponse{
								Response: &api.CreateReleaseResponse_TooLong{
									TooLong: &api.CreateReleaseResponseAppNameTooLong{
										AppName: "myappIsWayTooLongDontYouThink",
										RegExp:  "\\A[a-z0-9]+(?:-[a-z0-9]+)*\\z",
										MaxLen:  39,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			repo, err := setupRepositoryTestWithDB(t)
			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}
			ctx := testutil.MakeTestContext()
			_ = repo.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err2 := repo.ApplyTransformersInternal(ctx, transaction, tc.Setup...)
				if err2 != nil && err2.TransformerError != nil {
					t.Fatalf("error applying transformers: %v", err2.TransformerError)
				}
				return nil
			})
			svc := &BatchServer{
				Repository: repo,
			}
			response, processErr := svc.ProcessBatch(
				testutil.MakeTestContext(),
				&api.BatchRequest{
					Actions: tc.Batch,
				},
			)
			if diff := cmp.Diff(tc.ExpectedError, processErr, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.ExpectedResponse, response, protocmp.Transform()); diff != "" {
				t.Fatalf("response mismatch, diff (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestBatchServiceLimit(t *testing.T) {
	transformers := []repository.Transformer{
		&repository.CreateEnvironment{
			Environment: "production",
			Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
		},
		&repository.CreateApplicationVersion{
			Application: "test",
			Manifests: map[string]string{
				"production": "manifest",
			},
			Version: 1,
		},
		&repository.CreateApplicationVersion{
			Application: "test",
			Manifests: map[string]string{
				"production": "manifest2",
			},
			Version: 2,
		},
	}
	var two uint64 = 2
	tcs := []struct {
		Name            string
		Batch           []*api.BatchAction
		Setup           []repository.Transformer
		ShouldSucceed   bool
		ExpectedVersion *uint64
	}{
		{
			Name:            "exactly the maximum number of actions",
			Setup:           transformers,
			ShouldSucceed:   true,
			Batch:           getNBatchActions(maxBatchActions),
			ExpectedVersion: &two,
		},
		{
			Name:            "more than the maximum number of actions",
			Setup:           transformers,
			ShouldSucceed:   false,
			Batch:           getNBatchActions(maxBatchActions + 1), // more than max
			ExpectedVersion: nil,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			repo, err := setupRepositoryTestWithDB(t)
			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}
			ctx := testutil.MakeTestContext()
			_ = repo.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err2 := repo.ApplyTransformersInternal(ctx, transaction, tc.Setup...)
				if err2 != nil && err2.TransformerError != nil {
					t.Fatalf("error applying transformers: %v", err2.TransformerError)
				}
				return nil
			})
			svc := &BatchServer{
				Repository: repo,
			}
			_, err = svc.ProcessBatch(
				testutil.MakeTestContext(),
				&api.BatchRequest{
					Actions: tc.Batch,
				},
			)
			if !tc.ShouldSucceed {
				if err == nil {
					t.Fatal("expected an error but got none")
				}
				s, ok := status.FromError(err)
				if !ok {
					t.Fatalf("error is not a status error, got: %#v", err)
				}
				expectedMessage := fmt.Sprintf("cannot process batch: too many actions. limit is %d", maxBatchActions)
				if s.Message() != expectedMessage {
					t.Errorf("invalid error message: expected %q, actual: %q", expectedMessage, s.Message())
				}
			} else {
				_ = repo.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
					if err != nil {
						t.Fatal(err)
					}
					version, err := svc.Repository.State().GetEnvironmentApplicationVersion(context.Background(), transaction, "production", "test")
					if err != nil {
						t.Fatal(err)
					}
					if version == nil {
						t.Errorf("unexpected version: expected %d, actual: %d", *tc.ExpectedVersion, version)
					}
					if *version != *tc.ExpectedVersion {
						t.Errorf("unexpected version: expected %d, actual: %d", *tc.ExpectedVersion, *version)
					}
					return nil
				})
			}
		})
	}
}

func setupRepositoryTestWithDB(t *testing.T) (repository.Repository, error) {
	return setupRepositoryTestWithAllOptions(t, true)
}

func setupRepositoryTestWithAllOptions(t *testing.T, withBackgroundJob bool) (repository.Repository, error) {
	ctx := context.Background()
	migrationsPath, err := db.CreateMigrationsPath(4)
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}
	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	cmd.Start()
	cmd.Wait()
	t.Logf("test created dir: %s", localDir)

	dbConfig, err := db.ConnectToPostgresContainer(ctx, t, migrationsPath, false, t.Name())
	if err != nil {
		t.Fatalf("SetupPostgres: %v", err)
	}

	repoCfg := repository.RepositoryConfig{
		URL:                 remoteDir,
		ArgoCdGenerateFiles: true,
		DisableQueue:        true,
	}
	if dbConfig != nil {

		migErr := db.RunDBMigrations(ctx, *dbConfig)
		if migErr != nil {
			t.Fatal(migErr)
		}

		db, err := db.Connect(ctx, *dbConfig)
		if err != nil {
			t.Fatal(err)
		}
		repoCfg.DBHandler = db
	}

	if withBackgroundJob {
		repo, err := repository.New(
			testutil.MakeTestContext(),
			repoCfg,
		)
		if err != nil {
			t.Fatal(err)
		}
		return repo, nil
	} else {
		repo, _, err := repository.New2(
			testutil.MakeTestContext(),
			repoCfg,
		)
		if err != nil {
			t.Fatal(err)
		}
		return repo, nil
	}
}

func setupRepositoryTest(t *testing.T) (repository.Repository, error) {
	return setupRepositoryTestWithDB(t)
}

func TestReleaseTrain(t *testing.T) {
	tcs := []struct {
		Name             string
		Setup            []repository.Transformer
		Request          *api.BatchRequest
		ExpectedResponse *api.BatchResponse
	}{
		{
			Name: "Get Upstream env and TargetEnv",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: "production"}},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"acceptance": "manifest",
					},
					Version: 1,
				},
			},
			Request: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_ReleaseTrain{
							ReleaseTrain: &api.ReleaseTrainRequest{
								Target: "acceptance",

								Team: "team",
							},
						},
					},
				},
			},
			ExpectedResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					{
						Result: &api.BatchResult_ReleaseTrain{
							ReleaseTrain: &api.ReleaseTrainResponse{
								Target: "acceptance",
								Team:   "team",
							},
						},
					},
				},
			},
		},
		{
			Name: "Get Upstream (latest) and TargetEnv",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"acceptance": "manifest",
					},
					Version: 1,
				},
			},
			Request: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_ReleaseTrain{
							ReleaseTrain: &api.ReleaseTrainRequest{
								Target: "acceptance",

								Team: "team",
							},
						},
					},
				},
			},
			ExpectedResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					{
						Result: &api.BatchResult_ReleaseTrain{
							ReleaseTrain: &api.ReleaseTrainResponse{
								Target: "acceptance",
								Team:   "team",
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			repo, err := setupRepositoryTestWithDB(t)
			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}
			ctx := testutil.MakeTestContext()
			_ = repo.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err2 := repo.ApplyTransformersInternal(ctx, transaction, tc.Setup...)
				if err2 != nil && err2.TransformerError != nil {
					t.Fatalf("error applying transformers: %v", err2.TransformerError)
				}
				return nil
			})
			svc := &BatchServer{
				Repository: repo,
			}
			resp, err := svc.ProcessBatch(
				testutil.MakeTestContext(),
				tc.Request,
			)
			if err != nil {
				t.Errorf("unexpected error: %q", err)
			}
			if d := cmp.Diff(tc.ExpectedResponse, resp, protocmp.Transform()); d != "" {
				t.Errorf("batch response mismatch: %s", d)
			}
		})
	}
}

func TestCreateEnvironmentTrain(t *testing.T) {
	tcs := []struct {
		Name                 string
		Setup                []repository.Transformer
		Request              *api.BatchRequest
		ExpectedResponse     *api.BatchResponse
		ExpectedEnvironments map[string]config.EnvironmentConfig
	}{
		{
			Name:  "Minimal test case",
			Setup: []repository.Transformer{},
			Request: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_CreateEnvironment{
							CreateEnvironment: &api.CreateEnvironmentRequest{
								Environment: "env",
								Config: &api.EnvironmentConfig{
									Argocd: &api.EnvironmentConfig_ArgoCD{
										ConcreteEnvName: "placeholder",
									},
								},
							},
						},
					},
				},
			},
			ExpectedResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					nil,
				},
			},
			ExpectedEnvironments: map[string]config.EnvironmentConfig{
				"env": {
					ArgoCd: &config.EnvironmentConfigArgoCd{ConcreteEnvName: "placeholder"},
				},
			},
		},
		{
			Name:  "With upstream latest",
			Setup: []repository.Transformer{},
			Request: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_CreateEnvironment{
							CreateEnvironment: &api.CreateEnvironmentRequest{
								Environment: "env",
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Latest: conversion.Bool(true),
									},
									Argocd: &api.EnvironmentConfig_ArgoCD{
										ConcreteEnvName: "placeholder",
									},
								},
							},
						},
					},
				},
			},
			ExpectedResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					nil,
				},
			},
			ExpectedEnvironments: map[string]config.EnvironmentConfig{
				"env": config.EnvironmentConfig{
					Upstream: &config.EnvironmentConfigUpstream{Latest: true},
					ArgoCd:   &config.EnvironmentConfigArgoCd{ConcreteEnvName: "placeholder"},
				},
			},
		},
		{
			Name:  "With upstream env",
			Setup: []repository.Transformer{},
			Request: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_CreateEnvironment{
							CreateEnvironment: &api.CreateEnvironmentRequest{
								Environment: "env",
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Environment: conversion.FromString("other-env"),
									},
									Argocd: &api.EnvironmentConfig_ArgoCD{
										ConcreteEnvName: "placeholder",
									},
								},
							},
						},
					},
				},
			},
			ExpectedResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					nil,
				},
			},
			ExpectedEnvironments: map[string]config.EnvironmentConfig{
				"env": config.EnvironmentConfig{
					Upstream: &config.EnvironmentConfigUpstream{Environment: "other-env"},
					ArgoCd: &config.EnvironmentConfigArgoCd{
						ConcreteEnvName: "placeholder",
					},
				},
			},
		},
		{
			Name:  "With minimal argocd config",
			Setup: []repository.Transformer{},
			Request: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_CreateEnvironment{
							CreateEnvironment: &api.CreateEnvironmentRequest{
								Environment: "env",
								Config: &api.EnvironmentConfig{
									Argocd: &api.EnvironmentConfig_ArgoCD{},
								},
							},
						},
					},
				},
			},
			ExpectedResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					nil,
				},
			},
			ExpectedEnvironments: map[string]config.EnvironmentConfig{
				"env": config.EnvironmentConfig{
					ArgoCd: &config.EnvironmentConfigArgoCd{},
				},
			},
		},
		{
			Name:  "With full argocd config",
			Setup: []repository.Transformer{},
			Request: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_CreateEnvironment{
							CreateEnvironment: &api.CreateEnvironmentRequest{
								Environment: "env",
								Config: &api.EnvironmentConfig{
									Argocd: &api.EnvironmentConfig_ArgoCD{
										Destination: &api.EnvironmentConfig_ArgoCD_Destination{
											Name:                 "name",
											Server:               "server",
											Namespace:            conversion.FromString("namespace"),
											AppProjectNamespace:  conversion.FromString("app-project-namespace"),
											ApplicationNamespace: conversion.FromString("app-namespace"),
										},
										SyncWindows: []*api.EnvironmentConfig_ArgoCD_SyncWindows{
											&api.EnvironmentConfig_ArgoCD_SyncWindows{
												Schedule:     "schedule",
												Duration:     "duration",
												Kind:         "kind",
												Applications: []string{"applications"},
											},
										},
										AccessList: []*api.EnvironmentConfig_ArgoCD_AccessEntry{
											&api.EnvironmentConfig_ArgoCD_AccessEntry{
												Group: "group",
												Kind:  "kind",
											},
										},
										SyncOptions: []string{"sync-option"},
										IgnoreDifferences: []*api.EnvironmentConfig_ArgoCD_IgnoreDifferences{
											{
												Group:                 "group",
												Kind:                  "kind",
												Name:                  "name",
												Namespace:             "namespace",
												JsonPointers:          []string{"/json"},
												JqPathExpressions:     []string{".jq"},
												ManagedFieldsManagers: []string{"manager"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			ExpectedResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					nil,
				},
			},
			ExpectedEnvironments: map[string]config.EnvironmentConfig{
				"env": config.EnvironmentConfig{
					ArgoCd: &config.EnvironmentConfigArgoCd{
						Destination: config.ArgoCdDestination{
							Name:                 "name",
							Server:               "server",
							Namespace:            conversion.FromString("namespace"),
							AppProjectNamespace:  conversion.FromString("app-project-namespace"),
							ApplicationNamespace: conversion.FromString("app-namespace"),
						},
						SyncWindows: []config.ArgoCdSyncWindow{
							{
								Schedule: "schedule",
								Duration: "duration",
								Kind:     "kind",
								Apps:     []string{"applications"},
							},
						},
						ClusterResourceWhitelist: []config.AccessEntry{{Group: "group", Kind: "kind"}},
						SyncOptions:              []string{"sync-option"},
						IgnoreDifferences: []config.ArgoCdIgnoreDifference{
							{
								Group:                 "group",
								Kind:                  "kind",
								Name:                  "name",
								Namespace:             "namespace",
								JSONPointers:          []string{"/json"},
								JqPathExpressions:     []string{".jq"},
								ManagedFieldsManagers: []string{"manager"},
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {

			repo, err := setupRepositoryTestWithDB(t)
			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}
			ctx := testutil.MakeTestContext()
			_ = repo.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err2 := repo.ApplyTransformersInternal(ctx, transaction, tc.Setup...)
				if err2 != nil && err2.TransformerError != nil {
					t.Fatalf("error applying transformers: %v", err2.TransformerError)
				}
				return nil
			})
			svc := &BatchServer{
				Repository: repo,
			}
			resp, err := svc.ProcessBatch(
				testutil.MakeTestContext(),
				tc.Request,
			)
			if err != nil {
				t.Errorf("unexpected error: %q", err)
			}
			if d := cmp.Diff(tc.ExpectedResponse, resp, protocmp.Transform()); d != "" {
				t.Errorf("batch response mismatch: %s", d)
			}

			var envs map[string]config.EnvironmentConfig
			var envsPtr *map[string]config.EnvironmentConfig
			envsPtr, err = db.WithTransactionT(repo.State().DBHandler, ctx, db.DefaultNumRetries, true, func(ctx context.Context, transaction *sql.Tx) (*map[string]config.EnvironmentConfig, error) {
				envs, err := repo.State().GetAllEnvironmentConfigs(ctx, transaction)
				return &envs, err
			})
			envs = *envsPtr
			if err != nil {
				t.Errorf("unexpected error: %q", err)
			}
			if d := cmp.Diff(tc.ExpectedEnvironments, envs); d != "" {
				t.Errorf("batch response mismatch: %s", d)
			}
		})
	}
}

// this tests that we can get the time out of a time-uuid.
func TestActiveActiveEnvironmentNames(t *testing.T) {
	tcs := []struct {
		Name            string
		EnvironmentName string
		InputEnvConfig  config.EnvironmentConfig
		valid           bool
	}{
		{
			Name:            "invalid",
			EnvironmentName: "dev",
			InputEnvConfig: config.EnvironmentConfig{
				Upstream:         nil,
				ArgoCdConfigs:    testutil.MakeArgoCDConfigs("", "", 10),
				EnvironmentGroup: nil,
				ArgoCd:           nil,
			},
			valid: false,
		},
		{
			Name:            "invalid, no concrete name specified",
			EnvironmentName: "dev",
			InputEnvConfig: config.EnvironmentConfig{
				Upstream:         nil,
				ArgoCdConfigs:    testutil.MakeArgoCDConfigs("aa", "", 10),
				EnvironmentGroup: nil,
				ArgoCd:           nil,
			},
			valid: false,
		},
		{
			Name:            "invalid, no common name specified",
			EnvironmentName: "dev",
			InputEnvConfig: config.EnvironmentConfig{
				Upstream:         nil,
				ArgoCdConfigs:    testutil.MakeArgoCDConfigs("", "de", 10),
				EnvironmentGroup: nil,
				ArgoCd:           nil,
			},
			valid: false,
		},
		{
			Name:            "invalid, environmentName is invalid",
			EnvironmentName: "DeVeLoPment#",
			InputEnvConfig: config.EnvironmentConfig{
				Upstream:         nil,
				ArgoCdConfigs:    testutil.MakeArgoCDConfigs("aa", "de", 1),
				EnvironmentGroup: nil,
				ArgoCd:           nil,
			},
			valid: false,
		},
		{
			Name:            "valid as there is only one env specified",
			EnvironmentName: "dev",
			InputEnvConfig: config.EnvironmentConfig{
				Upstream:         nil,
				ArgoCdConfigs:    testutil.MakeArgoCDConfigs("", "", 1),
				EnvironmentGroup: nil,
				ArgoCd:           nil,
			},
			valid: true,
		},
		{
			Name:            "valid",
			EnvironmentName: "dev",
			InputEnvConfig: config.EnvironmentConfig{
				Upstream:         nil,
				ArgoCdConfigs:    testutil.MakeArgoCDConfigs("aa", "de", 10),
				EnvironmentGroup: nil,
				ArgoCd:           nil,
			},
			valid: true,
		},
		{
			Name:            "valid, only one env discards common and concrete",
			EnvironmentName: "dev",
			InputEnvConfig: config.EnvironmentConfig{
				Upstream:         nil,
				ArgoCdConfigs:    testutil.MakeArgoCDConfigs("InvalidCommonNAme#", "InvalidConcreteNAme#", 1),
				EnvironmentGroup: nil,
				ArgoCd:           nil,
			},
			valid: true,
		},
		{
			Name:            "invalid, must specify either argocd or ArgoCdConfigs",
			EnvironmentName: "dev",
			InputEnvConfig: config.EnvironmentConfig{
				Upstream:         nil,
				ArgoCdConfigs:    nil,
				EnvironmentGroup: nil,
				ArgoCd:           nil,
			},
			valid: false,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			err := ValidateEnvironment(tc.EnvironmentName, tc.InputEnvConfig)

			isValid := err == nil
			if isValid != tc.valid {
				t.Errorf("Invalid environment: %v, %v", tc.InputEnvConfig, err)
			}
		})
	}
}
