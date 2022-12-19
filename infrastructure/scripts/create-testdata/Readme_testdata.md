To set up kuberpult, you need a manifest repo.
For local testing, you can create one with
`git init --bare`
It's important that this directory is called `repository`
and is directly under `services/cd-service`.

Bare repositories are good for kuberpult, but not daily work.
You can check this repo out like this:
`git clone ../path/to/repo`

You now have a repo.
you still need to fill it with some basic data:
environments and releases.
For environments, just copy`testdata_template` to the root of your manifest repo.
Commit and push (you may need `--force` to push).

For releases, ensure kuberpult is running (use the docker-compose file),
and then run `./create-release.sh` and `./run-releasetrain.sh` several times.

All remaining operations should be easily doable via the UI.

If you still need to call a kuberpult grpc endpoint directly, you can use evans:

`evans --host localhost --port 8443 -r`

Then type `service DeployService` and `call Deploy` for example.
