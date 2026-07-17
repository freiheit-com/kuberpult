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
	cleanupCluster(t)
	tLogf(t, "runSuffix: %s", runSuffix)
	appAlwaysThere := "bg-app-always-" + runSuffix
	appTest := "bg-app-test-" + runSuffix
	bracket1 := "bg-bracket1-" + runSuffix
	bracket2 := "bg-bracket2-" + runSuffix

	//versionTested := "v13.55.0"
	versionTested := ""

	tLog(t, "step 1: upgrade and disable bracket mode")
	// 13.55.0 is the last version without the bracket gate fix
	helmUpgrade(t, HelmUpgradeParams{OldVersion: versionTested, BracketsEnabled: true, DevelopmentEnabled: false, StagingEnabled: false, ChannelSize: 50})

	tLog(t, fmt.Sprintf("step 2a: create v1 release for app %s bracket1", appAlwaysThere))
	createRelease(t, appAlwaysThere, "sreteam", bracket1, "1", map[string]string{
		devNamespace: stableManifest(appAlwaysThere, devNamespace, "1"),
	})
	tLog(t, fmt.Sprintf("step 2b: create v1 release for app %s in bracket2", appTest))
	createRelease(t, appTest, "sreteam", bracket2, "1", map[string]string{
		devNamespace: stableManifest(appTest, devNamespace, "1"),
	})

	tLog(t, "step 3: configure argo to deny app creation")
	denyArgoAppCreate(t, devNamespace, bracket2)

	tLog(t, "step 4: dev=true")
	helmUpgrade(t, HelmUpgradeParams{OldVersion: versionTested, BracketsEnabled: true, DevelopmentEnabled: true, StagingEnabled: false, ChannelSize: 50})
	// now the rollout-service tries to create 2 brackets, but is allowed to only create one of them

	tLog(t, "step 5a1: wait for argo app "+bracket1)
	waitForArgoApp(t, devNamespace+"-"+bracket1)
	tLog(t, "step 5a2: wait for deployment annotation")
	waitForDeploymentAnnotation(t, devNamespace, appAlwaysThere+"-bracket-dep", "1")
	tLog(t, "step 5a3: collect creationTimes")
	creationTimes := map[deploymentKey]string{}
	for _, app := range []string{appAlwaysThere, appTest} {
		for _, ns := range []string{devNamespace} { // staging is not deployed at all
			k := deploymentKey{ns, app + "-bracket-dep"}
			creationTimes[k] = deploymentCreationTime(t, ns, app+"-bracket-dep")
			tLogf(t, "  %s/%s: %s", ns, app+"-bracket-dep", creationTimes[k])
		}
	}

	tLog(t, "step 5b: wait for always-there app to disappear due to bracket migration")
	waitForArgoAppGone(t, devNamespace+"-"+appAlwaysThere)

	tLog(t, "step 5c: test app stays present") // this will fail on the old version v13.55.0, but succeed on the current version
	assertArgoAppStaysPresent(t, devNamespace, appTest)

	tLog(t, "step 5d: wait for b2")
	waitForArgoAppGone(t, devNamespace+"-"+bracket2)

	tLog(t, "step 6: reset argo")
	undoDenyArgoAppCreate(t)

	tLog(t, "step 7a: create release just to trigger the overview")
	createRelease(t, appAlwaysThere, "sreteam", bracket1, "2", map[string]string{
		devNamespace: stableManifest(appTest, devNamespace, "2"),
	})

	tLog(t, "step 7b: wait for argo app")
	waitForArgoApp(t, devNamespace+"-"+bracket2)

	tLog(t, "step 7c: wait for argo app")
	waitForArgoAppGone(t, devNamespace+"-"+appTest)

}
