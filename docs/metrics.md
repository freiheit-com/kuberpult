## Datadog Metrics
Kuberpult uploads mulitple metrics to datadog.

### `cd-service` Metrics
The cd-service uploads the following metrics to datadog:
* `environment_lock_count` - the count of current environment locks, for a given environment;
* `application_lock_count` - the count of current environment application locks, for a given application in a given environment;
* `lastDeployed` - the time since the last deployment in minutes;
* `request_queue_size` - the current size of the request queue;

