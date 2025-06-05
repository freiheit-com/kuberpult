### API

Release trains are accessible via REST API:

`PUT https://your.kuberpult.host.example.com/api/environments/${targetEnvironment}/releasetrain?team=${myTeam}`
or
`PUT https://your.kuberpult.host.example.com/api/environment-groups/${targetEnvironmentGroup}/releasetrain?team=${myTeam}`

* `${targetEnvironment}` is the *target* environment
* `team=${myTeam}` is an optional parameter. If set, the release train will only apply for
[apps](./app.md) that have exactly the give team set in the [`/release` endpoint](./release.md)

### CLI

There is a Kubepult command line client for communicating with the `/release-train` endpoint now at [`cli`](https://github.com/freiheit-com/kuberpult/tree/main/cli). The usage is as follows:

```
kuberpult-client --url=${kuberpult_URL} \
    release-train \
    --target-environment=staging \
    --team=sre-team
```

The flags:
```
  -target-environment value
    	the name of the environment to target with the release train (must be set exactly once)
  -team value
    	the target team. Only specified teams services will be taken into account when conducting the release train
  -use_dex_auth
    	if set to true, the /api/* endpoint will be used. Dex must be enabled on the server side and a dex token must be provided, otherwise the request will be denied
```