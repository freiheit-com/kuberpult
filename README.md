# kuberpult Readme for users

## Etymology

Kuberpult is a catapult for [kubernetes](https://kubernetes.io/) :) it catapults the containers of microservices to different stages in kubernetes clusters.

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
