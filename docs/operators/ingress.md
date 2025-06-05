# Ingress

Kuberpult comes with an optional Ingress.
The helm chart provides parameters to allow/deny individual paths.

| helm option                       | Use Case                                                                                                |
|-----------------------------------|---------------------------------------------------------------------------------------------------------|
| `ingress.allowedPaths.ui`         | Required to use kuberpult's UI. Most setups will need this to be enabled.                               |
| `ingress.allowedPaths.dex`        | Enable if you have dex enabled, see parameter `auth.dexAuth.enabled`                                    |
| `ingress.allowedPaths.oldRestApi` | Enable if you are using endpoints starting with `/release`,  `/environments/` or `/environment-groups/` |
| `ingress.allowedPaths.restApi`    | Enable if you are using endpoints starting with `/api/`                                                 |

Note that by default all of them are **disabled**.

This split can be used to improve security.
Only allow what is really required.

