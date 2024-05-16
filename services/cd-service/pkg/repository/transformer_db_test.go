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

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/freiheit-com/kuberpult/pkg/ptr"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository/testutil"
	"google.golang.org/protobuf/testing/protocmp"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// CreateMigrationsPath detects if it's running withing earthly/CI or locally and adapts the path to the migrations accordingly
func CreateMigrationsPath() (string, error) {
	const subDir = "/database/migrations"
	_, err := os.Stat("/kp")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			wd, err := os.Getwd()
			if err != nil {
				return "", err
			}
			// this ".." sequence is necessary, because Getwd() returns the path of this go file (when running in an idea like goland):
			return wd + "/../../../.." + subDir, nil
		}
		return "", err
	}
	return "/kp" + subDir, nil
}

func TestTransformerWritesEslDataRoundTrip(t *testing.T) {
	setupTransformers := []Transformer{
		&CreateEnvironment{
			Authentication: Authentication{},
			Environment:    "dev",
			Config: config.EnvironmentConfig{
				Upstream:         nil,
				ArgoCd:           nil,
				EnvironmentGroup: ptr.FromString("mygroup"),
			},
		},
		&CreateEnvironment{
			Authentication: Authentication{},
			Environment:    "staging",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Environment: "dev",
					Latest:      false,
				},
				ArgoCd:           nil,
				EnvironmentGroup: ptr.FromString("staging-group"),
			},
		},
		&CreateApplicationVersion{
			Authentication: Authentication{},
			Version:        666,
			Application:    "myapp",
			Manifests: map[string]string{
				"dev": "dev manifest",
			},
			SourceCommitId:  "",
			SourceAuthor:    "",
			SourceMessage:   "",
			SourceRepoUrl:   "",
			Team:            "myteam",
			DisplayVersion:  "",
			WriteCommitData: false,
			PreviousCommit:  "",
			NextCommit:      "",
		},
		&CreateUndeployApplicationVersion{
			Authentication:  Authentication{},
			Application:     "myapp",
			WriteCommitData: false,
		},
		&CreateEnvironmentLock{
			Authentication: Authentication{},
			Environment:    "dev",
			LockId:         "setup-lock-1",
			Message:        "msg321",
		},
		&CreateEnvironmentTeamLock{
			Authentication: Authentication{},
			Environment:    "dev",
			LockId:         "setup-lock-2",
			Message:        "msg321",
			Team:           "myteam",
		},
		&CreateEnvironmentGroupLock{
			Authentication:   Authentication{},
			LockId:           "setup-lock-3",
			Message:          "msg321",
			EnvironmentGroup: "mygroup",
		},
		&CreateEnvironmentApplicationLock{
			Authentication: Authentication{},
			Environment:    "dev",
			Application:    "myapp",
			LockId:         "setup-lock-4",
			Message:        "msg321",
		},
	}
	tcs := []struct {
		Name              string
		Transformers      Transformer
		expectedEventJson string
		dataType          interface{}
	}{

		// each transformer should appear here once:
		{
			Name: "CreateApplicationVersion",
			Transformers: &CreateApplicationVersion{
				Authentication:  Authentication{},
				Version:         0,
				Application:     "dummy",
				Manifests:       nil,
				SourceCommitId:  "",
				SourceAuthor:    "",
				SourceMessage:   "",
				SourceRepoUrl:   "",
				Team:            "dummyteam",
				DisplayVersion:  "",
				WriteCommitData: false,
				PreviousCommit:  "",
				NextCommit:      "",
			},
			dataType: &CreateApplicationVersion{},
		},
		{
			Name: "DeployApplicationVersion",
			Transformers: &DeployApplicationVersion{
				Authentication:  Authentication{},
				Environment:     "dev",
				Application:     "myapp",
				Version:         666,
				LockBehaviour:   0,
				WriteCommitData: false,
				SourceTrain:     nil,
				Author:          "",
			},
			dataType: &DeployApplicationVersion{},
		},
		{
			Name: "CreateUndeployApplicationVersion",
			Transformers: &CreateUndeployApplicationVersion{
				Authentication:  Authentication{},
				Application:     "myapp",
				WriteCommitData: false,
			},
			dataType: &CreateUndeployApplicationVersion{},
		},
		{
			Name: "UndeployApplication",
			Transformers: &UndeployApplication{
				Authentication: Authentication{},
				Application:    "myapp",
			},
			dataType: &UndeployApplication{},
		},
		{
			Name: "DeleteEnvFromApp",
			Transformers: &DeleteEnvFromApp{
				Authentication: Authentication{},
				Application:    "myapp",
				Environment:    "dev",
			},
			dataType: &DeleteEnvFromApp{},
		},
		{
			Name: "CreateEnvironmentLock",
			Transformers: &CreateEnvironmentLock{
				Authentication: Authentication{},
				Environment:    "dev",
				LockId:         "lock123",
				Message:        "msg321",
			},
			dataType: &CreateEnvironmentLock{},
		},
		{
			Name: "DeleteEnvironmentLock",
			Transformers: &DeleteEnvironmentLock{
				Authentication: Authentication{},
				Environment:    "dev",
				LockId:         "setup-lock-1",
			},
			dataType: &DeleteEnvironmentLock{},
		},
		{
			Name: "CreateEnvironmentTeamLock",
			Transformers: &CreateEnvironmentTeamLock{
				Authentication: Authentication{},
				Environment:    "dev",
				LockId:         "dontcare",
				Message:        "msg321",
				Team:           "myteam",
			},
			dataType: &CreateEnvironmentTeamLock{},
		},
		{
			Name: "DeleteEnvironmentTeamLock",
			Transformers: &DeleteEnvironmentTeamLock{
				Authentication: Authentication{},
				Environment:    "dev",
				LockId:         "setup-lock-2",
				Team:           "myteam",
			},
			dataType: &DeleteEnvironmentTeamLock{},
		},
		{
			Name: "CreateEnvironmentGroupLock",
			Transformers: &CreateEnvironmentGroupLock{
				Authentication:   Authentication{},
				EnvironmentGroup: "mygroup",
				LockId:           "lock123",
				Message:          "msg321",
			},
			dataType: &CreateEnvironmentGroupLock{},
		},
		{
			Name: "DeleteEnvironmentGroupLock",
			Transformers: &DeleteEnvironmentGroupLock{
				Authentication:   Authentication{},
				LockId:           "setup-lock-3",
				EnvironmentGroup: "mygroup",
			},
			dataType: &DeleteEnvironmentGroupLock{},
		},
		{
			Name: "CreateEnvironment",
			Transformers: &CreateEnvironment{
				Authentication: Authentication{},
				Environment:    "temp-env",
				Config: config.EnvironmentConfig{
					Upstream:         nil,
					ArgoCd:           nil,
					EnvironmentGroup: nil,
				},
			},
			dataType: &CreateEnvironment{},
		},
		{
			Name: "CreateEnvironmentApplicationLock",
			Transformers: &CreateEnvironmentApplicationLock{
				Authentication: Authentication{},
				Environment:    "dev",
				LockId:         "lock123",
				Message:        "msg321",
				Application:    "myapp",
			},
			dataType: &CreateEnvironmentApplicationLock{},
		},
		{
			Name: "DeleteEnvironmentApplicationLock",
			Transformers: &DeleteEnvironmentApplicationLock{
				Authentication: Authentication{},
				Environment:    "dev",
				LockId:         "setup-lock-4",
				Application:    "myapp",
			},
			dataType: &DeleteEnvironmentApplicationLock{},
		},
		{
			Name: "ReleaseTrain",
			Transformers: &ReleaseTrain{
				Authentication:  Authentication{},
				Target:          "staging",
				Team:            "",
				CommitHash:      "",
				WriteCommitData: false,
				Repo:            nil,
			},
			dataType: &ReleaseTrain{},
		},
	}

	dir, err := CreateMigrationsPath()
	if err != nil {
		t.Fatalf("setup error could not detect dir \n%v", err)
		return
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Logf("detected dir: %s - err=%v", dir, err)
			t.Parallel()
			ctx := testutil.MakeTestContext()
			cfg := DBConfig{
				MigrationsPath: dir,
				DriverName:     "sqlite3",
			}
			repo, err := setupRepositoryTestWithDB(t, &cfg)
			if err != nil {
				t.Errorf("setup error\n%v", err)
			}
			r := repo.(*repository)
			row := &EslEventRow{}
			err = repo.Apply(ctx, setupTransformers...)
			if err != nil {
				t.Errorf("setup error could not set up transformers \n%v", err)
			}

			err = r.DB.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err2 := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers)
				if err2 != nil {
					return err2
				}
				tmp, batchErr := r.DB.DBReadEslEventInternal(ctx, transaction)
				if batchErr != nil {
					return batchErr
				}
				if tmp == nil && batchErr == nil {
					return errors.New("expected at least one row, but got 0")
				}
				row = tmp
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
			var jsonInterface interface{} = tc.dataType
			err = json.Unmarshal(([]byte)(row.EventJson), &jsonInterface)
			if err != nil {
				t.Fatalf("marshal error: %v\njson: \n%s\n", err, row.EventJson)
			}

			if diff := cmp.Diff(tc.Transformers, jsonInterface, protocmp.Transform()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
