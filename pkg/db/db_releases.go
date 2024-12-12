package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"slices"
	"strings"
	"time"
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
	Manifests map[string]string
}

type DBReleaseWithMetaData struct {
	ReleaseNumber uint64
	Created       time.Time
	App           string
	Manifests     DBReleaseManifests
	Metadata      DBReleaseMetaData
	Environments  []string
}

// SELECTS

func (h *DBHandler) DBSelectAnyRelease(ctx context.Context, tx *sql.Tx, ignorePrepublishes bool) (*DBReleaseWithMetaData, error) {
	selectQuery := h.AdaptQuery(`
	SELECT created, appName, metadata, releaseVersion, environments 
	FROM releases 
	LIMIT 1;`)
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAnyRelease")
	defer span.Finish()
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(ctx, selectQuery)
	return h.processReleaseRow(ctx, err, rows, ignorePrepublishes, false)
}

func (h *DBHandler) DBSelectReleasesWithoutEnvironments(ctx context.Context, tx *sql.Tx) ([]*DBReleaseWithMetaData, error) {
	selectQuery := h.AdaptQuery(`
	SELECT created, appName, metadata, manifests, releaseVersion, environments
	FROM releases
	WHERE COALESCE(environments, '') = '' AND COALESCE(manifests, '') != ''
	LIMIT 100;`)
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectReleasesWithoutEnvironments")
	defer span.Finish()
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(ctx, selectQuery)
	return h.processReleaseRows(ctx, err, rows, true, true)
}

func (h *DBHandler) DBSelectReleasesByVersions(ctx context.Context, tx *sql.Tx, app string, releaseVersions []uint64, ignorePrepublishes bool) ([]*DBReleaseWithMetaData, error) {
	if len(releaseVersions) == 0 {
		return []*DBReleaseWithMetaData{}, nil
	}
	repeatedQuestionMarks := strings.Repeat(",?", len(releaseVersions)-1)
	selectQuery := h.AdaptQuery(`
	SELECT created, appName, metadata, releaseVersion, environments FROM releases
	WHERE appname=? AND releaseversion IN (?` + repeatedQuestionMarks + `)
	LIMIT ?`)
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectReleasesByVersions")
	defer span.Finish()
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

func (h *DBHandler) DBSelectReleaseByVersion(ctx context.Context, tx *sql.Tx, app string, releaseVersion uint64, ignorePrepublishes bool) (*DBReleaseWithMetaData, error) {
	selectQuery := h.AdaptQuery(`
	SELECT created, appName, metadata, manifests, releaseVersion, environments  
	FROM releases  
	WHERE appName=? AND releaseVersion=? 
	LIMIT 1;`)
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectReleaseByVersion")
	defer span.Finish()
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		app,
		releaseVersion,
	)
	return h.processReleaseRow(ctx, err, rows, ignorePrepublishes, true)
}

func (h *DBHandler) DBSelectReleaseByVersionAtTimestamp(ctx context.Context, tx *sql.Tx, app string, releaseVersion uint64, ignorePrepublishes bool, ts time.Time) (*DBReleaseWithMetaData, error) {
	selectQuery := h.AdaptQuery(`
	SELECT created, appName, metadata, manifests, releaseVersion, environments 
	FROM releases
	WHERE appName=? AND releaseVersion=? AND created <= (?)
	LIMIT 1;`)
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectReleaseByVersion")
	defer span.Finish()
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

func (h *DBHandler) DBSelectAllManifestsForAllReleases(ctx context.Context, tx *sql.Tx) (map[string]map[uint64][]string, error) {
	selectQuery := h.AdaptQuery(`
	SELECT appname, releaseVersion, environments
	FROM releases;`)
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllManifestsForAllReleases")
	defer span.Finish()
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)

	return h.processReleaseManifestRows(ctx, err, rows)
}

func (h *DBHandler) DBSelectReleasesByAppLatestEslVersion(ctx context.Context, tx *sql.Tx, app string, ignorePrepublishes bool) ([]*DBReleaseWithMetaData, error) {
	selectQuery := h.AdaptQuery(`
	SELECT created, appName, metadata, manifests, releaseVersion, environments
	FROM releases
	WHERE appname=?
	ORDER BY releaseversion DESC;`,
	)
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectReleasesByAppLatestEslVersion")
	defer span.Finish()
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		app,
	)

	return h.processReleaseRows(ctx, err, rows, ignorePrepublishes, true)
}

func (h *DBHandler) DBSelectLatestReleaseOfApp(ctx context.Context, tx *sql.Tx, app string, ignorePrepublishes bool) (*DBReleaseWithMetaData, error) {
	selectQuery := h.AdaptQuery(`
	SELECT created, appName, metadata, releaseVersion, environments
	FROM releases
	WHERE appName=?
	ORDER BY releaseVersion DESC
	LIMIT 1;`)
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectReleasesByAppOrderedByEslVersion")
	defer span.Finish()
	span.SetTag("query", selectQuery)
	span.SetTag("appName", app)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		app,
	)

	return h.processReleaseRow(ctx, err, rows, ignorePrepublishes, false)
}

func (h *DBHandler) DBSelectAllReleasesOfApp(ctx context.Context, tx *sql.Tx, app string) ([]int64, error) {
	selectQuery := h.AdaptQuery(`SELECT releaseVersion
	FROM releases
	WHERE appName=?
	ORDER BY releaseVersion;`)
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllReleasesOfApp")
	defer span.Finish()
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
	FROM releases;`)

	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	return h.processAllAppsReleaseVersionsRows(ctx, err, rows)
}

// INSERT, UPDATE, DELETES

func (h *DBHandler) DBUpdateOrCreateRelease(ctx context.Context, transaction *sql.Tx, release DBReleaseWithMetaData) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBInsertRelease")
	defer span.Finish()
	retrievedRelease, err := h.DBSelectReleaseByVersion(ctx, transaction, release.App, release.ReleaseNumber, false)
	if err != nil {
		return err
	}
	if retrievedRelease == nil {
		err = h.insertReleaseRow(ctx, transaction, release)
		if err != nil {
			return err
		}
	} else {
		err = h.updateReleaseRow(ctx, transaction, release)
		if err != nil {
			return err
		}
	}
	err = h.insertReleaseHistoryRow(ctx, transaction, release, false)
	if err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) DBDeleteFromReleases(ctx context.Context, transaction *sql.Tx, application string, releaseToDelete uint64) error {
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
	deleteQuery := h.AdaptQuery(`DELETE FROM releases WHERE appname=? AND releaseversion=?`)
	span, _ := tracer.StartSpanFromContext(ctx, "DBInsertRelease")
	defer span.Finish()
	span.SetTag("query", deleteQuery)
	_, err := transaction.Exec(
		deleteQuery,
		release.App,
		release.ReleaseNumber,
	)
	if err != nil {
		return fmt.Errorf(
			"could not delete release for app '%s' and version '%v' from DB. Error: %w\n",
			release.App,
			release.ReleaseNumber,
			err)
	}
	return nil
}

func (h *DBHandler) updateReleaseRow(ctx context.Context, transaction *sql.Tx, release DBReleaseWithMetaData) error {
	insertQuery := h.AdaptQuery(`
		UPDATE releases SET created=?, manifests=?, metadata=?, environments=?
		WHERE appname=? AND releaseVersion=?;`)
	span, ctx := tracer.StartSpanFromContext(ctx, "DBInsertRelease")
	defer span.Finish()
	span.SetTag("query", insertQuery)
	metadataJson, err := json.Marshal(release.Metadata)
	if err != nil {
		return fmt.Errorf("insert release: could not marshal json data: %w", err)
	}
	manifestJson, err := json.Marshal(release.Manifests)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	envs := make([]string, 0)
	for env := range release.Manifests.Manifests {
		envs = append(envs, env)
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
		manifestJson,
		metadataJson,
		environmentStr,
		release.App,
		release.ReleaseNumber,
	)
	if err != nil {
		return fmt.Errorf(
			"could not insert release for app '%s' and version '%v' into DB. Error: %w\n",
			release.App,
			release.ReleaseNumber,
			err)
	}
	return nil
}

func (h *DBHandler) insertReleaseRow(ctx context.Context, transaction *sql.Tx, release DBReleaseWithMetaData) error {
	insertQuery := h.AdaptQuery(
		"INSERT INTO releases (created, releaseVersion, appName, manifests, metadata, environments)  VALUES (?, ?, ?, ?, ?, ?);",
	)
	span, ctx := tracer.StartSpanFromContext(ctx, "DBInsertRelease")
	defer span.Finish()
	span.SetTag("query", insertQuery)
	metadataJson, err := json.Marshal(release.Metadata)
	if err != nil {
		return fmt.Errorf("insert release: could not marshal json data: %w", err)
	}
	manifestJson, err := json.Marshal(release.Manifests)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	envs := make([]string, 0)
	for env := range release.Manifests.Manifests {
		envs = append(envs, env)
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
		release.ReleaseNumber,
		release.App,
		manifestJson,
		metadataJson,
		environmentStr,
	)
	if err != nil {
		return fmt.Errorf(
			"could not insert release for app '%s' and version '%v' into DB. Error: %w\n",
			release.App,
			release.ReleaseNumber,
			err)
	}
	return nil
}

func (h *DBHandler) insertReleaseHistoryRow(ctx context.Context, transaction *sql.Tx, release DBReleaseWithMetaData, deleted bool) error {
	insertQuery := h.AdaptQuery(
		"INSERT INTO releases_history (created, releaseVersion, appName, manifests, metadata, deleted, environments)  VALUES (?, ?, ?, ?, ?, ?, ?);",
	)
	span, ctx := tracer.StartSpanFromContext(ctx, "DBInsertRelease")
	defer span.Finish()
	span.SetTag("query", insertQuery)
	metadataJson, err := json.Marshal(release.Metadata)
	if err != nil {
		return fmt.Errorf("insert release: could not marshal json data: %w", err)
	}
	manifestJson, err := json.Marshal(release.Manifests)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	envs := make([]string, 0)
	for env := range release.Manifests.Manifests {
		envs = append(envs, env)
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
		release.ReleaseNumber,
		release.App,
		manifestJson,
		metadataJson,
		deleted,
		environmentStr,
	)
	if err != nil {
		return fmt.Errorf(
			"could not insert release for app '%s' and version '%v' into DB. Error: %w\n",
			release.App,
			release.ReleaseNumber,
			err)
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
	span, ctx := tracer.StartSpanFromContext(ctx, "processReleaseRows")
	defer span.Finish()

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
			err = rows.Scan(&row.Created, &row.App, &metadataStr, &manifestStr, &row.ReleaseNumber, &environmentsStr)
		} else {
			err = rows.Scan(&row.Created, &row.App, &metadataStr /*manifests*/, &row.ReleaseNumber, &environmentsStr)
		}
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning releases row from DB withManifests=%v. Error: %w\n", withManifests, err)
		}
		if row.ReleaseNumber != lastSeenRelease {
			lastSeenRelease = row.ReleaseNumber
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
			Manifests: map[string]string{},
		}
		if withManifests {
			err = json.Unmarshal(([]byte)(manifestStr), &manifestData)
			if err != nil {
				return nil, fmt.Errorf("Error during json unmarshal of manifests for releases. Error: %w. Data: %s\n", err, metadataStr)
			}
		}
		row.Manifests = manifestData
		environments := make([]string, 0)
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

func (h *DBHandler) processReleaseManifestRows(ctx context.Context, err error, rows *sql.Rows) (map[string]map[uint64][]string, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "processReleaseManifestRows")
	defer span.Finish()

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
	var result = make(map[string]map[uint64][]string)
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
		environments := make([]string, 0)
		if environmentsStr.Valid && environmentsStr.String != "" {
			err = json.Unmarshal(([]byte)(environmentsStr.String), &environments)
			if err != nil {
				return nil, fmt.Errorf("Error during json unmarshal of environments for releases. Error: %w. Data: %s\n", err, environmentsStr.String)
			}
		}
		if _, exists := result[appName]; !exists {
			result[appName] = make(map[uint64][]string)
		}
		result[appName][releaseVersion] = environments
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (h *DBHandler) processAppReleaseVersionsRows(ctx context.Context, err error, rows *sql.Rows) ([]int64, error) {
	if err != nil {
		return nil, fmt.Errorf("could not query releases table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("releases: row could not be closed: %v", err)
		}
	}(rows)
	result := []int64{}
	//exhaustruct:ignore
	var row int64
	for rows.Next() {
		err := rows.Scan(&row)
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
	span, ctx := tracer.StartSpanFromContext(ctx, "processAllReleasesForAllAppsRow")
	defer span.Finish()

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
