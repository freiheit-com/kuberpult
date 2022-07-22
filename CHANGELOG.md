# Change Log

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
