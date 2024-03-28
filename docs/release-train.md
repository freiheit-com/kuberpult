
## Release Train Overview

### Concept

A release train is a concept that ensures that we deploy *often* and *regularly*.
The idea is that the train does not wait for you - it will leave (deploy) on time, regardless of how many services/commits are ready.

The train should run *often enough* to not slow down development, while also giving the testers enough time to look at changes before they go live.

### What happens under the hood

A release train takes the versions that are currently deployed on one environment and deploys those version to another environment.

So there are 2 environments involved:
* *target*:  this is where the services will be deployed (where the version changes happen), *target* can be either a single `environment` or an `environmentGroup`
  in the case of `environmentGroup` the train will run for all environments belonging to this `environmentGroup`. If one environment cannot be changed (e.g. because of a lock), the other environments will still be processed.
* *upstream*: This is the source for the *versions* of the apps. You should run system tests on this environment before running the release train.
  See [environment-config](./environment.md) for configuration.

  
### Triggering a Release Train

The release train needs to be triggered externally - there is nothing inside `Kuberpult` that triggers it.
The trigger is usually implemented as a GitHub Action, Google Cloud Build, etc.
See [Release Train Recommendations](./release-train-recommendations.md) on how combine locking, running tests and triggering a release train.


### API

Release trains are accessible via REST API:

`PUT https://your.kuberpult.host.example.com/environments/${env}/releasetrain?team=${myTeam}`

* `${env}` is the *target* environment
* `team=${myTeam}` is an optional parameter. If set, the release train will only apply for
[apps](./app.md) that have exactly the give team set in the [`/release` endpoint](./release.md)


### Prognosis


It is possible to get the prognosis, or plan, of a release train without triggering one. A release train prognosis does not alter the manifest repo anyhow.


Prognoses are exposes on the REST API:


`GET https://your.kuberpult.host.example.com/environments/${env}/releasetrain/prognosis?team=${myTeam}`


The response is merely the serialized JSON of the protobuf message `GetReleaseTrainPrognosisResponse` found [here](https://github.com/freiheit-com/kuberpult/blob/main/pkg/api/v1/api.proto).

