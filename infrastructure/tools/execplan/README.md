# Execution plan

The idea is to create a plan, that is read easily by humans and machines, that can then be provided to an executioner (github actions, gitlab ci, azure pipelines, etc) for continuous integration of projects.
This provides an output at the outset of CI that can be tested and verified, before proceeding any further, making the CI pipeline testable and robust.

The plan is split into five distinct phases,
- `stageA`, which builds any docker images and tools. Runs for each changed docker image or tool.
- `stageB`, which builds every service in the project. Runs for each service that is changed.
- `integration_test`, which runs integration tests on the previously built images. Runs for each integration test required.
- `publishA`, which pushes all the images inside `stageA` into a registry.
- `publishB`, which pushes all the services built in `stageB` into a registry.
- `cleanup`, A single command that should clean up any changes made for integration tests. This is useful if the integration tests spins up services / infrastructure.
  `cleanup` is run, even when `integration_test` fails.

The list of files changed would be given as input to the execution plan, which uses it to create the plan. Also a secondary input, `trigger` is also provided (whether the execution plan is triggered by a pull-request, or by merging to master)

# Usage

To use execution plan. The repository needs to be modified somewhat. The details of which are given below.
To represent a docker image or service, we use `Buildfile.yaml` file with the properties that are described below.
Each Buildfile.yaml follows a yaml structure.

## Stage A

Stage A creates the build images: that is, tooling in docker images used for
building the base image itself, which is later used to build a microservice in Stage B. For this, create
a `Buildfile.yaml` in the path of the directory containing the instructions for
building. Note that currently only the docker makefile is supported in Stage A
and that spec.baseImageConfig is required to be missing in it, lest it will
become a part of stage B (not A).

For a docker image or tool, it is required that its Buildfile.yaml contains the
following parameters:
- `metadata.registry`, where it indicates the registry url for the docker image
  or tool.

Any changes to a file within a Directory with a `Buildfile.yaml` will cause
this docker image or tool to be built.

Eg: Changing the file `infrastructure/docker/config.json` will check if
`infrastructure/docker/Buildfile.yaml`, `infrastructure/Buildfile.yaml` or
`Buildfile.yaml` exists.  It will build the first one it finds.

Note that execplanner provides the variable `ADDITIONAL_IMAGE_TAGS` containing
`dir${HASH}`, where `${HASH}` is a hash over the source directory. Execplanner
expects to be able to use the tooling and build images generates in stage A
using these tags in stage B.

### Stage A Buildfile.yaml structure

The Buildfile.yaml for `infrastructure/docker/ci` (see below) is to be considered into `StageA`.

```yaml
metadata:
  registry: europe-docker.pkg.dev/fdc-standard-setup-dev-env/all-artifacts
```
`Required Parameters`

- `metadata.registry` : A string. This string indicates the registry url for the docker image or tool.

`Optional Parameters`

- `pre_build_actions` : A set of custom flags. These flags can be used in the `pre-build-action/action.yml` to add custom code specific to some buildables.
- `post_build_actions` : Similar to `pre_build_actions`, these flags can be used to add custom code specific to some buildables by using `post-build-action/action.yml`
- `additional_artifacts` : A list of files that will be uploaded as an artifact after buildable stage completes.
- `cache.cachefiles` : These files are common during build stage, so instead of having to download them each time, they can be cached instead. The files in `cachefiles` (including paths from home directory) will be cached each build.
- `cache.hashfiles` : Cache will be invalidated when any of these files change
  NOTE: When adding cache actions in github actions, ensure that the builder image has GNU tar installed. [See issue for more details](https://github.com/actions/toolkit/issues/634)
- `integration_tests` : After build stage completes, for integration test stage, the folders provided in this list will be used. For each directory in this list (relative path), if there is a Buildfile.yaml at that path, then integration test will be run in that directory.

## Stage B

Stage B is supposed to build the actual product using the tooling images
created in Stage A.  Stage B uses a similar structure as Stage A. Create a
`Buildfile.yaml` in the path of the directory containing the instructions for
building. Note that the existence of `spec.baseImageConfig` is required for
making this build part of stage B!  For a service, it is required for it to
have a set of required parameters (see below).  Any changes to a file within a
directory with a Buildfile.yaml will cause this service to be built.

### Stage B Buildfile.yaml structure

The Buildfile.yaml for `services/echo` (see bellow) is to be considered into stage B is given below.

```yaml
kind: Service
metadata:
  builder: golang
spec:
  dependsOn:
    - ../../infrastructure/make/go
  buildWith: infrastructure/docker/ci
  baseImageConfig:
    baseImage:  infrastructure/docker/go
```

`Required Parameters`
- `spec.dependsOn`: An array of strings. Indicates which buildables depend on this `StageB` buildable, causing them to be rebuilt as well.
- `spec.baseImageConfig`, that indicates that a buildfile has a baseImageConfig making the buildfile to be considered into stageB. It is composed of two values:
  - `spec.baseImageConfig.required`, specifies if the stageB buildfile needs a baseImage. Default value is `true`.
  - `spec.baseImageConfig.baseImage`, specifies which base image the stageB buildable is going to use.
- `spec.buildWith`, that specifies what image is going to build the service.

`Optional Parameters`

See the `Optional Parameters` for `StageA`.


## StageA/StageB Makefile structure

Any path with a Buildfile.yaml must also have a Makefile with at least the following targets. This will be used by the executioner (Eg: github actions) for the actual build.

```make

build-pr:
  # commands to unit-test, build and push on a pull request (or use dependencies)

build-main:
  # commands to unit-test, build and push on main branch (or use dependencies)
```

These commands are intended to build the buildables that are specified in both `StageA` and `StageB`.
In `StageA`, the `build-pr` and `build-main` commands are used to build and push docker images. In `StageB`,
the services are built with both commands.

## IntegrationTest

If a `Buildfile.yaml` in buildable directory points to another directory for `integration_tests`, it will be added into the integration test stage. If a `Buildfile.yaml` is not found in the directory pointed, then an error will be thrown.

The `Buildfile.yaml` has the same structure as above, and follows the same conventions.

## Makefile structure

Any path with a Buildfile.yaml should also have a Makefile with at least the following targets. This will be used by the executioner (Eg: github actions) for the integration test.

```make

integration-test-pr:
  # commands to run integration test on a pull request (or use dependancies)

integration-test-main:
  # commands to run integration test on main branch (or use dependancies)
```


## PublishA / PublishB
In this stage, every build from `StageA` is published into a registry.
Therefore, for every buildable in `StageA`, there will be a build to be published in `PublishA`.

The same concept applies to `PublishB`: The builds from `StageB` are published in `PublishB`. This stage
publishes a service and releases into Kuberpult. So every service in `StageB` will be published in `PublishB`.

### Makefile structure

Any path with a Buildfile.yaml should also have a Makefile with the following targets:

```make

publish-pr:
  # commands to sign, validate and push builds into a registry

publish-main:
  # commands to sign, validate and push builds into a registry
```

## Cleanup

If at least one buildable is present in the first stage, then a cleanup stage is added as well. This consists of a single command, `make cleanup-pr` or `make cleanup-main` executed at the root of the repository. This is used to clean up any changes that may have been made for integration tests.
## Makefile structure

In the root makefile, these targets should exist.

```make

cleanup-pr:
  # commands to cleanup after pull request

cleanup-main:
  # command to cleanup after integration-test on main
```


# Github Actions

The execution plan printed by the planner is intended to be used in a CI pipeline, for now, only github actions is used. After the execution plan is made, and converted into a github appropriate format, it is passed to three distinct jobs.

A buildable job, that takes a vector of objects as input, which has the directories to be build, the build command, the builder image and other data like cache and artifacts.
A matrix job is created, for each buildable, building each of them independant of the others.

After the build job, the second part of the json output pertaining to integration tests is passed on to the second job. which runs integration tests for each directory independantly.

In the third phase, a cleanup job with a single command is used to cleanup any loose ends created during build and integration test.

A working example of github actions using execution plan to build can be seen in the kuberpult repo. It consists of a [execution-plan-snippet.yaml](https://github.com/freiheit-com/kuberpult/blob/main/.github/workflows/execution-plan-snippet.yml) which is reused by execution-plan-pr and execution-plan-main to build items for pull request and main.
These are maintained by the SRE team.

However, for ease of use and adding custom code, it is possible to add flags in the execution plan by using pre-build and post-build actions. These can then be used to add custom actions in the curresponding [actions.yaml](https://github.com/freiheit-com/kuberpult/blob/main/.github/actions/post-build-action/action.yml), which are not SRE maintained.  
