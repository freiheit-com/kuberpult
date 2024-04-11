# Removing ArgoCD presync hooks

A Kubernetes resource may be labeled with `argocd.argoproj.io/hook: PreSync` to marked and tracked by ArgoCD as a _presync hook_. An ArgoCD presync hook is a resource that ArgoCD applies before the actual application manifests are applied. More information is avaiable on the [ArgoCD documentation](https://argo-cd.readthedocs.io/en/stable/user-guide/resource_hooks/).

A typical use case of a presync hook is running a Job prior to application release. In that case, an alternative way of achieving the same effect of a presync hook would be to package whatever your presync hook is into an `initContainer` in your deployments. Keep in mind that these `initContainer`s will run for every pod in the deployment, so it's up to you to ensure they are idempotent, and preferably the idempotent containers should be cheap to run.
