# Etymology

Kuberpult is a catapult for [kubernetes](https://kubernetes.io/) :) it catapults the containers of microservices to different stages in kubernetes clusters.

# About

**Kuberpult** helps you manage different versions of different microservices in different cluster.
While [Argo CD](https://argo-cd.readthedocs.io/en/stable) applies the *current* version of your services in clusters,
Kuberpult also helps you with managing what is deployed *next*.

# Purpose
The purpose of Kuberpult is to help roll out quickly yet organized.
We use it for requirements like this:
* We have 3 clusters, development, staging, production
* Any merged changes should instantly be deployed to development.
* After running some test on development, roll out to staging.
* Once a day we trigger a release train that deploys everything to production - using the versions of staging as source.
* Sometimes we want to prevent certain deployments, either of single services, or of entire clusters.
* We never want to deploy (to production) between 9am and 11am as these are peak business hours.


# Kuberpult Design Principles

We use these principles to decide what features to focus on. We may deviate a little, but in general
we don't want features in kuberpult that violate these points:

* **Automatic and regular deployments**: Kuberpult is built for teams who want to deploy often (e.g. daily) and setup automated processes for the deployments.
* **All power to the engineers**: Kuberpult never stops an engineer from deploying manually. The engineers know their services best, so they can decide which version to deploy.
* **Microservices**: Kuberpult is built on the assumption that our teams work with kubernetes microservices.
* **Monorepo**: Kuberpult is built for a monorepo setup. One product should be one monorepo. If you have multiple products, consider giving each one a kuberpult instance.

# Concepts
In its core, Kuberpult uses several concepts like `Releases`, `ReleaseTrains`, `Deployments`, `Locks` and etc. We will explain all these concepts later in the docs.
