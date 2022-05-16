# kuberpult readme for developers

## introduction

kuberpult watches a remote repository, unlike ArgoCD it is not triggered based on push to the repository, It is triggered by rest api instead (or ui which in turn calls the rest api). 
when a `/release` api is called with the manifest files, it checks the repository for additional information (ArgoCD related) and commits and pushes the manifests back into the respository which are then handled by ArgoCD. 
For full usage instructions please check the [readme](https://github.com/freiheit-com/kuberpult/blob/main/readme.md).

It is split into two parts, the backend logic in `cd-service`, and the frontend which is split into two microservices, the `frontend-service` which provides the rest backing for the ui, and the `ui-service` with the actual ui. 
The `cd-service` takes the url of the repository to watch from the environment variable `KUBERPULT_GIT_URL` and the branch to watch from the environment variable `KUBERPULT_GIT_BRANCH`.

## pre requisite software

- [docker](https://docs.docker.com/get-docker/)
- [docker-compose](https://docs.docker.com/compose/install/)

## setup and run instructions (with docker compose)

- in `services/cd-service`, initialize a bare repository with the name `repository_remote`

```bash
cd services/cd-service
git init --bare repository_remote
cd ../..
```
- the value of environment variables are defaulted to `KUBERPULT_GIT_URL=./repository_remote` and `KUBERPULT_GIT_BRANCH=master`
- run the following command to start all the services required, the `--build` parameter is added to build any changes you may have added to the code

```bash
docker compose up --build
```
- the `cd-service` is available at `localhost:8080` and the kuberpult ui is available at `localhost:3000`


### to test setup was done correctly

- for adding changes and testing releasing, clone the `repository_remote` folder. 
- calling curl command to `/release` api with form data for manifest file should have update the remote repository with a new relase.
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

Go tests would be part of the same package as the main code, but ending the file names with `_test.go`. When adding new testcases, please use [table driven tests](https://revolution.dev/app/-JqFGExX46gs9mH7vxR5/WORKSPACE_DOCUMENT/-MjkBXy5_eugWYQsxyHl/) 

To run tests, the root makefile has test command which runs the test commands in `services/cd-service/Makefile` and `services/frontend-service/Makefile`, which in turn run tests for go and yarn files.

```bash
make test
```

When there is build issues in the test code, it would show up as a build failure during make test with the proper error.

When a single test case fails, the test case shows up with the curresponding error.

For a more verbose version, you could go into the service directory and run the tests manually in verbose mode

```bash
cd services/cd-service
go test ./... -v
```

# Legacy docs 

## pre requisite software 

- [docker](https://docs.docker.com/get-docker/)
- [node](https://nodejs.org/en/download/) - ensure you're using an LTS version (or use [nvm](https://github.com/nvm-sh/nvm#installing-and-updating))
- [yarn](https://classic.yarnpkg.com/lang/en/docs/install/#mac-stable)

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
  afterwards, set your library path, e.g.: `export LD_LIBRARY_PATH='/usr/local/lib/'`
- chart testing: 
  - install `helm`, `yamale`, `yamllint` as prerequisites to `ct` from https://github.com/helm/chart-testing#installation 
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
make run

# For running without docker containers use
# WITHOUT_DOCKER=true make run
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

building with libgit2 is tricky atm. run `./dmake make -c services/cd-service bin/main` once to generate the binary for the cd-service.
afterwards run `make release`. this will push the docker image, package the helm chart and create a git tag. the helm chart must be uploaded manually to the github release at the moment.
afterwards bump the version in the `version` file.

## Notes

- there is a dev image based on alpine in `docker/build`. you can start a shell in the image using the `./dmake` command.

- The first version of this tool was written using go-git v5. Sadly the performance was abysmal. Adding a new manifest took > 20 seconds. Therefore, we switched to libgit2 which is much faster but less ergonomic.
