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
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"google.golang.org/grpc/status"
)

func TestDeployService(t *testing.T) {
	tcs := []struct {
		Name  string
		Setup []repository.Transformer
		Test  func(t *testing.T, svc *DeployServiceServer)
	}{
		{
			Name: "Deploying a version",
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
			},
			Test: func(t *testing.T, svc *DeployServiceServer) {
				_, err := svc.Deploy(
					context.Background(),
					&api.DeployRequest{
						Environment: "production",
						Application: "test",
						Version:     1,
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
					if version == nil {
						t.Errorf("unexpected version: expected 1, actual: %d", version)
					}
					if *version != 1 {
						t.Errorf("unexpected version: expected 1, actual: %d", *version)
					}
				}
			},
		},
		{
			Name: "Deploying a version to a locked environment",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "production",
				},
				&repository.CreateEnvironmentLock{
					Environment: "production",
					LockId:      "a",
					Message:     "b",
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "manifest",
					},
				},
				&repository.CreateEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					LockId:      "c",
				},
			},
			Test: func(t *testing.T, svc *DeployServiceServer) {
				_, err := svc.Deploy(
					context.Background(),
					&api.DeployRequest{
						Environment:  "production",
						Application:  "test",
						Version:      1,
						LockBehavior: api.LockBehavior_Fail,
					},
				)
				if err == nil {
					t.Fatal("expected an error but got none")
				}
				stat, ok := status.FromError(err)
				if !ok {
					t.Fatalf("error is not a status error, got: %#v", err)
				}
				details := stat.Details()
				if len(details) == 0 {
					t.Fatalf("error is a status error, but has no details: %s", err.Error())
				}
				lockErr := details[0].(*api.LockedError)
				if _, ok := lockErr.EnvironmentLocks["a"]; !ok {
					t.Errorf("lockErr doesn't contain the environment lock")
				}
				if _, ok := lockErr.EnvironmentApplicationLocks["c"]; !ok {
					t.Errorf("lockErr doesn't contain the application environment lock")
				}
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
				if err := repo.Apply(context.Background(), tr); err != nil {
					t.Fatal(err)
				}
			}
			svc := &DeployServiceServer{
				Repository: repo,
			}
			tc.Test(t, svc)
		})
	}
}
