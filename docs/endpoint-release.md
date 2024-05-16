
## Release Endpoint

### Concept

In order to let Kuberpult know about a change in your service, you need to invoke its `/release` http endpoint
and supply the kubernetes manifests for each environment.

### Parameters

An example for this can be found [here](https://github.com/freiheit-com/kuberpult/blob/main/infrastructure/scripts/create-testdata/create-release.sh#L80).
The `/release` endpoint accepts several parameters:
* `manifests` the (kubernetes) manifests that belong to this service. Needs to be unique for each version. You can achieve this by adding the git commit id to the docker image tag of your kubernetes Deployment.
* `application` name of the microservice. Must be the same name over all releases, otherwise Kuberpult assumes this is a separate microservice.
* `source_commit_id` git commit hash, we recommend to use the whole 40 characters, and require all 40 characters to use the feature `git.enableWritingCommitData`. To get the current git commit hash with 40 characters, run `git show --quiet "--format=format:%H"`.
* `previous_commit_id` git commit hash of the commit right before the current one. Recommended (but not required) for the feature  `git.enableWritingCommitData`. To get the previous git commit hash with 40 characters, run `git rev-parse @~`
* `source_author` git author of the new change.
* `source_message` git commit message of the new change.
* `author-email` and `author-name` are base64 encoded http headers. They define the `git author` that pushes to the manifest repository.
* `version` (optional, but recommended) If not set, Kuberpult will just use `last release number + 1`. It is recommended to set this to a unique number, for example the number of commits in your git main branch. This way, if you have parallel executions of `/release` for the same service, Kuberpult will sort them in the right order.
* `team` (optional) team name of the microservice. Used to filter more easily for relevant services in kuberpult's UI and also written as label to the Argo CD app to allow filtering in the Argo CD UI. The team name has a maximum size of 20 characters.



### Caveats
Note that the `/release` endpoint can be rather slow. This is because it involves running `git push` to a real repository, which in itself is a slow operation. Usually this takes about 1 second, but it highly depends on your Git Hosting Provider. This applies to all endpoints that have to write to the git repo (which is most of the endpoints).

### CLI

There is a Kubeprult command line client for communicating with the `/release` endpoint now at [`cli`](https://github.com/freiheit-com/kuberpult/tree/main/cli). The usage is as follows:

```
kuberpult-client release \
    --application=my-customer-data-service \
    --environment=development --manifest=manifest-dev.yaml  --signature=signature-dev.gpg \
    --environment=production  --manifest=manifest-prod.yaml --signature=signature-prod.gpg --team=blabla-team \
    --previous_commit_id=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
    --source_commit_id=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb \
    --source_author=someone@something.com \
    --source_message="some commit message\nthat can be multiline" \
    --version=1234 \
    --display-version=v1.23.4
```

The flags:
```
  -application value
        the name of the application to deploy (must be set exactly once)
  -display_version value
        display version (must be a string between 1 and 15 characters long)
  -environment value
        an environment to deploy to (must have -manifest set immediately afterwards)
  -manifest value
        the name of the file containing manifests to be deployed (must be set immediately after -environment)
  -previous_commit_id value
        the SHA1 hash of the previous commit (must not be set more than once and can only be set when source_commit_id is set)
  -signature value
        the name of the file containing the signature of the manifest to be deployed (must be set immediately after -manifest)
  -skip_signatures
        if set to true, then the command line does not accept the -signature args
  -source_author value
        the souce author (must not be set more than once)
  -source_commit_id value
        the SHA1 hash of the source commit (must not be set more than once)
  -source_message value
        the source commit message (must not be set more than once)
  -team value
        the name of the team to which this release belongs (must not be set more than once)
  -version value
        the release version (must be a positive integer)
```
