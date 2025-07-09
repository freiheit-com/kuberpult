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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"slices"
	"strings"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/tracing"
)

type DBReleaseMetaData struct {
	SourceAuthor    string
	SourceCommitId  string
	SourceMessage   string
	DisplayVersion  string
	UndeployVersion bool
	IsMinor         bool
	CiLink          string
	IsPrepublish    bool
}

type DBReleaseManifests struct {
	Manifests map[types.EnvName]string
}

type DBReleaseWithMetaData struct {
	ReleaseNumbers types.ReleaseNumbers
	Created        time.Time
	App            string
	Manifests      DBReleaseManifests
	Metadata       DBReleaseMetaData
	Environments   []types.EnvName
}

// SELECTS

func (h *DBHandler) DBSelectAnyRelease(ctx context.Context, tx *sql.Tx, ignorePrepublishes bool) (*DBReleaseWithMetaData, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAnyRelease")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT created, appName, metadata, releaseVersion, environments, revision
		FROM releases 
		LIMIT 1;
	`)
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(ctx, selectQuery)
	return h.processReleaseRow(ctx, err, rows, ignorePrepublishes, false)
}

func (h *DBHandler) DBSelectReleasesWithoutEnvironments(ctx context.Context, tx *sql.Tx) ([]*DBReleaseWithMetaData, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectReleasesWithoutEnvironments")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT created, appName, metadata, manifests, releaseVersion, environments, revision
		FROM releases
		WHERE COALESCE(environments, '') = '' AND COALESCE(manifests, '') != ''
		LIMIT 100;
	`)
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(ctx, selectQuery)
	return h.processReleaseRows(ctx, err, rows, true, true)
}

func (h *DBHandler) DBSelectReleasesByVersions(ctx context.Context, tx *sql.Tx, app string, releaseVersions []uint64, ignorePrepublishes bool) ([]*DBReleaseWithMetaData, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectReleasesByVersions")
	defer span.Finish()
	if len(releaseVersions) == 0 {
		return []*DBReleaseWithMetaData{}, nil
	}
	repeatedQuestionMarks := strings.Repeat(",?", len(releaseVersions)-1)
	selectQuery := h.AdaptQuery(`
		SELECT created, appName, metadata, releaseVersion, environments, revision FROM releases
		WHERE appname=? AND releaseversion IN (?` + repeatedQuestionMarks + `)
		LIMIT ?
	`)
	span.SetTag("query", selectQuery)

	args := []any{}
	args = append(args, app)
	for _, version := range releaseVersions {
		args = append(args, version)
	}
	args = append(args, uint64(len(releaseVersions)))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		args...,
	)
	return h.processReleaseRows(ctx, err, rows, ignorePrepublishes, false)
}

func (h *DBHandler) DBSelectReleasesByVersionsAndRevision(ctx context.Context, tx *sql.Tx, app string, releaseVersions []uint64, ignorePrepublishes bool) ([]*DBReleaseWithMetaData, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectReleasesByVersions")
	defer span.Finish()
	if len(releaseVersions) == 0 {
		return []*DBReleaseWithMetaData{}, nil
	}
	repeatedQuestionMarks := strings.Repeat(",?", len(releaseVersions)-1)
	selectQuery := h.AdaptQuery(`
		SELECT created, appName, metadata, releaseVersion, environments, revision FROM releases
		WHERE appname=? AND releaseversion IN (?` + repeatedQuestionMarks + `) ORDER BY releaseVersion DESC, revision DESC; 
	`)
	span.SetTag("query", selectQuery)

	args := []any{}
	args = append(args, app)
	for _, version := range releaseVersions {
		args = append(args, version)
	}
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		args...,
	)
	return h.processReleaseRows(ctx, err, rows, ignorePrepublishes, false)
}

func (h *DBHandler) DBSelectReleaseByVersion(ctx context.Context, tx *sql.Tx, app string, version types.ReleaseNumbers, ignorePrepublishes bool) (*DBReleaseWithMetaData, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectReleaseByVersion")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT created, appName, metadata, manifests, releaseVersion, environments, revision
		FROM releases  
		WHERE appName=? AND releaseVersion=?  AND revision = ?
		LIMIT 1;
	`)
	span.SetTag("query", selectQuery)
	span.SetTag("app", app)
	span.SetTag("releaseVersion", version.Version)
	span.SetTag("revision", version.Revision)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		app,
		version.Version,
		version.Revision,
	)
	return h.processReleaseRow(ctx, err, rows, ignorePrepublishes, true)
}

func (h *DBHandler) DBSelectReleaseByVersionAtTimestamp(ctx context.Context, tx *sql.Tx, app string, releaseVersion uint64, ignorePrepublishes bool, ts time.Time) (*DBReleaseWithMetaData, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectReleaseByVersionAtTimestamp")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT created, appName, metadata, manifests, releaseVersion, environments, revision
		FROM releases_history
		WHERE appName=? AND releaseVersion=? AND created <= (?)
		ORDER BY version DESC
		LIMIT 1;
	`)
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		app,
		releaseVersion,
		ts,
	)
	return h.processReleaseRow(ctx, err, rows, ignorePrepublishes, true)
}

type AppVersionEnvironments map[string]map[uint64][]types.EnvName // first key is the appName

func (h *DBHandler) DBSelectAllEnvironmentsForAllReleases(ctx context.Context, tx *sql.Tx) (AppVersionEnvironments, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllManifestsForAllReleases")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT appname, releaseVersion, environments
		FROM releases;
	`)
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)

	return h.processReleaseEnvironmentRows(ctx, err, rows)
}

func (h *DBHandler) DBSelectReleasesByAppLatestEslVersion(ctx context.Context, tx *sql.Tx, app string, ignorePrepublishes bool) ([]*DBReleaseWithMetaData, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectReleasesByAppLatestEslVersion")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT created, appName, metadata, manifests, releaseVersion, environments, revision
		FROM releases
		WHERE appname=?
		ORDER BY releaseversion DESC;
	`)
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		app,
	)

	return h.processReleaseRows(ctx, err, rows, ignorePrepublishes, true)
}

func (h *DBHandler) DBSelectLatestReleaseOfApp(ctx context.Context, tx *sql.Tx, app string, ignorePrepublishes bool) (*DBReleaseWithMetaData, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectLatestReleaseOfApp")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT created, appName, metadata, releaseVersion, environments, revision
		FROM releases
		WHERE appName=?
		ORDER BY releaseVersion DESC
		LIMIT 1;
	`)
	span.SetTag("query", selectQuery)
	span.SetTag("appName", app)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		app,
	)

	return h.processReleaseRow(ctx, err, rows, ignorePrepublishes, false)
}

func (h *DBHandler) DBSelectAllReleasesOfApp(ctx context.Context, tx *sql.Tx, app string) ([]types.ReleaseNumbers, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllReleasesOfApp")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT releaseVersion, revision
		FROM releases
		WHERE appName=?
		ORDER BY releaseVersion DESC, revision DESC;
	`)
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		app,
	)
	return h.processAppReleaseVersionsRows(ctx, err, rows)
}

func (h *DBHandler) DBSelectAllReleasesOfAllApps(ctx context.Context, tx *sql.Tx) (map[string][]int64, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllReleasesOfAllApps")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT appname, releaseVersion 
		FROM releases
		ORDER BY releaseVersion;
	`)
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	return h.processAllAppsReleaseVersionsRows(ctx, err, rows)
}

// INSERT, UPDATE, DELETES

func (h *DBHandler) DBUpdateOrCreateRelease(ctx context.Context, transaction *sql.Tx, release DBReleaseWithMetaData) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBUpdateOrCreateRelease")
	defer span.Finish()
	err := h.upsertReleaseRow(ctx, transaction, release)
	if err != nil {
		return err
	}
	err = h.insertReleaseHistoryRow(ctx, transaction, release, false)
	if err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) DBDeleteFromReleases(ctx context.Context, transaction *sql.Tx, application string, releaseToDelete types.ReleaseNumbers) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBDeleteFromReleases")
	defer span.Finish()

	targetRelease, err := h.DBSelectReleaseByVersion(ctx, transaction, application, releaseToDelete, true)
	if err != nil {
		return err
	}
	if targetRelease == nil {
		return nil
	}
	err = h.deleteReleaseRow(ctx, transaction, *targetRelease)
	if err != nil {
		return err
	}
	err = h.insertReleaseHistoryRow(ctx, transaction, *targetRelease, true)
	if err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) DBClearReleases(ctx context.Context, transaction *sql.Tx, application string) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBClearReleases")
	defer span.Finish()

	allReleases, err := h.DBSelectAllReleasesOfApp(ctx, transaction, application)
	if err != nil {
		return err
	}
	if allReleases == nil {
		logger.FromContext(ctx).Sugar().Infof("App %s does not contain any releases. No action taken", application)
		return nil
	}
	for _, releaseToDelete := range allReleases {
		err = h.DBDeleteFromReleases(ctx, transaction, application, uint64(releaseToDelete))
		if err != nil {
			return err
		}
	}

	return nil
}

// actual changes in tables

func (h *DBHandler) deleteReleaseRow(ctx context.Context, transaction *sql.Tx, release DBReleaseWithMetaData) error {
	span, _ := tracer.StartSpanFromContext(ctx, "deleteReleaseRow")
	defer span.Finish()
	deleteQuery := h.AdaptQuery(`
		DELETE FROM releases WHERE appname=? AND releaseversion=?
	`)
	span.SetTag("query", deleteQuery)
	_, err := transaction.Exec(
		deleteQuery,
		release.App,
		*release.ReleaseNumbers.Version,
	)
	if err != nil {
		return fmt.Errorf(
			"could not delete release for app '%s' and version '%v' from DB. Error: %w\n",
			release.App,
			*release.ReleaseNumbers.Version,
			err)
	}
	return nil
}

func (h *DBHandler) upsertReleaseRow(ctx context.Context, transaction *sql.Tx, release DBReleaseWithMetaData) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "upsertReleaseRow")
	defer span.Finish()
	upsertQuery := h.AdaptQuery(`
		INSERT INTO releases (created, releaseVersion, appName, manifests, metadata, environments, revision)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(releaseVersion, appname, revision)
		DO UPDATE SET created = excluded.created, manifests = excluded.manifests, metadata = excluded.metadata, environments = excluded.environments;
	`)
	span.SetTag("query", upsertQuery)
	metadataJson, err := json.Marshal(release.Metadata)
	if err != nil {
		return fmt.Errorf("upsert release: could not marshal json data: %w", err)
	}
	manifestJson, err := json.Marshal(release.Manifests)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	envs := make([]types.EnvName, 0)
	for env := range release.Manifests.Manifests {
		envs = append(envs, types.EnvName(env))
	}
	release.Environments = envs
	slices.Sort(release.Environments)
	environmentStr, err := json.Marshal(release.Environments)
	if err != nil {
		return fmt.Errorf("could not marshal release environments: %w", err)
	}

	now, err := h.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return fmt.Errorf("DBInsertRelease unable to get transaction timestamp: %w", err)
	}
	_, err = transaction.Exec(
		upsertQuery,
		*now,
		*(release.ReleaseNumbers.Version),
		release.App,
		manifestJson,
		metadataJson,
		environmentStr,
		release.ReleaseNumbers.Revision,
	)
	if err != nil {
		return fmt.Errorf(
			"could not insert release for app '%s' and version '%v' into DB. Error: %w\n",
			release.App,
			*release.ReleaseNumbers.Version,
			err)
	}
	return nil
}

func (h *DBHandler) insertReleaseHistoryRow(ctx context.Context, transaction *sql.Tx, release DBReleaseWithMetaData, deleted bool) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "insertReleaseHistoryRow")
	defer span.Finish()
	insertQuery := h.AdaptQuery(`
		INSERT INTO releases_history (created, releaseVersion, appName, manifests, metadata, deleted, environments, revision)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?);
	`)
	span.SetTag("query", insertQuery)
	metadataJson, err := json.Marshal(release.Metadata)
	if err != nil {
		return fmt.Errorf("insert release: could not marshal json data: %w", err)
	}
	manifestJson, err := json.Marshal(release.Manifests)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	envs := make([]types.EnvName, 0)
	for env := range release.Manifests.Manifests {
		envs = append(envs, types.EnvName(env))
	}
	release.Environments = envs
	slices.Sort(release.Environments)
	environmentStr, err := json.Marshal(release.Environments)
	if err != nil {
		return fmt.Errorf("could not marshal release environments: %w", err)
	}

	now, err := h.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return fmt.Errorf("DBInsertRelease unable to get transaction timestamp: %w", err)
	}
	_, err = transaction.Exec(
		insertQuery,
		*now,
		*release.ReleaseNumbers.Version,
		release.App,
		manifestJson,
		metadataJson,
		deleted,
		environmentStr,
		release.ReleaseNumbers.Revision,
	)
	if err != nil {
		return fmt.Errorf(
			"could not insert release_history for app '%s' and version '%v' into DB. Error: %w\n",
			release.App,
			*release.ReleaseNumbers.Version,
			err)
	}
	return nil
}

func (h *DBHandler) DBMigrationUpdateReleasesTimestamp(ctx context.Context, transaction *sql.Tx, application string, releaseversion uint64, createAt time.Time) error {
	span, _, onErr := tracing.StartSpanFromContext(ctx, "DBMigrationUpdateReleasesTimestamp")
	defer span.Finish()
	historyUpdateQuery := h.AdaptQuery(`
		UPDATE releases_history SET created=? WHERE appname=? AND releaseversion=?;
	`)
	span.SetTag("query", historyUpdateQuery)

	_, err := transaction.Exec(
		historyUpdateQuery,
		createAt,
		application,
		releaseversion,
	)
	if err != nil {
		return onErr(fmt.Errorf(
			"could not update releases_history timestamp for app '%s' and version '%v' into DB. Error: %w",
			application,
			releaseversion,
			err))
	}

	releasesUpdateQuery := h.AdaptQuery(`
		UPDATE releases SET created=? WHERE appname=? AND releaseversion=?;
	`)
	span.SetTag("query", releasesUpdateQuery)

	_, err = transaction.Exec(
		releasesUpdateQuery,
		createAt,
		application,
		releaseversion,
	)
	if err != nil {
		return onErr(fmt.Errorf(
			"could not update releases timestamp for app '%s' and version '%v' into DB. Error: %w",
			application,
			releaseversion,
			err))
	}
	return nil

}

// process rows functions

func (h *DBHandler) processReleaseRow(ctx context.Context, err error, rows *sql.Rows, ignorePrepublishes bool, withManifests bool) (*DBReleaseWithMetaData, error) {
	processedRows, err := h.processReleaseRows(ctx, err, rows, ignorePrepublishes, withManifests)
	if err != nil {
		return nil, err
	}
	if len(processedRows) == 0 {
		return nil, nil
	}
	return processedRows[0], nil
}

func (h *DBHandler) processReleaseRows(ctx context.Context, err error, rows *sql.Rows, ignorePrepublishes bool, withManifests bool) ([]*DBReleaseWithMetaData, error) {
	if err != nil {
		return nil, fmt.Errorf("could not query releases table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("releases: row could not be closed: %v", err)
		}
	}(rows)
	//exhaustruct:ignore
	var result []*DBReleaseWithMetaData
	var lastSeenRelease uint64 = 0
	for rows.Next() {
		//exhaustruct:ignore
		var row = &DBReleaseWithMetaData{}
		var metadataStr string
		var manifestStr string
		var environmentsStr sql.NullString
		var err error
		if withManifests {
			err = rows.Scan(&row.Created, &row.App, &metadataStr, &manifestStr, &row.ReleaseNumbers.Version, &environmentsStr, &row.ReleaseNumbers.Revision)
		} else {
			err = rows.Scan(&row.Created, &row.App, &metadataStr /*manifests*/, &row.ReleaseNumbers.Version, &environmentsStr, &row.ReleaseNumbers.Revision)
		}
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning releases row from DB withManifests=%v. Error: %w\n", withManifests, err)
		}
		if *row.ReleaseNumbers.Version != lastSeenRelease {
			lastSeenRelease = *row.ReleaseNumbers.Version
		} else {
			continue
		}
		// handle meta data
		var metaData = DBReleaseMetaData{
			SourceAuthor:    "",
			SourceCommitId:  "",
			SourceMessage:   "",
			DisplayVersion:  "",
			UndeployVersion: false,
			IsMinor:         false,
			CiLink:          "",
			IsPrepublish:    false,
		}
		err = json.Unmarshal(([]byte)(metadataStr), &metaData)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal of metadata for releases. Error: %w. Data: %s\n", err, metadataStr)
		}
		row.Metadata = metaData

		// handle manifests
		var manifestData = DBReleaseManifests{
			Manifests: map[types.EnvName]string{},
		}
		if withManifests {
			err = json.Unmarshal(([]byte)(manifestStr), &manifestData)
			if err != nil {
				return nil, fmt.Errorf("Error during json unmarshal of manifests for releases. Error: %w. Data: %s\n", err, metadataStr)
			}
		}
		row.Manifests = manifestData
		environments := make([]types.EnvName, 0)
		if environmentsStr.Valid && environmentsStr.String != "" {
			err = json.Unmarshal(([]byte)(environmentsStr.String), &environments)
			if err != nil {
				return nil, fmt.Errorf("Error during json unmarshal of environments for releases. Error: %w. Data: %s\n", err, environmentsStr.String)
			}
		}
		row.Environments = environments
		if ignorePrepublishes && row.Metadata.IsPrepublish {
			continue
		}
		result = append(result, row)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (h *DBHandler) processReleaseEnvironmentRows(ctx context.Context, err error, rows *sql.Rows) (AppVersionEnvironments, error) {
	if err != nil {
		return nil, fmt.Errorf("could not query releases table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("releases: row could not be closed: %v", err)
		}
	}(rows)
	//exhaustruct:ignore
	var result = make(map[string]map[uint64][]types.EnvName)
	for rows.Next() {
		var environmentsStr sql.NullString
		var appName string
		var releaseVersion uint64
		err := rows.Scan(&appName, &releaseVersion, &environmentsStr)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning releases row from DB. Error: %w\n", err)
		}
		environments := make([]types.EnvName, 0)
		if environmentsStr.Valid && environmentsStr.String != "" {
			err = json.Unmarshal(([]byte)(environmentsStr.String), &environments)
			if err != nil {
				return nil, fmt.Errorf("Error during json unmarshal of environments for releases. Error: %w. Data: %s\n", err, environmentsStr.String)
			}
		}
		if _, exists := result[appName]; !exists {
			result[appName] = make(map[uint64][]types.EnvName)
		}
		result[appName][releaseVersion] = environments
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (h *DBHandler) processAppReleaseVersionsRows(ctx context.Context, err error, rows *sql.Rows) ([]types.ReleaseNumbers, error) {
	if err != nil {
		return nil, fmt.Errorf("could not query releases table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("releases: row could not be closed: %v", err)
		}
	}(rows)
	var row types.ReleaseNumbers
	var result []types.ReleaseNumbers
	for rows.Next() {
		err := rows.Scan(&row.Version, &row.Revision)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning releases row from DB. Error: %w\n", err)
		}
		result = append(result, row)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (h *DBHandler) processAllAppsReleaseVersionsRows(ctx context.Context, err error, rows *sql.Rows) (map[string][]int64, error) {
	if err != nil {
		return nil, fmt.Errorf("could not query releases table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("releases: row could not be closed: %v", err)
		}
	}(rows)

	var result = make(map[string][]int64)
	for rows.Next() {
		var appName string
		var releaseVersion int64

		err := rows.Scan(&appName, &releaseVersion)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning releases row from DB. Error: %w\n", err)
		}

		if _, ok := result[appName]; !ok {
			result[appName] = []int64{}
		}
		result[appName] = append(result[appName], releaseVersion)
	}

	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type ReleaseKey struct {
	AppName        string
	ReleaseVersion uint64
}

func (h *DBHandler) DBSelectCommitHashesTimeWindow(ctx context.Context, transaction *sql.Tx, startDate, endDate time.Time) (map[ReleaseKey]string, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectCommitHashesTimeWindow")
	defer span.Finish()
	if h == nil {
		return nil, nil
	}
	if transaction == nil {
		return nil, fmt.Errorf("DBSelectCommitHashesTimeWindow: no transaction provided")
	}

	var releases = make(map[ReleaseKey]string)
	//Get releases for which we found any relevant deployment. We want to extract the commit hash for that release
	query := h.AdaptQuery(`
			SELECT appName, metadata, releaseVersion FROM releases_history
			WHERE releaseversion IS NOT NULL AND created >= (?) AND created <= (?) ORDER BY version;
		`)
	releasesRows, err := transaction.QueryContext(
		ctx,
		query,
		startDate,
		endDate,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query releases table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("releases: row could not be closed: %v", err)
		}
	}(releasesRows)

	for releasesRows.Next() {
		var releaseVersion uint64
		var appName string
		var metadataStr string

		//Get the metadata
		err := releasesRows.Scan(&appName, &metadataStr, &releaseVersion)
		if err != nil {
			return nil, err
		}

		var metaData = DBReleaseMetaData{
			SourceAuthor:    "",
			SourceCommitId:  "",
			SourceMessage:   "",
			DisplayVersion:  "",
			UndeployVersion: false,
			IsMinor:         false,
			CiLink:          "",
			IsPrepublish:    false,
		}
		err = json.Unmarshal(([]byte)(metadataStr), &metaData)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal of metadata for releases. Error: %w. Data: %s\n", err, metadataStr)
		}
		releases[ReleaseKey{AppName: appName, ReleaseVersion: releaseVersion}] = metaData.SourceCommitId
	}
	err = closeRows(releasesRows)
	if err != nil {
		return nil, err
	}
	return releases, nil
}
