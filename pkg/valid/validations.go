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

Copyright 2023 freiheit.com*/

package valid

import (
	"regexp"
	"strings"
)

const (
	MaxAppNameLen  = 39
	AppNameRegExp  = `\A[a-z0-9]+(?:-[a-z0-9]+)*\z`
	TeamNameRegExp = AppNameRegExp
	EnvNameRegExp  = AppNameRegExp
	CommitIDRegExp = `^[0-9a-fA-F]{40}$`
)

var (
	applicationNameRx = regexp.MustCompile(AppNameRegExp)
	teamNameRx        = regexp.MustCompile(TeamNameRegExp)
	envNameRx         = regexp.MustCompile(EnvNameRegExp)
	commitIDRx        = regexp.MustCompile(CommitIDRegExp)
)

// {application}-{environment} should be a valid dns name
func EnvironmentName(env string) bool {
	return len(env) < 21 && envNameRx.MatchString(env)
}
func ApplicationName(name string) bool {
	return len(name) <= MaxAppNameLen && applicationNameRx.MatchString(name)
}

func TeamName(name string) bool {
	return len(name) < 21 && teamNameRx.MatchString(name)
}

// Lock names must be valid file names
func LockId(lockId string) bool {
	return len(lockId) < 100 && len(lockId) > 1 && lockId != ".." && lockId != "." && !strings.ContainsAny(lockId, "/")
}

func SHA1CommitID(commitID string) bool {
	return commitIDRx.MatchString(commitID)
}