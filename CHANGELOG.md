# Change Log

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
