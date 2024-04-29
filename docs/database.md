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
sqlite3 cd_database/db.sqlite
```

To make sqlite print nicely formatted columns,
write the following to the file `~/.sqliterc`:
```text
.headers on
.mode column
```

