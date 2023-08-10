# Note on Semantic Versioning

As of now, while we are still on version `0.*`,
we do not use [Semantic Versioning](https://semver.org/).
Specifically this means, that minor upgrades can contain **breaking changes**.

# Change Log

## 0.4.83
**released 2023-08-10**

### Breaking Changes
none

### Major Changes
* [Fixed bugs in release train](https://github.com/freiheit-com/kuberpult/pull/867)
* [Allow ArgoCd to use Webhooks to improve performance](https://github.com/freiheit-com/kuberpult/pull/868)

### Minor Changes
* [Update CHANGELOG.md: fix version number 81->82 ](https://github.com/freiheit-com/kuberpult/pull/861)
* [Update module golang.org/x/oauth2 to v0.11.0](https://github.com/freiheit-com/kuberpult/pull/792)
* [Update module gopkg.in/DataDog/dd-trace-go.v1 to v1.53.0](https://github.com/freiheit-com/kuberpult/pull/856)
* [Update k8s.io/utils digest to 3b25d92](https://github.com/freiheit-com/kuberpult/pull/842)


## 0.4.82
**released 2023-08-07**

### Breaking Changes
none

### Major Changes
* [Do not render undeployed argoCd apps](https://github.com/freiheit-com/kuberpult/pull/854)
* [Render spinner while loading](https://github.com/freiheit-com/kuberpult/pull/858)

### Minor Changes
* [DeleteEnvironmentApplicationLock Role Based Access Control](https://github.com/freiheit-com/kuberpult/pull/833)
* [DeployApplicationVersion Role based Access Control](https://github.com/freiheit-com/kuberpult/pull/831)
* [DeleteEnvironmentApplication Role Based Access Control](https://github.com/freiheit-com/kuberpult/pull/836)
* [CreateApplicationVersion Role Based Access Control ](https://github.com/freiheit-com/kuberpult/pull/849)
* [ReleaseTrain Role Based Access Control](https://github.com/freiheit-com/kuberpult/pull/851)
* [CreateUndeployApplicationVersion Role Based Access Control](https://github.com/freiheit-com/kuberpult/pull/837)
* [Update module google.golang.org/api to v0.134.0](https://github.com/freiheit-com/kuberpult/pull/771)
* [CreateEnvironment add Role Based Access Control ](https://github.com/freiheit-com/kuberpult/pull/848)
* [UndeployApplication Role Based Access Control](https://github.com/freiheit-com/kuberpult/pull/850)
* [Update dependency node to v18.17.0](https://github.com/freiheit-com/kuberpult/pull/846)
* [Add files related to run-kind.sh to gitignore ](https://github.com/freiheit-com/kuberpult/pull/859)

## 0.4.81
**released 2023-08-03**

### Breaking Changes
none

### Major Changes
* [Fix broken link in small environmentGroupChip](https://github.com/freiheit-com/kuberpult/pull/844)

### Minor Changes
* [Improve azure error handling](https://github.com/freiheit-com/kuberpult/pull/840)
* [Add Wildcard Check for Env/EnvGroup/Application](https://github.com/freiheit-com/kuberpult/pull/841)
* [Add App/Environment wildcard permission check](https://github.com/freiheit-com/kuberpult/pull/794)
* [Update golang Docker tag to v1.20.7](https://github.com/freiheit-com/kuberpult/pull/843)
* [Update module github.com/argoproj/argo-cd/v2 to v2.7.10](https://github.com/freiheit-com/kuberpult/pull/841)
* [Update module google.golang.org/grpc to v1.57.0](https://github.com/freiheit-com/kuberpult/pull/794)
* [Update module github.com/coreos/go-oidc/v3 to v3.6.0](https://github.com/freiheit-com/kuberpult/pull/795)

---

## 0.4.80
**released 2023-08-02**

### Breaking Changes
none

### Major Changes
* [Add links to ArgoCd in the UI](https://github.com/freiheit-com/kuberpult/pull/835)

### Minor Changes
* [Added Changelog information about IAP for /release endpoint](https://github.com/freiheit-com/kuberpult/pull/821)
* [Block Users without Permission from Creating Environment Locks](https://github.com/freiheit-com/kuberpult/pull/815)
* [Refactoring: rename httperrors.go to grpc/errors.go](https://github.com/freiheit-com/kuberpult/pull/825)
* [Refactor checkUserPermissions in auth/rbac.go](https://github.com/freiheit-com/kuberpult/pull/823)
* [CheckPermissions function moved to transformer](https://github.com/freiheit-com/kuberpult/pull/826)
* [DeleteEnvironmentLock Role Based access Checked if Dex is Enabled](https://github.com/freiheit-com/kuberpult/pull/828)
* [CreateEnvironmentApplicationLock Role Based access](https://github.com/freiheit-com/kuberpult/pull/830)
* [Refactoring: Use own tooltip component](https://github.com/freiheit-com/kuberpult/pull/832)
* [Refactor permission policy RBAC](https://github.com/freiheit-com/kuberpult/pull/834)

---

## 0.4.79
**released 2023-07-26**

### Breaking Changes

If you are directly calling the /release endpoint in the cd-service, then this is a breaking change for you.

#### What you need to do
If you use the helm variable `ingress.exposeReleaseEndpoint`, you need to remove it, before using the helm chart.
This variable was necessary to open a connection directly to the cd-service - bypassing the frontend-service.
The frontend-service itself does not require this. The helm chart now returns an error, if the variable is set at all.

#### IAP specifics
Additionally, if you are using google IAP (ingress.iap.enabled=true in the helm chart),
you need to now provide an IAP token when invoking the `/release` endpoint.
To get a token, you can find an example script in [Google's documentation](https://cloud.google.com/iap/docs/authentication-howto#bash).

#### Explanation
The `/release` endpoint was moved from the cd-service to the frontend-service.
Some very specific http error codes for `/release` also changed.
We now return the http code 500 less often, and replaced it with 400, for example when the signature does not match.
Apart from the status codes in very specific situations, the endpoint works as before.

For more details see the Pull Request [Replaced /release http endpoint in cd-service with grpc](https://github.com/freiheit-com/kuberpult/pull/814).

### Major Changes
none

### Minor Changes
* [Read Dex ClientID, ClientSecret and BaseURL from config map ](https://github.com/freiheit-com/kuberpult/pull/808)
* [Refactor checkbox to reduce material-ui usage](https://github.com/freiheit-com/kuberpult/pull/817)
* [Fix log level](https://github.com/freiheit-com/kuberpult/pull/819)


## 0.4.78
**released 2023-07-21**

### Breaking Changes
none

### Major Changes
none

### Minor Changes
* [Move functionality from DeploymentService and EnvironmentService to BatchService](https://github.com/freiheit-com/kuberpult/pull/807)
* [Update README.md with notes about /release endpoint](https://github.com/freiheit-com/kuberpult/pull/810)
* [Add RBAC policy parser methods](https://github.com/freiheit-com/kuberpult/pull/806)
* [Add Dex RBAC Config Map](https://github.com/freiheit-com/kuberpult/pull/801)
* [Add a dummy user to the rollout service](https://github.com/freiheit-com/kuberpult/pull/805)
* [Add missing argocd token secret](https://github.com/freiheit-com/kuberpult/pull/803)
* [Fix buf build](https://github.com/freiheit-com/kuberpult/pull/804)

## 0.4.77
**released 2023-07-07**

### Breaking Changes
none

### Major Changes
* [Hide Buttons behind a "..." menu in service lane](https://github.com/freiheit-com/kuberpult/pull/786)
* [Bugfix: Display locks only for the correct application](https://github.com/freiheit-com/kuberpult/pull/787)
* [UI: Delete environments from an app](https://github.com/freiheit-com/kuberpult/pull/788)

### Minor Changes
* [Update module k8s.io/component-helpers to v0.27.3](https://github.com/freiheit-com/kuberpult/pull/754)
* [Remove outdated LockService](https://github.com/freiheit-com/kuberpult/pull/789)
* [Add github pipeline badges to readme](https://github.com/freiheit-com/kuberpult/pull/797)
* [Removed renovate autorebase](https://github.com/freiheit-com/kuberpult/pull/798)
* [Add forwarder for rollout status to frontend service](https://github.com/freiheit-com/kuberpult/pull/758)
* [Use correct service names in all services](https://github.com/freiheit-com/kuberpult/pull/799)


## 0.4.76
**released 2023-07-04**

### Breaking Changes
no actual *breaking* changes, but we do now recommend setting the git author in the helm chart (values "git.author.name" and "git.author.email")
and whenever you call the "/release" endpoint in the cd-service (most of our users have a script called publish.sh which does just that).
See [this script](https://github.com/freiheit-com/kuberpult/blob/main/infrastructure/scripts/create-testdata/create-release.sh#L81) for an example.
See [this PR](https://github.com/freiheit-com/kuberpult/pull/765) for details.

If you do not care what string appears as git author in the manifest repo when kuberpult creates commits, you don't have to change anything.

### Major Changes
* [Display person who triggered deployment](https://github.com/freiheit-com/kuberpult/pull/784)
* [Make git author configurable in helm chart & refactor context usage](https://github.com/freiheit-com/kuberpult/pull/765)

### Minor Changes
* [Store person who triggered deployment](https://github.com/freiheit-com/kuberpult/pull/767)
* [Adds DEX methods to Auth Package ](https://github.com/freiheit-com/kuberpult/pull/782)
* [Update module google.golang.org/protobuf to v1.31.0](https://github.com/freiheit-com/kuberpult/pull/772)
* [Update alpine Docker tag to v3.18](https://github.com/freiheit-com/kuberpult/pull/751)
* [Update k8s.io/utils digest to 9f67429](https://github.com/freiheit-com/kuberpult/pull/748)
* [Update dependency typescript to v5.1.6](https://github.com/freiheit-com/kuberpult/pull/769)



## 0.4.75
**released 2023-07-04**
### Breaking Changes
none
### Major Changes
* [Replaced x/go-crypto with ProtonMail/go-crypto](https://github.com/freiheit-com/kuberpult/pull/781) This should fix a few pgp related issues.


## 0.4.74
**released 2023-07-03**
### Breaking Changes
none
### Major Changes
none
### Minor Changes
* [Explain replicas=1 of cd-service](https://github.com/freiheit-com/kuberpult/pull/766)
* [Added correct datadog annotations to frontend service](https://github.com/freiheit-com/kuberpult/pull/773)
* [Added error messages and error logs to the release endpoint](https://github.com/freiheit-com/kuberpult/pull/774)
* [Remove outdated Readme section about queues](https://github.com/freiheit-com/kuberpult/pull/775)
* [testdata: use correct port to call frontend service](https://github.com/freiheit-com/kuberpult/pull/776)
* [build all services if go.mod changed](https://github.com/freiheit-com/kuberpult/pull/778)
* [Refactoring: Move interceptors to frontend](https://github.com/freiheit-com/kuberpult/pull/779)


## 0.4.73
**released 2023-06-27**
### Breaking Changes
none
### Major Changes
* [Unify commit messages for unlocking](https://github.com/freiheit-com/kuberpult/pull/736)
* [Add team label to argo app](https://github.com/freiheit-com/kuberpult/pull/757)
* [Do not ignore git push errors](https://github.com/freiheit-com/kuberpult/pull/747)
* [Append email and username and pass it along to the batchservice](https://github.com/freiheit-com/kuberpult/pull/759)
* [Adding username/email as datadog tags to span](https://github.com/freiheit-com/kuberpult/pull/760)

### Minor Changes
* [Add rollout service base](https://github.com/freiheit-com/kuberpult/pull/660)
* [Add license to scss files](https://github.com/freiheit-com/kuberpult/pull/734)
* [Print container logs if integration tests fail](https://github.com/freiheit-com/kuberpult/pull/737)
* [Update module google.golang.org/grpc to v1.56.0 ](https://github.com/freiheit-com/kuberpult/pull/730)
* [Update module golang.org/x/crypto to v0.10.0](https://github.com/freiheit-com/kuberpult/pull/728)
* [Update module google.golang.org/api to v0.128.0 ](https://github.com/freiheit-com/kuberpult/pull/729)
* [Prepare for upgrade to node 18](https://github.com/freiheit-com/kuberpult/pull/741)
* [Update Node.js to v18 ](https://github.com/freiheit-com/kuberpult/pull/632)
* [Update dependency typescript to v5.1.3](https://github.com/freiheit-com/kuberpult/pull/709)
* [Adapt jest.useFakeTimers for jest upgrade ](https://github.com/freiheit-com/kuberpult/pull/743)
* [Update module github.com/MicahParks/keyfunc to v2 and jwt to v5](https://github.com/freiheit-com/kuberpult/pull/744)
* [Update dependency @types/jest to v29](https://github.com/freiheit-com/kuberpult/pull/616)
* [Add dashboard to renovate](https://github.com/freiheit-com/kuberpult/pull/745)
* [Add broadcast implementation and tests ](https://github.com/freiheit-com/kuberpult/pull/746)
* [Allow kuberpult to run on different machines and architectures](https://github.com/freiheit-com/kuberpult/pull/742)
* [Update module github.com/argoproj/argo-cd/v2 to v2.7.6](https://github.com/freiheit-com/kuberpult/pull/752)
* [Update module github.com/argoproj/gitops-engine to v0.7.3](https://github.com/freiheit-com/kuberpult/pull/749)
* [Revert "Update module github.com/argoproj/gitops-engine to v0.7.3"](https://github.com/freiheit-com/kuberpult/pull/763)
* [Update module google.golang.org/grpc to v1.56.1](https://github.com/freiheit-com/kuberpult/pull/750)
* [Update module gopkg.in/DataDog/dd-trace-go.v1 to v1.52.0](https://github.com/freiheit-com/kuberpult/pull/753)
* [fix port in create-release for local setup](https://github.com/freiheit-com/kuberpult/pull/762)


## 0.4.72
**released 2023-06-19**
### Breaking Changes
none
### Major Changes
* [Encode username and mail with base64](https://github.com/freiheit-com/kuberpult/pull/721)
* [Render warnings for unusual deployment situations](https://github.com/freiheit-com/kuberpult/pull/731)
* [Also lock when deploying manually](https://github.com/freiheit-com/kuberpult/pull/732)
### Minor Changes
* [Add documentation for how to obtain the gke config](https://github.com/freiheit-com/kuberpult/pull/711)
* [Run kind in CI with kuberpult & git server & environments as test data ](https://github.com/freiheit-com/kuberpult/pull/708)
* [Use team name for codeowners](https://github.com/freiheit-com/kuberpult/pull/713)
* [Remove unused endpoints Get/StreamDeployedOverview ](https://github.com/freiheit-com/kuberpult/pull/713)
* [Enable tracing for release endpoint](https://github.com/freiheit-com/kuberpult/pull/717)
* [Removed unused Field "environments"](https://github.com/freiheit-com/kuberpult/pull/716)
* [Run ArgoCd in Kind for integration tests](https://github.com/freiheit-com/kuberpult/pull/714)
* [Allow Origin * by default](https://github.com/freiheit-com/kuberpult/pull/720)
* [Update golang Docker tag to v1.20.5](https://github.com/freiheit-com/kuberpult/pull/718)
* [Allow easier app removal](https://github.com/freiheit-com/kuberpult/pull/722)
* [Delete Environment from App in backend](https://github.com/freiheit-com/kuberpult/pull/723)
* [fix release train script](https://github.com/freiheit-com/kuberpult/pull/726)
* [Build services when go files in /pkg were changed](https://github.com/freiheit-com/kuberpult/pull/724)
* [Update module google.golang.org/api to v0.127.0](https://github.com/freiheit-com/kuberpult/pull/700)
* [Update module gopkg.in/DataDog/dd-trace-go.v1 to v1.51.0](https://github.com/freiheit-com/kuberpult/pull/701)

## 0.4.71
**released 2023-05-31**
### Breaking Changes
* [Separate tags for services](https://github.com/freiheit-com/kuberpult/pull/705)

## 0.4.70
**released 2023-05-26**
### Minor Changes
* [Add X-Content-Option nosniff header](https://github.com/freiheit-com/kuberpult/pull/702)

## 0.4.69
**released 2023-05-25**
### Minor Changes
* [Fix CSP](https://github.com/freiheit-com/kuberpult/pull/698)

## 0.4.68
**released 2023-05-24**
### Major Changes
* [Add annotations to ArgoCD application](https://github.com/freiheit-com/kuberpult/pull/694)
* [Add X-Frame-Options, Referrer-Policy and Permission-Policy header](https://github.com/freiheit-com/kuberpult/pull/696)
* [Add more secure header settings](https://github.com/freiheit-com/kuberpult/pull/695)

## 0.4.67
**released 2023-05-19**
### Major Changes
* [Added git commit parameter and field to the overview service](https://github.com/freiheit-com/kuberpult/pull/663)
* [Refactoring Test: Do not connect to remote service during test](https://github.com/freiheit-com/kuberpult/pull/666)
* [Use same lock ID for locks in one batch](https://github.com/freiheit-com/kuberpult/pull/669)
* [Add button to delete all similar locks](https://github.com/freiheit-com/kuberpult/pull/670)
* [Validate environment groups and environments distance to upstream](https://github.com/freiheit-com/kuberpult/pull/683)
* [Make all UI paths available with hard reload](https://github.com/freiheit-com/kuberpult/pull/686)
* [Show more detailed Relative time, + refactoring](https://github.com/freiheit-com/kuberpult/pull/689)

### Minor Changes
* small bugfixes / improvements
  * [Reduce Whitespace in ReleaseDialog](https://github.com/freiheit-com/kuberpult/pull/682)
  * [Render kuberpult version in the html title](https://github.com/freiheit-com/kuberpult/pull/668)
  * [Render commit message nicer](https://github.com/freiheit-com/kuberpult/pull/671)
  * [Bugfix for deleting similar locks](https://github.com/freiheit-com/kuberpult/pull/681)
  * [fix hidden releases tooltip showing version undefined](https://github.com/freiheit-com/kuberpult/pull/678)
  * [Make Lock action summary consistent](https://github.com/freiheit-com/kuberpult/pull/667)
  * [Fix environment colors on Environments Page](https://github.com/freiheit-com/kuberpult/pull/679)
  * [Fix Navigation to History page clears search filters](https://github.com/freiheit-com/kuberpult/pull/684)
* dependency updates
  * [Update dependency rxjs to v7](https://github.com/freiheit-com/kuberpult/pull/649)
  * [Update dependency @testing-library/react to v14](https://github.com/freiheit-com/kuberpult/pull/596)
  * [Update dependency typescript to v5](https://github.com/freiheit-com/kuberpult/pull/622)
  * [Update docker Docker tag to v23.0.6](https://github.com/freiheit-com/kuberpult/pull/687)
  * [Update golang Docker tag to v1.20.4](https://github.com/freiheit-com/kuberpult/pull/672)
  * [Update alpine Docker tag to v3.18](https://github.com/freiheit-com/kuberpult/pull/688)
  * [Update dependency madge to v6](https://github.com/freiheit-com/kuberpult/pull/617)
  * [Update dependency protobufjs to v7](https://github.com/freiheit-com/kuberpult/pull/618)
  * [Update module google.golang.org/grpc to v1.55.0](https://github.com/freiheit-com/kuberpult/pull/674)
  * [Update module google.golang.org/api to v0.121.0](https://github.com/freiheit-com/kuberpult/pull/673)
  * [Update module golang.org/x/crypto to v0.9.0](https://github.com/freiheit-com/kuberpult/pull/685)
  * [Update module gopkg.in/DataDog/dd-trace-go.v1 to v1.50.1](https://github.com/freiheit-com/kuberpult/pull/675)

## 0.4.66
**released 2023-04-28**
### Major Changes
* [Fix UI scaling - HomePage](https://github.com/freiheit-com/kuberpult/pull/653)
* [Update docker Docker tag to v23.0.4](https://github.com/freiheit-com/kuberpult/pull/655)
* [Nicer commit message for release trains](https://github.com/freiheit-com/kuberpult/pull/656)
* [Add button to lock an entire environment group at once](https://github.com/freiheit-com/kuberpult/pull/662)

### Minor Changes
* [Update module github.com/cenkalti/backoff/v4 to v4.2.1](https://github.com/freiheit-com/kuberpult/pull/648)
* [Update module google.golang.org/api to v0.120.0](https://github.com/freiheit-com/kuberpult/pull/659)

## 0.4.65
**released 2023-04-18**

Note: this release contains a change that switches the underlying storage from git package files to sqlite. The change is completely transparent and should have no downsides. In case of problems, the helm chart has a new option `enableSqlite` that can be set to false to disable the new behaviour.

### Major Changes
* [Switch default storage backend to sqlite](https://github.com/freiheit-com/kuberpult/pull/592)
* [Fix: Allow emptying manifest of individual environments](https://github.com/freiheit-com/kuberpult/pull/650)
* [Allow url paths starting with home to be served](https://github.com/freiheit-com/kuberpult/pull/646)
* [Always Show the latest release on homepage](https://github.com/freiheit-com/kuberpult/pull/629)

### Minor Changes
* [Update module google.golang.org/api to v0.118.0](https://github.com/freiheit-com/kuberpult/pull/630)
* [Update module k8s.io/apimachinery to v0.27.1](https://github.com/freiheit-com/kuberpult/pull/631)
* [fix(deps): update module gopkg.in/datadog/dd-trace-go.v1 to v1.49.1](https://github.com/freiheit-com/kuberpult/pull/615)
* [fix(deps): update module golang.org/x/crypto to v0.8.0](https://github.com/freiheit-com/kuberpult/pull/621)
* Lots of small refactorings in order to enable [type assertions checks (633)](https://github.com/freiheit-com/kuberpult/pull/633)
  * [645](https://github.com/freiheit-com/kuberpult/pull/645)
  * [643](https://github.com/freiheit-com/kuberpult/pull/643)
  * [642](https://github.com/freiheit-com/kuberpult/pull/642)
  * [641](https://github.com/freiheit-com/kuberpult/pull/641)
  * [640](https://github.com/freiheit-com/kuberpult/pull/640)
  * [638](https://github.com/freiheit-com/kuberpult/pull/638)
  * [639](https://github.com/freiheit-com/kuberpult/pull/639)
  * [634](https://github.com/freiheit-com/kuberpult/pull/634)
  * [637](https://github.com/freiheit-com/kuberpult/pull/637)


## 0.4.64
**released 2023-04-13**
### Major Changes
* [Fix: Azure Auth for batch service in new UI](https://github.com/freiheit-com/kuberpult/pull/624)
* [Allow Release Dialog to open via query parameters](https://github.com/freiheit-com/kuberpult/pull/619)

### Minor Changes
* [improved readme on releasing kuberpult](https://github.com/freiheit-com/kuberpult/pull/627)
* [chore(deps): update docker docker tag to v23.0.3](https://github.com/freiheit-com/kuberpult/pull/612)


## 0.4.63
**released 2023-04-11**

### Major Changes
* [Workaround git repack issue by restarting the pod](https://github.com/freiheit-com/kuberpult/pull/601)
* [Display app locks in Overview](https://github.com/freiheit-com/kuberpult/pull/605)
* [Bugfix: Allow hard reload on UI ](https://github.com/freiheit-com/kuberpult/pull/604)

### Minor Changes
* [fix pnpm to version 7.30.5 in Docker images](https://github.com/freiheit-com/kuberpult/pull/599)
* [Update alpine image and add sqlite of the build image](https://github.com/freiheit-com/kuberpult/pull/600)
* [Update alpine image + libgit ](https://github.com/freiheit-com/kuberpult/pull/597)
* [fix(deps): update module google.golang.org/api to v0.114.0](https://github.com/freiheit-com/kuberpult/pull/583)
* [fix(deps): update module github.com/cenkalti/backoff/v4 to v4.2.0 (](https://github.com/freiheit-com/kuberpult/pull/568)
* [fix(deps): update module github.com/grpc-ecosystem/go-grpc-middleware](https://github.com/freiheit-com/kuberpult/pull/571)
* [fix(deps): update module k8s.io/apimachinery to v0.26.3](https://github.com/freiheit-com/kuberpult/pull/595)
* [fix(deps): update module google.golang.org/grpc to v1.54.0 ](https://github.com/freiheit-com/kuberpult/pull/584)
* [fix(deps): update module github.com/improbable-eng/grpc-web to v0.15.0](https://github.com/freiheit-com/kuberpult/pull/572)
* [fix(deps): update module google.golang.org/api to v0.117.0](https://github.com/freiheit-com/kuberpult/pull/614)
* [chore(deps): update golang docker tag to v1.20.3 ](https://github.com/freiheit-com/kuberpult/pull/613)



## 0.4.62
**released 2023-03-31**
### Summary:
* Old UI was removed
* Implemented various features that were missing from the old UI
* various library upgrades and bugfixes

### Major Changes:
* [New UI is now available under /](https://github.com/freiheit-com/kuberpult/pull/594)
* [Fix bug in app deletion when there are app locks](https://github.com/freiheit-com/kuberpult/pull/590)
* [Show Application and Team on Release Dialog](https://github.com/freiheit-com/kuberpult/pull/586)
* [Fix issue with deleting locks](https://github.com/freiheit-com/kuberpult/pull/589)
* [disable apply button, fix button ripples bug](https://github.com/freiheit-com/kuberpult/pull/579)
* [Allow deleting env locks from release dialog](https://github.com/freiheit-com/kuberpult/pull/585)
* [new UI: add prepareToUndeploy and Undeploy functions](https://github.com/freiheit-com/kuberpult/pull/580)
* [Retry connection on errors with exponential backoff](https://github.com/freiheit-com/kuberpult/pull/552)

### Internal changes & version updates:
* [update module gopkg.in/datadog/dd-trace-go.v1 to v1.48.0](https://github.com/freiheit-com/kuberpult/pull/587)
* [update module github.com/go-git/go-billy/v5 to v5.4.1](https://github.com/freiheit-com/kuberpult/pull/569)
* [update module github.com/golang-jwt/jwt/v4 to v4.5.0](https://github.com/freiheit-com/kuberpult/pull/570)
* [update module github.com/micahparks/keyfunc to v1.9.0](https://github.com/freiheit-com/kuberpult/pull/573)
* [update module go.uber.org/zap to v1.24.0](https://github.com/freiheit-com/kuberpult/pull/574)
* [refactoring: remove version -1 from everywhere](https://github.com/freiheit-com/kuberpult/pull/581)
* [Return undeploy summary in cd-service](https://github.com/freiheit-com/kuberpult/pull/578)
* [Update Github SSH key in certificates test](https://github.com/freiheit-com/kuberpult/pull/575)
* [Comments for queuing of transformers/requests](https://github.com/freiheit-com/kuberpult/pull/561)
* [update golang docker tag to v1.20.2](https://github.com/freiheit-com/kuberpult/pull/530)
* [update dependency @improbable-eng/grpc-web to ^0.15.0](https://github.com/freiheit-com/kuberpult/pull/527)
* [Integ tests Workflow: Add case for abbreviated version](https://github.com/freiheit-com/kuberpult/pull/564)
* [Update softprops-action-gh-release to use node16](https://github.com/freiheit-com/kuberpult/pull/562)
* [chore(deps): update node.js to v14.21.3](https://github.com/freiheit-com/kuberpult/pull/525)
* [update module github.com/google/go-cmp to v0.5.9](https://github.com/freiheit-com/kuberpult/pull/522)
* [update module github.com/libgit2/git2go/v33 to v33.0.9](https://github.com/freiheit-com/kuberpult/pull/523)
* [update module google.golang.org/protobuf to v1.30.0](https://github.com/freiheit-com/kuberpult/pull/524)
* [update module github.com/datadog/datadog-go/v5 to v5.3.0](https://github.com/freiheit-com/kuberpult/pull/549)
* [update module golang.org/x/crypto to v0.7.0](https://github.com/freiheit-com/kuberpult/pull/582)
* [Add api.go to pkg/api](https://github.com/freiheit-com/kuberpult/pull/560)


## 0.4.61
**released 2023-03-20**

This release contains 2 major bugfixes:
* Fix for creating older versions in release endpoint [#556](https://github.com/freiheit-com/kuberpult/pull/556)
* Fix undeploy for apps that are not in all environments [#555](https://github.com/freiheit-com/kuberpult/pull/555)

Other changes:
* (new UI) Fix rendering of group labels on ReleaseCards [#544](https://github.com/freiheit-com/kuberpult/pull/544)
* Warn on startup if an upstream does not exist [#550](https://github.com/freiheit-com/kuberpult/pull/550)
* (new UI) Fix 0 deployment [#554](https://github.com/freiheit-com/kuberpult/pull/554)


## 0.4.60
**released 2023-03-09**
* Add snackbar notifications [#517](https://github.com/freiheit-com/kuberpult/pull/517)
* Rephrase release dialog text [#532](https://github.com/freiheit-com/kuberpult/pull/532)
* Remove whitespace in ReleaseDialog of new UI [#531](https://github.com/freiheit-com/kuberpult/pull/531)
* Configure regular updates with Renovate [#515](https://github.com/freiheit-com/kuberpult/pull/515)

## 0.4.59
**released 2023-03-02**
* Allow deleting locks from the locks page [#509](https://github.com/freiheit-com/kuberpult/pull/509)
* Bugfix: fix distance to upstream [#513](https://github.com/freiheit-com/kuberpult/pull/513)

## 0.4.58
**released 2023-03-01**
* Update release train documentation [#507](https://github.com/freiheit-com/kuberpult/pull/507)
* Added Automatically open cart when actions [#505](https://github.com/freiheit-com/kuberpult/pull/505)
* Whitelist create environment endpoint in Azure Auth [#510](https://github.com/freiheit-com/kuberpult/pull/510)

## 0.4.57
**released 2023-02-27**
* Added warning if there are no envs configured during startup [#502](https://github.com/freiheit-com/kuberpult/pull/502)
* Update homepage design, add Tooltip, fix bug with date [#496](https://github.com/freiheit-com/kuberpult/pull/496)
* Added environment group release train [#504](https://github.com/freiheit-com/kuberpult/pull/504)
* Update release card design, show more releases on home [#503](https://github.com/freiheit-com/kuberpult/pull/503)

## 0.4.56
**released 2023-02-20**
* Improved error handling for release trains [#482](https://github.com/freiheit-com/kuberpult/pull/482)
* Display queued version in v2 release dialog [#491](https://github.com/freiheit-com/kuberpult/pull/491)
* Bugfix: Add a check if the application version is available or not [#493](https://github.com/freiheit-com/kuberpult/pull/493)
* Add createEnvironment http endpoint [#489](https://github.com/freiheit-com/kuberpult/pull/489)

## 0.4.55
* Upgrade go to v1.19.4 [#472](https://github.com/freiheit-com/kuberpult/pull/472)
* Fix Sorting of EnvironmentGroups [#474](https://github.com/freiheit-com/kuberpult/pull/474)
* Add documentation for config.json files [#475](https://github.com/freiheit-com/kuberpult/pull/475)

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
