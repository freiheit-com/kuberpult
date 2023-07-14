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
	"errors"
	"fmt"
	"os/exec"
	"path"
	"testing"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

func getBatchActions() []*api.BatchAction {
	opDeploy := &api.BatchAction_Deploy{
		Deploy: &api.DeployRequest{
			Environment:  "production",
			Application:  "test",
			Version:      1,
			LockBehavior: api.LockBehavior_Fail,
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
	ops := []*api.BatchAction{ // it works through the batch in order
		{Action: opDeleteEnvLock},
		{Action: opDeleteAppLock},
		{Action: opDeploy},
		{Action: opCreateEnvLock},
		{Action: opCreateAppLock},
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
			LockBehavior: api.LockBehavior_Fail,
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
	tcs := []struct {
		Name  string
		Batch []*api.BatchAction
		Setup []repository.Transformer
	}{
		{
			Name: "5 sample actions",
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
				},
				&repository.CreateEnvironmentLock{ // will be deleted by the batch actions
					Environment: "production",
					LockId:      "1234",
					Message:     "EnvLock",
				},
				&repository.CreateEnvironmentApplicationLock{ // will be deleted by the batch actions
					Environment: "production",
					Application: "test",
					LockId:      "5678",
					Message:     "AppLock",
				},
			},
			Batch: getBatchActions(),
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			repo, err := setupRepositoryTest(t)
			if err != nil {
				t.Fatal(err)
			}
			for _, tr := range tc.Setup {
				if err := repo.Apply(testutil.MakeTestContext(), tr); err != nil {
					t.Fatal(err)
				}
			}
			svc := &BatchServer{
				Repository: repo,
			}
			resp, err := svc.ProcessBatch(
				testutil.MakeTestContext(),
				&api.BatchRequest{
					Actions: tc.Batch,
				},
			)
			if err != nil {
				t.Fatal(err.Error())
			}
			if len(resp.Results) != len(tc.Batch) {
				t.Errorf("got wrong number of batch results, expected %d but got %d", len(tc.Batch), len(resp.Results))
			}

			// check deployment version
			{
				version, err := svc.Repository.State().GetEnvironmentApplicationVersion("production", "test")
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
				envLocks, err := svc.Repository.State().GetEnvironmentLocks("production")
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
				appLocks, err := svc.Repository.State().GetEnvironmentApplicationLocks("production", "test")
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

		})
	}
}

type ErrorTransformer struct{}

func (p *ErrorTransformer) Transform(ctx context.Context, state *repository.State) (string, error) {
	return "error", errors.New("i am the transformer error")
}

func TestBatchServiceErrors(t *testing.T) {
	tcs := []struct {
		Name          string
		Batch         []*api.BatchAction
		Setup         []repository.Transformer
		ExpectedError string
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
			ExpectedError: "could not open manifest 'applications/myapp/releases/666/environments/dev/manifests.yaml': file does not exist",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			repo, err := setupRepositoryTest(t)
			if err != nil {
				t.Fatal(err)
			}
			for _, tr := range tc.Setup {
				if err := repo.Apply(testutil.MakeTestContext(), tr); err != nil {
					t.Fatal(err)
				}
			}
			svc := &BatchServer{
				Repository: repo,
			}
			_, err = svc.ProcessBatch(
				testutil.MakeTestContext(),
				&api.BatchRequest{
					Actions: tc.Batch,
				},
			)
			if err == nil {
				t.Fatal("Expected an error but got nil")
			}
			if err.Error() != tc.ExpectedError {
				t.Errorf("want:\n\"%v\"\nbut got:\n\"%v\"", tc.ExpectedError, err.Error())
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
		},
		&repository.CreateApplicationVersion{
			Application: "test",
			Manifests: map[string]string{
				"production": "manifest2",
			},
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
			repo, err := setupRepositoryTest(t)
			if err != nil {
				t.Fatal(err)
			}
			for _, tr := range tc.Setup {
				if err := repo.Apply(testutil.MakeTestContext(), tr); err != nil {
					t.Fatal(err)
				}
			}
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
				if err != nil {
					t.Fatal(err)
				}
				version, err := svc.Repository.State().GetEnvironmentApplicationVersion("production", "test")
				if err != nil {
					t.Fatal(err)
				}
				if version == nil {
					t.Errorf("unexpected version: expected %d, actual: %d", *tc.ExpectedVersion, version)
				}
				if *version != *tc.ExpectedVersion {
					t.Errorf("unexpected version: expected %d, actual: %d", *tc.ExpectedVersion, *version)
				}
			}
		})
	}
}

func setupRepositoryTest(t *testing.T) (repository.Repository, error) {
	t.Parallel()
	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	cmd.Start()
	cmd.Wait()
	repo, err := repository.New(
		testutil.MakeTestContext(),
		repository.RepositoryConfig{
			URL:            remoteDir,
			Path:           localDir,
			CommitterEmail: "kuberpult@freiheit.com",
			CommitterName:  "kuberpult",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, nil
}

func TestReleaseTrain(t *testing.T) {
	tcs := []struct {
		Name             string
		Setup            []repository.Transformer
		Request          *api.BatchRequest
		ExpectedResponse *api.BatchResponse
		Test             func(t *testing.T, svc *DeployServiceServer)
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
			repo, err := setupRepositoryTest(t)
			if err != nil {
				t.Fatal(err)
			}
			for _, tr := range tc.Setup {
				if err := repo.Apply(testutil.MakeTestContext(), tr); err != nil {
					t.Fatal(err)
				}
			}
			svc := &BatchServer{
				Repository: repo,
			}
			resp, err := svc.ProcessBatch(
				testutil.MakeTestContext(),
				tc.Request,
			)
			if d := cmp.Diff(tc.ExpectedResponse, resp, protocmp.Transform()); d != "" {
				t.Errorf("batch response mismatch: %s", d)
			}
		})
	}
}
