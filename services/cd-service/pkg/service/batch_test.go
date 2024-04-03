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
	"fmt"
	"github.com/go-git/go-billy/v5"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"testing"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository/testutil"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/ptr"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
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
	tcs := []struct {
		Name          string
		Batch         []*api.BatchAction
		Setup         []repository.Transformer
		context       context.Context
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
					Team: "test-team",
				},
				&repository.CreateEnvironmentLock{ // will be deleted by the batch actions
					Environment: prod,
					LockId:      "1234",
					Message:     "EnvLock",
				},
				&repository.CreateEnvironmentApplicationLock{ // will be deleted by the batch actions
					Environment: prod,
					Application: "test",
					LockId:      "5678",
					Message:     "AppLock",
				},
				&repository.CreateEnvironmentTeamLock{ // will be deleted by the batch actions
					Environment: prod,
					Team:        "test-team",
					LockId:      "91011",
					Message:     "TeamLock",
				},
			},

			Batch:   getBatchActions(),
			context: testutil.MakeTestContext(),
			svc:     &BatchServer{},
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
					Team: "test-team",
				},
				&repository.CreateEnvironmentLock{
					Environment: "production",
					LockId:      "1234",
					Message:     "EnvLock",
				},
				&repository.CreateEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					LockId:      "5678",
					Message:     "no message",
				},
				&repository.CreateEnvironmentTeamLock{ // will be deleted by the batch actions
					Environment: prod,
					Team:        "test-team",
					LockId:      "91011",
					Message:     "TeamLock",
				},
			},
			Batch:   getBatchActions(),
			context: testutil.MakeTestContextDexEnabled(),
			svc: &BatchServer{
				RBACConfig: auth.RBACConfig{
					DexEnabled: true,
					Policy: map[string]*auth.Permission{
						"developer,DeployRelease,production:production,*,allow": {Role: "Developer"},
						"developer,CreateLock,production:production,*,allow":    {Role: "Developer"},
						"developer,DeleteLock,production:production,*,allow":    {Role: "Developer"},
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
				err := repo.Apply(tc.context, tr)
				if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
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
			if tc.expectedError != nil {
				return
			}

			if len(resp.Results) != len(tc.Batch) {
				t.Errorf("got wrong number of batch results, expected %d but got %d", len(tc.Batch), len(resp.Results))
			}
			// check deployment version
			{
				version, err := tc.svc.Repository.State().GetEnvironmentApplicationVersion("production", "test")
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
				envLocks, err := tc.svc.Repository.State().GetEnvironmentLocks("production")
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
				appLocks, err := tc.svc.Repository.State().GetEnvironmentApplicationLocks("production", "test")
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
				teamLocks, err := tc.svc.Repository.State().GetEnvironmentTeamLocks("production", "test-team")
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

		})
	}
}
func listFilesHelper(fs billy.Filesystem, path string) []string {
	ret := make([]string, 0)

	files, err := fs.ReadDir(path)
	if err == nil {
		for _, file := range files {
			ret = append(ret, listFilesHelper(fs, fs.Join(path, file.Name()))...)
		}
	} else {
		ret = append(ret, path)
	}

	return ret
}
func listFiles(fs billy.Filesystem) []string {
	paths := listFilesHelper(fs, ".")
	sort.Slice(paths, func(i, j int) bool { return paths[i] < paths[j] })
	return paths
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
				},
				&repository.CreateEnvironmentLock{ // will be deleted by the batch actions
					Environment:    "production",
					LockId:         "1234",
					Message:        "EnvLock",
					Authentication: repository.Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true}},
				},
			},
			Batch:   []*api.BatchAction{},
			context: testutil.MakeTestContextDexEnabled(),
			svc:     &BatchServer{},
			// expectedSetupError: errMatcher{"error at index 0 of transformer batch: rpc error: code = PermissionDenied desc = PermissionDenied: The user 'test tester' with role 'developer' is not allowed to perform the action 'CreateLock' on environment 'production'"},
			expectedSetupError: &repository.TransformerBatchApplyError{
				Index: 0,
				TransformerError: auth.PermissionError{
					User:        "test tester",
					Role:        "developer",
					Action:      "CreateLock",
					Environment: "production",
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
			errSetupObserved := false
			for _, tr := range tc.Setup {
				err := repo.Apply(tc.context, tr)
				if err != nil {
					if diff := cmp.Diff(tc.expectedSetupError, err, cmpopts.EquateErrors()); diff != "" {
						t.Fatalf("error during setup mismatch (-want, +got):\n%s", diff)
					} else {
						errSetupObserved = true
					}
				}
			}
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
				TransformerError: errMatcher{"deployment failed: could not open manifest for app myapp with release 666 on env dev 'applications/myapp/releases/666/environments/dev/manifests.yaml': file does not exist"},
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
	t.Logf("test created dir: %s", localDir)
	repo, err := repository.New(
		testutil.MakeTestContext(),
		repository.RepositoryConfig{
			URL:                    remoteDir,
			Path:                   localDir,
			CommitterEmail:         "kuberpult@freiheit.com",
			CommitterName:          "kuberpult",
			EnvironmentConfigsPath: filepath.Join(remoteDir, "..", "environment_configs.json"),
			ArgoCdGenerateFiles:    true,
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
				"env": config.EnvironmentConfig{},
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
										Latest: ptr.Bool(true),
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
										Environment: ptr.FromString("other-env"),
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
											Namespace:            ptr.FromString("namespace"),
											AppProjectNamespace:  ptr.FromString("app-project-namespace"),
											ApplicationNamespace: ptr.FromString("app-namespace"),
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
							Namespace:            ptr.FromString("namespace"),
							AppProjectNamespace:  ptr.FromString("app-project-namespace"),
							ApplicationNamespace: ptr.FromString("app-namespace"),
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
			if err != nil {
				t.Errorf("unexpected error: %q", err)
			}
			if d := cmp.Diff(tc.ExpectedResponse, resp, protocmp.Transform()); d != "" {
				t.Errorf("batch response mismatch: %s", d)
			}
			envs, err := repo.State().GetEnvironmentConfigs()
			if err != nil {
				t.Errorf("unexpected error: %q", err)
			}
			if d := cmp.Diff(tc.ExpectedEnvironments, envs); d != "" {
				t.Errorf("batch response mismatch: %s", d)
			}
		})
	}
}
