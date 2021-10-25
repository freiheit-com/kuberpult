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
	"os/exec"
	"path"
	"testing"

	"github.com/freiheit-com/fdc-continuous-delivery/pkg/api"
	"github.com/freiheit-com/fdc-continuous-delivery/services/cd-service/pkg/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLockingService(t *testing.T) {
	tcs := []struct {
		Name  string
		Setup []repository.Transformer
		Test  func(t *testing.T, svc *LockServiceServer)
	}{
		{
			Name: "Locking an environment",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "production",
				},
			},
			Test: func(t *testing.T, svc *LockServiceServer) {
				_, err := svc.CreateEnvironmentLock(
					context.Background(),
					&api.CreateEnvironmentLockRequest{
						Environment: "production",
						LockId:      "manual",
						Message:     "please",
					},
				)
				if err != nil {
					t.Fatal(err)
				}
				// check that the lock was created
				{
					envLocks, err := svc.Repository.State().GetEnvironmentLocks("production")
					if err != nil {
						t.Fatal(err)
					}
					lock, exists := envLocks["manual"]
					if !exists {
						t.Error("lock was not created")
					}
					if lock.Message != "please" {
						t.Errorf("unexpected lock message: expected \"please\", actual: %q", lock.Message)
					}
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
			svc := &LockServiceServer{
				Repository: repo,
			}
			tc.Test(t, svc)
		})
	}
}

func TestInvalidCreateEnvironmentLockArguments(t *testing.T) {
	tcs := []struct {
		Name            string
		Request         *api.CreateEnvironmentLockRequest
		ExpectedCode    codes.Code
		ExpectedMessage string
		Test            func(t *testing.T, err *status.Status)
	}{
		{
			Name: "empty environment",
			Request: &api.CreateEnvironmentLockRequest{
				Environment: "",
				LockId:      "lock",
			},
			ExpectedCode:    codes.InvalidArgument,
			ExpectedMessage: "invalid environment",
		},
		{
			Name: "bad environment",
			Request: &api.CreateEnvironmentLockRequest{
				Environment: "nöt äctually",
				LockId:      "lock",
			},
			ExpectedCode:    codes.InvalidArgument,
			ExpectedMessage: "invalid environment",
		},
		{
			Name: "empty lock id",
			Request: &api.CreateEnvironmentLockRequest{
				Environment: "production",
				LockId:      "",
			},
			ExpectedCode:    codes.InvalidArgument,
			ExpectedMessage: "invalid lock id",
		},
		{
			Name: "bad lock id",
			Request: &api.CreateEnvironmentLockRequest{
				Environment: "production",
				LockId:      "../",
			},
			ExpectedCode:    codes.InvalidArgument,
			ExpectedMessage: "invalid lock id",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			svc := &LockServiceServer{}
			_, err := svc.CreateEnvironmentLock(
				context.Background(),
				tc.Request,
			)
			if err == nil {
				t.Fatal("expected an error, but got none")
			}
			if s, ok := status.FromError(err); !ok {
				t.Fatalf("expected a status error, but got %#v", err)
			} else {
				if s.Code() != tc.ExpectedCode {
					t.Errorf("invalid error code: expected %q, actual: %q", tc.ExpectedCode.String(), s.Code().String())
				}
				if s.Message() != tc.ExpectedMessage {
					t.Errorf("invalid error message: expected %q, actual: %q", tc.ExpectedMessage, s.Message())
				}
			}
		})
	}
}

func TestInvalidCreateEnvironmentApplicationLockArguments(t *testing.T) {
	tcs := []struct {
		Name            string
		Request         *api.CreateEnvironmentApplicationLockRequest
		ExpectedCode    codes.Code
		ExpectedMessage string
		Test            func(t *testing.T, err *status.Status)
	}{
		{
			Name: "empty environment",
			Request: &api.CreateEnvironmentApplicationLockRequest{
				Environment: "",
				Application: "app",
				LockId:      "lock",
			},
			ExpectedCode:    codes.InvalidArgument,
			ExpectedMessage: "invalid environment",
		},
		{
			Name: "bad environment",
			Request: &api.CreateEnvironmentApplicationLockRequest{
				Environment: "nöt äctually",
				Application: "app",
				LockId:      "lock",
			},
			ExpectedCode:    codes.InvalidArgument,
			ExpectedMessage: "invalid environment",
		},
		{
			Name: "empty lock id",
			Request: &api.CreateEnvironmentApplicationLockRequest{
				Environment: "production",
				Application: "app",
				LockId:      "",
			},
			ExpectedCode:    codes.InvalidArgument,
			ExpectedMessage: "invalid lock id",
		},
		{
			Name: "bad lock id",
			Request: &api.CreateEnvironmentApplicationLockRequest{
				Environment: "production",
				Application: "app",
				LockId:      "../",
			},
			ExpectedCode:    codes.InvalidArgument,
			ExpectedMessage: "invalid lock id",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			svc := &LockServiceServer{}
			_, err := svc.CreateEnvironmentApplicationLock(
				context.Background(),
				tc.Request,
			)
			if err == nil {
				t.Fatal("expected an error, but got none")
			}
			if s, ok := status.FromError(err); !ok {
				t.Fatalf("expected a status error, but got %#v", err)
			} else {
				if s.Code() != tc.ExpectedCode {
					t.Errorf("invalid error code: expected %q, actual: %q", tc.ExpectedCode.String(), s.Code().String())
				}
				if s.Message() != tc.ExpectedMessage {
					t.Errorf("invalid error message: expected %q, actual: %q", tc.ExpectedMessage, s.Message())
				}
			}
		})
	}
}
