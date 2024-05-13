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
The following command should do this for you.
* `make builder` 

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

## Build and run kuberpult with Earthly
- Download [Earthly](https://github.com/earthly/earthly/releases) binary and add it to your PATH.
- In the root of the repository run `make kuberpult-earthly`. This will build the services (frontend/cd/ui) in a containerised environment and run docker-compose using the built images.
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

### Best practices for unit tests

#### Always use `cmp.Diff`

When writing unit tests, aim to always compare with `cmp.Diff` and print the result.
For errors, the test should check via:
```go
_, err := unitUnderTest(…)
if diff := cmp.Diff(testcase.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
  t.Errorf("error mismatch (-want, +got):\n%s", diff)
}
```
For proto-messages, we need to use `protocmp.Transform()`
```go
if diff := cmp.Diff(testcase.ExpectedResponse, gotResponse, protocmp.Transform()); diff != "" {
  t.Errorf("response mismatch (-want, +got):\n%s", diff)
}
```

#### Do not rely on verbatim JSON

Tests should not rely on the actual JSON representation of objects, but rather compare actual objects, even if it is more verbose.
As the representation generated by protojson is not stable, this would otherwise lead to flaky tests.

**Bad**:
```go
testCase := TestCase{
  …
  expectedErrorMsg: `error at index 2 of transformer batch: already_exists_different:{first_differing_field:MANIFESTS  diff:"--- acceptance-existing\n+++ acceptance-request\n@@ -1 +1 @@\n-{}\n\\ No newline at end of file\n+{ \"different\": \"yes\" }\n\\ No newline at end of file\n"}`,
  …
}
```
**Good**:
```go
testCase := TestCase{
  …
  expectedError: &TransformerBatchApplyError{
    Index: 2,
    TransformerError: &CreateReleaseError{
      response: api.CreateReleaseResponse{
        Response: &api.CreateReleaseResponse_AlreadyExistsDifferent{
          AlreadyExistsDifferent: &api.CreateReleaseResponseAlreadyExistsDifferent{
            FirstDifferingField: api.DifferingField_MANIFESTS,
            Diff:                "--- acceptance-existing\n+++ acceptance-request\n@@ -1 +1 @@\n-{}\n\\ No newline at end of file\n+{ \"different\": \"yes\" }\n\\ No newline at end of file\n",
          },
        },
      },
    },
  },
  …
}
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
  cmake -DUSE_SSH=ON ..
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

To run the services: `make kuberpult`


## releasing a new version

Releases are half-automated via GitHub actions.

Go to the [release workflow pipeline](https://github.com/freiheit-com/kuberpult/actions/workflows/release.yml) and trigger "run pipeline" on the main branch.

## Changelog generation and semantic versioning
Use [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/) to
make your changes show up in the changelog. In short:
* `fix` will create a `PATCH` level semantic version
* `feat` will create `MINOR` level semantic version
* adding a `!` will mark a breaking change and create a `MAJOR` level semantic version

In addition to `fix`, `feat` and breaking changes, the following [types](https://github.com/go-semantic-release/changelog-generator-default/blob/master/pkg/generator/changelog_types.go#L32) can be considered, but are currently **not** allowed in kuberpult:
* revert
* perf
* docs
* test
* refactor
* style
* chore
* build
* ci

The changelog and the version is generated with
[go-semantic-release](https://go-semantic-release.xyz/) via its
[github action](https://github.com/go-semantic-release/action) and it generates the changelogs with the
[default generator](https://github.com/go-semantic-release/changelog-generator-default).

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

