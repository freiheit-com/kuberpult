# Re-render with Locks

## Concept
The button "Re-render environment <env>" triggers the manifest-export-service to write all relevant files for Argo CD
into the manifest-repo again.
This is useful if the repo lost files, or if someone manually changed files, due to an emergency.


## Written files

"Re-render" touches three file locations:
1) `argocd/v1alpha/<env>.yaml`: This is always re-rendered.
2) `environments/<env>/applications/<app>/manifests/manifests.yaml`: Skipped if a manifest-lock exists on `<app>/<env>`.
3) `environments/<env>/brackets/<bracket>/<app>.yaml`: Skipped if a manifest-lock exists on **any** app in `<bracket>`.

