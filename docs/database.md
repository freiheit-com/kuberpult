# Database

## The Database feature is not ready for production yet


## Background

Kuberpult is switching over to use a database. The rough timeline to have the database production-ready is summer 2024.
As of now Kuberpult is using the manifest repository to store data.
This worked fine for a while, and it had the added bonus
that ArgoCD is reading from the same manifest git repository.

However, with a high number of apps/clusters, this does not scale so well, and
kuberpult becomes slower and slower. Especially pushing to git as well as cloning
the repo on startup are very slow operations.

Therefore, we will connect to a database and not rely on git anymore in the future.


## Database Variants

Kuberpult avoids using language specific SQL features,
so we are not tied to one specific SQL dialect.

For local development and integration tests, we use [SQLite](https://www.sqlite.org/).

For use in production we support [PostgreSQL](https://www.postgresql.org/),
e.g. [Cloud SQL on GCP](https://cloud.google.com/sql?hl=en).

## Dev Hints

Make sure that kuberpult is running first, otherwise you won't have
the sqlite file yet.

Then you can look into the database, by using the sqlite command line client:
```shell
sqlite3 database/db.sqlite
```

To make sqlite print nicely formatted columns,
write the following to the file `~/.sqliterc`:
```text
.headers on
.mode column
```

### Database Modes

Kuberpult can run with 3 database modes:
1) No Database at all, just use the manifest repo as before
2) Use the Database only to write all incoming request in the "ESL" (event sourcing light) table.
This just records the history.
Note that the migrations are still running, so kuberpult will create some empty tables.
3) Use the Database for all tables. Note that this is not fully implemented yet.
With this mode, the cd-service writes to all database tables, and uses the manifest repo not at all.
Another service will be introduced to export the database content to a manifest repo.


#### Best Practice

To implement the database modes correctly,
we must use the functions `ShouldUseEslTable` before writing to the ESL table,
and `ShouldUseOtherTables` before writing to any other table.