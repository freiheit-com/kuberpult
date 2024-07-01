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
	"path"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/sorting"
	uuid2 "github.com/freiheit-com/kuberpult/pkg/uuid"
	"github.com/onokonem/sillyQueueServer/timeuuid"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	config "github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	psql "github.com/golang-migrate/migrate/v4/database/postgres"
	sqlite "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

type DBConfig struct {
	DbUser         string
	DbHost         string
	DbPort         string
	DbName         string
	DriverName     string
	DbPassword     string
	MigrationsPath string
	WriteEslOnly   bool
}

type DBHandler struct {
	DbName         string
	DriverName     string
	MigrationsPath string
	DB             *sql.DB
	DBDriver       *database.Driver

	/*
		There are 3 modes:
		1) DBHandler==nil: do not write anything to the DB
		2) DBHandler!=nil && WriteEslOnly==true: write only the ESL table to the database. Stores all incoming data in the DB, but does not read the DB.
		3) DBHandler!=nil && WriteEslOnly==false: write everything to the database.
	*/
	WriteEslOnly bool
}

type EslId int64

type AppStateChange string

const (
	InitialEslId EslId = 1

	AppStateChangeMigrate AppStateChange = "AppStateChangeMigrate"
	AppStateChangeCreate  AppStateChange = "AppStateChangeCreate"
	AppStateChangeUpdate  AppStateChange = "AppStateChangeUpdate"
	AppStateChangeDelete  AppStateChange = "AppStateChangeDelete"
)

func (h *DBHandler) ShouldUseEslTable() bool {
	return h != nil
}

func (h *DBHandler) ShouldUseOtherTables() bool {
	return h != nil && !h.WriteEslOnly
}

func Connect(cfg DBConfig) (*DBHandler, error) {
	db, driver, err := GetConnectionAndDriver(cfg)

	if err != nil {
		return nil, err
	}
	return &DBHandler{
		DbName:         cfg.DbName,
		DriverName:     cfg.DriverName,
		MigrationsPath: cfg.MigrationsPath,
		DB:             db,
		DBDriver:       &driver,
		WriteEslOnly:   cfg.WriteEslOnly,
	}, nil
}

func GetDBConnection(cfg DBConfig) (*sql.DB, error) {
	if cfg.DriverName == "postgres" {
		dbURI := fmt.Sprintf("host=%s user=%s password=%s port=%s database=%s sslmode=disable",
			cfg.DbHost, cfg.DbUser, cfg.DbPassword, cfg.DbPort, cfg.DbName)

		dbPool, err := sql.Open(cfg.DriverName, dbURI)
		if err != nil {
			return nil, fmt.Errorf("sql.Open: %w", err)
		}
		dbPool.SetConnMaxLifetime(5 * time.Minute)
		return dbPool, nil
	} else if cfg.DriverName == "sqlite3" {
		return sql.Open("sqlite3", path.Join(cfg.DbHost, "db.sqlite?_foreign_keys=on"))
	}
	return nil, fmt.Errorf("Driver: '%s' not supported. Supported: postgres and sqlite3.", cfg.DriverName)
}

func GetConnectionAndDriver(cfg DBConfig) (*sql.DB, database.Driver, error) {
	db, err := GetDBConnection(cfg)
	if err != nil {
		return nil, nil, err
	}
	if cfg.DriverName == "postgres" {
		driver, err := psql.WithInstance(db, &psql.Config{
			DatabaseName:          cfg.DbName,
			MigrationsTable:       "",
			MigrationsTableQuoted: false,
			MultiStatementEnabled: false,
			MultiStatementMaxSize: 0,
			SchemaName:            "",
			StatementTimeout:      time.Second * 10,
		})
		return db, driver, err
	} else if cfg.DriverName == "sqlite3" {
		driver, err := sqlite.WithInstance(db, &sqlite.Config{
			DatabaseName:    "",
			MigrationsTable: "",
			NoTxWrap:        false,
		})
		return db, driver, err
	}
	return nil, nil, fmt.Errorf("Driver: '%s' not supported. Supported: postgres and sqlite3.", cfg.DriverName)
}

func (h *DBHandler) getMigrationHandler() (*migrate.Migrate, error) {
	if h.DriverName == "postgres" {
		return migrate.NewWithDatabaseInstance("file://"+h.MigrationsPath, h.DbName, *h.DBDriver)
	} else if h.DriverName == "sqlite3" {
		return migrate.NewWithDatabaseInstance("file://"+h.MigrationsPath, "", *h.DBDriver) //FIX ME
	}
	return nil, fmt.Errorf("Driver: '%s' not supported. Supported: postgres and sqlite3.", h.DriverName)
}

func RunDBMigrations(cfg DBConfig) error {
	d, err := Connect(cfg)
	if err != nil {
		return fmt.Errorf("DB Error opening DB connection. Error:  %w\n", err)
	}
	defer d.DB.Close()

	m, err := d.getMigrationHandler()

	if err != nil {
		return fmt.Errorf("Error creating migration instance. Error: %w\n", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("Error running DB migrations. Error: %w\n", err)
		}
	}
	return nil
}

func (h *DBHandler) AdaptQuery(query string) string {
	if h.DriverName == "postgres" {
		return SqliteToPostgresQuery(query)
	} else if h.DriverName == "sqlite3" {
		return query
	}
	panic(fmt.Errorf("AdaptQuery: invalid driver: %s", h.DriverName))
}

// SqliteToPostgresQuery just replaces all "?" into "$1", "$2", etc
func SqliteToPostgresQuery(query string) string {
	var q = query
	var i = 1
	for strings.Contains(q, "?") {
		q = strings.Replace(q, "?", fmt.Sprintf("$%d", i), 1)
		i++
	}
	return q
}

func Remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

func closeRows(rows *sql.Rows) error {
	err := rows.Close()
	if err != nil {
		return fmt.Errorf("row closing error: %v\n", err)
	}
	err = rows.Err()
	if err != nil {
		return fmt.Errorf("row has error: %v\n", err)
	}
	return nil
}

type EventType string

const (
	EvtCreateApplicationVersion         EventType = "CreateApplicationVersion"
	EvtDeployApplicationVersion         EventType = "DeployApplicationVersion"
	EvtCreateUndeployApplicationVersion EventType = "CreateUndeployApplicationVersion"
	EvtUndeployApplication              EventType = "UndeployApplication"
	EvtDeleteEnvFromApp                 EventType = "DeleteEnvFromApp"
	EvtCreateEnvironmentLock            EventType = "CreateEnvironmentLock"
	EvtDeleteEnvironmentLock            EventType = "DeleteEnvironmentLock"
	EvtCreateEnvironmentTeamLock        EventType = "CreateEnvironmentTeamLock"
	EvtDeleteEnvironmentTeamLock        EventType = "DeleteEnvironmentTeamLock"
	EvtCreateEnvironmentGroupLock       EventType = "CreateEnvironmentGroupLock"
	EvtDeleteEnvironmentGroupLock       EventType = "DeleteEnvironmentGroupLock"
	EvtCreateEnvironment                EventType = "CreateEnvironment"
	EvtCreateEnvironmentApplicationLock EventType = "CreateEnvironmentApplicationLock"
	EvtDeleteEnvironmentApplicationLock EventType = "DeleteEnvironmentApplicationLock"
	EvtReleaseTrain                     EventType = "ReleaseTrain"
	EvtMigrationTransformer             EventType = "MigrationTransformer"
)

// ESL EVENTS

type ESLMetadata struct {
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail"`
}

// DBWriteEslEventInternal writes one event to the event-sourcing-light table, taking arbitrary data as input
func (h *DBHandler) DBWriteEslEventInternal(ctx context.Context, eventType EventType, tx *sql.Tx, data interface{}, metadata ESLMetadata) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteEslEventInternal: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteEslEventInternal")
	defer span.Finish()

	dataMap, err := convertObjectToMap(data)

	if err != nil {
		return fmt.Errorf("could not convert object to map: %w", err)
	}
	metadataMap, err := convertObjectToMap(metadata)
	if err != nil {
		return fmt.Errorf("could not convert object to map: %w", err)
	}
	dataMap["metadata"] = metadataMap
	jsonToInsert, err := json.Marshal(dataMap)
	if err != nil {
		return fmt.Errorf("could not marshal combined json data: %w", err)
	}

	insertQuery := h.AdaptQuery("INSERT INTO event_sourcing_light (created, event_type , json)  VALUES (?, ?, ?);")

	span.SetTag("query", insertQuery)
	_, err = tx.Exec(
		insertQuery,
		time.Now().UTC(),
		eventType,
		jsonToInsert)

	if err != nil {
		return fmt.Errorf("could not write internal esl event into DB. Error: %w\n", err)
	}
	return nil
}

func convertObjectToMap(obj interface{}) (map[string]interface{}, error) {
	if obj == nil {
		return map[string]interface{}{}, nil
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var result = make(map[string]interface{})
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type EslEventRow struct {
	EslId     EslId
	Created   time.Time
	EventType EventType
	EventJson string
}

// DBReadEslEventInternal returns either the first or the last row of the esl table
func (h *DBHandler) DBReadEslEventInternal(ctx context.Context, tx *sql.Tx, firstRow bool) (*EslEventRow, error) {
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBReadEslEventInternal: no transaction provided")
	}
	sort := "DESC"
	if firstRow {
		sort = "ASC"
	}
	selectQuery := h.AdaptQuery(fmt.Sprintf("SELECT eslId, created, event_type , json FROM event_sourcing_light ORDER BY created %s LIMIT 1;", sort))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query event_sourcing_light table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)
	var row = &EslEventRow{
		EslId:     0,
		Created:   time.Unix(0, 0),
		EventType: "",
		EventJson: "",
	}
	if rows.Next() {
		err := rows.Scan(&row.EslId, &row.Created, &row.EventType, &row.EventJson)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning event_sourcing_light row from DB. Error: %w\n", err)
		}
	} else {
		row = nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return row, nil
}

// DBReadEslEventLaterThan returns the first row of the esl table that has an eslId > the given eslId
func (h *DBHandler) DBReadEslEventLaterThan(ctx context.Context, tx *sql.Tx, eslId EslId) (*EslEventRow, error) {
	sort := "ASC"
	selectQuery := h.AdaptQuery(fmt.Sprintf("SELECT eslId, created, event_type, json FROM event_sourcing_light WHERE eslId > (?) ORDER BY created %s LIMIT 1;", sort))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		eslId,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query event_sourcing_light table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error for event_sourcing_light: %v", err)
		}
	}(rows)
	var row = &EslEventRow{
		EslId:     0,
		Created:   time.Unix(0, 0),
		EventType: "",
		EventJson: "",
	}
	if !rows.Next() {
		row = nil
	} else {
		err := rows.Scan(&row.EslId, &row.Created, &row.EventType, &row.EventJson)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("event_sourcing_light: Error scanning row from DB. Error: %w\n", err)
		}
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return row, nil
}

// RELEASES
// Releases work a bit different, because they are already immutable.
// We still store the eslId for consistency with other tables,
// but technically it's not required here.

type DBReleaseMetaData struct {
	SourceAuthor   string
	SourceCommitId string
	SourceMessage  string
	DisplayVersion string
}

type DBReleaseManifests struct {
	Manifests map[string]string
}

type DBReleaseWithMetaData struct {
	EslId         EslId
	ReleaseNumber uint64
	Created       time.Time
	App           string
	Manifests     DBReleaseManifests
	Metadata      DBReleaseMetaData
	Deleted       bool
}

type DBAllReleaseMetaData struct {
	Releases []int64
}
type DBAllReleasesWithMetaData struct {
	EslId    EslId
	Created  time.Time
	App      string
	Metadata DBAllReleaseMetaData
}

func (h *DBHandler) DBSelectAnyRelease(ctx context.Context, tx *sql.Tx) (*DBReleaseWithMetaData, error) {
	selectQuery := h.AdaptQuery(fmt.Sprintf(
		"SELECT eslVersion, created, appName, metadata, manifests, releaseVersion, deleted " +
			" FROM releases " +
			" LIMIT 1;"))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	return h.processReleaseRow(ctx, err, rows)
}

func (h *DBHandler) DBSelect(ctx context.Context, tx *sql.Tx, app string, releaseVersion uint64) (*DBReleaseWithMetaData, error) {
	selectQuery := h.AdaptQuery(fmt.Sprintf(
		"SELECT eslVersion, created, appName, metadata, manifests, releaseVersion, deleted " +
			" FROM releases " +
			" WHERE appName=? AND releaseVersion=?" +
			" ORDER BY eslVersion DESC " +
			" LIMIT 1;"))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		app,
		releaseVersion,
	)
	return h.processReleaseRow(ctx, err, rows)
}

func (h *DBHandler) DBSelectReleaseByVersion(ctx context.Context, tx *sql.Tx, app string, releaseVersion uint64) (*DBReleaseWithMetaData, error) {
	selectQuery := h.AdaptQuery(fmt.Sprintf(
		"SELECT eslVersion, created, appName, metadata, manifests, releaseVersion, deleted " +
			" FROM releases " +
			" WHERE appName=? AND releaseVersion=?" +
			" ORDER BY eslVersion DESC " +
			" LIMIT 1;"))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		app,
		releaseVersion,
	)
	return h.processReleaseRow(ctx, err, rows)
}

func (h *DBHandler) DBSelectAllReleasesOfApp(ctx context.Context, tx *sql.Tx, app string) (*DBAllReleasesWithMetaData, error) {
	selectQuery := h.AdaptQuery(fmt.Sprintf(
		"SELECT eslVersion, created, appName, metadata " +
			" FROM all_releases " +
			" WHERE appName=?" +
			" ORDER BY eslVersion DESC " +
			" LIMIT 1;"))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		app,
	)
	return h.processAllReleasesRow(ctx, err, rows)
}

func (h *DBHandler) processAllReleasesRow(ctx context.Context, err error, rows *sql.Rows) (*DBAllReleasesWithMetaData, error) {
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
	var row = &DBAllReleasesWithMetaData{}
	if rows.Next() {
		var metadataStr string
		err := rows.Scan(&row.EslId, &row.Created, &row.App, &metadataStr)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning releases row from DB. Error: %w\n", err)
		}
		var metaData = DBAllReleaseMetaData{
			Releases: []int64{},
		}
		err = json.Unmarshal(([]byte)(metadataStr), &metaData)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal of releases. Error: %w. Data: %s\n", err, metadataStr)
		}
		row.Metadata = metaData
	} else {
		row = nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (h *DBHandler) processReleaseRow(ctx context.Context, err error, rows *sql.Rows) (*DBReleaseWithMetaData, error) {
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
	var row = &DBReleaseWithMetaData{}
	if rows.Next() {
		var metadataStr string
		var manifestStr string
		err := rows.Scan(&row.EslId, &row.Created, &row.App, &metadataStr, &manifestStr, &row.ReleaseNumber, &row.Deleted)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning releases row from DB. Error: %w\n", err)
		}
		// handle meta data
		var metaData = DBReleaseMetaData{
			SourceAuthor:   "",
			SourceCommitId: "",
			SourceMessage:  "",
			DisplayVersion: "",
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
		err = json.Unmarshal(([]byte)(manifestStr), &manifestData)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal of manifests for releases. Error: %w. Data: %s\n", err, metadataStr)
		}
		row.Manifests = manifestData

	} else {
		row = nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (h *DBHandler) DBInsertRelease(ctx context.Context, transaction *sql.Tx, release DBReleaseWithMetaData, previousEslVersion EslId) error {
	span, _ := tracer.StartSpanFromContext(ctx, "DBInsertRelease")
	defer span.Finish()
	metadataJson, err := json.Marshal(release.Metadata)
	if err != nil {
		return fmt.Errorf("insert release: could not marshal json data: %w", err)
	}
	insertQuery := h.AdaptQuery(
		"INSERT INTO releases (eslVersion, created, releaseVersion, appName, manifests, metadata, deleted)  VALUES (?, ?, ?, ?, ?, ?, ?);",
	)
	span.SetTag("query", insertQuery)
	manifestJson, err := json.Marshal(release.Manifests)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	_, err = transaction.Exec(
		insertQuery,
		previousEslVersion+1,
		time.Now().UTC(),
		release.ReleaseNumber,
		release.App,
		manifestJson,
		metadataJson,
		release.Deleted,
	)
	if err != nil {
		return fmt.Errorf(
			"could not insert release for app '%s' and version '%v' and eslVersion '%v' into DB. Error: %w\n",
			release.App,
			release.ReleaseNumber,
			previousEslVersion+1,
			err)
	}
	logger.FromContext(ctx).Sugar().Infof(
		"inserted release: app '%s' and version '%v' and eslVersion %v",
		release.App,
		release.ReleaseNumber,
		previousEslVersion+1)
	return nil
}

func (h *DBHandler) DBDeleteReleaseFromAllReleases(ctx context.Context, transaction *sql.Tx, application string, releaseToDelete uint64) error {
	span, _ := tracer.StartSpanFromContext(ctx, "DBDeleteReleaseFromAllReleases")
	defer span.Finish()

	allReleases, err := h.DBSelectAllReleasesOfApp(ctx, transaction, application)
	if err != nil {
		return err
	}
	if allReleases == nil {
		logger.FromContext(ctx).Sugar().Infof("Could not find release '%d' for appliation '%s' to delete. No releases found.", releaseToDelete, application)
		return nil
	}
	idxToDelete := slices.Index(allReleases.Metadata.Releases, int64(releaseToDelete))

	if idxToDelete == -1 {
		logger.FromContext(ctx).Sugar().Infof("Could not find release '%d' for appliation '%s' to delete.", releaseToDelete, application)
		return nil //If we don't find it, not an error, but we do nothing
	}
	allReleases.Metadata.Releases = append(allReleases.Metadata.Releases[:idxToDelete], allReleases.Metadata.Releases[idxToDelete+1:]...)
	if err := h.DBInsertAllReleases(ctx, transaction, application, allReleases.Metadata.Releases, allReleases.EslId); err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) DBDeleteFromReleases(ctx context.Context, transaction *sql.Tx, application string, releaseToDelete uint64) error {
	span, _ := tracer.StartSpanFromContext(ctx, "DBDeleteFromReleases")
	defer span.Finish()

	targetRelease, err := h.DBSelectReleaseByVersion(ctx, transaction, application, releaseToDelete)
	if err != nil {
		return err
	}

	if targetRelease.Deleted {
		logger.FromContext(ctx).Sugar().Infof("Release '%d' for application '%s' has already been deleted.", releaseToDelete, application)
		return nil
	}

	targetRelease.Deleted = true
	if err := h.DBInsertRelease(ctx, transaction, *targetRelease, targetRelease.EslId); err != nil {
		return err
	}

	return nil
}

func (h *DBHandler) DBInsertAllReleases(ctx context.Context, transaction *sql.Tx, app string, allVersions []int64, previousEslVersion EslId) error {
	span, _ := tracer.StartSpanFromContext(ctx, "DBInsertRelease")
	defer span.Finish()
	slices.Sort(allVersions)
	metadataJson, err := json.Marshal(DBAllReleaseMetaData{
		Releases: allVersions,
	})
	if err != nil {
		return fmt.Errorf("insert release: could not marshal json data: %w", err)
	}
	insertQuery := h.AdaptQuery(
		"INSERT INTO all_releases (eslVersion, created, appName, metadata)  VALUES (?, ?, ?, ?);",
	)
	span.SetTag("query", insertQuery)

	_, err = transaction.Exec(
		insertQuery,
		previousEslVersion+1,
		time.Now().UTC(),
		app,
		metadataJson,
	)
	if err != nil {
		return fmt.Errorf("could not insert all_releases for app '%s' and esl '%v' into DB. Error: %w\n", app, previousEslVersion+1, err)
	}
	logger.FromContext(ctx).Sugar().Infof("inserted all_releases for app '%s'", app)
	return nil
}

// APPS

func (h *DBHandler) DBWriteAllApplications(ctx context.Context, transaction *sql.Tx, previousVersion int64, applications []string) error {
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteAllApplications")
	defer span.Finish()
	slices.Sort(applications) // we don't really *need* the sorting, it's just for convenience
	jsonToInsert, err := json.Marshal(AllApplicationsJson{
		Apps: applications,
	})
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}
	insertQuery := h.AdaptQuery("INSERT INTO all_apps (version , created , json)  VALUES (?, ?, ?);")
	span.SetTag("query", insertQuery)
	_, err = transaction.Exec(
		insertQuery,
		previousVersion+1,
		time.Now().UTC(),
		jsonToInsert)

	if err != nil {
		return fmt.Errorf("could not insert all apps into DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) writeEvent(ctx context.Context, transaction *sql.Tx, transformerID uint, eventuuid string, eventType event.EventType, sourceCommitHash string, eventJson []byte) error {
	span, _ := tracer.StartSpanFromContext(ctx, "writeEvent")
	defer span.Finish()
	insertQuery := h.AdaptQuery("INSERT INTO commit_events (uuid, timestamp, commitHash, eventType, json, transformerEslId)  VALUES (?, ?, ?, ?, ?, ?);")

	rawUUID, err := timeuuid.ParseUUID(eventuuid)
	if err != nil {
		return fmt.Errorf("error parsing UUID. Error: %w", err)
	}
	span.SetTag("query", insertQuery)
	_, err = transaction.Exec(
		insertQuery,
		rawUUID.String(),
		uuid2.GetTime(&rawUUID).AsTime(),
		sourceCommitHash,
		eventType,
		eventJson,
		transformerID)

	if err != nil {
		return fmt.Errorf("Error inserting event information into DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) DBWriteNewReleaseEvent(ctx context.Context, transaction *sql.Tx, transformerID uint, uuid, sourceCommitHash string, newReleaseEvent *event.NewRelease) error {
	metadata := event.Metadata{
		Uuid:      uuid,
		EventType: string(event.EventTypeNewRelease),
	}
	jsonToInsert, err := json.Marshal(event.DBEventGo{
		EventData:     newReleaseEvent,
		EventMetadata: metadata,
	})

	if err != nil {
		return fmt.Errorf("error marshalling lock new release event to Json. Error: %v\n", err)
	}
	return h.writeEvent(ctx, transaction, transformerID, uuid, event.EventTypeNewRelease, sourceCommitHash, jsonToInsert)
}

func (h *DBHandler) DBWriteLockPreventedDeploymentEvent(ctx context.Context, transaction *sql.Tx, transformerID uint, uuid, sourceCommitHash string, lockPreventedDeploymentEvent *event.LockPreventedDeployment) error {
	metadata := event.Metadata{
		Uuid:      uuid,
		EventType: string(event.EventTypeLockPreventeDeployment),
	}
	jsonToInsert, err := json.Marshal(event.DBEventGo{
		EventData:     lockPreventedDeploymentEvent,
		EventMetadata: metadata,
	})

	if err != nil {
		return fmt.Errorf("error marshalling lock prevented deployment event to Json. Error: %v\n", err)
	}
	return h.writeEvent(ctx, transaction, transformerID, uuid, event.EventTypeLockPreventeDeployment, sourceCommitHash, jsonToInsert)
}

func (h *DBHandler) DBWriteReplacedByEvent(ctx context.Context, transaction *sql.Tx, transformerID uint, uuid, sourceCommitHash string, replacedBy *event.ReplacedBy) error {
	metadata := event.Metadata{
		Uuid:      uuid,
		EventType: string(event.EventTypeReplaceBy),
	}
	jsonToInsert, err := json.Marshal(event.DBEventGo{
		EventData:     replacedBy,
		EventMetadata: metadata,
	})

	if err != nil {
		return fmt.Errorf("error marshalling replacedBys event to Json. Error: %v\n", err)
	}
	return h.writeEvent(ctx, transaction, transformerID, uuid, event.EventTypeReplaceBy, sourceCommitHash, jsonToInsert)
}

func (h *DBHandler) DBWriteDeploymentEvent(ctx context.Context, transaction *sql.Tx, transformerID uint, uuid, sourceCommitHash string, deployment *event.Deployment) error {
	metadata := event.Metadata{
		Uuid:      uuid,
		EventType: string(event.EventTypeDeployment),
	}
	jsonToInsert, err := json.Marshal(event.DBEventGo{
		EventData:     deployment,
		EventMetadata: metadata,
	})

	if err != nil {
		return fmt.Errorf("error marshalling deployment event to Json. Error: %v\n", err)
	}
	return h.writeEvent(ctx, transaction, transformerID, uuid, event.EventTypeDeployment, sourceCommitHash, jsonToInsert)
}

func (h *DBHandler) DBSelectAnyEvent(ctx context.Context, transaction *sql.Tx) (*EventRow, error) {
	if h == nil {
		return nil, nil
	}
	if transaction == nil {
		return nil, fmt.Errorf("DBSelectAnyEvent: no transaction provided")
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAnyEvent")
	defer span.Finish()

	query := h.AdaptQuery("SELECT uuid, timestamp, commitHash, eventType, json, transformerEslId FROM commit_events ORDER BY timestamp DESC LIMIT 1;")
	span.SetTag("query", query)
	rows, err := transaction.QueryContext(ctx, query)
	return h.processSingleEventsRow(ctx, rows, err)
}
func (h *DBHandler) DBSelectAllCommitEventsForTransformer(ctx context.Context, transaction *sql.Tx, transformerID uint, eventType event.EventType, limit uint) ([]event.DBEventGo, error) {
	if h == nil {
		return nil, nil
	}
	if transaction == nil {
		return nil, fmt.Errorf("DBSelectAllCommitEventsForTransformer: no transaction provided")
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllCommitEventsForTransformer")
	defer span.Finish()

	query := h.AdaptQuery("SELECT uuid, timestamp, commitHash, eventType, json, transformerEslId FROM commit_events WHERE eventType = (?) AND transformerEslId = (?) ORDER BY timestamp DESC LIMIT ?;")
	span.SetTag("query", query)

	rows, err := transaction.QueryContext(ctx, query, string(eventType), transformerID, limit)
	if err != nil {
		return nil, fmt.Errorf("Error querying commit_events. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("commit_events row could not be closed: %v", err)
		}
	}(rows)

	var result []event.DBEventGo
	for rows.Next() {
		//exhaustruct:ignore
		var row = &EventRow{}
		err := rows.Scan(&row.Uuid, &row.Timestamp, &row.CommitHash, &row.EventType, &row.EventJson, &row.TransformerID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning commit_events row from DB. Error: %w\n", err)
		}

		eventGo, err := event.UnMarshallEvent(row.EventType, row.EventJson)
		if err != nil {
			return nil, fmt.Errorf("Could not unmarshall commit event: %v\n", err)
		}
		result = append(result, eventGo)
	}
	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("commit_events: row closing error: %v\n", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("commit_events: row has error: %v\n", err)
	}
	return result, nil
}

func (h *DBHandler) processSingleEventsRow(ctx context.Context, rows *sql.Rows, err error) (*EventRow, error) {
	if err != nil {
		return nil, fmt.Errorf("Error querying commit_events. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("commit_events row could not be closed: %v", err)
		}
	}(rows)

	//exhaustruct:ignore
	var row = &EventRow{}
	if rows.Next() {
		err := rows.Scan(&row.Uuid, &row.Timestamp, &row.CommitHash, &row.EventType, &row.EventJson, &row.TransformerID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning commit_events row from DB. Error: %w\n", err)
		}
	} else {
		row = nil
	}
	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("commit_events: row closing error: %v\n", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("commit_events: row has error: %v\n", err)
	}
	return row, nil
}

func (h *DBHandler) DBSelectAllEventsForCommit(ctx context.Context, transaction *sql.Tx, commitHash string) ([]EventRow, error) {
	if h == nil {
		return nil, nil
	}
	if transaction == nil {
		return nil, fmt.Errorf("DBSelectAllEventsForCommit: no transaction provided")
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllEventsForCommit")
	defer span.Finish()

	query := h.AdaptQuery("SELECT uuid, timestamp, commitHash, eventType, json, transformerEslId FROM commit_events WHERE commitHash = (?) ORDER BY timestamp DESC LIMIT 100;")
	span.SetTag("query", query)

	rows, err := transaction.QueryContext(ctx, query, commitHash)
	if err != nil {
		return nil, fmt.Errorf("Error querying commit_events. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("commit_events row could not be closed: %v", err)
		}
	}(rows)

	var result []EventRow

	for rows.Next() {
		var row = EventRow{
			Uuid:          "",
			Timestamp:     time.Unix(0, 0), //will be overwritten, prevents CI linter from complaining from missing fields
			CommitHash:    "",
			EventType:     "",
			EventJson:     "",
			TransformerID: 0,
		}
		err := rows.Scan(&row.Uuid, &row.Timestamp, &row.CommitHash, &row.EventType, &row.EventJson, &row.TransformerID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning commit_events row from DB. Error: %w\n", err)
		}

		result = append(result, row)
	}
	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("commit_events: row closing error: %v\n", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("commit_events: row has error: %v\n", err)
	}
	return result, nil
}

// DBSelectAllApplications returns (nil, nil) if there are no rows
func (h *DBHandler) DBSelectAllApplications(ctx context.Context, transaction *sql.Tx) (*AllApplicationsGo, error) {
	if h == nil {
		return nil, nil
	}
	if transaction == nil {
		return nil, fmt.Errorf("DBSelectAllEventsForCommit: no transaction provided")
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllApplications")
	defer span.Finish()
	query := "SELECT version, created, json FROM all_apps ORDER BY version DESC LIMIT 1;"
	span.SetTag("query", query)
	rows := transaction.QueryRowContext(ctx, query)
	result := AllApplicationsRow{
		version: 0,
		created: time.Time{},
		data:    "",
	}
	err := rows.Scan(&result.version, &result.created, &result.data)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("Error scanning all_apps row from DB. Error: %w\n", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("all_apps: row has error: %v\n", err)
	}

	//exhaustruct:ignore
	var resultJson = AllApplicationsJson{}
	err = json.Unmarshal(([]byte)(result.data), &resultJson)
	if err != nil {
		return nil, fmt.Errorf("Error during json unmarshal of all_apps. Error: %w. Data: %s\n", err, result.data)
	}
	var resultGo = AllApplicationsGo{
		Version:             result.version,
		Created:             result.created,
		AllApplicationsJson: AllApplicationsJson{Apps: resultJson.Apps},
	}
	return &resultGo, nil
}

type DBDeployment struct {
	EslVersion     EslId
	Created        time.Time
	ReleaseVersion *int64
	App            string
	Env            string
	Metadata       string // json
}

type Deployment struct {
	EslVersion EslId
	Created    time.Time
	App        string
	Env        string
	Version    *int64
	Metadata   DeploymentMetadata
}

type DeploymentMetadata struct {
	DeployedByName  string
	DeployedByEmail string
}

type EnvironmentLock struct {
	EslVersion EslId
	Created    time.Time
	LockID     string
	Env        string
	Deleted    bool
	Metadata   LockMetadata
}

// DBEnvironmentLock Just used to fetch info from DB
type DBEnvironmentLock struct {
	EslVersion EslId
	Created    time.Time
	LockID     string
	Env        string
	Deleted    bool
	Metadata   string
}

type LockMetadata struct {
	CreatedByName  string
	CreatedByEmail string
	Message        string
}

type ReleaseWithManifest struct {
	Version uint64
	/**
	"UndeployVersion=true" means that this version is empty, and has no manifest that could be deployed.
	It is intended to help cleanup old services within the normal release cycle (e.g. dev->staging->production).
	*/
	UndeployVersion bool
	SourceAuthor    string
	SourceCommitId  string
	SourceMessage   string
	CreatedAt       time.Time
	DisplayVersion  string

	Manifests map[string]string // keys: environment; value: manifest
}

type AllDeployments []Deployment
type AllEnvLocks map[string][]EnvironmentLock
type AllReleases map[uint64]ReleaseWithManifest // keys: releaseVersion; value: release with manifests

// GetAllDeploymentsFun and other functions here are used during migration.
// They are supposed to read data from files in the manifest repo,
// and therefore should not need to access the Database at all.
type GetAllDeploymentsFun = func(ctx context.Context, transaction *sql.Tx) (AllDeployments, error)
type GetAllAppLocksFun = func(ctx context.Context) (AllAppLocks, error)

type AllAppLocks map[string]map[string][]ApplicationLock // EnvName-> AppName -> []Locks
type AllTeamLocks map[string]map[string][]TeamLock       // EnvName-> Team -> []Locks
type AllQueuedVersions map[string]map[string]*int64      // EnvName-> AppName -> queuedVersion
type AllCommitEvents map[string][]event.DBEventGo        // CommitId -> uuid -> Event

type GetAllEnvLocksFun = func(ctx context.Context) (AllEnvLocks, error)
type GetAllTeamLocksFun = func(ctx context.Context) (AllTeamLocks, error)
type GetAllReleasesFun = func(ctx context.Context, app string) (AllReleases, error)
type GetAllQueuedVersionsFun = func(ctx context.Context) (AllQueuedVersions, error)
type GetAllEventsFun = func(ctx context.Context) (AllCommitEvents, error)

// GetAllAppsFun returns a map where the Key is an app name, and the value is a team name of that app
type GetAllAppsFun = func() (map[string]string, error)

// return value is a map from environment name to environment config
type GetAllEnvironmentsFun = func(ctx context.Context) (map[string]config.EnvironmentConfig, error)

func (h *DBHandler) RunCustomMigrations(
	ctx context.Context,
	getAllAppsFun GetAllAppsFun,
	getAllDeploymentsFun GetAllDeploymentsFun,
	getAllReleasesFun GetAllReleasesFun,
	getAllEnvLocksFun GetAllEnvLocksFun,
	getAllAppLocksFun GetAllAppLocksFun,
	getAllTeamLocksFun GetAllTeamLocksFun,
	getAllEnvironmentsFun GetAllEnvironmentsFun,
	getAllQueuedVersionsFun GetAllQueuedVersionsFun,
	getAllEventsFun GetAllEventsFun,
) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrations")
	defer span.Finish()
	err := h.RunCustomMigrationsEventSourcingLight(ctx)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationAllAppsTable(ctx, getAllAppsFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationApps(ctx, getAllAppsFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationDeployments(ctx, getAllDeploymentsFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationReleases(ctx, getAllAppsFun, getAllReleasesFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationEnvLocks(ctx, getAllEnvLocksFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationAppLocks(ctx, getAllAppLocksFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationTeamLocks(ctx, getAllTeamLocksFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationQueuedApplicationVersions(ctx, getAllQueuedVersionsFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationsCommitEvents(ctx, getAllEventsFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationEnvironments(ctx, getAllEnvironmentsFun)
	if err != nil {
		return err // better wrap the error in a descriptive message?
	}
	return nil
}

func (h *DBHandler) DBSelectDeployment(ctx context.Context, tx *sql.Tx, appSelector string, envSelector string) (*Deployment, error) {
	span, _ := tracer.StartSpanFromContext(ctx, "DBSelectDeployment")
	defer span.Finish()

	selectQuery := h.AdaptQuery(fmt.Sprintf(
		"SELECT eslVersion, created, releaseVersion, appName, envName, metadata" +
			" FROM deployments " +
			" WHERE appName=? AND envName=? " +
			" ORDER BY eslVersion DESC " +
			" LIMIT 1;"))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		appSelector,
		envSelector,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query deployments table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("deployments: row closing error: %v", err)
		}
	}(rows)
	var row = &DBDeployment{
		EslVersion:     0,
		Created:        time.Time{},
		ReleaseVersion: nil,
		App:            "",
		Env:            "",
		Metadata:       "",
	}
	var releaseVersion sql.NullInt64
	//exhaustruct:ignore
	var resultJson = DeploymentMetadata{}
	if rows.Next() {
		err := rows.Scan(&row.EslVersion, &row.Created, &releaseVersion, &row.App, &row.Env, &row.Metadata)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning deployments row from DB. Error: %w\n", err)
		}
		if releaseVersion.Valid {
			row.ReleaseVersion = &releaseVersion.Int64
		}

		err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal in deployments. Error: %w. Data: %s\n", err, row.Metadata)
		}
	}
	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("deployments: row closing error: %v\n", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("deployments: row has error: %v\n", err)
	}
	return &Deployment{
		EslVersion: row.EslVersion,
		Created:    row.Created,
		App:        row.App,
		Env:        row.Env,
		Version:    row.ReleaseVersion,
		Metadata:   resultJson,
	}, nil
}

func (h *DBHandler) DBSelectAnyDeployment(ctx context.Context, tx *sql.Tx) (*DBDeployment, error) {
	selectQuery := h.AdaptQuery(fmt.Sprintf(
		"SELECT eslVersion, created, releaseVersion, appName, envName" +
			" FROM deployments " +
			" LIMIT 1;"))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query deployments table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("deployments row could not be closed: %v", err)
		}
	}(rows)
	//exhaustruct:ignore
	var row = &DBDeployment{}
	if rows.Next() {
		var releaseVersion sql.NullInt64
		err := rows.Scan(&row.EslVersion, &row.Created, &releaseVersion, &row.App, &row.Env)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning deployments row from DB. Error: %w\n", err)
		}
		if releaseVersion.Valid {
			row.ReleaseVersion = &releaseVersion.Int64
		}
	} else {
		row = nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return row, nil
}

type DBApp struct {
	EslId EslId
	App   string
}

type DBAppMetaData struct {
	Team string
}

type DBAppWithMetaData struct {
	EslId       EslId
	App         string
	Metadata    DBAppMetaData
	StateChange AppStateChange
}

func (h *DBHandler) DBInsertApplication(ctx context.Context, transaction *sql.Tx, appName string, previousEslVersion EslId, stateChange AppStateChange, metaData DBAppMetaData) error {
	span, _ := tracer.StartSpanFromContext(ctx, "DBInsertApplication")
	defer span.Finish()
	jsonToInsert, err := json.Marshal(metaData)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}
	insertQuery := h.AdaptQuery(
		"INSERT INTO apps (eslVersion, created, appName, stateChange, metadata)  VALUES (?, ?, ?, ?, ?);",
	)
	span.SetTag("query", insertQuery)
	_, err = transaction.Exec(
		insertQuery,
		previousEslVersion+1,
		time.Now().UTC(),
		appName,
		stateChange,
		jsonToInsert,
	)
	if err != nil {
		return fmt.Errorf("could not insert app %s into DB. Error: %w\n", appName, err)
	}
	return nil
}

func NewNullInt(s *int64) sql.NullInt64 {
	if s == nil {
		return sql.NullInt64{
			Int64: 0,
			Valid: false,
		}
	}
	return sql.NullInt64{
		Int64: *s,
		Valid: true,
	}
}

func (h *DBHandler) DBSelectAnyApp(ctx context.Context, tx *sql.Tx) (*DBAppWithMetaData, error) {
	selectQuery := h.AdaptQuery(fmt.Sprintf(
		"SELECT eslVersion, appName, metadata " +
			" FROM apps " +
			" LIMIT 1;"))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query apps table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row could not be closed: %v", err)
		}
	}(rows)
	//exhaustruct:ignore
	var row = &DBAppWithMetaData{}
	if rows.Next() {
		var metadataStr string
		err := rows.Scan(&row.EslId, &row.App, &metadataStr)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning apps row from DB. Error: %w\n", err)
		}
		var metaData = DBAppMetaData{
			Team: "",
		}
		err = json.Unmarshal(([]byte)(metadataStr), &metaData)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal of apps. Error: %w. Data: %s\n", err, metadataStr)
		}
		row.Metadata = metaData
	} else {
		row = nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (h *DBHandler) DBSelectApp(ctx context.Context, tx *sql.Tx, appName string) (*DBAppWithMetaData, error) {
	selectQuery := h.AdaptQuery(fmt.Sprintf(
		"SELECT eslVersion, appName, stateChange, metadata" +
			" FROM apps " +
			" WHERE appName=? " +
			" ORDER BY eslVersion DESC " +
			" LIMIT 1;"))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		appName,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query apps table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row could not be closed: %v", err)
		}
	}(rows)

	//exhaustruct:ignore
	var row = &DBAppWithMetaData{}
	if rows.Next() {
		var metadataStr string
		err := rows.Scan(&row.EslId, &row.App, &row.StateChange, &metadataStr)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning apps row from DB. Error: %w\n", err)
		}
		var metaData = DBAppMetaData{Team: ""}
		err = json.Unmarshal(([]byte)(metadataStr), &metaData)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal of apps. Error: %w. Data: %s\n", err, metadataStr)
		}
		row.Metadata = metaData
	} else {
		row = nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return row, nil
}

// DBWriteDeployment writes one deployment, meaning "what should be deployed"
func (h *DBHandler) DBWriteDeployment(ctx context.Context, tx *sql.Tx, deployment Deployment, previousEslVersion EslId) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteEslEventInternal: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteEslEventInternal")
	defer span.Finish()

	jsonToInsert, err := json.Marshal(deployment.Metadata)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	insertQuery := h.AdaptQuery(
		"INSERT INTO deployments (eslVersion, created, releaseVersion, appName, envName, metadata) VALUES (?, ?, ?, ?, ?, ?);")

	span.SetTag("query", insertQuery)
	nullVersion := NewNullInt(deployment.Version)
	_, err = tx.Exec(
		insertQuery,
		previousEslVersion+1,
		time.Now().UTC(),
		nullVersion,
		deployment.App,
		deployment.Env,
		jsonToInsert)

	if err != nil {
		return fmt.Errorf("could not write deployment into DB. Error: %w\n", err)
	}
	return nil
}

// CUSTOM MIGRATIONS

func (h *DBHandler) RunCustomMigrationReleases(ctx context.Context, getAllAppsFun GetAllAppsFun, getAllReleasesFun GetAllReleasesFun) error {
	return h.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		allReleasesDb, err := h.DBSelectAnyRelease(ctx, transaction)
		if err != nil {
			l.Warnf("could not get releases from database - assuming the manifest repo is correct: %v", err)
		}
		if allReleasesDb != nil {
			l.Warnf("There are already deployments in the DB - skipping migrations")
			return nil
		}

		allAppsMap, err := getAllAppsFun()
		if err != nil {
			return err
		}
		for app := range allAppsMap {
			l.Infof("processing app %s ...", app)

			releases, err := getAllReleasesFun(ctx, app)
			if err != nil {
				return fmt.Errorf("geAllReleases failed %v", err)
			}

			releaseNumbers := []int64{}
			for r := range releases {
				repoRelease := releases[r]
				dbRelease := DBReleaseWithMetaData{
					EslId:         InitialEslId,
					Created:       time.Now().UTC(),
					ReleaseNumber: repoRelease.Version,
					App:           app,
					Manifests: DBReleaseManifests{
						Manifests: repoRelease.Manifests,
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor:   repoRelease.SourceAuthor,
						SourceCommitId: repoRelease.SourceCommitId,
						SourceMessage:  repoRelease.SourceMessage,
						DisplayVersion: repoRelease.DisplayVersion,
					},
					Deleted: false,
				}
				err = h.DBInsertRelease(ctx, transaction, dbRelease, InitialEslId-1)
				if err != nil {
					return fmt.Errorf("error writing Release to DB for app %s: %v", app, err)
				}
				releaseNumbers = append(releaseNumbers, int64(repoRelease.Version))
			}
			l.Infof("done with app %s", app)
			err = h.DBInsertAllReleases(ctx, transaction, app, releaseNumbers, InitialEslId-1)
			if err != nil {
				return fmt.Errorf("error writing all_releases to DB for app %s: %v", app, err)
			}
		}
		return nil
	})
}

func (h *DBHandler) RunCustomMigrationDeployments(ctx context.Context, getAllDeploymentsFun GetAllDeploymentsFun) error {
	return h.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		allAppsDb, err := h.DBSelectAnyDeployment(ctx, transaction)
		if err != nil {
			l.Warnf("could not get applications from database - assuming the manifest repo is correct: %v", err)
			allAppsDb = nil
		}
		if allAppsDb != nil {
			l.Warnf("There are already deployments in the DB - skipping migrations")
			return nil
		}

		allDeploymentsInRepo, err := getAllDeploymentsFun(ctx, transaction)
		if err != nil {
			return fmt.Errorf("could not get current deployments to run custom migrations: %v", err)
		}

		for i := range allDeploymentsInRepo {
			deploymentInRepo := allDeploymentsInRepo[i]
			err = h.DBWriteDeployment(ctx, transaction, deploymentInRepo, 0)
			if err != nil {
				return fmt.Errorf("error writing Deployment to DB for app %s in env %s: %v",
					deploymentInRepo.App, deploymentInRepo.Env, err)
			}
		}
		return nil
	})
}

type AllApplicationsJson struct {
	Apps []string `json:"apps"`
}

type AllApplicationsRow struct {
	version int64
	created time.Time
	data    string
}

type AllApplicationsGo struct {
	Version int64
	Created time.Time
	AllApplicationsJson
}

type EventRow struct {
	Uuid          string
	Timestamp     time.Time
	CommitHash    string
	EventType     event.EventType
	EventJson     string
	TransformerID uint
}

func (h *DBHandler) RunCustomMigrationEnvLocks(ctx context.Context, getAllEnvLocksFun GetAllEnvLocksFun) error {
	return h.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		allEnvLocksDb, err := h.DBSelectAnyActiveEnvLocks(ctx, transaction)
		if err != nil {
			l.Infof("could not get environment locks from database - assuming the manifest repo is correct: %v", err)
			allEnvLocksDb = nil
		}
		if allEnvLocksDb != nil {
			l.Infof("There are already environment locks in the DB - skipping migrations")
			return nil
		}

		allEnvLocksInRepo, err := getAllEnvLocksFun(ctx)
		if err != nil {
			return fmt.Errorf("could not get current environment locks to run custom migrations: %v", err)
		}

		for envName, locks := range allEnvLocksInRepo {
			var activeLockIds []string
			for _, currentLock := range locks {
				activeLockIds = append(activeLockIds, currentLock.LockID)

				err = h.DBWriteEnvironmentLockInternal(ctx, transaction, currentLock, 0, true)
				if err != nil {
					return fmt.Errorf("error writing environment locks to DB for environment %s: %v",
						envName, err)
				}
			}

			if len(activeLockIds) == 0 {
				activeLockIds = []string{}
			}
			err = h.DBWriteAllEnvironmentLocks(ctx, transaction, 0, envName, activeLockIds)
			if err != nil {
				return fmt.Errorf("error writing environment locks ids to DB for environment %s: %v",
					envName, err)
			}
		}

		return nil
	})
}

func (h *DBHandler) RunCustomMigrationAppLocks(ctx context.Context, getAllAppLocksFun GetAllAppLocksFun) error {
	return h.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		allAppLocksDb, err := h.DBSelectAnyActiveAppLock(ctx, transaction)
		if err != nil {
			l.Infof("could not get application locks from database - assuming the manifest repo is correct: %v", err)
			allAppLocksDb = nil
		}
		if allAppLocksDb != nil {
			l.Infof("There are already application locks in the DB - skipping migrations")
			return nil
		}

		allAppLocksInRepo, err := getAllAppLocksFun(ctx)
		if err != nil {
			return fmt.Errorf("could not get current application locks to run custom migrations: %v", err)
		}

		for envName, apps := range allAppLocksInRepo {
			for appName, currentAppLocks := range apps {
				var activeLockIds []string
				for _, currentLock := range currentAppLocks {
					activeLockIds = append(activeLockIds, currentLock.LockID)
					err = h.DBWriteApplicationLockInternal(ctx, transaction, currentLock, 0, true)
					if err != nil {
						return fmt.Errorf("error writing application locks to DB for application '%s' on '%s': %v",
							appName, envName, err)
					}
				}
				if len(activeLockIds) == 0 {
					activeLockIds = []string{}
				}

				err := h.DBWriteAllAppLocks(ctx, transaction, 0, envName, appName, activeLockIds)
				if err != nil {
					return fmt.Errorf("error writing existing locks to DB for application '%s' on environment '%s': %v",
						appName, envName, err)
				}
			}
		}
		return nil
	})
}

func (h *DBHandler) RunCustomMigrationTeamLocks(ctx context.Context, getAllTeamLocksFun GetAllTeamLocksFun) error {
	return h.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		allTeamLocksDb, err := h.DBSelectAnyActiveTeamLock(ctx, transaction)
		if err != nil {
			l.Infof("could not get team locks from database - assuming the manifest repo is correct: %v", err)
			allTeamLocksDb = nil
		}
		if allTeamLocksDb != nil {
			l.Infof("There are already team locks in the DB - skipping migrations")
			return nil
		}

		allTeamLocksInRepo, err := getAllTeamLocksFun(ctx)
		if err != nil {
			return fmt.Errorf("could not get current team locks to run custom migrations: %v", err)
		}

		for envName, apps := range allTeamLocksInRepo {
			for teamName, currentTeamLocks := range apps {
				var activeLockIds []string
				for _, currentLock := range currentTeamLocks {
					activeLockIds = append(activeLockIds, currentLock.LockID)
					err = h.DBWriteTeamLockInternal(ctx, transaction, currentLock, 0, true)
					if err != nil {
						return fmt.Errorf("error writing team locks to DB for team '%s' on '%s': %v",
							teamName, envName, err)
					}
				}
				if len(activeLockIds) == 0 {
					activeLockIds = []string{}
				}
				err := h.DBWriteAllTeamLocks(ctx, transaction, 0, envName, teamName, activeLockIds)
				if err != nil {
					return fmt.Errorf("error writing existing locks to DB for team '%s' on environment '%s': %v",
						teamName, envName, err)
				}
			}
		}
		return nil
	})
}

func (h *DBHandler) RunCustomMigrationsCommitEvents(ctx context.Context, getAllEvents GetAllEventsFun) error {
	return h.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		ev, err := h.DBSelectAnyEvent(ctx, transaction)
		if err != nil {
			l.Infof("could not get commit events from database - assuming the manifest repo is correct: %v", err)
			ev = nil
		}
		if ev != nil {
			l.Infof("There are already commit events in the DB - skipping migrations")
			return nil
		}

		allEvents, err := getAllEvents(ctx)
		if err != nil {
			return fmt.Errorf("could not get current commit events to run custom migrations: %v", err)
		}
		for commitID, events := range allEvents {
			for _, currentEvent := range events {
				eventJson, err := json.Marshal(currentEvent)
				if err != nil {
					return fmt.Errorf("Could not marshal event: %v\n", err)
				}
				err = h.writeEvent(ctx, transaction, 0, currentEvent.EventMetadata.Uuid, event.EventType(currentEvent.EventMetadata.EventType), commitID, eventJson)
				if err != nil {
					return fmt.Errorf("error writing existing event version: %v", err)
				}
			}
		}
		return nil
	})
}

func (h *DBHandler) RunCustomMigrationQueuedApplicationVersions(ctx context.Context, getAllQueuedVersionsFun GetAllQueuedVersionsFun) error {
	return h.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		allTeamLocksDb, err := h.DBSelectAnyDeploymentAttempt(ctx, transaction)
		if err != nil {
			l.Infof("could not get queued deployments friom database - assuming the manifest repo is correct: %v", err)
			allTeamLocksDb = nil
		}
		if allTeamLocksDb != nil {
			l.Infof("There are already queued deployments in the DB - skipping migrations")
			return nil
		}

		allQueuedVersionsInRepo, err := getAllQueuedVersionsFun(ctx)
		if err != nil {
			return fmt.Errorf("could not get current queued versions to run custom migrations: %v", err)
		}

		for envName, apps := range allQueuedVersionsInRepo {
			for appName, v := range apps {
				err := h.DBWriteDeploymentAttempt(ctx, transaction, envName, appName, v)
				if err != nil {
					return fmt.Errorf("error writing existing queued application version '%d' to DB for app '%s' on environment '%s': %v",
						*v, appName, envName, err)
				}
			}
		}
		return nil
	})
}

// For commit_events migrations, we need some transformer to be on the database before we run their migrations.
func (h *DBHandler) RunCustomMigrationsEventSourcingLight(ctx context.Context) error {
	return h.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		eslEvent, err := h.DBReadEslEventInternal(ctx, transaction, false)
		if err != nil {
			l.Warnf("could not get applications from database - assuming the manifest repo is correct: %v", err)
			return err
		}
		if eslEvent != nil {
			return nil
		}
		span, _ := tracer.StartSpanFromContext(ctx, "RunCustomMigrationsEventSourcingLight")
		defer span.Finish()
		return h.DBWriteMigrationsTransformer(ctx, transaction)
	})
}

func (h *DBHandler) DBWriteMigrationsTransformer(ctx context.Context, transaction *sql.Tx) error {
	if h == nil {
		return nil
	}
	if transaction == nil {
		return fmt.Errorf("DBWriteMigrationsTransformer: no transaction provided")
	}

	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteMigrationsTransformer")
	defer span.Finish()

	dataMap := make(map[string]interface{})
	metadata := ESLMetadata{AuthorName: "Migration", AuthorEmail: "Migration"}
	metadataMap, err := convertObjectToMap(metadata)
	if err != nil {
		return fmt.Errorf("could not convert object to map: %w", err)
	}
	dataMap["metadata"] = metadataMap
	jsonToInsert, err := json.Marshal(dataMap)

	if err != nil {
		return fmt.Errorf("could not marshal json transformer: %w", err)
	}

	insertQuery := h.AdaptQuery("INSERT INTO event_sourcing_light VALUES (0, ?, ?, ?);")

	span.SetTag("query", insertQuery)
	_, err = transaction.Exec(
		insertQuery,
		time.Now().UTC(),
		EvtMigrationTransformer,
		jsonToInsert)

	if err != nil {
		return fmt.Errorf("could not write internal esl event into DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) RunCustomMigrationAllAppsTable(ctx context.Context, getAllAppsFun GetAllAppsFun) error {
	return h.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		allAppsDb, err := h.DBSelectAllApplications(ctx, transaction)
		if err != nil {
			l.Warnf("could not get applications from database - assuming the manifest repo is correct: %v", err)
			allAppsDb = nil
		}

		allAppsRepo, err := getAllAppsFun()
		if err != nil {
			return fmt.Errorf("could not get applications to run custom migrations: %v", err)
		}
		var version int64
		if allAppsDb != nil {
			slices.Sort(allAppsDb.Apps)
			version = allAppsDb.Version
		} else {
			version = 1
		}
		sortedApps := sorting.SortKeys(allAppsRepo)

		if allAppsDb != nil && reflect.DeepEqual(allAppsDb.Apps, sortedApps) {
			// nothing to do
			logger.FromContext(ctx).Sugar().Infof("Nothing to do, all apps are equal")
			return nil
		}
		// if there is any difference, we assume the manifest wins over the database state,
		// so we use `allAppsRepo`:
		return h.DBWriteAllApplications(ctx, transaction, version, sortedApps)
	})
}

func (h *DBHandler) RunCustomMigrationApps(ctx context.Context, getAllAppsFun GetAllAppsFun) error {
	return h.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
		dbApp, err := h.DBSelectAnyApp(ctx, transaction)
		if err != nil {
			return fmt.Errorf("could not get dbApp from database - assuming the manifest repo is correct: %v", err)
		}
		if dbApp != nil {
			// the migration was already done
			logger.FromContext(ctx).Info("migration to apps was done already")
			return nil
		}

		appsMap, err := getAllAppsFun()
		if err != nil {
			return fmt.Errorf("could not get dbApp to run custom migrations: %v", err)
		}

		for app := range appsMap {
			team := appsMap[app]
			err = h.DBInsertApplication(ctx, transaction, app, InitialEslId, AppStateChangeMigrate, DBAppMetaData{Team: team})
			if err != nil {
				return fmt.Errorf("could not write dbApp %s: %v", app, err)
			}
		}
		return nil
	})
}

// ENV LOCKS

func (h *DBHandler) DBSelectAnyActiveEnvLocks(ctx context.Context, tx *sql.Tx) (*AllEnvLocksGo, error) {
	selectQuery := h.AdaptQuery(
		"SELECT version, created, environment, json FROM all_env_locks ORDER BY version DESC LIMIT 1;")
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query environment_locks table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	//exhaustruct:ignore
	var row = AllEnvLocksRow{}
	if rows.Next() {
		err := rows.Scan(&row.Version, &row.Created, &row.Environment, &row.Data)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning environment lock row from DB. Error: %w\n", err)
		}
		err = closeRows(rows)
		if err != nil {
			return nil, err
		}
		//exhaustruct:ignore
		var dataJson = AllEnvLocksJson{}
		err = json.Unmarshal(([]byte)(row.Data), &dataJson)
		if err != nil {
			return nil, fmt.Errorf("Error scanning application lock row from DB. Error: %w\n", err)
		}
		return &AllEnvLocksGo{
			Version:         row.Version,
			Created:         row.Created,
			Environment:     row.Environment,
			AllEnvLocksJson: AllEnvLocksJson{EnvLocks: dataJson.EnvLocks}}, nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return nil, nil // no rows, but also no error
}

func (h *DBHandler) DBSelectEnvironmentLock(ctx context.Context, tx *sql.Tx, environment, lockID string) (*EnvironmentLock, error) {
	selectQuery := h.AdaptQuery(fmt.Sprintf(
		"SELECT eslVersion, created, lockID, envName, metadata, deleted" +
			" FROM environment_locks " +
			" WHERE envName=? AND lockID=? " +
			" ORDER BY eslVersion DESC " +
			" LIMIT 1;"))

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environment,
		lockID,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query environment locks table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	if rows.Next() {
		var row = DBEnvironmentLock{
			EslVersion: 0,
			Created:    time.Time{},
			LockID:     "",
			Env:        "",
			Deleted:    true,
			Metadata:   "",
		}

		err := rows.Scan(&row.EslVersion, &row.Created, &row.LockID, &row.Env, &row.Metadata, &row.Deleted)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning environment locks row from DB. Error: %w\n", err)
		}

		//exhaustruct:ignore
		var resultJson = LockMetadata{}
		err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Metadata)
		}
		err = closeRows(rows)
		if err != nil {
			return nil, err
		}
		return &EnvironmentLock{
			EslVersion: row.EslVersion,
			Created:    row.Created,
			LockID:     row.LockID,
			Env:        row.Env,
			Deleted:    row.Deleted,
			Metadata:   resultJson,
		}, nil
	}

	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return nil, nil // no rows, but also no error

}

func (h *DBHandler) DBWriteEnvironmentLock(ctx context.Context, tx *sql.Tx, lockID, environment, message, authorName, authorEmail string) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteEnvironmentLock: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteEnvironmentLock")
	defer span.Finish()

	var previousVersion EslId

	existingEnvLock, err := h.DBSelectEnvironmentLock(ctx, tx, environment, lockID)

	if err != nil {
		return fmt.Errorf("Could not obtain existing environment lock: %w\n", err)
	}

	if existingEnvLock == nil {
		previousVersion = 0
	} else {
		previousVersion = existingEnvLock.EslVersion
	}

	envLock := EnvironmentLock{
		EslVersion: 0,
		LockID:     lockID,
		Created:    time.Time{},
		Env:        environment,
		Metadata: LockMetadata{
			Message:        message,
			CreatedByName:  authorName,
			CreatedByEmail: authorEmail,
		},
		Deleted: false,
	}
	return h.DBWriteEnvironmentLockInternal(ctx, tx, envLock, previousVersion, false)
}

func (h *DBHandler) DBWriteEnvironmentLockInternal(ctx context.Context, tx *sql.Tx, envLock EnvironmentLock, previousEslVersion EslId, useTimeInLock bool) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteEnvironmentLockInternal: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteEnvironmentLockInternal")
	defer span.Finish()

	jsonToInsert, err := json.Marshal(envLock.Metadata)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	insertQuery := h.AdaptQuery(
		"INSERT INTO environment_locks (eslVersion, created, lockID, envName, deleted, metadata) VALUES (?, ?, ?, ?, ?, ?);")

	var timetoInsert time.Time
	if useTimeInLock {
		timetoInsert = envLock.Created
	} else {
		timetoInsert = time.Now().UTC()
	}
	span.SetTag("query", insertQuery)
	_, err = tx.Exec(
		insertQuery,
		previousEslVersion+1,
		timetoInsert,
		envLock.LockID,
		envLock.Env,
		envLock.Deleted,
		jsonToInsert)

	if err != nil {
		return fmt.Errorf("could not write environment lock into DB. Error: %w\n", err)
	}
	return nil
}

// DBSelectEnvLockHistory returns the last N events associated with some lock on some environment. Currently only used in testing.
func (h *DBHandler) DBSelectEnvLockHistory(ctx context.Context, tx *sql.Tx, environmentName, lockID string, limit int) ([]EnvironmentLock, error) {
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectEnvLocks: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBSelectEnvLocks")
	defer span.Finish()

	selectQuery := h.AdaptQuery(
		fmt.Sprintf(
			"SELECT eslVersion, created, lockID, envName, metadata, deleted" +
				" FROM environment_locks " +
				" WHERE envName=? AND lockID=?" +
				" ORDER BY eslVersion DESC " +
				" LIMIT ?;"))

	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environmentName,
		lockID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("could not read environment lock from DB. Error: %w\n", err)
	}
	envLocks := make([]EnvironmentLock, 0)
	for rows.Next() {
		var row = DBEnvironmentLock{
			EslVersion: 0,
			Created:    time.Time{},
			LockID:     "",
			Env:        "",
			Deleted:    true,
			Metadata:   "",
		}

		err := rows.Scan(&row.EslVersion, &row.Created, &row.LockID, &row.Env, &row.Metadata, &row.Deleted)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning environment locks row from DB. Error: %w\n", err)
		}

		//exhaustruct:ignore
		var resultJson = LockMetadata{}
		err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Metadata)
		}
		envLocks = append(envLocks, EnvironmentLock{
			EslVersion: row.EslVersion,
			Created:    row.Created,
			LockID:     row.LockID,
			Env:        row.Env,
			Deleted:    row.Deleted,
			Metadata:   resultJson,
		})
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return envLocks, nil
}

func (h *DBHandler) DBSelectAllEnvironmentLocks(ctx context.Context, tx *sql.Tx, environment string) (*AllEnvLocksGo, error) {
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAllEnvironmentLocks: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBSelectAllEnvironmentLocks")
	defer span.Finish()
	selectQuery := h.AdaptQuery(
		"SELECT version, created, environment, json FROM all_env_locks WHERE environment = ? ORDER BY version DESC LIMIT 1;")

	rows, err := tx.QueryContext(ctx, selectQuery, environment)
	if err != nil {
		return nil, fmt.Errorf("could not query all env locks table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	if rows.Next() {
		var row = AllEnvLocksRow{
			Version:     0,
			Created:     time.Time{},
			Environment: "",
			Data:        "",
		}

		err := rows.Scan(&row.Version, &row.Created, &row.Environment, &row.Data)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning environment locks row from DB. Error: %w\n", err)
		}

		//exhaustruct:ignore
		var resultJson = AllEnvLocksJson{}
		err = json.Unmarshal(([]byte)(row.Data), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Data)
		}

		var resultGo = AllEnvLocksGo{
			Version:         row.Version,
			Created:         row.Created,
			Environment:     row.Environment,
			AllEnvLocksJson: AllEnvLocksJson{EnvLocks: resultJson.EnvLocks},
		}
		err = closeRows(rows)
		if err != nil {
			return nil, err
		}
		return &resultGo, nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (h *DBHandler) DBSelectEnvironmentLockSet(ctx context.Context, tx *sql.Tx, environment string, lockIDs []string) ([]EnvironmentLock, error) {
	if len(lockIDs) == 0 {
		return nil, nil
	}
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectEnvironmentLockSet: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBSelectEnvironmentLockSet")
	defer span.Finish()

	var envLocks []EnvironmentLock
	var rows *sql.Rows
	defer func(rows *sql.Rows) {
		if rows == nil {
			return
		}
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)
	//Get the latest change to each lock
	for _, id := range lockIDs {
		var err error
		selectQuery := h.AdaptQuery(
			"SELECT eslVersion, created, lockID, envName, metadata, deleted" +
				" FROM environment_locks " +
				" WHERE envName=? AND lockID=? " +
				" ORDER BY eslVersion DESC " +
				" LIMIT 1;")
		rows, err = tx.QueryContext(ctx, selectQuery, environment, id)
		if err != nil {
			return nil, fmt.Errorf("could not query environment locks table from DB. Error: %w\n", err)
		}

		var row = DBEnvironmentLock{
			EslVersion: 0,
			Created:    time.Time{},
			LockID:     "",
			Env:        "",
			Deleted:    false,
			Metadata:   "",
		}
		if rows.Next() {
			err = rows.Scan(&row.EslVersion, &row.Created, &row.LockID, &row.Env, &row.Metadata, &row.Deleted)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, nil
				}
				return nil, fmt.Errorf("Error scanning environment locks row from DB. Error: %w\n", err)
			}

			//exhaustruct:ignore
			var resultJson = LockMetadata{}
			err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
			if err != nil {
				return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Metadata)
			}
			envLocks = append(envLocks, EnvironmentLock{
				EslVersion: row.EslVersion,
				Created:    row.Created,
				LockID:     row.LockID,
				Env:        row.Env,
				Deleted:    row.Deleted,
				Metadata:   resultJson,
			})
		}
		err = closeRows(rows)
		if err != nil {
			return nil, err
		}
	}
	err := closeRows(rows)
	if err != nil {
		return nil, err
	}
	return envLocks, nil
}

func (h *DBHandler) DBWriteAllEnvironmentLocks(ctx context.Context, transaction *sql.Tx, previousVersion int64, environment string, lockIds []string) error {
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteAllEnvironmentLocks")
	defer span.Finish()
	slices.Sort(lockIds) // we don't really *need* the sorting, it's just for convenience
	jsonToInsert, err := json.Marshal(AllEnvLocksJson{
		EnvLocks: lockIds,
	})
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}
	insertQuery := h.AdaptQuery("INSERT INTO all_env_locks (version , created, environment, json)  VALUES (?, ?, ?, ?);")
	span.SetTag("query", insertQuery)
	_, err = transaction.Exec(
		insertQuery,
		previousVersion+1,
		time.Now().UTC(),
		environment,
		jsonToInsert)
	if err != nil {
		return fmt.Errorf("could not insert all env locks into DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) DBDeleteEnvironmentLock(ctx context.Context, tx *sql.Tx, environment, lockID string) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBDeleteEnvironmentLock: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBDeleteEnvironmentLock")
	defer span.Finish()
	var previousVersion EslId

	//See if there is an existing lock with the same lock id in this environment. If it exists, just add a +1 to the eslversion
	existingEnvLock, err := h.DBSelectEnvironmentLock(ctx, tx, environment, lockID)

	if err != nil {
		return fmt.Errorf("Could not obtain existing environment lock: %w\n", err)
	}

	if existingEnvLock == nil {
		logger.FromContext(ctx).Sugar().Warnf("could not delete lock. The environment lock '%s' on environment '%s' does not exist. Continuing anyway.", lockID, environment)
		return nil
	}

	if existingEnvLock.Deleted {
		logger.FromContext(ctx).Sugar().Warnf("could not delete lock. The environment lock '%s' on environment '%s' has already been deleted. Continuing anyway.", lockID, environment)
		return nil
	} else {
		previousVersion = existingEnvLock.EslVersion
	}

	existingEnvLock.Deleted = true
	err = h.DBWriteEnvironmentLockInternal(ctx, tx, *existingEnvLock, previousVersion, false)

	if err != nil {
		return fmt.Errorf("could not delete environment lock from DB. Error: %w\n", err)
	}
	return nil
}

type AllEnvLocksJson struct {
	EnvLocks []string `json:"envLocks"`
}

type AllEnvLocksRow struct {
	Version     int64
	Created     time.Time
	Environment string
	Data        string
}

type AllEnvLocksGo struct {
	Version     int64
	Created     time.Time
	Environment string
	AllEnvLocksJson
}

type AllAppLocksJson struct {
	AppLocks []string `json:"appLocks"`
}

type AllAppLocksRow struct {
	Version     int64
	Created     time.Time
	Environment string
	AppName     string
	Data        string
}

type AllAppLocksGo struct {
	Version     int64
	Created     time.Time
	Environment string
	AppName     string
	AllAppLocksJson
}

type ApplicationLock struct {
	EslVersion EslId
	Created    time.Time
	LockID     string
	Env        string
	App        string
	Deleted    bool
	Metadata   LockMetadata
}

// DBApplicationLock Just used to fetch info from DB
type DBApplicationLock struct {
	EslVersion EslId
	Created    time.Time
	LockID     string
	Env        string
	App        string
	Deleted    bool
	Metadata   string
}

func (h *DBHandler) DBWriteAllAppLocks(ctx context.Context, transaction *sql.Tx, previousVersion int64, environment, appName string, lockIds []string) error {
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteAllAppLocks")
	defer span.Finish()
	slices.Sort(lockIds) // we don't really *need* the sorting, it's just for convenience
	jsonToInsert, err := json.Marshal(AllAppLocksJson{
		AppLocks: lockIds,
	})
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}
	insertQuery := h.AdaptQuery("INSERT INTO all_app_locks (version , created, environment, appName, json)  VALUES (?, ?, ?, ?, ?);")
	span.SetTag("query", insertQuery)
	_, err = transaction.Exec(
		insertQuery,
		previousVersion+1,
		time.Now().UTC(),
		environment,
		appName,
		jsonToInsert)
	if err != nil {
		return fmt.Errorf("could not insert all app locks into DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) DBSelectAllAppLocks(ctx context.Context, tx *sql.Tx, environment, appName string) (*AllAppLocksGo, error) {
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAllAppLocks: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBSelectAllAppLocks")
	defer span.Finish()
	selectQuery := h.AdaptQuery(
		"SELECT version, created, environment, appName, json FROM all_app_locks WHERE environment = ? AND appName = ? ORDER BY version DESC LIMIT 1;")

	rows, err := tx.QueryContext(ctx, selectQuery, environment, appName)
	if err != nil {
		return nil, fmt.Errorf("could not query all app locks table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	if rows.Next() {
		var row = AllAppLocksRow{
			Version:     0,
			Created:     time.Time{},
			Environment: "",
			AppName:     "",
			Data:        "",
		}

		err := rows.Scan(&row.Version, &row.Created, &row.Environment, &row.AppName, &row.Data)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning application locks row from DB. Error: %w\n", err)
		}

		//exhaustruct:ignore
		var resultJson = AllAppLocksJson{}
		err = json.Unmarshal(([]byte)(row.Data), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Data)
		}

		var resultGo = AllAppLocksGo{
			Version:         row.Version,
			Created:         row.Created,
			Environment:     row.Environment,
			AppName:         row.AppName,
			AllAppLocksJson: AllAppLocksJson{AppLocks: resultJson.AppLocks},
		}
		err = closeRows(rows)
		if err != nil {
			return nil, err
		}
		return &resultGo, nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (h *DBHandler) DBSelectAppLock(ctx context.Context, tx *sql.Tx, environment, appName, lockID string) (*ApplicationLock, error) {
	selectQuery := h.AdaptQuery(fmt.Sprintf(
		"SELECT eslVersion, created, lockID, envName, appName, metadata, deleted" +
			" FROM application_locks " +
			" WHERE envName=? AND appName=? AND lockID=? " +
			" ORDER BY eslVersion DESC " +
			" LIMIT 1;"))

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environment,
		appName,
		lockID,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query application locks table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	if rows.Next() {
		var row = DBApplicationLock{
			EslVersion: 0,
			Created:    time.Time{},
			LockID:     "",
			Env:        "",
			App:        "",
			Deleted:    true,
			Metadata:   "",
		}

		err := rows.Scan(&row.EslVersion, &row.Created, &row.LockID, &row.Env, &row.App, &row.Metadata, &row.Deleted)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning application locks row from DB. Error: %w\n", err)
		}

		//exhaustruct:ignore
		var resultJson = LockMetadata{}
		err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Metadata)
		}
		err = closeRows(rows)
		if err != nil {
			return nil, err
		}
		return &ApplicationLock{
			EslVersion: row.EslVersion,
			Created:    row.Created,
			LockID:     row.LockID,
			Env:        row.Env,
			App:        row.App,
			Deleted:    row.Deleted,
			Metadata:   resultJson,
		}, nil
	}

	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return nil, nil // no rows, but also no error

}

func (h *DBHandler) DBSelectAppLockSet(ctx context.Context, tx *sql.Tx, environment, appName string, lockIDs []string) ([]ApplicationLock, error) {
	if len(lockIDs) == 0 {
		return nil, nil
	}
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAppLockSet: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBSelectAppLockSet")
	defer span.Finish()

	var appLocks []ApplicationLock
	var rows *sql.Rows
	defer func(rows *sql.Rows) {
		if rows == nil {
			return
		}
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)
	//Get the latest change to each lock
	for _, id := range lockIDs {
		var err error
		selectQuery := h.AdaptQuery(
			"SELECT eslVersion, created, lockID, envName, appName, metadata, deleted" +
				" FROM application_locks " +
				" WHERE envName=? AND lockID=? AND appName=?" +
				" ORDER BY eslVersion DESC " +
				" LIMIT 1;")
		rows, err = tx.QueryContext(ctx, selectQuery, environment, id, appName)
		if err != nil {
			return nil, fmt.Errorf("could not query application locks table from DB. Error: %w\n", err)
		}

		var row = DBApplicationLock{
			EslVersion: 0,
			Created:    time.Time{},
			LockID:     "",
			Env:        "",
			App:        "",
			Deleted:    false,
			Metadata:   "",
		}
		if rows.Next() {
			err = rows.Scan(&row.EslVersion, &row.Created, &row.LockID, &row.Env, &row.App, &row.Metadata, &row.Deleted)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, nil
				}
				return nil, fmt.Errorf("Error scanning application locks row from DB. Error: %w\n", err)
			}

			//exhaustruct:ignore
			var resultJson = LockMetadata{}
			err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
			if err != nil {
				return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Metadata)
			}
			appLocks = append(appLocks, ApplicationLock{
				EslVersion: row.EslVersion,
				Created:    row.Created,
				LockID:     row.LockID,
				Env:        row.Env,
				App:        row.App,
				Deleted:    row.Deleted,
				Metadata:   resultJson,
			})
		}
		err = closeRows(rows)
		if err != nil {
			return nil, err
		}
	}
	err := closeRows(rows)
	if err != nil {
		return nil, err
	}
	return appLocks, nil
}

func (h *DBHandler) DBWriteApplicationLock(ctx context.Context, tx *sql.Tx, lockID, environment, appName, message, authorName, authorEmail string) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteApplicationLock: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteApplicationLock")
	defer span.Finish()

	var previousVersion EslId

	existingEnvLock, err := h.DBSelectAppLock(ctx, tx, environment, appName, lockID)

	if err != nil {
		return fmt.Errorf("Could not obtain existing application lock: %w\n", err)
	}

	if existingEnvLock == nil {
		previousVersion = 0
	} else {
		previousVersion = existingEnvLock.EslVersion
	}

	appLock := ApplicationLock{
		EslVersion: 0,
		LockID:     lockID,
		Created:    time.Time{},
		Env:        environment,
		Metadata: LockMetadata{
			Message:        message,
			CreatedByName:  authorName,
			CreatedByEmail: authorEmail,
		},
		App:     appName,
		Deleted: false,
	}
	return h.DBWriteApplicationLockInternal(ctx, tx, appLock, previousVersion, false)
}

func (h *DBHandler) DBWriteApplicationLockInternal(ctx context.Context, tx *sql.Tx, appLock ApplicationLock, previousEslVersion EslId, useTimeInLock bool) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteApplicationLockInternal: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteApplicationLockInternal")
	defer span.Finish()

	jsonToInsert, err := json.Marshal(appLock.Metadata)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	insertQuery := h.AdaptQuery(
		"INSERT INTO application_locks (eslVersion, created, lockID, envName, appName, deleted, metadata) VALUES (?, ?, ?, ?, ?, ?, ?);")

	var timetoInsert time.Time
	if useTimeInLock {
		timetoInsert = appLock.Created
	} else {
		timetoInsert = time.Now().UTC()
	}
	span.SetTag("query", insertQuery)
	_, err = tx.Exec(
		insertQuery,
		previousEslVersion+1,
		timetoInsert,
		appLock.LockID,
		appLock.Env,
		appLock.App,
		appLock.Deleted,
		jsonToInsert)

	if err != nil {
		return fmt.Errorf("could not write application lock into DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) DBDeleteApplicationLock(ctx context.Context, tx *sql.Tx, environment, appName, lockID string) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBDeleteApplicationLock: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBDeleteApplicationLock")
	defer span.Finish()
	var previousVersion EslId

	existingAppLock, err := h.DBSelectAppLock(ctx, tx, environment, appName, lockID)

	if err != nil {
		return fmt.Errorf("Could not obtain existing application lock: %w\n", err)
	}

	if existingAppLock == nil {
		logger.FromContext(ctx).Sugar().Warnf("could not delete application lock. The application lock '%s' on application '%s' on environment '%s' does not exist. Continuing anyway.", lockID, appName, environment)
		return nil
	}
	if existingAppLock.Deleted {
		logger.FromContext(ctx).Sugar().Warnf("could not delete application lock. The application lock '%s' on application '%s' on environment '%s' has already been deleted. Continuing anyway.", lockID, appName, environment)
		return nil
	} else {
		previousVersion = existingAppLock.EslVersion
	}

	existingAppLock.Deleted = true
	err = h.DBWriteApplicationLockInternal(ctx, tx, *existingAppLock, previousVersion, false)

	if err != nil {
		return fmt.Errorf("could not delete application lock from DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) DBSelectAnyActiveAppLock(ctx context.Context, tx *sql.Tx) (*AllAppLocksGo, error) {
	selectQuery := h.AdaptQuery(
		"SELECT version, created, environment, appName, json FROM all_app_locks ORDER BY version DESC LIMIT 1;")
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query all_app_locks table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	//exhaustruct:ignore
	var row = AllAppLocksRow{}
	if rows.Next() {
		err := rows.Scan(&row.Version, &row.Created, &row.Environment, &row.AppName, &row.Data)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning application lock row from DB. Error: %w\n", err)
		}
		err = closeRows(rows)
		if err != nil {
			return nil, err
		}
		//exhaustruct:ignore
		var dataJson = AllAppLocksJson{}
		err = json.Unmarshal(([]byte)(row.Data), &dataJson)
		if err != nil {
			return nil, fmt.Errorf("Error unmarshalling error. Error: %w\n", err)
		}
		return &AllAppLocksGo{
			Version:         row.Version,
			Created:         row.Created,
			Environment:     row.Environment,
			AppName:         row.AppName,
			AllAppLocksJson: AllAppLocksJson{AppLocks: dataJson.AppLocks}}, nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return nil, nil // no rows, but also no error
}

// DBSelectAppLockHistory returns the last N events associated with some lock on some environment for some app. Currently only used in testing.
func (h *DBHandler) DBSelectAppLockHistory(ctx context.Context, tx *sql.Tx, environmentName, appName, lockID string, limit int) ([]ApplicationLock, error) {
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAppLockHistory: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBSelectAppLockHistory")
	defer span.Finish()

	selectQuery := h.AdaptQuery(
		fmt.Sprintf(
			"SELECT eslVersion, created, lockID, envName, appName, metadata, deleted" +
				" FROM application_locks " +
				" WHERE envName=? AND lockID=? AND appName=?" +
				" ORDER BY eslVersion DESC " +
				" LIMIT ?;"))

	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environmentName,
		lockID,
		appName,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("could not read application lock from DB. Error: %w\n", err)
	}
	envLocks := make([]ApplicationLock, 0)
	for rows.Next() {
		var row = DBApplicationLock{
			EslVersion: 0,
			Created:    time.Time{},
			LockID:     "",
			App:        "",
			Env:        "",
			Deleted:    true,
			Metadata:   "",
		}

		err := rows.Scan(&row.EslVersion, &row.Created, &row.LockID, &row.Env, &row.App, &row.Metadata, &row.Deleted)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning application locks row from DB. Error: %w\n", err)
		}

		//exhaustruct:ignore
		var resultJson = LockMetadata{}
		err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Metadata)
		}
		envLocks = append(envLocks, ApplicationLock{
			EslVersion: row.EslVersion,
			Created:    row.Created,
			LockID:     row.LockID,
			Env:        row.Env,
			Deleted:    row.Deleted,
			App:        row.App,
			Metadata:   resultJson,
		})
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return envLocks, nil
}

type AllTeamLocksJson struct {
	TeamLocks []string `json:"teamLocks"`
}

type AllTeamLocksRow struct {
	Version     int64
	Created     time.Time
	Environment string
	Team        string
	Data        string
}

type AllTeamLocksGo struct {
	Version     int64
	Created     time.Time
	Environment string
	Team        string
	AllTeamLocksJson
}

type TeamLock struct {
	EslVersion EslId
	Created    time.Time
	LockID     string
	Env        string
	Team       string
	Deleted    bool
	Metadata   LockMetadata
}

// DBTeamLock Just used to fetch info from DB
type DBTeamLock struct {
	EslVersion EslId
	Created    time.Time
	LockID     string
	Env        string
	TeamName   string
	Deleted    bool
	Metadata   string
}

func (h *DBHandler) DBSelectAnyActiveTeamLock(ctx context.Context, tx *sql.Tx) (*AllTeamLocksGo, error) {
	selectQuery := h.AdaptQuery(
		"SELECT version, created, environment, teamName, json FROM all_team_locks ORDER BY version DESC LIMIT 1;")
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	return h.processAllTeamLocksRow(ctx, err, rows)
}

func (h *DBHandler) DBWriteTeamLock(ctx context.Context, tx *sql.Tx, lockID, environment, teamName, message, authorName, authorEmail string) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteTeamLock: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteTeamLock")
	defer span.Finish()

	var previousVersion EslId

	existingEnvLock, err := h.DBSelectTeamLock(ctx, tx, environment, teamName, lockID)

	if err != nil {
		return fmt.Errorf("Could not obtain existing team lock: %w\n", err)
	}

	if existingEnvLock == nil {
		previousVersion = 0
	} else {
		previousVersion = existingEnvLock.EslVersion
	}

	teamLock := TeamLock{
		EslVersion: 0,
		LockID:     lockID,
		Created:    time.Time{},
		Env:        environment,
		Metadata: LockMetadata{
			Message:        message,
			CreatedByName:  authorName,
			CreatedByEmail: authorEmail,
		},
		Team:    teamName,
		Deleted: false,
	}
	return h.DBWriteTeamLockInternal(ctx, tx, teamLock, previousVersion, false)
}

func (h *DBHandler) DBWriteTeamLockInternal(ctx context.Context, tx *sql.Tx, teamLock TeamLock, previousEslVersion EslId, useTimeInLock bool) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteTeamLockInternal: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteTeamLockInternal")
	defer span.Finish()

	jsonToInsert, err := json.Marshal(teamLock.Metadata)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	insertQuery := h.AdaptQuery(
		"INSERT INTO team_locks (eslVersion, created, lockID, envName, teamName, deleted, metadata) VALUES (?, ?, ?, ?, ?, ?, ?);")

	var timetoInsert time.Time
	if useTimeInLock {
		timetoInsert = teamLock.Created
	} else {
		timetoInsert = time.Now().UTC()
	}
	span.SetTag("query", insertQuery)
	_, err = tx.Exec(
		insertQuery,
		previousEslVersion+1,
		timetoInsert,
		teamLock.LockID,
		teamLock.Env,
		teamLock.Team,
		teamLock.Deleted,
		jsonToInsert)

	if err != nil {
		return fmt.Errorf("could not write team lock into DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) DBWriteAllTeamLocks(ctx context.Context, transaction *sql.Tx, previousVersion int64, environment, teamName string, lockIds []string) error {
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteAllTeamLocks")
	defer span.Finish()
	slices.Sort(lockIds) // we don't really *need* the sorting, it's just for convenience
	jsonToInsert, err := json.Marshal(AllTeamLocksJson{
		TeamLocks: lockIds,
	})
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}
	insertQuery := h.AdaptQuery("INSERT INTO all_team_locks (version , created, environment, teamName, json)  VALUES (?, ?, ?, ?, ?);")
	span.SetTag("query", insertQuery)
	_, err = transaction.Exec(
		insertQuery,
		previousVersion+1,
		time.Now().UTC(),
		environment,
		teamName,
		jsonToInsert)
	if err != nil {
		return fmt.Errorf("could not insert all team locks into DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) DBSelectTeamLock(ctx context.Context, tx *sql.Tx, environment, teamName, lockID string) (*TeamLock, error) {
	selectQuery := h.AdaptQuery(fmt.Sprintf(
		"SELECT eslVersion, created, lockID, envName, teamName, metadata, deleted" +
			" FROM team_locks " +
			" WHERE envName=? AND teamName=? AND lockID=? " +
			" ORDER BY eslVersion DESC " +
			" LIMIT 1;"))

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environment,
		teamName,
		lockID,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query team locks table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	if rows.Next() {
		var row = DBTeamLock{
			EslVersion: 0,
			Created:    time.Time{},
			LockID:     "",
			Env:        "",
			TeamName:   "",
			Deleted:    true,
			Metadata:   "",
		}

		err := rows.Scan(&row.EslVersion, &row.Created, &row.LockID, &row.Env, &row.TeamName, &row.Metadata, &row.Deleted)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning team locks row from DB. Error: %w\n", err)
		}

		//exhaustruct:ignore
		var resultJson = LockMetadata{}
		err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Metadata)
		}
		err = closeRows(rows)
		if err != nil {
			return nil, err
		}
		return &TeamLock{
			EslVersion: row.EslVersion,
			Created:    row.Created,
			LockID:     row.LockID,
			Env:        row.Env,
			Team:       row.TeamName,
			Deleted:    row.Deleted,
			Metadata:   resultJson,
		}, nil
	}

	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return nil, nil // no rows, but also no error
}
func (h *DBHandler) DBSelectAllTeamLocks(ctx context.Context, tx *sql.Tx, environment, teamName string) (*AllTeamLocksGo, error) {
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAllTeamLocks: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBSelectAllTeamLocks")
	defer span.Finish()
	selectQuery := h.AdaptQuery(
		"SELECT version, created, environment, teamName, json FROM all_team_locks WHERE environment = ? AND teamName = ? ORDER BY version DESC LIMIT 1;")

	rows, err := tx.QueryContext(ctx, selectQuery, environment, teamName)
	return h.processAllTeamLocksRow(ctx, err, rows)
}

func (h *DBHandler) DBDeleteTeamLock(ctx context.Context, tx *sql.Tx, environment, teamName, lockID string) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBDeleteTeamLock: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBDeleteTeamLock")
	defer span.Finish()
	var previousVersion EslId

	existingTeamLock, err := h.DBSelectTeamLock(ctx, tx, environment, teamName, lockID)

	if err != nil {
		return fmt.Errorf("Could not obtain existing team lock: %w\n", err)
	}

	if existingTeamLock == nil {
		logger.FromContext(ctx).Sugar().Warnf("could not delete team lock. The team lock '%s' on team '%s' on environment '%s' does not exist. Continuing anyway.", lockID, teamName, environment)
		return nil
	}
	if existingTeamLock.Deleted {
		logger.FromContext(ctx).Sugar().Warnf("could not delete team lock. The team lock '%s' on team '%s' on environment '%s' has already been deleted. Continuing anyway.", lockID, teamName, environment)
		return nil
	} else {
		previousVersion = existingTeamLock.EslVersion
	}

	existingTeamLock.Deleted = true
	err = h.DBWriteTeamLockInternal(ctx, tx, *existingTeamLock, previousVersion, false)

	if err != nil {
		return fmt.Errorf("could not delete team lock from DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) DBSelectTeamLockSet(ctx context.Context, tx *sql.Tx, environment, teamName string, lockIDs []string) ([]TeamLock, error) {
	if len(lockIDs) == 0 {
		return nil, nil
	}
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectTeamLockSet: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBSelectTeamLockSet")
	defer span.Finish()

	var teamLocks []TeamLock
	var rows *sql.Rows
	defer func(rows *sql.Rows) {
		if rows == nil {
			return
		}
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)
	//Get the latest change to each lock
	for _, id := range lockIDs {
		var err error
		selectQuery := h.AdaptQuery(
			"SELECT eslVersion, created, lockID, envName, teamName, metadata, deleted" +
				" FROM team_locks " +
				" WHERE envName=? AND lockID=? AND teamName=?" +
				" ORDER BY eslVersion DESC " +
				" LIMIT 1;")
		rows, err = tx.QueryContext(ctx, selectQuery, environment, id, teamName)
		if err != nil {
			return nil, fmt.Errorf("could not query team locks table from DB. Error: %w\n", err)
		}

		var row = DBTeamLock{
			EslVersion: 0,
			Created:    time.Time{},
			LockID:     "",
			Env:        "",
			TeamName:   "",
			Deleted:    false,
			Metadata:   "",
		}
		if rows.Next() {
			err = rows.Scan(&row.EslVersion, &row.Created, &row.LockID, &row.Env, &row.TeamName, &row.Metadata, &row.Deleted)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, nil
				}
				return nil, fmt.Errorf("Error scanning team locks row from DB. Error: %w\n", err)
			}

			//exhaustruct:ignore
			var resultJson = LockMetadata{}
			err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
			if err != nil {
				return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Metadata)
			}
			teamLocks = append(teamLocks, TeamLock{
				EslVersion: row.EslVersion,
				Created:    row.Created,
				LockID:     row.LockID,
				Env:        row.Env,
				Team:       row.TeamName,
				Deleted:    row.Deleted,
				Metadata:   resultJson,
			})
		}
		err = closeRows(rows)
		if err != nil {
			return nil, err
		}
	}
	err := closeRows(rows)
	if err != nil {
		return nil, err
	}
	return teamLocks, nil
}

// DBSelectTeamLockHistory returns the last N events associated with some lock on some environment for some team. Currently only used in testing.
func (h *DBHandler) DBSelectTeamLockHistory(ctx context.Context, tx *sql.Tx, environmentName, teamName, lockID string, limit int) ([]TeamLock, error) {
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectTeamLockHistory: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBSelectTeamLockHistory")
	defer span.Finish()

	selectQuery := h.AdaptQuery(
		fmt.Sprintf(
			"SELECT eslVersion, created, lockID, envName, teamName, metadata, deleted" +
				" FROM team_locks " +
				" WHERE envName=? AND lockID=? AND teamName=?" +
				" ORDER BY eslVersion DESC " +
				" LIMIT ?;"))

	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environmentName,
		lockID,
		teamName,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("could not read team lock from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("team locks: row could not be closed: %v", err)
		}
	}(rows)
	teamLocks := make([]TeamLock, 0)
	for rows.Next() {
		var row = DBTeamLock{
			EslVersion: 0,
			Created:    time.Time{},
			LockID:     "",
			TeamName:   "",
			Env:        "",
			Deleted:    true,
			Metadata:   "",
		}

		err := rows.Scan(&row.EslVersion, &row.Created, &row.LockID, &row.Env, &row.TeamName, &row.Metadata, &row.Deleted)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning team locks row from DB. Error: %w\n", err)
		}

		//exhaustruct:ignore
		var resultJson = LockMetadata{}
		err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Metadata)
		}
		teamLocks = append(teamLocks, TeamLock{
			EslVersion: row.EslVersion,
			Created:    row.Created,
			LockID:     row.LockID,
			Env:        row.Env,
			Deleted:    row.Deleted,
			Team:       row.TeamName,
			Metadata:   resultJson,
		})
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return teamLocks, nil
}

func (h *DBHandler) processAllTeamLocksRow(ctx context.Context, err error, rows *sql.Rows) (*AllTeamLocksGo, error) {
	if err != nil {
		return nil, fmt.Errorf("could not query all_team_locks table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("releases: row could not be closed: %v", err)
		}
	}(rows)
	//exhaustruct:ignore
	var row = &AllTeamLocksRow{}
	var result *AllTeamLocksGo
	if rows.Next() {

		err := rows.Scan(&row.Version, &row.Created, &row.Environment, &row.Team, &row.Data)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning releases row from DB. Error: %w\n", err)
		}
		//exhaustruct:ignore
		var resultJson = AllTeamLocksJson{}
		err = json.Unmarshal(([]byte)(row.Data), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Data)
		}

		result = &AllTeamLocksGo{
			Version:          row.Version,
			Created:          row.Created,
			Environment:      row.Environment,
			Team:             row.Team,
			AllTeamLocksJson: AllTeamLocksJson{TeamLocks: resultJson.TeamLocks},
		}
	} else {
		row = nil
		result = nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type QueuedDeployment struct {
	EslVersion EslId
	Created    time.Time
	Env        string
	App        string
	Version    *int64
}

func (h *DBHandler) DBSelectAnyDeploymentAttempt(ctx context.Context, tx *sql.Tx) (*QueuedDeployment, error) {
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAnyDeploymentAttempt: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBSelectAnyDeploymentAttempt")
	defer span.Finish()

	insertQuery := h.AdaptQuery(
		"SELECT eslVersion, created, envName, appName, queuedReleaseVersion FROM deployment_attempts ORDER BY eslVersion DESC LIMIT 1;")

	span.SetTag("query", insertQuery)
	rows, err := tx.QueryContext(
		ctx,
		insertQuery)
	return h.processDeploymentAttemptsRow(ctx, rows, err)
}

func (h *DBHandler) DBSelectDeploymentAttemptHistory(ctx context.Context, tx *sql.Tx, environmentName, appName string, limit int) ([]QueuedDeployment, error) {
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectDeploymentAttemptHistory: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBSelectDeploymentAttemptHistory")
	defer span.Finish()

	insertQuery := h.AdaptQuery(
		"SELECT eslVersion, created, envName, appName, queuedReleaseVersion FROM deployment_attempts WHERE envName=? AND appName=? ORDER BY eslVersion DESC LIMIT ?;")

	span.SetTag("query", insertQuery)
	rows, err := tx.QueryContext(
		ctx,
		insertQuery,
		environmentName,
		appName, limit)

	if err != nil {
		return nil, fmt.Errorf("could not query deployment attempts table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	queuedDeployments := make([]QueuedDeployment, 0)
	for rows.Next() {
		row, err := h.processSingleDeploymentAttemptsRow(rows)
		if err != nil {
			return nil, err
		}
		queuedDeployments = append(queuedDeployments, QueuedDeployment{
			EslVersion: row.EslVersion,
			Created:    row.Created,
			Env:        row.Env,
			App:        row.App,
			Version:    row.Version,
		})
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return queuedDeployments, nil
}

func (h *DBHandler) DBSelectLatestDeploymentAttempt(ctx context.Context, tx *sql.Tx, environmentName, appName string) (*QueuedDeployment, error) {
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectLatestDeploymentAttempt: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBSelectLatestDeploymentAttempt")
	defer span.Finish()

	insertQuery := h.AdaptQuery(
		"SELECT eslVersion, created, envName, appName, queuedReleaseVersion FROM deployment_attempts WHERE envName=? AND appName=? ORDER BY eslVersion DESC LIMIT 1;")

	span.SetTag("query", insertQuery)
	rows, err := tx.QueryContext(
		ctx,
		insertQuery,
		environmentName,
		appName)
	return h.processDeploymentAttemptsRow(ctx, rows, err)
}

func (h *DBHandler) DBWriteDeploymentAttempt(ctx context.Context, tx *sql.Tx, envName, appName string, version *int64) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteDeploymentAttempt: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteDeploymentAttempt")
	defer span.Finish()
	return h.dbWriteDeploymentAttemptInternal(ctx, tx, &QueuedDeployment{
		EslVersion: 0,
		Created:    time.Time{},
		Env:        envName,
		App:        appName,
		Version:    version,
	})
}

func (h *DBHandler) DBDeleteDeploymentAttempt(ctx context.Context, tx *sql.Tx, envName, appName string) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBDeleteDeploymentAttempt: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteDeploymentAttempt")
	defer span.Finish()

	return h.dbWriteDeploymentAttemptInternal(ctx, tx, &QueuedDeployment{
		EslVersion: 0,
		Created:    time.Time{},
		Env:        envName,
		App:        appName,
		Version:    nil,
	})
}

func (h *DBHandler) dbWriteDeploymentAttemptInternal(ctx context.Context, tx *sql.Tx, deployment *QueuedDeployment) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("dbWriteDeploymentAttemptInternal: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "dbWriteDeploymentAttemptInternal")
	defer span.Finish()

	latestDeployment, err := h.DBSelectLatestDeploymentAttempt(ctx, tx, deployment.Env, deployment.App)

	if err != nil {
		return fmt.Errorf("Could not get latest deployment attempt from deployments table")
	}
	var previousEslVersion EslId

	if latestDeployment == nil {
		previousEslVersion = 0
	} else {
		previousEslVersion = latestDeployment.EslVersion
	}
	nullVersion := NewNullInt(deployment.Version)

	insertQuery := h.AdaptQuery(
		"INSERT INTO deployment_attempts (eslVersion, created, envName, appName, queuedReleaseVersion) VALUES (?, ?, ?, ?, ?);")

	span.SetTag("query", insertQuery)
	_, err = tx.Exec(
		insertQuery,
		previousEslVersion+1,
		time.Now().UTC(),
		deployment.Env,
		deployment.App,
		nullVersion)

	if err != nil {
		return fmt.Errorf("could not write deployment attempts table in DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) processDeploymentAttemptsRow(ctx context.Context, rows *sql.Rows, err error) (*QueuedDeployment, error) {
	if err != nil {
		return nil, fmt.Errorf("could not query deployment attempts table from DB. Error: %w\n", err)

	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)
	var row *QueuedDeployment
	if rows.Next() {
		row, err = h.processSingleDeploymentAttemptsRow(rows)
		if err != nil {
			return nil, err
		}
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return row, nil
}

// processSingleDeploymentAttemptsRow only processes the row. It assumes that there is an element ready to be processed in rows.
func (h *DBHandler) processSingleDeploymentAttemptsRow(rows *sql.Rows) (*QueuedDeployment, error) {
	//exhaustruct:ignore

	var row = QueuedDeployment{}
	var releaseVersion sql.NullInt64

	err := rows.Scan(&row.EslVersion, &row.Created, &row.Env, &row.App, &releaseVersion)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("Error scanning deployment attempts row from DB. Error: %w\n", err)
	}

	if releaseVersion.Valid {
		row.Version = &releaseVersion.Int64
	}
	return &row, nil

}

// Environments

type DBAllEnvironments struct {
	Created      time.Time
	Version      int64
	Environments []string
}

type DBAllEnvironmentsRow struct {
	Created      time.Time
	Version      int64
	Environments string
}

type DBEnvironment struct {
	Created time.Time
	Version int64
	Name    string
	Config  config.EnvironmentConfig
}

type DBEnvironmentRow struct {
	Created time.Time
	Version int64
	Name    string
	Config  string
}

func (h *DBHandler) DBSelectEnvironment(ctx context.Context, tx *sql.Tx, environmentName string) (*DBEnvironment, error) {
	selectQuery := h.AdaptQuery(
		`
SELECT created, version, name, json
FROM environments
WHERE name=?
ORDER BY version DESC
LIMIT 1;
`,
	)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environmentName,
	)

	if err != nil {
		return nil, fmt.Errorf("could not query the environments table for environment %s, error: %w", environmentName, err)
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("error while closing row of environments, error: %w", err)
		}
	}(rows)

	if rows.Next() {
		//exhaustruct:ignore
		row := DBEnvironmentRow{}
		err := rows.Scan(&row.Created, &row.Version, &row.Name, &row.Config)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning the environments table, error: %w", err)
		}

		//exhaustruct:ignore
		parsedConfig := config.EnvironmentConfig{}
		err = json.Unmarshal([]byte(row.Config), &parsedConfig)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshal the JSON in the database, JSON: %s, error: %w", row.Config, err)
		}
		err = closeRows(rows)
		if err != nil {
			return nil, fmt.Errorf("error while closing database rows, error: %w", err)
		}

		return &DBEnvironment{
			Created: row.Created,
			Version: row.Version,
			Name:    environmentName,
			Config:  parsedConfig,
		}, nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, fmt.Errorf("errpr while closing database rows, error: %w", err)
	}
	return nil, nil
}

func (h *DBHandler) DBWriteEnvironment(ctx context.Context, tx *sql.Tx, environmentName string, environmentConfig config.EnvironmentConfig) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("attempting to write to the environmets table without a transaction")
	}

	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteEnvironment")
	defer span.Finish()

	jsonToInsert, err := json.Marshal(environmentConfig)
	if err != nil {
		return fmt.Errorf("error while marshalling the environment config %v, error: %w", environmentConfig, err)
	}

	existingEnvironment, err := h.DBSelectEnvironment(ctx, tx, environmentName)
	if err != nil {
		return fmt.Errorf("error while selecting environment %s from database, error: %w", environmentName, err)
	}

	var existingEnvironmentVersion int64
	if existingEnvironment == nil {
		existingEnvironmentVersion = 0
	} else {
		existingEnvironmentVersion = existingEnvironment.Version
	}

	insertQuery := h.AdaptQuery(
		"INSERT Into environments (created, version, name, json) VALUES (?, ?, ?, ?);",
	)

	span.SetTag("query", insertQuery)
	_, err = tx.Exec(
		insertQuery,
		time.Now(),
		existingEnvironmentVersion+1,
		environmentName,
		jsonToInsert,
	)
	if err != nil {
		return fmt.Errorf("could not write environment %s with config %v to environments table, error: %w", environmentName, environmentConfig, err)
	}
	return nil
}

func (h *DBHandler) DBSelectAllEnvironments(ctx context.Context, transaction *sql.Tx) (*DBAllEnvironments, error) {
	if h == nil {
		return nil, nil
	}
	if transaction == nil {
		return nil, fmt.Errorf("no transaction provided when selecting all environments from all_environments table")
	}

	span, _ := tracer.StartSpanFromContext(ctx, "DBSelectAllEnvironments")
	defer span.Finish()

	selectQuery := h.AdaptQuery(
		"SELECT created, version, json FROM all_environments ORDER BY version DESC LIMIT 1;",
	)

	rows, err := transaction.QueryContext(ctx, selectQuery)
	if err != nil {
		return nil, fmt.Errorf("error while execuring query to get all environments, error: %w", err)
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("error while closing row on all_environments table, error: %w", err)
		}
	}(rows)

	if rows.Next() {
		//exhaustruct:ignore
		row := DBAllEnvironmentsRow{}

		err := rows.Scan(&row.Created, &row.Version, &row.Environments)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error while scanning all_environments row, error: %w", err)
		}

		parsedEnvironments := make([]string, 0)
		err = json.Unmarshal([]byte(row.Environments), &parsedEnvironments)
		if err != nil {
			return nil, fmt.Errorf("error occured during JSON unmarshalling, JSON: %s, error: %w", row.Environments, err)
		}

		err = closeRows(rows)
		if err != nil {
			return nil, fmt.Errorf("error while closing rows, error: %w", err)
		}

		return &DBAllEnvironments{
			Created:      row.Created,
			Version:      row.Version,
			Environments: parsedEnvironments,
		}, nil
	}

	err = closeRows(rows)
	if err != nil {
		return nil, fmt.Errorf("error while closing rows, error: %w", err)
	}
	return nil, nil
}

func (h *DBHandler) DBWriteAllEnvironments(ctx context.Context, transaction *sql.Tx, environmentNames []string) error {
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteAllEnvironments")
	defer span.Finish()

	jsonToInsert, err := json.Marshal(environmentNames)
	if err != nil {
		return fmt.Errorf("could not marshal the environment names list %v, error: %w", environmentNames, err)
	}

	allEnvironments, err := h.DBSelectAllEnvironments(ctx, transaction)
	if err != nil {
		return fmt.Errorf("unable to select all environments, error: %w", err)
	}

	previousVersion := int64(0)
	if allEnvironments != nil {
		previousVersion = allEnvironments.Version
	}

	insertQuery := h.AdaptQuery("INSERT INTO all_environments (created, version, json) VALUES (?, ?, ?)")
	span.SetTag("query", insertQuery)
	_, err = transaction.Exec(
		insertQuery,
		time.Now(),
		previousVersion+1,
		jsonToInsert,
	)
	if err != nil {
		return fmt.Errorf("unable to perform the insert query, error: %w", err)
	}

	return nil
}

func (h *DBHandler) DBSelectAnyEnvironment(ctx context.Context, tx *sql.Tx) (*DBAllEnvironments, error) {
	selectQuery := h.AdaptQuery(
		"SELECT created, version, json FROM all_environments ORDER BY version DESC LIMIT 1;",
	)
	rows, err := tx.QueryContext(ctx, selectQuery)
	if err != nil {
		return nil, fmt.Errorf("could not query the all_environments table, error: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("error while closing the row of all_environments, error: %w", err)
		}
	}(rows)

	//exhaustruct:ignore
	row := DBAllEnvironmentsRow{}
	if rows.Next() {
		err := rows.Scan(&row.Created, &row.Version, &row.Environments)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning the results of the query for selecting any row in all_environments, error: %w", err)
		}
		err = closeRows(rows)
		if err != nil {
			return nil, fmt.Errorf("error while closing the rows of the query for selecting any row in all_environments, error: %w", err)
		}

		jsonData := make([]string, 0)
		err = json.Unmarshal([]byte(row.Environments), &jsonData)

		if err != nil {
			return nil, fmt.Errorf("error parsing the value of the JSON column of the all_environments table, JSON content: %s, error: %w", row.Environments, err)
		}

		return &DBAllEnvironments{
			Version:      row.Version,
			Created:      row.Created,
			Environments: jsonData,
		}, nil
	}

	err = closeRows(rows)
	if err != nil {
		return nil, fmt.Errorf("error while closing the rows of the query for selecting any row in all_environments (where no rows were returned), error: %w", err)
	}

	return nil, nil
}

func (h *DBHandler) RunCustomMigrationEnvironments(ctx context.Context, getAllEnvironmentsFun GetAllEnvironmentsFun) error {
	return h.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
		log := logger.FromContext(ctx).Sugar()

		arbitraryAllEnvsRow, err := h.DBSelectAnyEnvironment(ctx, transaction)

		if err != nil {
			return fmt.Errorf("unable to check if custom migration for environments has already occured, error: %w", err)
		}
		if arbitraryAllEnvsRow != nil {
			log.Infof("custom migration for environments already ran because row %v was found, skipping custom migration", arbitraryAllEnvsRow)
			return nil
		}

		allEnvironments, err := getAllEnvironmentsFun(ctx)
		if err != nil {
			return fmt.Errorf("could not get environments, error: %w", err)
		}

		allEnvironmentNames := make([]string, 0)
		for envName, config := range allEnvironments {
			allEnvironmentNames = append(allEnvironmentNames, envName)
			err = h.DBWriteEnvironment(ctx, transaction, envName, config)
			if err != nil {
				return fmt.Errorf("unable to write manifest for environment %s to the database, error: %w", envName, err)
			}
		}
		err = h.DBWriteAllEnvironments(ctx, transaction, allEnvironmentNames)
		if err != nil {
			return fmt.Errorf("unable to write to write all environments list to the database, error: %w", err)
		}
		return nil
	})
}
