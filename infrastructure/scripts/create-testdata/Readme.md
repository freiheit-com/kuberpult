# How to set up test data

To set up kuberpult, you need a manifest repo.
For local testing, you can create one with
`git init --bare`
It's important that this directory is called `repository_remote`
and is directly under `services/cd-service`.

Bare repositories are good for kuberpult, but not daily work.
You can check this repo out like this:
`git clone ../path/to/repo`

You now have a repo.
you still need to fill it with some basic data:
environments and releases.
For environments, ensure that kuberpult is running (use the docker-compose file),
and then run `./create-environments.sh` to create environments. This defaults to the 
environments inside tesdata_template/environments, but you can also provide your own
environments and respective configurations by running `./create-environments.sh /path/to/envs`

For releases, ensure kuberpult is running (use the docker-compose file),
and then run `./create-release.sh my-service my-team` to create releases
and then run both `./run-releasetrain.sh staging`  and `./run-releasetrain.sh fakeprod` so that the release shows up in the UI under staging and fakeprod.
All remaining operations should be easily doable via the UI.

If you still need to call a kuberpult grpc endpoint directly, you can use evans:

`evans --host localhost --port 8443 -r`

Then type `service DeployService` and `call Deploy` for example.

### Add tags to the repository

In order to test some of the features for Kuberpult you will need to add tags to the manifest repo. To do this, within the manifest repo run the command: `git tag <name of tag> && git push --tags`
To see more useful data with the tag, create some dummy deployments prior to creating the tag. Once the dummy deployments are done, pull the latest changes to your local manifest repo then create the tag.


### Why "fakeprod"?
We want to make it as clear as possible that this is testdata.
We therefore recommend to have no "prod" environment locally.
