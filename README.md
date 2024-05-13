![Main Pipeline](https://github.com/freiheit-com/kuberpult/actions/workflows/execution-plan-main.yml/badge.svg)
![Release Pipeline](https://github.com/freiheit-com/kuberpult/actions/workflows/release.yml/badge.svg)


# Kuberpult Readme for users

## Etymology

Kuberpult is a catapult for [kubernetes](https://kubernetes.io/) :) it catapults the containers of microservices to different stages in kubernetes clusters.

## About

**Kuberpult** helps you manage different versions of different microservices in different cluster.
While [Argo CD](https://argo-cd.readthedocs.io/en/stable) applies the *current* version of your services in clusters,
Kuberpult also helps you with managing what is deployed *next*.

## Purpose
The purpose of Kuberpult is to help roll out quickly yet organized.
We use it for requirements like this:
* We have 3 clusters, development, staging, production
* Any merged changes should instantly be deployed to development.
* After running some test on development, roll out to staging.
* Once a day we trigger a release train that deploys everything to production - using the versions of staging as source.
* Sometimes we want to prevent certain deployments, either of single services, or of entire clusters.
* We never want to deploy (to production) between 9am and 11am as these are peak business hours.


## Kuberpult Design Principles

We use these principles to decide what features to focus on. We may deviate a little, but in general
we don't want features in kuberpult that violate these points:

* **Automatic and regular deployments**: Kuberpult is built for teams who want to deploy often (e.g. daily) and setup automated processes for the deployments.
* **All power to the engineers**: Kuberpult never stops an engineer from deploying manually. The engineers know their services best, so they can decide which version to deploy.
* **Microservices**: Kuberpult is built on the assumption that our teams work with kubernetes microservices.
* **Monorepo**: Kuberpult is built for a monorepo setup. One product should be one monorepo. If you have multiple products, consider giving each one a kuberpult instance.


## API
Kuberpult has an API that is intended to be used in CI/CD (GitHub Actions, Azure Pipelines, etc.) to release new versions of one (or more) microservices.
The API can also roll out many services at the same time via "release trains". It also supports rolling out some groups of services.

# Argo CD
Kuberpult works best with [Argo CD](https://argo-cd.readthedocs.io/en/stable/) which applies the
manifests to your clusters and Kuberpult helps you to manage those manifests in the repository.

`Kuberpult` does not actually `deploy`. That part is usually handled by Argo CD.

# App Locks & Environment Locks
`Kuberpult` can handle *locks* in its UI. When something is locked, it's version will not be changed via the API.
Both *environments* and *microservices* can be `locked`.

## Public releases of Kuberpult

### Docker Registries
Kuberpult's docker images are currently available in 2 docker registries: (Example with version 0.4.55)
* `docker pull europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/kuberpult-frontend-service:0.4.55` ([Link for Kuberpult devs](https://console.cloud.google.com/artifacts/docker/fdc-public-docker-registry/europe-west3/kuberpult/kuberpult-frontend-service))
* `docker pull ghcr.io/freiheit-com/kuberpult/kuberpult-frontend-service:0.4.55` ([Link for Kuberpult devs](https://github.com/freiheit-com/kuberpult/pkgs/container/kuberpult%2Fkuberpult-frontend-service))
And the same applies for the `kuberpult-cd-service` - just replace "frontend" by "cd".

We may deprecate one of the registries in the future for simplicity.

If you're using Kuberpult's helm chart, generally you don't have to worry about that.

### GitHub Releases

To use the helm chart, you can use [this url](https://github.com/freiheit-com/kuberpult/releases/download/0.4.55/kuberpult-0.4.55.tgz) (replace both versions with the current version!).
You can see all releases on the [Releases page on GitHub](https://github.com/freiheit-com/kuberpult/releases)

#### Helm Chart 
See [values.yaml.tpl](https://github.com/freiheit-com/kuberpult/blob/main/charts/kuberpult/values.yaml.tpl) for details like default values.

Most important helm chart parameters are:
* `git.url`: **required** - The url of the git manifest repository. This is a shared repo between Kuberpult and Argo CD.
* `git.branch`: **recommended** - Branch name of the git repo. Must be the same one that Argo CD uses.
* `ssh.identity`: **required** - The ssh private key to access the git manifest repo.
* `ssh.known_hosts`: - The ssh key fingerprints of for your git provider (e.g. [GitHub](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/githubs-ssh-key-fingerprints))
* `pgp.keyring`: **recommended** - Additional security. Highly recommended if you do not us IAP. If enabled, calls to the REST API need to provide signatures with each call. See [create-release](https://github.com/freiheit-com/kuberpult/blob/main/infrastructure/scripts/create-testdata/create-release.sh) for an example.
* `ingress.create`: **recommended** - If you want to use your own ingress, set to false.
* `ingress.iap`: **recommended** - We recommend to use IAP, but note that this a GCP-only feature.
* `datadogTracing`: **recommended** - We recommend using Datadog for tracing. Requires the [Datadog daemons to run on the cluster](https://docs.datadoghq.com/containers/kubernetes/installation/?tab=operator).
* `dogstatsdMetrics`: **optional** - As of now Kuberpult sends very limited metrics to Datadog, so this is optional.
* `auth.aureAuth.enabled`: **recommended** - Enable this on Azure to limit who can use Kuberpult. Alternative to IAP. Requires an Azure "App" to be set up.

## Releasing a new version
See [endpoint-release](./docs/endpoint-release.md)

## Release Train Overview
See [release train](./docs/release-train.md)

#### Environment Config
See [environment](./docs/environment.md)


## Best practices

### Remove individual environments from a service
See [Remove Env From Service](./docs/remove-env-from-service.md)

### Remove a service entirely
See [Remove Service Entirely](./docs/remove-service.md)

