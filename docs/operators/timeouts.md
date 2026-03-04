
## Timeouts

There are a couple different timeouts that can be configured in kuberpult.

### `git.networkTimeout`

This is used for the git operations `fetch` and `push` in the manifest-export-service.

Pushing to a remote git repo is currently a bottleneck for kuberpult.

Note that for big repositories, it can take over a minute to push!

### `frontend.batchClient.timeout`

This is the time the frontend-service waits for the cd-service.
Must be `>= git.networkTimeout`.

### `cd.backendConfig.timeoutSec`

This is the timeout of the GCP loadbalancer, with a default (in GCP) of 30 seconds.

Must be `>= frontend.batchClient.timeout`.

