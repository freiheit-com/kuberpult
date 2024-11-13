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
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/migrations"
)

type MigrationServer struct {
	DBHandler *db.DBHandler
}

func (s *MigrationServer) EnsureCustomMigrationApplied(ctx context.Context, in *api.EnsureCustomMigrationAppliedRequest) (*api.EnsureCustomMigrationAppliedResponse, error) {
	log := logger.FromContext(ctx).Sugar()
	log.Warn("EnsureCustomMigrationApplied start")

	// 1) Check if migrations are done:
	dbDone, err := s.CustomMigrationsDone(ctx, in.Version)
	if err != nil {
		return nil, fmt.Errorf("could not check if migrations are done: %w", err)
	}
	if dbDone {
		log.Warn("EnsureCustomMigrationApplied end 1 nothing to do")
		return &api.EnsureCustomMigrationAppliedResponse{
			MigrationsApplied: true,
		}, nil
	}

	err = s.RunMigrations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Warn("EnsureCustomMigrationApplied end 2")
	return &api.EnsureCustomMigrationAppliedResponse{
		MigrationsApplied: false,
	}, nil
}

func (s *MigrationServer) CustomMigrationsDone(ctx context.Context, version *api.KuberpultVersion) (bool, error) {
	type Done struct {
		done bool
	}
	dbVersion, err := db.WithTransactionT(s.DBHandler, ctx, 0, true, func(ctx context.Context, transaction *sql.Tx) (*api.KuberpultVersion, error) {
		dbVersion, tErr := migrations.DBReadMigrationCutoff(s.DBHandler, ctx, transaction, version)
		if tErr != nil {
			return nil, tErr
		}
		return dbVersion, nil
	})
	if err != nil {
		return false, fmt.Errorf("could not check if migrations are done: %w", err)
	}
	if dbVersion == version {
		return true, nil
	}
	log := logger.FromContext(ctx).Sugar()
	log.Warnf("CustomMigrationsDone diff: %s!=%s", dbVersion, version) // TODO SU: this should be INFO or not logged
	return false, nil
}

func (s *MigrationServer) RunMigrations(ctx context.Context) error {
	// TODO IMPLEMENT
	return nil
}
