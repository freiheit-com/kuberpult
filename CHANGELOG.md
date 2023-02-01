# Note on Semantic Versioning

As of now, while we are still on version `0.*`,
we do not use [Semantic Versioning](https://semver.org/).
Specifically this means, that minor upgrades can contain **breaking changes**.

# Change Log

## 0.4.54
* This release is only useful if you want to take a look at the new kuberpult UI - which is still beta and not shown by default. It's reachable under the url path `/v2/home`
* Render environment groups in UI [#465](https://github.com/freiheit-com/kuberpult/pull/465)

## 0.4.53
* Bugfix: Do not require authentication in frontend-service health check [#467](https://github.com/freiheit-com/kuberpult/pull/467)

## 0.4.52
* Publish the docker images additionally to the ghcr.io registry [#459](https://github.com/freiheit-com/kuberpult/pull/459)

## 0.4.51

**released 2023-01-17**
* Bugfix: Write undeploy versions correctly into manifest repo [#412](https://github.com/freiheit-com/kuberpult/pull/412)
* route /v2/home to index.html in build directory [#454](https://github.com/freiheit-com/kuberpult/pull/454)
* Add EnvironmentGroup to getOverview endpoint [#449](https://github.com/freiheit-com/kuberpult/pull/449)
* Backend: Add environmentGroup to envConfig [#447](https://github.com/freiheit-com/kuberpult/pull/447)
* Removed misleading tooltip about queues in old UI [#443](https://github.com/freiheit-com/kuberpult/pull/443)

## 0.4.50
* Bugfix: Show lock message text & label properly [#427](https://github.com/freiheit-com/kuberpult/pull/427)

## 0.4.49
* Add teams filter to releasetrain endpoint over rest [#417](https://github.com/freiheit-com/kuberpult/pull/417)

## 0.4.48
* Make Ingress optional [#400](https://github.com/freiheit-com/kuberpult/pull/400)
  This allows you to use the kuberpult helm chart without the ingress.

## 0.4.47

**released 2022-11-18**
* cd-service changed from StatefulSet to Deployment [#397](https://github.com/freiheit-com/kuberpult/pull/397)
This means that the bug regarding "no space left on device" is fixed: The kuberpult cd-service will now automatically restart with a new disk when this happens.
In order to really benefit from this, you need to have some form of retry for the failed curl/grpc request though.
* Add action item and list, component design, logic, and tests [#394](https://github.com/freiheit-com/kuberpult/pull/394)

### Breaking change
* After the upgrade, the PersistantVolumeClaim with name:`repository` should be removed manually, because it's not needed anymore.
* Stop deploying from the queue after deleting the lock directly and remove the `Delete Queue` button from the UI [#396](https://github.com/freiheit-com/kuberpult/pull/396)

### About the Queue behavior change
When kuberpult gets a request to deploy a microservice, and at the same time there is a lock, that puts us in a tricky situation. On one hand the user wants to deploy this, on the other the service is locked, indicating they don't want to deploy.

In the past (version <= 0.4.46) kuberpult queued deployments.
This means that it saved the version that was requested to deployed, but didn't actually deploy it yet.
Once the last lock on that microservice (incl environment locks) was removed, the queued version was deployed and the queue was removed.
This was reasonable, but never easy to explain.
Especially because deployment requests that encounter an *environment lock* behaved different: These did not create a queue at all.

If this is still not clear, that's exactly the point ;) It's difficult to understand this behavior. That's why we changed it!

From now on (version >= 0.4.47) there is never a *magical* deployment that happens just because someone deletes a lock.
Queues still exist in the database (git repository) and the UI, however they don't deploy anything anywhere ever.
They only document the fact that "hey, someone tried to deploy this, but kuberpult couldn't do that because there was a lock".

Release trains that run into an *environment lock* will still cancel completely, as there is nothing to deploy. This behavior is unchanged.

Note that for both versions, manual deployments (via the UI) were and are always allowed, no matter the lock situation. All power to the engineers!

## 0.4.46

**released 2022-11-03**
* Add a response body to releaseTrains endpoint [#389](https://github.com/freiheit-com/kuberpult/pull/389)
* Add latest as upstream option in releaseTrains response body [#391](https://github.com/freiheit-com/kuberpult/pull/391)

## 0.4.45

**released 2022-10-25**
* Implement Environments Page [#381](https://github.com/freiheit-com/kuberpult/pull/381)
* Add Dropdown to select team in new UI [#380](https://github.com/freiheit-com/kuberpult/pull/380)
* SRX-WQYHIS Always sort apps by team name [#383](https://github.com/freiheit-com/kuberpult/pull/383)
* Add Azure Auth to V2 [#379](https://github.com/freiheit-com/kuberpult/pull/379)
* Implement Release train from latest [#386](https://github.com/freiheit-com/kuberpult/pull/386)

## 0.4.44

**released 2022-10-11**
* More frequent Datadog metrics [#375](https://github.com/freiheit-com/kuberpult/pull/375)
* Implementation for application name filtering [#374](https://github.com/freiheit-com/kuberpult/pull/374)
* Refresh overview after getting id token (logging in) [#377](https://github.com/freiheit-com/kuberpult/pull/377)

## 0.4.43

**released 2022-10-07**
* Fixed an issue where the legacy ui did not reload the data on connection errors [#371](https://github.com/freiheit-com/kuberpult/pull/371)

## 0.4.42

**released 2022-09-27**
* implement locks page in new ui [#363](https://github.com/freiheit-com/kuberpult/pull/363)
* Verify put and delete environment lock requests with pgp [#365](https://github.com/freiheit-com/kuberpult/pull/365)

## 0.4.41

**released 2022-09-26**
* Upgrade to React 18, Add react-use-sub [#345](https://github.com/freiheit-com/kuberpult/pull/345)
* Refactoring in new ui components [#340](https://github.com/freiheit-com/kuberpult/pull/340)
* Use logged-in userdata from Azure [#343](https://github.com/freiheit-com/kuberpult/pull/343)
* Add service lanes and show the new kuberpult homepage [#347](https://github.com/freiheit-com/kuberpult/pull/347)
* Force using gpg when on Azure authentication [#355](https://github.com/freiheit-com/kuberpult/pull/355)
* Verify releasetrain requests with pgp [#357](https://github.com/freiheit-com/kuberpult/pull/357)

## 0.4.40

**released 2022-09-07**
* Update kuberpult to use multi-stage execution plan [#324](https://github.com/freiheit-com/kuberpult/pull/324)
* Add sidebar in kuberpult ui [#325](https://github.com/freiheit-com/kuberpult/pull/325)
* Add chip component [#328](https://github.com/freiheit-com/kuberpult/pull/328)
* Add Release card component [#329](https://github.com/freiheit-com/kuberpult/pull/329)
* Add release api in frontend [#334](https://github.com/freiheit-com/kuberpult/pull/334)

## 0.4.39
**released 2022-08-30**
* Increase memory limit for frontend service [#321](https://github.com/freiheit-com/kuberpult/pull/321)
* Add PR number in overview service [#316](https://github.com/freiheit-com/kuberpult/pull/316)

## 0.4.38
**released 2022-08-29**

* don't show delete queue button for undeployed version [#310](https://github.com/freiheit-com/kuberpult/pull/310)
* Add navigation indicator [#311](https://github.com/freiheit-com/kuberpult/pull/311)
* Return deployed releases [#315](https://github.com/freiheit-com/kuberpult/pull/315)
#### Breaking Changes:
* run the migration script during downtime in deployment to avoid errors [Readme](https://github.com/freiheit-com/kuberpult/blob/main/infrastructure/scripts/metadata-migration/Migration.md)
* Remove History package and persistent cache. [#282](https://github.com/freiheit-com/kuberpult/pull/282)
* Add scripts for metadata migration. [#307](https://github.com/freiheit-com/kuberpult/pull/307)

## 0.4.37
**released 2022-08-18**

* Remove authentication requirement from path "/static/js" and "/static/css" in frontend [#305](https://github.com/freiheit-com/kuberpult/pull/305)

## 0.4.36
**released 2022-08-18**

* Remove authentication requirement from path "/" in frontend [#302](https://github.com/freiheit-com/kuberpult/pull/302)

## 0.4.35

**released 2022-08-18**

* Add authentication to grpc api calls in frontend [#299](https://github.com/freiheit-com/kuberpult/pull/299)
* Add Basic Structure of the New UI [#294](https://github.com/freiheit-com/kuberpult/pull/294)

## 0.4.34

**released 2022-08-15**

* Adding azure AD auth in UI on kuberpult [#285](https://github.com/freiheit-com/kuberpult/pull/285)
* Add material components [#287](https://github.com/freiheit-com/kuberpult/pull/287)

## 0.4.33

**released 2022-08-11**

### Fixed
* Temporarily going back to use a persistant volume [#290](https://github.com/freiheit-com/kuberpult/pull/290)

## 0.4.32

**released 2022-07-27**

### NOTE
For helm installation:
Due to a change in the statefulset, it is required to first tear down the `helm_release` kuberpult resource and then re-create it.
Reason: Kubernetes forbids certain changes on stateful sets on the fly.

### Fixed
* Fixed issue with disk running full [#266](https://github.com/freiheit-com/kuberpult/pull/266)


## 0.4.31

**released 2022-07-26**

### Fixed
* fix argocd get full url function [#262](https://github.com/freiheit-com/kuberpult/pull/262)

## 0.4.30

**released 2022-07-26**

### Added
* Added link to Argocd in UI [#254](https://github.com/freiheit-com/kuberpult/pull/254)
* Added v2 route for the new kuberpult UI  [#245](https://github.com/freiheit-com/kuberpult/pull/245)

## 0.4.29

**released 2022-07-25**

### Added

* When the upstream version of an application is different, and the application is deployed manually, a lock for the application is added automatically to prevent release train override [#258](https://github.com/freiheit-com/kuberpult/pull/258)

## 0.4.28

**released 2022-07-22**

### Changed

* Added fields `appProjectNamespace` and `applicationNamespace` to ArgoCD application config to allow for better control over generated manifests [#250](https://github.com/freiheit-com/kuberpult/pull/250)

## 0.4.27

**released 2022-07-21**

### Removed

* Removed the argocd sync endpoint [#234](https://github.com/freiheit-com/kuberpult/pull/234)

### Added

* Added support for bootstrap mode in the helm chart [#240](https://github.com/freiheit-com/kuberpult/pull/240)
* Display the owner of an application and add an url parameter to filter them [#222](https://github.com/freiheit-com/kuberpult/pull/222)

### Improved

* Better error messages for certain transformers [#244](https://github.com/freiheit-com/kuberpult/pull/244)
* Improved the build system by generating the version in a dedicated step [#242](https://github.com/freiheit-com/kuberpult/pull/242) [#246](https://github.com/freiheit-com/kuberpult/pull/246) [#248](https://github.com/freiheit-com/kuberpult/pull/248)

## 0.4.26

**released 2022-07-14**

### Added

* Added HTST security header to frontend service [#241](https://github.com/freiheit-com/kuberpult/pull/241)

## 0.4.25

**released 2022-07-06**

### Added

* Added support to ignore argo fields managedFieldsManagers and jqPathExpressions [#230](https://github.com/freiheit-com/kuberpult/pull/230)

## 0.4.24

**released 2022-07-06**

### Fixed

* Fix ArgoCd SyncWindow configuration [#227](https://github.com/freiheit-com/kuberpult/pull/227)

## 0.4.23

**released 2022-07-05**

### Added

* Push multiple actions together as one [#202](https://github.com/freiheit-com/kuberpult/pull/202)

## 0.4.22

**released 2022-07-04**

### Added

## 0.4.22

**released 2022-07-04**

### Added

* Allow configuration of "environment" datadog metrics and traces get reported to [#197](https://github.com/freiheit-com/kuberpult/pull/197)
* Enhanced support for tracing Kuberpult internals [#198](https://github.com/freiheit-com/kuberpult/pull/198)
* Get builder images from make get-builder-image [#199](https://github.com/freiheit-com/kuberpult/pull/199)
* Update release instructions by @mnishamk-freiheit in https://github.com/freiheit-com/kuberpult/pull/193
* Make datadog environment configurable by @fdcds in https://github.com/freiheit-com/kuberpult/pull/197
* Support tracing of gRPC requests by @fdcds in https://github.com/freiheit-com/kuberpult/pull/198
* Clearer job names for matrix jobs by @mnishamk-freiheit in https://github.com/freiheit-com/kuberpult/pull/202
* Fix lint and test helm charts by @mnishamk-freiheit in https://github.com/freiheit-com/kuberpult/pull/196
* Add documentation for Podman, fix typos and small errors by @tameremad in https://github.com/freiheit-com/kuberpult/pull/138
* add Etymology of kuberpult by @sven-urbanski-freiheit-com in https://github.com/freiheit-com/kuberpult/pull/208
* Move lock message inputs from inline to actions cart by @tameremad in https://github.com/freiheit-com/kuberpult/pull/128
* Fix: Show sync window warning by @fdcds in https://github.com/freiheit-com/kuberpult/pull/203
* SRX-4SBVE2 Add tracing envs to frontend by @hannesg in https://github.com/freiheit-com/kuberpult/pull/215

## 0.4.21

**released 2022-06-17**

### Added

* Customize annotations on the kuberpult ingress [#191](https://github.com/freiheit-com/kuberpult/pull/191)
* Support per-application ArgoCD sync windows [#180](https://github.com/freiheit-com/kuberpult/pull/180)

## 0.4.20

**releases 2022-06-16**

### Added

* Warning when manually deployed to production [#186](https://github.com/freiheit-com/kuberpult/pull/172)

## 0.4.19

**releases 2022-06-14**

### Added

* Release on tag creation  [#172](https://github.com/freiheit-com/kuberpult/pull/172)
* Increase default loadbalancer timeout to 300 [#183](https://github.com/freiheit-com/kuberpult/pull/183)

### Fixed

* Increase Kuberpult's memory limit & request [#182](https://github.com/freiheit-com/kuberpult/pull/182)

## 0.4.18

**releases 2022-06-13**

### Fixed

* Increase Kuberpult's Cpu limit & request [#173](https://github.com/freiheit-com/kuberpult/pull/173)

## 0.4.17

**releases 2022-05-31**

### Added

* It's now possible to specify sync options for argocd apps [#163](https://github.com/freiheit-com/kuberpult/pull/163)

### Fixed

* The rest endpoints of the frontend service work again [#164](https://github.com/freiheit-com/kuberpult/pull/164)

## 0.4.16

**released 2022-05-27**

### Added

* Add option to configure timeouts in loadbalancer [#156](https://github.com/freiheit-com/kuberpult/pull/156)

### Removed

* removed cd.pvc.storage from values [#155](https://github.com/freiheit-com/kuberpult/pull/155)


## 0.4.14

**released 2022-04-21**

### Added

* Custom configuration per application ( Owner )  [#123](https://github.com/freiheit-com/kuberpult/pull/123)
* Add option to replace actions on conflict [#124](https://github.com/freiheit-com/kuberpult/pull/124)

## 0.4.13

**released 2022-04-01**

### Changed

* Use `networking.k8s.io/v1` API Version for ingress. [#121](https://github.com/freiheit-com/kuberpult/pull/121)

## 0.4.12

**released 2022-03-30**

### Added

* Add locks table [#118](https://github.com/freiheit-com/kuberpult/pull/118)
* Add lock id to overview request and to release dialog [#119](https://github.com/freiheit-com/kuberpult/pull/119)

### Changed

* Optimize history calculation to reuse previously computed results [#115](https://github.com/freiheit-com/kuberpult/pull/115)

## 0.4.11

**released 2022-03-18**

## Added
* Added support in helm chart to customize the size of cd servcie PVC disk. [#116](https://github.com/freiheit-com/kuberpult/pull/116)

## 0.4.0

**released 2022-02-01**
## Added
- Support deleting applications [#71](https://github.com/freiheit-com/kuberpult/pull/71) [#72](https://github.com/freiheit-com/kuberpult/pull/72)
- Persist Action Authors in Manifest repo [#69](https://github.com/freiheit-com/kuberpult/pull/69)

## Changed

- health check improvement [#76](https://github.com/freiheit-com/kuberpult/pull/76)
- Reimplement History.Change in a much faster way [#47](https://github.com/freiheit-com/kuberpult/pull/47)

## 0.2.5

**released 2021-11-02**

## Changed

- Increase timeout for sync endpoint. [#15](https://github.com/freiheit-com/kuberpult/pull/15)

## 0.2.0

**released 2021-09-07**

## Added

- Sync endpoint for syncing all argocd apps in an environment [75](https://github.com/freiheit-com/fdc-continuous-delivery/pull/75)
