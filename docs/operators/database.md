# Database

## The Database feature is now required
We expect that all Kuberpult users have fully switched to the Kuberpult version that uses the database at this point. 
The option to use Kuberpult without a database will be removed from future versions and support for 
migrations from older Kuberpult versions that do not use the database towards Kuberpult versions that use the database 
will only be possible in versions `v13.43.3` or earlier.

## Background

Kuberpult switched over to use a database.
In the past, Kuberpult was using the manifest repository to store data.
This worked fine for a while, and it had the added bonus
that ArgoCD is reading from the same manifest git repository.

However, with a high number of apps/clusters, this does not scale so well, and
kuberpult becomes slower and slower. Especially pushing to git as well as cloning
the repo on startup are very slow operations.

Therefore, we will use a database and not rely on git anymore in the future.

Git will still be used as an *output* of kuberpult, but not as the source of truth.

## Recommendations

Kuberpult reads and writes from the database, so the database is the (only) source of truth.
The manifest repository is only an "export", which is handled by the new `manifest-repo-export-service`.
It is recommended to *not* push into the manifest repo, since kuberpult will not take these changes into account.
However, in an urgent case, you can push files into it, including manifests for Argo CD. Each push will slow down the manifest-repo-export a little bit, because it needs to pull the changes again. This is why we recommend against pushing into it on a regular basis.
Note that if you do push manifests into it, the manifest-repo-export will override them when that service is deployed next time.

### Monitoring
Create alerts and backups for the database itself.
If you are using datadog (`datadogTracing.enabled: true`), then it can also be helpful
to inspect some traces/spans, to see how long operations take with the database.
Kuberpult generates spans for each database query. These should generally take
only a few milliseconds (10-20), otherwise the database needs more resources.

Apart from the general Database monitors that every database should have,
we also recommend setting up alerts for these kuberpult specific metrics.

#### Monitoring Push Failures

Metric `Kuberpult.manifest_export_push_failures`.
This metric measures each failure in the manifest-repo-export-service to push to the git repository.
It measures 0 if there are currently no failures, and 1 if there are.

This metric is allways written, even if there is nothing for kuberpult to push.
In case kuberpult has nothing to push this metric writes 0 every `manifestRepoExport.eslProcessingIdleTimeSeconds` seconds.

#### Monitoring Processing Delay

Metric: `Kuberpult.process_delay_seconds`.
This metric measures the time in seconds between "now" and the creation time of the currently processed event
in the manifest-repo-export-service.
This is essentially the time difference between writing to the database,
and writing to the git repo.

If there are lots of events to process (or "big" events like release trains),
this may be significant.

Consider alerting when this value is >= 10 minutes.

## Database Variants

For all database operations, kuberpult uses [PostgreSQL](https://www.postgresql.org/),
e.g. [Cloud SQL on GCP](https://cloud.google.com/sql?hl=en).

## Best Practice (Dev Notes)

## Custom Migrations
If you've deleted the custom migrations cutoff table and you want to bring it back you can run:
```Sql
CREATE TABLE IF NOT EXISTS custom_migration_cutoff
(
    migration_done_at TIMESTAMP NOT NULL,
    kuberpult_version varchar(100) PRIMARY KEY -- the version as it appears on GitHub, e.g. "1.2.3"
);
```
This way you can have this table back.