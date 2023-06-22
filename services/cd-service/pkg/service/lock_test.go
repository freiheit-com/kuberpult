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
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/logger/testlogger"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository/testrepository"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLockingService(t *testing.T) {
	tcs := []struct {
		Name  string
		Setup []repository.Transformer
		Test  func(t *testing.T, ctx context.Context, svc *LockServiceServer)
	}{
		{
			Name: "Locking an environment",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "production",
				},
			},
			Test: func(t *testing.T, ctx context.Context, svc *LockServiceServer) {
				_, err := svc.CreateEnvironmentLock(
					ctx,
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
			logs := testlogger.Wrap(context.Background(), func(ctx context.Context) {
				repo, err := setupRepositoryTest(t)
				if err != nil {
					t.Fatal(err)
				}
				for _, tr := range tc.Setup {
					if err := repo.Apply(ctx, tr); err != nil {
						t.Fatal(err)
					}
				}
				svc := &LockServiceServer{
					Repository: repo,
				}
				tc.Test(t, ctx, svc)
			})
			testlogger.AssertEmpty(t, logs)
		})
	}
}

func TestInvalidCreateEnvironmentLockArguments(t *testing.T) {
	tcs := []struct {
		Name            string
		Request         *api.CreateEnvironmentLockRequest
		ExpectedCode    codes.Code
		ExpectedMessage string
	}{
		{
			Name: "empty environment",
			Request: &api.CreateEnvironmentLockRequest{
				Environment: "",
				LockId:      "lock",
			},
			ExpectedCode:    codes.InvalidArgument,
			ExpectedMessage: "error: cannot create environment lock: invalid environment: ''",
		},
		{
			Name: "bad environment",
			Request: &api.CreateEnvironmentLockRequest{
				Environment: "nöt äctually",
				LockId:      "lock",
			},
			ExpectedCode:    codes.InvalidArgument,
			ExpectedMessage: "error: cannot create environment lock: invalid environment: 'nöt äctually'",
		},
		{
			Name: "empty lock id",
			Request: &api.CreateEnvironmentLockRequest{
				Environment: "production",
				LockId:      "",
			},
			ExpectedCode:    codes.InvalidArgument,
			ExpectedMessage: "error: cannot create environment lock: invalid lock id: ''",
		},
		{
			Name: "bad lock id",
			Request: &api.CreateEnvironmentLockRequest{
				Environment: "production",
				LockId:      "../",
			},
			ExpectedCode:    codes.InvalidArgument,
			ExpectedMessage: "error: cannot create environment lock: invalid lock id: '../'",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			//t.Parallel()
			logs := testlogger.Wrap(context.Background(), func(ctx context.Context) {
				repo, err := setupRepositoryTest(t)
				if err != nil {
					t.Fatal(err)
				}
				svc := &LockServiceServer{
					Repository: repo,
				}
				_, err = svc.CreateEnvironmentLock(
					ctx,
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
			testlogger.AssertEmpty(t, logs)
		})
	}
}

func TestInvalidCreateEnvironmentApplicationLockArguments(t *testing.T) {
	tcs := []struct {
		Name            string
		Request         *api.CreateEnvironmentApplicationLockRequest
		ExpectedCode    codes.Code
		ExpectedMessage string
	}{
		{
			Name: "empty environment",
			Request: &api.CreateEnvironmentApplicationLockRequest{
				Environment: "",
				Application: "app",
				LockId:      "lock",
			},
			ExpectedCode:    codes.InvalidArgument,
			ExpectedMessage: "error: cannot create environment application lock: invalid environment: ''",
		},
		{
			Name: "bad environment",
			Request: &api.CreateEnvironmentApplicationLockRequest{
				Environment: "nöt äctually",
				Application: "app",
				LockId:      "lock",
			},
			ExpectedCode:    codes.InvalidArgument,
			ExpectedMessage: "error: cannot create environment application lock: invalid environment: 'nöt äctually'",
		},
		{
			Name: "empty lock id",
			Request: &api.CreateEnvironmentApplicationLockRequest{
				Environment: "production",
				Application: "app",
				LockId:      "",
			},
			ExpectedCode:    codes.InvalidArgument,
			ExpectedMessage: "error: cannot create environment application lock: invalid lock id: ''",
		},
		{
			Name: "bad lock id",
			Request: &api.CreateEnvironmentApplicationLockRequest{
				Environment: "production",
				Application: "app",
				LockId:      "../",
			},
			ExpectedCode:    codes.InvalidArgument,
			ExpectedMessage: "error: cannot create environment application lock: invalid lock id: '../'",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			logs := testlogger.Wrap(context.Background(), func(ctx context.Context) {
				repo, err := setupRepositoryTest(t)
				if err != nil {
					t.Fatal(err)
				}
				svc := &LockServiceServer{
					Repository: repo,
				}
				_, err = svc.CreateEnvironmentApplicationLock(
					ctx,
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
			testlogger.AssertEmpty(t, logs)
		})
	}
}

func TestErrorLock(t *testing.T) {
	testerror := fmt.Errorf("testerror")

	tcs := []struct {
		Name          string
		Request       *api.CreateEnvironmentApplicationLockRequest
		LogAssertions []testlogger.LogAssertion
	}{
		{
			Name: "create an environment lock",
			Request: &api.CreateEnvironmentApplicationLockRequest{
				Environment: "bar",
				Application: "app",
				LockId:      "lock",
			},
			LogAssertions: []testlogger.LogAssertion{
				func(t *testing.T, entry observer.LoggedEntry) {
					if entry.Level != zapcore.ErrorLevel {
						t.Errorf("expected log level %s, but got %s", zapcore.ErrorLevel, entry.Level)
					}
					cxmap := entry.ContextMap()
					if err := cxmap["error"]; err == nil {
						t.Errorf("expected error field to be set")
					} else if err != testerror.Error() {
						t.Errorf("expected error field to be %q, but got %q", testerror.Error(), err)
					}
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			logs := testlogger.Wrap(context.Background(), func(ctx context.Context) {
				svc := &LockServiceServer{
					Repository: testrepository.Failing(testerror),
				}
				svc.CreateEnvironmentApplicationLock(
					ctx,
					tc.Request,
				)
			})
			testlogger.AssertLogs(t,
				logs,
				tc.LogAssertions...,
			)
		})
	}
}
