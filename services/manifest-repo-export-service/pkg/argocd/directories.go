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

package argocd

import (
	"fmt"
	"path/filepath"

	"github.com/freiheit-com/kuberpult/pkg/types"
)

type BracketDirectoryNames struct {
	BracketDirectory string // just the directory
	BracketPath      string // complete directory and filename
}

// BracketPaths returns path related to bracket rendering
func BracketPaths(env types.EnvName, bracket types.ArgoBracketName, appName types.AppName) *BracketDirectoryNames {
	dir := filepath.Join("environments", string(env), "brackets", string(bracket))
	manifestFilename := filepath.Join(dir, fmt.Sprintf("%s.yaml", appName))
	return &BracketDirectoryNames{
		BracketDirectory: dir,
		BracketPath:      manifestFilename,
	}
}
