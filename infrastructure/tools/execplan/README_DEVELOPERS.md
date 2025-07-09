# Execution plan

The CLI tool is used to get a list of commands to execute, the execution plan, given a list of changed files.
The plan contains a series of steps to be executed on individual containers which are dynamically generated based on the list of changed files.

Read README.md on how to configure execution plan with Buildfiles

# Design

We need to be able to configure the output of the execution plan so that each unit of the output is independantly configureable without having to change the code.
For this we need to have configuration files, further we need to identify where each unit lay in the source code.
To solve both of these we use `Buildfile`s, yaml files with data to configure the output for each unit, and the existance of the file, or its path gives us the location of each unit.
(A unit here could be a buildable, or an integration test directory).

---

We need to have a tiny footprint in the build system, ie make it so that each team is free to add additional features necessary for customizing their own build without having to add functionality into the execution planner itself.
We faced issue in jenkins were the builds were getting convoluted. So we wanted the execution planner to not care about the specifics of the build.
So we make use of Makefiles.
A consistent command `make build-pr` or `make integration-test-pr` is used, and the teams are free to use any build process they need for implementation.

---

We figured we need better dependancy management, but when investigated, what we found was that almost all services were independantly buildable. ie, there was little run-time dependancy,
so we could forego needing custom stages in the execution plan and have a fixed number of stages (buildable, integration-test, cleanup). There are services that need to be rebuild when others change,
but these could be handled by adding those to be built at the same time as well. The only exception to this was integration tests, which required a seperate stage,
since not all buildables would have their own integration tests and some would be shared.

---

Some teams require different behaviours when a build is done in a pull request vs when it is done in on the main branch. So an input trigger, pr or main is also to be given to the program.


# Algorithm

The execution plan does the following in its code.

- At the start of the tool, it goes through all the Buildfiles in the source repo and parses them. If any Buildfile is not valid, it will throw an error.
  - From the list of Buildfiles, it creates a map of the directory -> to the buildfile content. buildFileMap
- It reads the input of the changed files. For each of the files in the list,
  - It checks if any of its ancestors are present in the buildFileMap, and it will find its nearest Buildfile. If none are present, then this file is ignored. If some are found, those are added to an array foldersToBuild
- After the first set of Buildfiles are found. The dependsOn field of every buildfile in buildFileMap is checked, to see if matches any of the input files, or any of the directories in foldersToBuild
  - If any of them match, those buildfiles are also added to foldersToBuild
  - This is repeated until there are no changes (so that all transitive dependancies are found)
- To create the buildable stage of the output
  - for each of the foldersToBuild, the builder image is found by using a hash calculated based on the changes on its builder image and a provided registry.
  - The buildfile is parsed to find the other parameters like artifacts, pre and post build actions, etc
  - The hashfiles are used to create a unique hashkey which is added to the output
  - The command would be `make -C directory build-pr`, or `make -C directory build-main` depending on the trigger.
- For each buildable, check if they have any integration test directories, if any add them to a set (to remove duplicates)
  - For each item in integration test directory, similar to the buildable stage.
  - The command would be `make -C directory integration-test-pr`, or `make -C directory integration-test-main` depending on the trigger.
- If the number of items in buildable stage is more than one, then a cleanup stage is added with a single command `make cleanup-pr`

This will be converted by another program to be more ideally suited for each ci/cd platform, like github actions.

# Output format

The output format is JSON with three distinct stages. With each stage having multiple items with the required data for one independant build command to be done.

Sample output

```json
{
  "buildable": [
    {
      "container": {
        "image": "europe-docker.pkg.dev/fdc-standard-setup-dev-env/all-artifacts/images/golang-ci:golang-1.17.3-alpine3.15-NG-5"
      },
      "commands": [
        "make -C integrationtest/serviceA build-pr"
      ],
      "artifacts": [
        "integrationtest/serviceA/gateway/k8s/report.json"
      ],
      "cachefiles": [
        "integrationtest/gateway/k8s"
      ],
      "cacheKey": "AMKXICrEa/LU2fqQlUjhQLQbblx/iXlqdNLzGCPzfSI="
      "preBuildActions": [
        "javaKeySet",
      ],
      "postBuildActions": [
        "notifyfailure"
      ]
      "directory": "integrationtest/serviceA"
    }
  ],
  "integration_test": [
    {
      "container": {
        "image": "europe-docker.pkg.dev/fdc-standard-setup-dev-env/all-artifacts/images/golang-ci:golang-1.17.3-alpine3.15-NG-5"
      },
      "commands": [
        "make -C integrationtest integration-test-pr"
      ],
      "directory": "integrationtest"
    }
  ],
  "cleanup": [
    {
      "container": {
        "image": "europe-docker.pkg.dev/fdc-standard-setup-dev-env/all-artifacts/images/golang-ci:golang-1.17.3-alpine3.15-NG-5"
      },
      "commands": [
        "make -C integrationtest/serviceA cleanup-pr"
      ],
      "directory": "integrationtest/serviceA"
    }
  ]
}
```

# Usage

To run it with docker container

`cat inputFiles | docker run -i -v $(pwd):/repo europe-docker.pkg.dev/fdc-standard-setup-dev-env/all-artifacts/images/execution-plan:1.0-scratch-NG-2 pr`

To get the json output with the binary run

`git diff --name-only | execution-plan {main|pr}`

Note: When using with github-actions matrix, if you want them to be failed independant of each other, please use the flag `failfast: false`
