/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
package service

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"os/exec"
	"path"
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

func TestBatchService(t *testing.T) {
	tcs := []struct {
		Name  string
		Setup []repository.Transformer
		Test  func(t *testing.T, svc *BatchServer)
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
				&repository.CreateEnvironmentLock{   // will be deleted
					Environment: "production",
					LockId: "1234",
					Message: "EnvLock",
				},
				&repository.CreateEnvironmentApplicationLock{  // will be deleted
					Environment: "production",
					Application: "test",
					LockId: "5678",
					Message: "AppLock",
				},
			},
			Test: func(t *testing.T, svc *BatchServer) {
				opDeploy := &api.BatchAction_Deploy{
					Deploy: &api.DeployRequest{
						Environment: "production",
						Application: "test",
						Version:     1,
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
				opDeleteEnvLock := &api.BatchAction_DeleteEnvironmentLock{
					DeleteEnvironmentLock: &api.DeleteEnvironmentLockRequest{
						Environment: "production",
						LockId:      "1234",
					},
				}
				opDeleteAppLock := &api.BatchAction_DeleteEnvironmentApplicationLock{
					DeleteEnvironmentApplicationLock: &api.DeleteEnvironmentApplicationLockRequest{
						Environment: "production",
						Application: "test",
						LockId:      "5678",
					},
				}
				ops := []*api.BatchAction {		// it works through the batch in order
					{Action: opDeleteEnvLock},
					{Action: opDeleteAppLock},
					{Action: opDeploy},
					{Action: opCreateEnvLock},
					{Action: opCreateAppLock},
				}
				_, err := svc.ProcessBatch(
					context.Background(),
					&api.BatchRequest{
						Actions: ops,
					},
				)
				if err != nil {
					t.Fatal(err.Error())
				}
				// check deployment version
				{
					version, err := svc.Repository.State().GetEnvironmentApplicationVersion("production", "test")
					if err != nil {
						t.Fatal(err)
					}
					if version == nil || *version != 1 {
						t.Errorf("unexpected version: expected 1, actual: %d", *version)
					}
				}
				// check that the envlock was created
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
				// check that the applock was created
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
			},
		},
		{
			Name: "more than 100 actions",
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
				&repository.CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "manifest2",
					},
				},
			},
			Test: func(t *testing.T, svc *BatchServer) {
				ops := []*api.BatchAction {}
				for i := uint64(1); i <= 100; i++ {	// test exactly 100 first
					ops = append(ops, &api.BatchAction{
						Action: &api.BatchAction_Deploy{
							Deploy: &api.DeployRequest{
								Environment:  "production",
								Application:  "test",
								Version:      2-i%2,  // alternate between 1 and 2
								LockBehavior: api.LockBehavior_Fail,
							},
						},
					})
				}
				_, err := svc.ProcessBatch(
					context.Background(),
					&api.BatchRequest{
						Actions: ops,
					},
				)
				if err != nil {
					t.Fatal(err)
				}
				{
					version, err := svc.Repository.State().GetEnvironmentApplicationVersion("production", "test")
					if err != nil {
						t.Fatal(err)
					}
					if version == nil || *version != 2 {
						t.Errorf("unexpected version: expected 2, actual: %d", *version)
					}
				}
				// add one more action
				ops = append(ops, &api.BatchAction{
					Action: &api.BatchAction_Deploy{
						Deploy: &api.DeployRequest{
							Environment:  "production",
							Application:  "test",
							Version:      1,
							LockBehavior: api.LockBehavior_Fail,
						},
					},
				})
				_, err = svc.ProcessBatch(
					context.Background(),
					&api.BatchRequest{
						Actions: ops,
					},
				)
				if err == nil {
					t.Fatal("expected an error but got none")
				}
				s, ok := status.FromError(err)
				if !ok {
					t.Fatalf("error is not a status error, got: %#v", err)
				}
				if s.Code() != codes.InvalidArgument {
					t.Errorf("invalid error code: expected %q, actual: %q", codes.InvalidArgument.String(), s.Code().String())
				}
				if s.Message() != "too many actions. limit is 100" {
					t.Errorf("invalid error message: expected %q, actual: %q", "too many actions. limit is 100", s.Message())
				}
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			remoteDir := path.Join(dir, "remote")
			localDir := path.Join(dir, "local")
			cmd := exec.Command("git", "init", "--bare", remoteDir)
			cmd.Start()
			cmd.Wait()
			repo, err := repository.NewWait(
				context.Background(),
				repository.Config{
					URL:            remoteDir,
					Path:           localDir,
					CommitterEmail: "kuberpult@freiheit.com",
					CommitterName:  "kuberpult",
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			for _, tr := range tc.Setup {
				if err := repo.Apply(context.Background(), tr); err != nil {
					t.Fatal(err)
				}
			}
			svc := &BatchServer{
				Repository: repo,
			}
			tc.Test(t, svc)
		})
	}
}
