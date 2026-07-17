/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com*/

package kindbracketstest

import (
	"fmt"
	"testing"
)

// manifestsFor returns a stableManifest for the app/version on each given env.
func manifestsFor(app, version string, envs []string) map[string]string {
	manifests := map[string]string{}
	for _, env := range envs {
		manifests[env] = stableManifest(app, env, version)
	}
	return manifests
}

// TestBracketGate is the regression test for the bug where the faulty
// gate for app deletions triggered too early
// The basic setup is (all on dev!):
// - 1) helmUpgrade(dev=false)
// - 2a) create always-there-app with bracket b1 (this is there to "trick" the old code as it relied on any bracket to be there)
// - 2b) create test-app with bracket b2 (for now deployed without bracket)
// - 3) denyArgoCreation(bracket b2)
// - 4) helmUpgrade(dev=true)
// - // now rollout-svc will try to create two brackets, but we expect to see only one
// - 5a) waitForArgoApp(b1) //
// - 5b) waitForArgoAppGone(plain always-there-app)
// - 5c) assertArgoAppStaysPresent(plain test-app)
// - 5d) waitForArgoAppGone(b2)
// - 6) undoDenyArgoCreation(bracket b2)
// - 7a) waitForArgoApp(b2)
// - 7b) waitForArgoAppGone(plain test-app)
func TestBracketGate(t *testing.T) {
	tcs := []struct {
		Name                 string
		InputNamespace       string // either dev or AA
		InputIsAA            bool
		InputAllowedPrefixes []string
		InputDeniedPrefixes  []string
	}{
		{
			Name:                 "normal env",
			InputNamespace:       devNamespace,
			InputIsAA:            false,
			InputAllowedPrefixes: nil,
			InputDeniedPrefixes:  []string{devNamespace},
		},
		{
			Name:           "active/active env",
			InputNamespace: aaNamespace,
			InputIsAA:      true,
			// concrete Argo app names are "aa-<aaNamespace>-<concreteEnv>-<app>",
			// see aa-test/config.json (commonEnvPrefix "aa", concreteEnvs dev-1/dev-2)
			InputAllowedPrefixes: []string{"aa-" + aaNamespace + "-dev-1"},
			InputDeniedPrefixes:  []string{"aa-" + aaNamespace + "-dev-2"},
		},
	}

	for _, tc := range tcs {
		cleanupCluster(t)
		tLogf(t, "runSuffix: %s", runSuffix)
		appAlwaysThere := "bg-app-always-" + runSuffix
		appTest := "bg-app-test-" + runSuffix
		bracket1 := "bg-bracket1-" + runSuffix
		bracket2 := "bg-bracket2-" + runSuffix
		allPrefixes := append(append([]string{}, tc.InputAllowedPrefixes...), tc.InputDeniedPrefixes...)
		manifestEnvs := []string{tc.InputNamespace}
		if tc.InputIsAA {
			// aa-test upstreams from development2 (upstream.latest): releases
			// auto-deploy there and are promoted into aa-test via a release train
			manifestEnvs = []string{devTwoNamespace, aaNamespace}
		}

		//versionTested := "v13.55.0"
		versionTested := ""

		tLog(t, "step 1: upgrade and disable bracket mode")
		// 13.55.0 is the last version without the bracket gate fix
		helmUpgrade(t, HelmUpgradeParams{OldVersion: versionTested, BracketsEnabled: true, DevelopmentEnabled: false, StagingEnabled: false, ChannelSize: 50})

		tLog(t, fmt.Sprintf("step 2a: create v1 release for app %s bracket1", appAlwaysThere))
		createRelease(t, appAlwaysThere, "sreteam", bracket1, "1", manifestsFor(appAlwaysThere, "1", manifestEnvs))
		tLog(t, fmt.Sprintf("step 2b: create v1 release for app %s in bracket2", appTest))
		createRelease(t, appTest, "sreteam", bracket2, "1", manifestsFor(appTest, "1", manifestEnvs))

		if tc.InputIsAA {
			// to deploy to AA we need to run a train, because it's not upstream.latest:
			tLog(t, "step 2c: release train into "+aaNamespace)
			releaseTrain(t, aaNamespace)
		}
		tLog(t, "step 2d: wait for plain argo apps")
		for _, prefix := range allPrefixes {
			waitForArgoApp(t, prefix+"-"+appAlwaysThere)
			waitForArgoApp(t, prefix+"-"+appTest)
		}

		tLog(t, "step 3: configure argo to deny app creation")
		deniedApps := make([]string, 0, len(tc.InputDeniedPrefixes))
		for _, prefix := range tc.InputDeniedPrefixes {
			deniedApps = append(deniedApps, prefix+"-"+bracket2)
		}
		denyArgoAppCreate(t, deniedApps...)

		tLog(t, "step 4: enable bracket mode")
		upgradeParams := HelmUpgradeParams{OldVersion: versionTested, BracketsEnabled: true, ChannelSize: 50}
		if tc.InputIsAA {
			upgradeParams.AATestEnabled = true
		} else {
			upgradeParams.DevelopmentEnabled = true
		}
		helmUpgrade(t, upgradeParams)
		// now the rollout-service tries to create the brackets, but is allowed to only create one of them

		tLog(t, "step 5a1: wait for argo app "+bracket1)
		for _, prefix := range allPrefixes {
			waitForArgoApp(t, prefix+"-"+bracket1)
		}

		tLog(t, "step 5a2: wait for deployment annotation")
		waitForDeploymentAnnotation(t, tc.InputNamespace, appAlwaysThere+"-bracket-dep", "1")
		tLog(t, "step 5a3: collect creationTimes")
		creationTimes := map[deploymentKey]string{}
		for _, app := range []string{appAlwaysThere, appTest} {
			k := deploymentKey{tc.InputNamespace, app + "-bracket-dep"}
			creationTimes[k] = deploymentCreationTime(t, tc.InputNamespace, app+"-bracket-dep")
			tLogf(t, "  %s/%s: %s", tc.InputNamespace, app+"-bracket-dep", creationTimes[k])
		}

		tLog(t, "step 5b: wait for always-there app to disappear due to bracket migration")
		for _, prefix := range allPrefixes {
			waitForArgoAppGone(t, prefix+"-"+appAlwaysThere)
		}

		tLog(t, "step 5c: test app stays present")
		for _, prefix := range tc.InputAllowedPrefixes {
			waitForArgoApp(t, prefix+"-"+bracket2)
			waitForArgoAppGone(t, prefix+"-"+appTest)
		}

		tLog(t, "step 5d: wait for b2")
		for _, prefix := range tc.InputDeniedPrefixes { // this will fail on the old version v13.55.0, but succeed on the current version
			assertArgoAppStaysPresent(t, prefix, appTest)
			waitForArgoAppGone(t, prefix+"-"+bracket2)
		}

		tLog(t, "step 6: reset argo")
		undoDenyArgoAppCreate(t)

		tLog(t, "step 7a: create release just to trigger the overview")
		createRelease(t, appTest, "sreteam", bracket2, "2", manifestsFor(appTest, "2", manifestEnvs))

		if tc.InputIsAA {
			// to deploy to AA we need to run a train, because it's not upstream.latest:
			tLog(t, "step 7a2: release train into "+aaNamespace)
			releaseTrain(t, aaNamespace)
		}
		tLog(t, "step 7b: wait for argo app")
		for _, prefix := range tc.InputDeniedPrefixes { // this will fail on the old version v13.55.0, but succeed on the current version
			waitForArgoApp(t, prefix+"-"+bracket2)
		}

		tLog(t, "step 7c: wait for argo app")
		for _, prefix := range tc.InputDeniedPrefixes { // this will fail on the old version v13.55.0, but succeed on the current version
			waitForArgoAppGone(t, prefix+"-"+appTest)

		}
	}
}
