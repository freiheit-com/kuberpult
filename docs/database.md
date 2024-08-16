# Database

## The Database feature is not ready for production yet

However, you can already prepare for the database feature,
see [Preparation](#preparation)


## Background

Kuberpult is switching over to use a database. The rough timeline to have the database production-ready is summer 2024.
As of now Kuberpult is using the manifest repository to store data.
This worked fine for a while, and it had the added bonus
that ArgoCD is reading from the same manifest git repository.

However, with a high number of apps/clusters, this does not scale so well, and
kuberpult becomes slower and slower. Especially pushing to git as well as cloning
the repo on startup are very slow operations.

Therefore, we will use a database and not rely on git anymore in the future.

Git will still be used as an *output* of kuberpult, but not as the source of truth.

As of now, kuberpult still supports the option to not have the database,
but this option will be removed in a few weeks with another breaking change in kuberpult.


## Preparation

Our recommendation is to enable the database mode in 2 steps:

### Step 1: writeEslTableOnly=true

Enable the Database with `db.dbOption: "postgreSQL"` and `db.writeEslTableOnly: true`.
This means that kuberpult will connect to the DB, but only write one table.
Kuberpult will not read from the database in this state,
so the manifest-repository is still considered the source of truth.
You can use this option as a proof of concept that the database connection works.

This is also a good time to create alerts and backups for the database itself.
If you are using datadog (`datadogTracing.enabled: true`), then it can also be helpful
to inspect some traces/spans, to see how long operations take with the database.
Kuberpult generates spans for each database query. These should generally take
only a few milliseconds (10-20), otherwise the database needs more resources.

Apart from the general Database monitors that every database should have,
we also recommend setting up alerts for these kuberpult specific metrics.

Note that these metrics only work if the database is fully rolled out, so only after 
"Step 2: writeEslTableOnly=false".

#### Monitoring Push Failures

Metric `Kuberpult.manifest_export_push_failures`.
This metric measures each failure in the manifest-repo-export-service to push to the git repository.
It measures 0 if there are currently no failures, and 1 if there are.

This metric is only written, if there is something for kuberpult to push.


#### Monitoring Processing Delay

Metric: `Kuberpult.process_delay_seconds`.
This metric measures the time in seconds between "now" and the creation time of the currently processed event
in the manifest-repo-export-service.
This is essentially the time difference between writing to the database,
and writing to the git repo.

If there are lots of events to process (or "big" events like release trains),
this may be significant.

Consider alerting when this value is >= 10 minutes.


### Step 2: writeEslTableOnly=false
Enable the Database with `db.dbOption: "postgreSQL"` and  `db.writeEslTableOnly: false`.
Kuberpult will now read and write from the database,
so the database is the (only) source of truth.
The manifest repository is now only an "export", which is handled by the new `manifest-repo-export-service`.
On the first startup with this option, it will read the manifest repo and insert all needed data
into the database (about 25 tables). This process can take a few minutes,
depending on the size of your repository and the resources you provide to both the database and Kuberpult's cd-service.
We tested this with hundreds of apps and dozens of environments,
and the migration was done in about 5-10 minutes.


## Pushing to the manifest repo
With the database option fully enabled, it is recommended to *never* push into the repo manually,
since kuberpult will not take these changes into account.


## Database Variants

For integration tests, Kuberpult use [SQLite](https://www.sqlite.org/).

For use in production and also local development Kuberpult supports [PostgreSQL](https://www.postgresql.org/),
e.g. [Cloud SQL on GCP](https://cloud.google.com/sql?hl=en).


## Hints for Kuberpult Developers

When using sqlite, make sure that kuberpult is running first, otherwise you won't have
the sqlite file yet.

Then you can look into the database, by using the sqlite command line client:
```shell
sqlite3 database/db.sqlite
```
or for postgres:
```shell
PGPASSWORD=mypassword psql -h localhost -p 5432 -U postgres -d kuberpult
```

To make sqlite print nicely formatted columns,
write the following to the file `~/.sqliterc`:
```text
.headers on
.mode column
```


#### Best Practice (Dev Notes)

To implement the database modes correctly,
we must use the functions `ShouldUseEslTable` before writing to the ESL table,
and `ShouldUseOtherTables` before writing to any other table.
