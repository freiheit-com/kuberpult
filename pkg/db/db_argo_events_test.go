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

package db

import (
	"context"
	"database/sql"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"testing"
)

func TestDBArgoEvent(t *testing.T) {
	const environment = "env"
	const app = "app"
	const anotherApp = "anotherApp"
	tcs := []struct {
		Name           string
		Input          []*ArgoEvent
		ExpectedEvents []*ArgoEvent
	}{
		{
			Name: "Write and Read",
			Input: []*ArgoEvent{
				{
					Env:       environment,
					App:       app,
					JsonEvent: []byte("{}"),
					Discarded: false,
				},
			},
			ExpectedEvents: []*ArgoEvent{
				{
					Env:       environment,
					App:       app,
					JsonEvent: []byte("{}"),
					Discarded: false,
				},
			},
		},
		{
			Name: "Two writes same pair last persists",
			Input: []*ArgoEvent{
				{
					Env:       environment,
					App:       app,
					JsonEvent: []byte("{FirstEvent}"),
					Discarded: false,
				},
				{
					Env:       environment,
					App:       app,
					JsonEvent: []byte("{SecondEvent}"),
					Discarded: false,
				},
			},
			ExpectedEvents: []*ArgoEvent{
				{
					Env:       environment,
					App:       app,
					JsonEvent: []byte("{SecondEvent}"),
					Discarded: false,
				},
			},
		},
		{
			Name: "Multiple Writes to different pairs of env and app",
			Input: []*ArgoEvent{
				{
					Env:       environment,
					App:       app,
					JsonEvent: []byte("{FirstEvent}"),
					Discarded: false,
				},
				{
					Env:       environment,
					App:       anotherApp,
					JsonEvent: []byte("{SecondEvent}"),
					Discarded: false,
				},
			},
			ExpectedEvents: []*ArgoEvent{
				{
					Env:       environment,
					App:       app,
					JsonEvent: []byte("{FirstEvent}"),
					Discarded: false,
				},
				{
					Env:       environment,
					App:       anotherApp,
					JsonEvent: []byte("{SecondEvent}"),
					Discarded: false,
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			errW := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.InsertArgoEvents(ctx, transaction, tc.Input)
				if err != nil {
					return err
				}
				return nil
			})
			if errW != nil {
				t.Fatalf("transaction error: %v", errW)
			}
			errR := dbHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
				for _, curr := range tc.ExpectedEvents {
					currActualEvent, err := dbHandler.DBReadArgoEvent(ctx, transaction, curr.App, curr.Env)
					if err != nil {
						return err
					}
					if diff := cmp.Diff(curr, currActualEvent); diff != "" {
						t.Fatalf("argo event mismatch (-want, +got):\n%s", diff)
					}
				}
				return nil
			})
			if errR != nil {
				t.Fatalf("transaction error: %v", errR)
			}
		})
	}
}
