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
	"testing"
	"time"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestGetFailedEslsService(t *testing.T) {
	tcs := []struct {
		Name             string
		FailedEsls       []*db.EslEventRow
		ExpectedResponse *api.GetFailedEslsResponse
	}{
		{
			Name: "One failed Esl",
			FailedEsls: []*db.EslEventRow{
				{
					EslId:     1,
					EventJson: `{"env":"dev","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`,
					EventType: db.EvtCreateApplicationVersion,
					Created:   time.Now(),
				},
			},
			ExpectedResponse: &api.GetFailedEslsResponse{
				FailedEsls: []*api.EslItem{
					{
						EslId:     1,
						CreatedAt: timestamppb.New(time.Now()),
						EventType: string(db.EvtCreateApplicationVersion),
						Json:      `{"env":"dev","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`,
					},
				},
			},
		},
		{
			Name: "Multiple failed Esls",
			FailedEsls: []*db.EslEventRow{
				{
					EslId:     1,
					EventJson: `{"env":"dev","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`,
					EventType: db.EvtCreateApplicationVersion,
					Created:   time.Now(),
				},
				{
					EslId:     2,
					EventJson: `{"env":"dev2","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`,
					EventType: db.EvtCreateEnvironment,
					Created:   time.Now(),
				},
			},
			ExpectedResponse: &api.GetFailedEslsResponse{
				FailedEsls: []*api.EslItem{
					{
						EslId:     2,
						CreatedAt: timestamppb.New(time.Now()),
						EventType: string(db.EvtCreateEnvironment),
						Json:      `{"env":"dev2","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`,
					},
					{
						EslId:     1,
						CreatedAt: timestamppb.New(time.Now()),
						EventType: string(db.EvtCreateApplicationVersion),
						Json:      `{"env":"dev","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`,
					},
				},
			},
		},
		{
			Name:       "No failed Esls",
			FailedEsls: []*db.EslEventRow{},
			ExpectedResponse: &api.GetFailedEslsResponse{
				FailedEsls: []*api.EslItem{},
			},
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
				t.Fatal(err)
			}
			svc := &EslServiceServer{
				Repository: repo,
			}
			err = repo.State().DBHandler.WithTransaction(testutil.MakeTestContext(), false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, failedEsl := range tc.FailedEsls {
					err := repo.State().DBHandler.DBWriteFailedEslEvent(ctx, transaction, failedEsl)
					if err != nil {
						return err
					}
				}
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			response, err := svc.GetFailedEsls(context.Background(), &api.GetFailedEslsRequest{})
			if err != nil {
				t.Fatal(err)
			}
			opts := cmp.Options{cmpopts.IgnoreFields(api.EslItem{}, "CreatedAt"), cmpopts.IgnoreUnexported(api.GetFailedEslsResponse{}, api.EslItem{}, timestamppb.Timestamp{})}
			if diff := cmp.Diff(tc.ExpectedResponse, response, opts); diff != "" {
				t.Logf("response: %+v", response)
				t.Logf("expected: %+v", tc.ExpectedResponse)
				t.Fatal("Output mismatch (-want +got):\n", diff)
			}
		})
	}
}
