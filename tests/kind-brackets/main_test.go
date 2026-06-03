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
	"os"
	"testing"
)

var globalPFM   *portForwardManager
var globalCDPFM *portForwardManager
var globalCfg   testConfig

func TestMain(m *testing.M) {
	globalCfg = mustLoadConfig()
	globalPFM = startPFManager(globalCfg, "deployment/kuberpult-frontend-service", "5002:8081")
	globalCDPFM = startPFManager(globalCfg, "deployment/kuberpult-cd-service", "5004:8443")
	code := m.Run()
	globalPFM.stop()
	globalCDPFM.stop()
	os.Exit(code)
}
