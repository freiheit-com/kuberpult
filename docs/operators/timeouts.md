
## Timeouts

There are a couple different timeouts that can be configured in kuberpult.

### `git.networkTimeout`

This is used for the git operations `fetch` and `push` in the manifest-export-service.

Pushing to a remote git repo is currently a bottleneck for kuberpult.

Note that for big repositories, it can take over a minute to push!

### `manifestRepoExport.networkTimeoutSeconds`

This is the timeout (in seconds, env var `KUBERPULT_NETWORK_TIMEOUT_SECONDS`) for the
manifest-repo-export-service's own git network operations, including its `push`. This is the timeout
that governs the export push — not `git.networkTimeout`, which is a separate cd-service knob.

### `manifestRepoExport.maxExportBatchSize`

The export can process a run of adjacent `CreateApplicationVersion` events as a single `git push`
(batching). `maxExportBatchSize` caps how many events go into one push. The default is a conservative
value greater than 1; set it to `1` to disable batching (one push per event) as a safe rollback.

A batched push still transfers all of the batch's commits' objects, so the batch's single push must
finish comfortably within `manifestRepoExport.networkTimeoutSeconds`. If a batch's push trips that
timeout, the cutoff does not advance and the *entire batch* reprocesses — strictly worse than not
batching. Choose `maxExportBatchSize` against the measured per-commit push cost (start conservative,
e.g. 5-10, and tune up with evidence), keeping the worst-case batch push well under
`networkTimeoutSeconds`.

### `frontend.batchClient.timeout`

This is the time the frontend-service waits for the cd-service.
Must be `>= git.networkTimeout`.

### `cd.backendConfig.timeoutSec`

This is the timeout of the GCP loadbalancer, with a default (in GCP) of 30 seconds.

Must be `>= frontend.batchClient.timeout`.

