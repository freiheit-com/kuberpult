## API

Release trains are accessible via REST API:

`PUT https://your.kuberpult.host.example.com/api/environments/${targetEnvironment}/releasetrain?team=${myTeam}`
or
`PUT https://your.kuberpult.host.example.com/api/environment-groups/${targetEnvironmentGroup}/releasetrain?team=${myTeam}`

* `${targetEnvironment}` is the *target* environment, meaning this is where the deployments will happen.
* `team=${myTeam}` is an optional parameter. If set, the release train will only apply for
[apps](./app.md) that have exactly the give team set in the [`/release` endpoint](./release.md)
* `sourceGitTag=${myManifestRepoGitTag}` is an optional parameter. If set, the release train will not look for the **source** environment in the current state,
but in the state given by this git tag. If the tag does not exist, the endpoint will fail.
* `gitTag=${myNonExistingManifestRepoGitTag}` is an optional parameter. If set, the manifest-export will create the git tag on the manifest repo after successfully pushing the commit.
What happens if the creation of the git tag fails depends on the option `manifestRepoExport.failOnErrorWithGitPushTags` (see `charts/kuberpult/values.yaml`).
If the option `git.minimizeExportedData` is `true`, and the release train changes nothing (meaning all deployments are already in the desired version),
then no git commit and no git tag will be created, even if `gitTag` is set.

### Git Tag Support
* All mentioned git tags refer to the manifest-repository.
* Note that in general, changing the manifest-repo outside kuberpult is not supported.
This means you should not push commits or tags apart from the kuberpult endpoints. In short, don't execute `git push`.
  * If you did add a commit or tag, you can restart the manifest-repo-export service to make kuberpult aware.


## CLI

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