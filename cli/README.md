# Kuberpult client


The Kuberpult client is a Go command line client and library for interacting with a Kuberpult deployment. It essentially just wraps the Kuberpult API in a high-level, human-friendly interface.

The Kuberpult client offers functionality to interact with kuberpult, able to perform four actions:

* Create releases
* Conduct release trains
* Create locks
* Delete locks
## Building

For local use, you can build kuberpult by:

```shell
cd cmd/kuberpult-client/ && go build
```

Add the Kuberpult CLI to your PATH:

```shell
export PATH="<path-to-kuberpult>/kuberpult/cli/cmd/kuberpult-client:$PATH"
```

## Usage

The general usage of the CLI is as follows:

```shell
kuberpult-client --url <kuberpult_url> <command> [parameters]
```
Where:
* kuberpult_url: is the location of the kuberpult instance you are trying to contact
* command: is one of the available commands used to interact with Kuberpult
  * release
  * release-train
  * <create/delete>-env-lock
  * <create/delete>-app-lock
  * <create/delete>-team-lock
  * <create/delete>-group-lock
* parameters: command-specific parameters

The Kuberpult CLI is devided into subcommands, each performing on single action on Kuberpult.

### Authentication

#### IAP
If your Kuberpult instance is behind IAP, you will need to provide an IAP token in order to perform any action.
You can do so by providing it as an environment variable like:
```bash
export KUBERPULT_IAP_TOKEN=<YOUR_IAP_TOKEN>
```
#### Dex
If you have enabled the Dex feature on the server side, you will need to provide a Dex token in order to perform any action.
You can do so by providing it as an environment variable like:
```bash
export KUBERPULT_DEX_TOKEN=<YOUR_DEX_TOKEN>
```


These tokens aren't mutually exclusive. If you have IAP and Dex enabled, you will need to provide them both.

## Commands

### Creating releases
You can create releases on Kuberpult through the **release** command.

The release command accepts the following parameters:
```
-application value
      the name of the application to deploy (must be set exactly once)
-display_version value
      display version (must be a string between 1 and characters long)
-environment value
      an environment to deploy to (must have --manifest set immediately afterwards)
-manifest value
      the name of the file containing manifests to be deployed (must be set immediately after --environment)
-previous_commit_id value
      the SHA1 hash of the previous commit (must not be set more than once and can only be set when source_commit_id is set)
-signature value
      the name of the file containing the signature of the manifest to be deployed (must be set immediately after --manifest)
-skip_signatures
      if set to true, then the command line does not accept the --signature args
-source_author value
      the souce author (must not be set more than once)
-source_commit_id value
      the SHA1 hash of the source commit (must not be set more than once)
-source_message value
      the source commit message (must not be set more than once)
-team value
      the name of the team to which this release belongs (must not be set more than once)
-use_dex_auth
      use /api/release endpoint, if set to true, dex must be enabled and dex token must be provided otherwise the request will be denied
-version value
      the release version (must be a positive integer)
```

You can create a new release by running:

```shell
kuberpult-client --url <kuberpult_url> release [parameters]
```

### Conducting a release train

You can also trigger a release train through the CLI by using the release-train command.

The **release-train** command accepts the following parameters:

```
-target-environment value
      the name of the environment to target with the release train (must be set exactly once)
-team value
      the target team. Only specified teams services will be taken into account when conducting the release train
-use_dex_auth
      if set to true, the /api/* endpoint will be used. Dex must be enabled on the server side and a dex token must be provided, otherwise the request will be denied
```

You can conduct a release train by running:

```shell
kuberpult-client --url <kuberpult_url> release-train [parameters]
```

### Environment Locks

You are able to **create and delete environment locks** through the CLI.

#### Creating
You can create an environment lock by using the **create-env-lock** command.

The CLI offers the following parameters for creating an environment lock:
```
-environment value
      the environment to lock
-lockID value
      the ID of the lock you are trying to create
-message value
      lock message
```

You can create an environment lock by running:

```shell
kuberpult-client --url <kuberpult_url> create-env-lock [parameters]
```

#### Deleting
You can delete an environment lock by using the **delete-env-lock** command.

The CLI offers the following parameters for deleting an environment lock:
```
-environment value
      the environment to lock
-lockID value
      the ID of the lock you are trying to create
```

You can delete an environment lock by running:

```shell
kuberpult-client --url <kuberpult_url> delete-env-lock [parameters]
```

### Application Locks

You are able to create and delete application locks through the CLI.

#### Creating

You can create an application lock by using the **create-app-lock** command.

The CLI offers the following parameters for creating an application lock:
```
-application value
      application to lock
-environment value
      the environment to lock
-lockID value
      the ID of the lock you are trying to create
-message value
      lock message
```

You can create an application lock by running:

```shell
kuberpult-client --url <kuberpult_url> create-app-lock [parameters]
```

#### Deleting
You can delete an application lock by using the **delete-app-lock** command.

The CLI offers the following parameters for deleting an environment lock:
```
-application value
      application to lock
-environment value
      the environment to lock
-lockID value
      the ID of the lock you are trying to create
```

You can delete an application lock by running:

```shell
kuberpult-client --url <kuberpult_url> delete-app-lock [parameters]
```

### Team Locks

You are able to create and delete team locks through the CLI.

#### Creating
You can create a team lock by using the **create-team-lock** command.

The CLI offers the following parameters for creating a team lock:
```
-environment value
      the environment to lock
-lockID value
      the ID of the lock you are trying to create
-message value
      lock message
-team value
      team to lock
```

You can create a team lock by running:

```shell
kuberpult-client --url <kuberpult_url> create-team-lock [parameters]
```

#### Deleting

You can delete a team lock by using the **delete-team-lock** command.

The CLI offers the following parameters for deleting a team lock:
```
-environment value
      the environment to lock
-lockID value
      the ID of the lock you are trying to create
-team value
      team to lock
```

You can delete team lock by running:

```shell
kuberpult-client --url <kuberpult_url> delete-team-lock [parameters]
```

### Group Locks

You are able to create and delete group locks through the CLI.

#### Creating

You can create a group lock by using the **create-group-lock** command.


The CLI offers the following parameters for creating a group lock:
```
-environment-group value
      the environment-group to lock
-lockID value
      the ID of the lock you are trying to create
-message value
      lock message
```

You can create a group lock by running:

```shell
kuberpult-client --url <kuberpult_url> create-group-lock [parameters]
```

#### Deleting

You can delete a group lock by using the **delete-group-lock** command.

The CLI offers the following parameters for deleting a group lock:
```
-environment-group value
      the environment-group to lock
-lockID value
      the ID of the lock you are trying to create
```

You delete a group lock by running:

```shell
kuberpult-client --url <kuberpult_url> delete-group-lock [parameters]
```

### Getting commit deployments

You can get the deployment status of a commit by using the **get-commit-deployments** command. This answers the question "is my commit deployed yet, according to kuberpult", for all environments.

The CLI offers the following parameters for getting commit deployments:
```
-commit value
      the commit ID to get deployments for
```

```
-out value
      the file to write the output to
```

You can get commit deployments by running:

```shell
kuberpult-client --url <kuberpult_url> get-commit-deployments [parameters]
```
