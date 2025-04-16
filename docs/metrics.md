## Datadog Metrics
Kuberpult uploads mulitple metrics to datadog.

### `cd-service` Metrics
The cd-service uploads the following metrics to datadog:
* `environment_lock_count` - the count of current environment locks, for a given environment;
* `application_lock_count` - the count of current environment application locks, for a given application in a given environment;
* `lastDeployed` - the time since the last deployment in minutes;
* `request_queue_size` - the current size of the request queue;
* `git_sync_unsynced` - Number of the applications that have unsynced git sync status
* `git_sync_failed` - Number of the applications that have failed git sync status

### `manifest-repo-export-service` Metrics
The manifest-repo-export-service uploads the following metrics to datadog:
* `manifest_export_push_failures` - number of failures in pushing changes to git
* `process_delay_seconds` - The time it took to process each event in the export-service, in seconds.
* `git_sync_unsynced` - Number of the applications that have unsynced git sync status
* `git_sync_failed` - Number of the applications that have failed git sync status

### `rollout-service` Metrics
The rollout-service uploads the following metrics to datadog:
* `argo_events_fill_rate` - Number of events that are currently in the argo events queue divided by its capacity
* `kuberpult_events_fill_rate` - Number of events that are currently in the kuberpult events queue divided by its capacity
* `dora_failed_events` - Number of failed attempts to send dora events to revolution
* `dora_successful_events` - Number of successful attempts to send dora events to revolution
* `argo_discarded_events` - Number of argo events that were discarded because the channel was full

