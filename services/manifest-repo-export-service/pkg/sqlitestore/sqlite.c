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
#include <git2.h>
#include <git2/sys/odb_backend.h>
#include <sqlite3.h>
#include <string.h>
#include "sqlite.h"

typedef struct {
	git_odb_backend parent;
	sqlite3 *db;
	sqlite3_stmt *read;
	sqlite3_stmt *write;
	sqlite3_stmt *read_header;
} kp_backend;

int kp_backend__read_header(size_t *len_out, git_otype *type_out, git_odb_backend *_backend, const git_oid *oid)
{
	kp_backend *backend = (kp_backend *)_backend;
        if( sqlite3_bind_text(backend->read_header, 1, (char *)oid->id, 20, SQLITE_TRANSIENT) != SQLITE_OK) {
	  sqlite3_reset(backend->read_header);
          return GIT_ERROR;
        }
	if (sqlite3_step(backend->read_header) == SQLITE_ROW) {
	  *type_out = (git_otype)sqlite3_column_int(backend->read_header, 0);
	  *len_out = (size_t)sqlite3_column_int(backend->read_header, 1);
	  sqlite3_reset(backend->read_header);
	  return GIT_OK;
	} else {
	  sqlite3_reset(backend->read_header);
	  return GIT_ENOTFOUND;
	}
}

int kp_backend__read(void **data_out, size_t *len_out, git_otype *type_out, git_odb_backend *_backend, const git_oid *oid)
{
	kp_backend *backend = (kp_backend *)_backend;
	int error = GIT_ERROR;

	if (sqlite3_bind_text(backend->read, 1, (char *)oid->id, 20, SQLITE_TRANSIENT) != SQLITE_OK) {
	  sqlite3_reset(backend->read);
          return GIT_ERROR;
        }
	if (sqlite3_step(backend->read) == SQLITE_ROW) {
		*type_out = (git_otype)sqlite3_column_int(backend->read, 0);
		*len_out = (size_t)sqlite3_column_int(backend->read, 1);
		*data_out = malloc(*len_out);

		if (*data_out == NULL) {
			giterr_set_oom();
	                sqlite3_reset(backend->read);
			return GIT_ERROR;
		} else {
			memcpy(*data_out, sqlite3_column_blob(backend->read, 2), *len_out);
			sqlite3_reset(backend->read);
			return GIT_OK;
		}
	} else {
	    sqlite3_reset(backend->read);
	    return GIT_ENOTFOUND;
	}

}

int kp_backend__read_prefix(git_oid *out_oid, void **data_out, size_t *len_out, git_otype *type_out, git_odb_backend *_backend,
					const git_oid *short_oid, size_t len) {
	if (len >= GIT_OID_HEXSZ) {
		int error = kp_backend__read(data_out, len_out, type_out, _backend, short_oid);
		if (error == GIT_OK)
			git_oid_cpy(out_oid, short_oid);

		return error;
	}
	return GIT_ERROR;
}

int kp_backend__exists(git_odb_backend *_backend, const git_oid *oid)
{
	kp_backend *backend;
	int found;


	backend = (kp_backend *)_backend;
	found = 0;

	if (sqlite3_bind_text(backend->read_header, 1, (char *)oid->id, 20, SQLITE_TRANSIENT) == SQLITE_OK) {
		if (sqlite3_step(backend->read_header) == SQLITE_ROW) {
			found = 1;
		}
	}

	sqlite3_reset(backend->read_header);
	return found;
}


int kp_backend__write(git_odb_backend *_backend, const git_oid *id, const void *data, size_t len, git_otype type)
{
	int error;
	kp_backend *backend;


	backend = (kp_backend *)_backend;

	error = SQLITE_ERROR;

	if (sqlite3_bind_text(backend->write, 1, (char *)id->id, 20, SQLITE_TRANSIENT) == SQLITE_OK &&
		sqlite3_bind_int(backend->write, 2, (int)type) == SQLITE_OK &&
		sqlite3_bind_int(backend->write, 3, len) == SQLITE_OK &&
		sqlite3_bind_blob(backend->write, 4, data, len, SQLITE_TRANSIENT) == SQLITE_OK) {
		error = sqlite3_step(backend->write);
	}

	sqlite3_reset(backend->write);
	return (error == SQLITE_DONE) ? GIT_OK : GIT_ERROR;
}


void kp_backend__free(git_odb_backend *_backend)
{
	kp_backend *backend;
	backend = (kp_backend *)_backend;

	sqlite3_finalize(backend->read);
	sqlite3_finalize(backend->read_header);
	sqlite3_finalize(backend->write);
	sqlite3_close(backend->db);

	free(backend);
}

static int create_table_if_not_exists(sqlite3 *db)
{
	return sqlite3_exec(db, 
                "CREATE TABLE IF NOT EXISTS 'odb' ("
		"'oid' CHARACTER(20) PRIMARY KEY NOT NULL,"
		"'type' INTEGER NOT NULL,"
		"'size' INTEGER NOT NULL,"
		"'data' BLOB);", NULL, NULL, NULL);
}

static int init_statements(kp_backend *backend)
{
        int error = SQLITE_ERROR;
	error = sqlite3_prepare_v2(backend->db,
			    "SELECT type, size, data FROM 'odb' WHERE oid = ?;"
			    , -1, &backend->read, NULL);
        if( error != SQLITE_OK ){
		return error;
        }
        error = sqlite3_prepare_v2(backend->db,
				   "SELECT type, size FROM 'odb' WHERE oid = ?;",
				   -1, &backend->read_header, NULL);
        if( error != SQLITE_OK ){
		return error;
        }
	return sqlite3_prepare_v2(backend->db,
			   "INSERT OR IGNORE INTO 'odb' VALUES (?, ?, ?, ?);",
			   -1, &backend->write, NULL);
}

int kp_backend_sqlite(git_odb_backend **backend_out, const char *sqlite_db, const char **err_out)
{
	int error = SQLITE_ERROR;
	kp_backend *backend = calloc(1, sizeof(kp_backend));
	if (backend == NULL) {
		giterr_set_oom();
		return SQLITE_ERROR;
	}

        error = sqlite3_open(sqlite_db, &backend->db);
	if (error != SQLITE_OK){
          *err_out = sqlite3_errmsg(backend->db);
	  kp_backend__free((git_odb_backend *)backend);
	  return error;
        }

	error = create_table_if_not_exists(backend->db);
	if (error != SQLITE_OK){
          *err_out = sqlite3_errmsg(backend->db);
	  kp_backend__free((git_odb_backend *)backend);
	  return error;
        }

	error = init_statements(backend);
	if (error != SQLITE_OK) {
          *err_out = sqlite3_errmsg(backend->db);
	  kp_backend__free((git_odb_backend *)backend);
	  return error;
        }

	backend->parent.version = GIT_ODB_BACKEND_VERSION;
	backend->parent.read = &kp_backend__read;
	backend->parent.read_prefix = &kp_backend__read_prefix;
	backend->parent.read_header = &kp_backend__read_header;
	backend->parent.write = &kp_backend__write;
	backend->parent.exists = &kp_backend__exists;
	backend->parent.free = &kp_backend__free;

	*backend_out = (git_odb_backend *)backend;
        return SQLITE_OK;
}
