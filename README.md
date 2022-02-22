# kuberpult Readme for users

## About

**Kuberpult** is a tool that allows you to manage Kubernetes manifests for your services in a
Git repository and manage the same version of those services in different environments
with differnt configs according to the environment.

kuberpult works best with [ArgoCD](https://argo-cd.readthedocs.io/en/stable/) which applies the
manifests to your clusters and kuberpult helps you to manage those manifests in the repository.

kuberpult allows you to lock some services or an entire environment, so automatic deployments (via a typical api call) to
those services/environments will be queued until the last lock is removed.
Manual deployments (via the UI or a flag in the api) are always possible.

`kuberpult` does not actually `deploy`. That part is usually handled by argoCD.

`kuberpult` has a UI, and it can handle *locks*. When something is locked, it's version will not be changed.
Both *environments* and *microservices* can be `locked`.

## Current Version and Queued Version

Every app has a current version on every env (including `nil` for no version).
If a deployment starts while the app/env is locked,
instead of changing the current version, the `queued_version` will be set.
When the lock is deleted, the queued version will be deployed.

There is currently no visualization for the queue in the ui,
so it can only be seen in the manifest repo as "queued_version" symlink next to "version".

The queue has a length of 0 or 1.
Attempting to put a version into the full queue, will overwrite it instead ("last deployment wins").

## Release train Overview

### What is that?

A release train is a concept that ensures that we deploy *often* and *regularly*.
The idea is that the train does not wait for you - it will leave (deploy) on time, regardless of how many services/commits are ready.

The train should run *often enough* to not slow down development, while also giving the testers enough time to look at changes before they go live.

### Trigger

The release train needs to be triggered externally - there is nothing in `kuberpult` that triggers it.
The trigger is usually implemented as a jenkins pipeline with a cronjob.
See `k8s-jenkins-cac.tf` in your project.

### Environments

There are 2 environments involved:
* *target*: this is where the services will be deployed (where the version changes happen).
* *upstream*: this is where the system tests are run. It is also the source for the *versions* of the apps.

## Getting started

### Creating a new environment

The first thing you need to do after installing kuberpult is creating an environment. There is currently no UI component for doing this.

Since kuberpult uses a git repository as its database, you only need to create a folder under `environments`. The folder name is the environment name.
Kuberpult doesn't care too much how you name your environment but to make sure that apps in argocd are easy to recognize, environment names are limited to alpha numeric characters and dash and its length may not exceed 21 characters.

You can put a file named `config.json` in the environment's directory to add further configuration. The options are documented further below. The implit default content of this file is `{}`. It's not necessary to create this file but we highly suggest to create it to enable automatic provisioning from the start. In order to do that, use `{"upstream":{"latest":true}}` as the content of this file.

Your initial repository could look like this:

```
environments/
+- development/
  +- config.json # contains just '{"upstream":{"latest":true}}'
```

### Creating a new application

After creating the first environment, you can directly create new releases. Applications will be automatically created as needed.

### Creating a new release 

In the default configuration, the endpoint to create a new a release is only exposed internally in the k8s cluster hosting the kuberpult deployment.

In order to create a new release issue an http POST request to the `/release` endpoint of the `kuberpult-cd-service` in the cluster containing your application name and the manifests for the inidividual environments. `curl` is completely sufficient for this. Assuming that the manifests for the development enviornment are in `manifests_development.yaml` you can run:

```
curl --form-string 'application=myawesomeapplication' --form 'manifests[development]=@manifests_development.yaml' kuberpult-cd-service/release
```

This will create a new application and directly create a new release.

You can add more details to the release using the form fields `source_commit_id`, `source_author` and `source_message`. Also remember to check the http response code ( curl doesn't do that automatically ).

## Configuration

### Environments

In an environment's config.json file the following top-level options are recognized.

`upstream`: configures the automatic deployment for this environment. Valid values are either `{"latest": true}` to indicate that this environment is supposed to always run on the latest version of an application or `{"environment":"<upstream env>"}` to indicate that this environment should be deployed from `upstream env` when using the release train.

`argocd`: controlls how the argocd applications are created in this environment. The most important 


```
{
  "argocd": {
    // The destination of this environments application in argocd
    "destination": {
      "server": "https://34.90.211.131",
      "namespace": "*"
    },
    // Annotations to apply to all argocd applications.
    "applicationAnnotations": {
      "notifications.argoproj.io/subscribe.on-degraded.teams":"",
      "notifications.argoproj.io/subscribe.on-sync-failed.teams":""
    },
    // Cluster-wide resources must be listed here.
    "accessList": [
      {
        "group": "*",
        "kind": "ClusterSecretStore"
      },
      {
        "group": "*",
        "kind": "ClusterIssuer"
      }
    ],
    // Configure ignored differences for all applications here.
    "ignoreDifferences": [
      {
        "group": "apps",
        "kind": "Deployment",
        "jsonPointers": [
          "/spec/replicas"
        ]
      }
    ]
  },
  "upstream": {
    // Setting latest to true causes this environment to use always the latest manifests.
    "latest": true

    // Setting an environment here will use that environment in the release train.
    "environment": "$upstream env"

    // "latest" and "environment" are mutually exclusive.
  }
}
```
