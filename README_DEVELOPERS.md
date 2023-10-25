# Kuberpult Readme for developers

## Introduction

Unlike ArgoCD, Kuberpult is not triggered based on push to the repository. It is triggered by REST api instead (or ui which, in turn, calls the REST api).
When a `/release` endpoint is called with the manifest files, it checks the repository for additional information (ArgoCD related), then commits and pushes the manifests to the repository which is then handled by ArgoCD.
For full usage instructions, please check the [readme](https://github.com/freiheit-com/kuberpult/blob/main/readme.md).

## Install dev tools
It is split into two parts. The backend logic is in the `cd-service`. The frontend is also split into two parts (but they are both deployed as one microservice), the `frontend-service` that provides the REST backing for the ui, and the `ui-service` with the actual ui.
The `cd-service` takes the URL of the repository to watch from the environment variable `KUBERPULT_GIT_URL` and the branch to watch from the environment variable `KUBERPULT_GIT_BRANCH`.

## Prerequisite software

- [docker](https://docs.docker.com/get-docker/)
- [docker-compose](https://docs.docker.com/compose/install/) v1.29.2

## Setup builder image

You need a `builder` image that is tagged as `latest` to build services locally.
There's 2 ways to get an image:
* `IMAGE_TAG=latest make -C infrastructure/docker/builder build`
* `docker pull docker pull europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder:1.10.0` (replace with current version)

Once you have the image locally, you need to tag it as `latest` (replace `${IMAGE_FROM_LAST_STEP}`):
* `docker tag ${IMAGE_FROM_LAST_STEP} europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder:latest`

There's no need to push the image.

## Setup and run instructions (with docker-compose)

- in `services/cd-service`, initialize a bare repository with the name `repository_remote`

```bash
cd services/cd-service
git init --bare repository_remote
cd ../..
```
- This repository is bare, to populate it, fill it with data as described in `README.md` or https://github.com/freiheit-com/kuberpult/pull/95
- the value of environment variables are defaulted to `KUBERPULT_GIT_URL=./repository_remote` and `KUBERPULT_GIT_BRANCH=master`
- run the following command to start all the services required.
```bash
make kuberpult
```

For details on how to fill the repo, see the
[Readme for testdata](infrastructure/scripts/create-testdata/Readme.md)

- the `cd-service` is available at `localhost:8080`. And Kuberpult ui is available at `localhost:3000`

## GRCP Calls (with docker-compose setup)

Most calls can be made directly from the UI.
To make specific calls for manual testing, install [evans](https://github.com/ktr0731/evans).

When the services are running with `docker-compose`, start evans like this:

`evans --host localhost --port 8443  -r`

```
header author-name=YXV0aG9y
header author-email=YXV0aG9yQGF1dGhvcg==
package api.v1
service DeployService
```

```
api.v1.DeployService@localhost:8443> call Deploy
environment (TYPE_STRING) => development
application (TYPE_STRING) => app-alerting-service
version (TYPE_UINT64) => 91
ignoreAllLocks (TYPE_BOOL) => false
✔ Queue
{}
```


#### Why the author headers?

With a recent change, the cd-service now always expect author headers to be set, both in grpc and http endpoints.
`/release` is the exception to that, but it logs a warning, when there is no author.
(And of course `/health` is another exception).
The frontend-service is now the only point that knows about default-author (see helm chart `git.author.name` & `git.author.email`).
The frontend-service can be called with headers, then those will be used. If none are found, we use the default headers from the helm chart.

### Test that setup was done correctly

- for adding changes and testing releasing, clone the `repository_remote` folder.
- calling curl command to `/release` api with form data for the manifest file should have updated the remote repository with a new release.
- view the changes in ui as well

```bash
cd services/cd-service
git clone ./repository_remote repository_checkedout
cd repository_checkedout
touch manifest.yaml
# This should cause the release to be pushed to the git repository
curl --form-string 'application=helloworld' --form 'manifests[development]=@manifest.yaml' localhost:8080/release
git pull
cd ../../..
```

## Unit tests

Go tests would be part of the same package as the main code, but ending the file names with `_test.go`. When adding new test cases, please use [table driven tests](https://revolution.dev/app/-JqFGExX46gs9mH7vxR5/WORKSPACE_DOCUMENT/-MjkBXy5_eugWYQsxyHl/)

To run tests, the root makefile has the test command, which runs the test commands in `services/cd-service/Makefile` and `services/frontend-service/Makefile`, which, in turn, run tests for go and pnpm files.

```bash
make test
```

When there are build issues in the test code, it will show up as a build failure during make test with the proper error.

When a single test case fails, the test case shows up with the corresponding error.

For a more verbose version, you could go into the service directory and run the tests manually in verbose mode.

```bash
cd services/cd-service
go test ./... -v
```

# Installation outside of docker

## Podman and podman-compose
If you use Podman (and `podman-compose`) instead (e.g. on macOS), you might need to specify `user: 0` for **each** container.
because otherwise the process in the container does not have access to the filesystem mounted from the user's home directory into the container.
Using UID 0 should be fine with Podman, as it (unlike Docker) runs the container with the privileges of the current user (the UID of the current user is mapped to UID 0 inside the container). e.g.:
```
backend:
    build: infrastructure/docker/backend
    container_name: kuberpult-cd-service
    ports:
      - "8080:8080"
      - "8443:8443"
>>> user: 0
    volumes:
      - .:/kp/kuberpult
```
## prerequisite software

- [docker](https://docs.docker.com/get-docker/) - for docker build for cd-service - optional
- [node](https://nodejs.org/en/download/) - ensure you're using an LTS version (or use [nvm](https://github.com/nvm-sh/nvm#installing-and-updating))
Ideally use the same version as in the [package.json](https://github.com/freiheit-com/kuberpult/blob/main/services/frontend-service/package.json#L42)
- [pnpm](https://pnpm.io/installation)

## Libraries required
- libgit2 >= 1.0
  download tar file and follow instructions here: https://github.com/libgit2/libgit2#installation
  it worked for me to run: (the instructions are slightly different)
  ```
  sudo apt-get install libssl-dev
  mkdir build && cd build
  cmake ..
  sudo cmake --build . --target install
  ```
  Afterwards, set your library path, e.g.: `export LD_LIBRARY_PATH='/usr/local/lib/'`

  For m1 mac:
  brew and macports don't have the version of libgit2 that we need (1.3.0)
  so what we do is we install macports, then travel back in timie to when 1.3.0 was the latest and then install it.

  - install macports from [official site](https://www.macports.org/install.php)
  - install libgit2

  ```
# Get macports repo
git clone https://github.com/macports/macports-ports.git
cd macports-ports
# Travel back in time to when libgit2 version was 1.3.0
git checkout b2b896fb904cfd14d8d6f3063c0b620b52b94f31
# Install libgit2
cd devel/libgit2
sudo port install 
# Convince package config that we do infact have libgit2 (change to rc file of whichever shell you use)
echo "export PKG_CONFIG_PATH=/opt/local/lib/pkgconfig" >> ~/.zshrc
source ~/.zshrc
  ```

-  libsqlite3

On ubuntu: install the apt package `libsqlite3-dev`
On mac: install the macports package `sqlite3`


- Chart Testing:
  - install `helm`, `Yamale`, `Yamllint` as prerequisites to `ct` from https://github.com/helm/chart-testing#installation
  - then follow the instructions to install `ct`
- golang >= 1.16
- protoc >=3.15
- buf from https://docs.buf.build/installation

## Setup and Run

### With makefiles

- in `services/cd-service`, initialize a bare repository with the name `repository_remote`

```bash
cd services/cd-service
git init --bare repository_remote
```

- for cd-service

```bash
cd services/cd-service
# Running with docker container (recommended)
WITH_DOCKER=true make run

# For running without docker containers use
# make run
```

- for frontend service - Note, frontend services are not
```bash
cd services/frontend-service
make run
```

- for ui
```
cd services/frontend-service
make start
```


## releasing a new version

Releases are half-automated via GitHub actions.

### Changelog file
To create a release, ensure that the release version and accompanying changes are added to the `CHANGELOG.md` file.
To help with that, we use a changelog generator.
You need a GitHub personal token to run this script and set the environment variable `TOKEN`.
* Go to [GitHub settings](https://github.com/settings/tokens?type=beta)
* Create a fine-grained access token
* Limit the lifetime to <= 7 days. You can easily regerate the token next time.
* Resource Owner: <Your own account> (not an org)
* Repository access: `Public Repositories (read-only)`
* `Generate Token`

You now need to create a remote git tag, so that the generator knows what the release is called:
```shell
VERSION=.... # 0.1.2
git tag --sign --edit "$VERSION"
git push origin "$VERSION"
```
Cancel the triggered pipeline in GitHub, so that it doesn't create a draft release - or delete the draft release.

Now run the changelog generator:
```bash
export TOKEN="INSERT_YOUR_TOKEN_HERE"
./infrastructure/scripts/generate-changelog.sh
```
This will generate a file `CHANGELOG.tmp.md`.

Ensure that:
* ...every change is sorted into the sections "major", "minor" or "patch".
* ...there are no PRs mentioned multiple times (this can happen if the git tag does not exist).
* ...PRs are labeled correctly.

You can change the labels, and run the generator again if necessary.

* Copy the `CHANGELOG.tmp.md` into a new section in `CHANGELOG.md`.
* Create a release commit with the changelog and set the label to `exclude`.
* Merge the release commit.
* Delete the remote tag created earlier:
`git push --delete origin "$VERSION" && git tag -d "$VERSION"`.
* Create the git tag (same as before) and push again. This will trigger the
[release workflow pipeline](https://github.com/freiheit-com/kuberpult/actions/workflows/execution-plan-tag.yml) and create a draft release.
  Verify that the release draft is correct in the GitHub UI and publish it for release.

## Changelog Labels
To exclude PRs, label them as `exclude`.
To set PRs to `major`, label them as `major`.
To set PRs to `patch`, label them as `patch`. The `renovate` label is also considered as `patch`.


## Notes

- there is a dev image based on alpine in `docker/build`. You can start a shell in the image using the `./dmake` command.

- The first version of this tool was written using go-git v5. Sadly the performance was abysmal. Adding a new manifest took > 20 seconds. Therefore, we switched to libgit2, which is much faster but less ergonomic.

## Running locally with 2 images
The normal docker-compose.yml file starts 3 containers: cd-service, frontend-service, ui.
The file `docker-compose.tpl.yml` starts 2 containers: cd-service and frontend+ui in one.
In the helm chart, there are also only 2 containers.

Pros of running 2 containers:
* closer to the "real world", meaning the helm chart
* You can (manually) test things like path redirects much better

Cons of running 2 containers:
* There's no UI hot-reload

To run with 2 containers (you need to run this with every change):
```shell
# replace "sven" with any other prefix or your choice:
docker-compose stop
PREFIX=sven-e
VERSION=$(git describe --always --long --tags)
export IMAGE_REGISTRY=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult 
IMAGENAME="$IMAGE_REGISTRY"/kuberpult-cd-service:"$PREFIX"-"$VERSION" make docker -C services/cd-service/
IMAGENAME="$IMAGE_REGISTRY"/kuberpult-frontend-service:"$PREFIX"-"$VERSION" make docker -C services/frontend-service/
IMAGE_TAG_CD="$PREFIX"-"$VERSION" IMAGE_TAG_FRONTEND="$PREFIX"-"$VERSION" dc -f ./docker-compose.tpl.yml up -d --remove-orphans
```
Now open a browser to `http://localhost:8081/`.

