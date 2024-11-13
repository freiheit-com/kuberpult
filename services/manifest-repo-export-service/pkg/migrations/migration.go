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

package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"strconv"
	"strings"
)

func DBReadMigrationCutoff(h *db.DBHandler, ctx context.Context, transaction *sql.Tx, requestedVersion *api.KuberpultVersion) (*api.KuberpultVersion, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBReadMigrationCutoff")
	defer span.Finish()

	requestedVersionString := formatKuberpultVersion(requestedVersion)

	selectQuery := h.AdaptQuery(`
SELECT eslVersion
FROM migration_cutoff
WHERE kuberpultVersion=?
LIMIT 1;`)
	span.SetTag("query", selectQuery)
	span.SetTag("requestedVersion", requestedVersionString)
	rows, err := transaction.QueryContext(
		ctx,
		selectQuery,
		requestedVersionString,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query cutoff table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("migration_cutoff: row closing error: %v", err)
		}
	}(rows)

	if !rows.Next() {
		return nil, nil
	}
	var rawVersion string
	err = rows.Scan(&rawVersion)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("migration_cutoff: Error scanning row from DB. Error: %w\n", err)
	}
	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("migration_cutoff: row closing error: %v\n", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("migration_cutoff: row has error: %v\n", err)
	}

	var kuberpultVersion *api.KuberpultVersion
	kuberpultVersion, err = parseKuberpultVersion(rawVersion)
	if err != nil {
		return nil, fmt.Errorf("migration_cutoff: Error parsing kuberpult version. Error: %w", err)
	}
	return kuberpultVersion, nil
}

func parseKuberpultVersion(version string) (*api.KuberpultVersion, error) {
	version = strings.TrimPrefix(version, "v")
	split := strings.Split(version, ".")
	if len(split) != 3 {
		return nil, fmt.Errorf("migration_cutoff: Error parsing kuberpult version '%s', must have 3 dots", version)
	}
	majorRaw := split[0]
	minorRaw := split[1]
	patchRaw := split[2]

	ma, err := strconv.ParseUint(majorRaw, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("migration_cutoff: Error parsing kuberpult major version'%s'. Error: %w", majorRaw, err)
	}
	mi, err := strconv.ParseUint(minorRaw, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("migration_cutoff: Error parsing kuberpult major version'%s'. Error: %w", majorRaw, err)
	}
	pa, err := strconv.ParseUint(patchRaw, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("migration_cutoff: Error parsing kuberpult major version'%s'. Error: %w", majorRaw, err)
	}
	return &api.KuberpultVersion{
		Major: int32(ma),
		Minor: int32(mi),
		Patch: int32(pa),
	}, nil
}

func formatKuberpultVersion(version *api.KuberpultVersion) string {
	return fmt.Sprintf("%d.%d.%d", version.Major, version.Minor, version.Patch)
}

//func DBWriteCutoff(h *DBHandler, ctx context.Context, tx *sql.Tx, eslVersion EslVersion) error {
//	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteCutoff")
//	defer span.Finish()
//
//	insertQuery := h.AdaptQuery("INSERT INTO cutoff (eslVersion, processedTime) VALUES (?, ?);")
//	span.SetTag("query", insertQuery)
//
//	_, err := tx.Exec(
//		insertQuery,
//		eslVersion,
//		time.Now().UTC(),
//	)
//	if err != nil {
//		return fmt.Errorf("could not write to cutoff table from DB. Error: %w\n", err)
//	}
//	return nil
//}
