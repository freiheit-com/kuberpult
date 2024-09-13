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

package valid

import (
	"fmt"
	"net/mail"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	FallbackMaxAppNameLength       = 39
	LongAppNameLength              = 70
	KUBERPULT_ALLOW_LONG_APP_NAMES = "KUBERPULT_ALLOW_LONG_APP_NAMES"
	AppNameRegExp                  = `\A[a-z0-9]+(?:-[a-z0-9]+)*\z`
	TeamNameRegExp                 = AppNameRegExp
	EnvNameRegExp                  = AppNameRegExp
	SHA1CommitIDLength             = 40
	commitIDPrefixRegExp           = `^[0-9a-fA-F]*$`
)

var (
	applicationNameRx = regexp.MustCompile(AppNameRegExp)
	teamNameRx        = regexp.MustCompile(TeamNameRegExp)
	envNameRx         = regexp.MustCompile(EnvNameRegExp)
	commitIDPrefixRx  = regexp.MustCompile(commitIDPrefixRegExp)
	MaxAppNameLen     = setupMaxAppNameLen()
)

func setupMaxAppNameLen() int {
	maxAppNameLength := FallbackMaxAppNameLength
	res, ok := os.LookupEnv(KUBERPULT_ALLOW_LONG_APP_NAMES)
	if ok && res == "true" {
		maxAppNameLength = LongAppNameLength
	}
	return maxAppNameLength
}

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

func UserEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

// Lock names must be valid file names
func LockId(lockId string) bool {
	return len(lockId) < 100 && len(lockId) > 1 && lockId != ".." && lockId != "." && !strings.ContainsAny(lockId, "/")
}

func SHA1CommitID(commitID string) bool {
	if len(commitID) != SHA1CommitIDLength {
		return false
	}
	return commitIDPrefixRx.MatchString(commitID)
}

func SHA1CommitIDPrefix(prefix string) bool {
	return commitIDPrefixRx.MatchString(prefix)
}

func ReadEnvVar(envName string) (string, error) {
	envValue, ok := os.LookupEnv(envName)
	if !ok {
		return "", fmt.Errorf("could not read environment variable '%s'", envName)
	}
	return envValue, nil
}

func ReadEnvVarUInt(envName string) (uint, error) {
	envValue, err := ReadEnvVar(envName)
	if err != nil {
		return 0, err
	}
	i, err := strconv.ParseUint(envValue, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("could not convert environment variable '%s=%s' to int", envName, envValue)
	}
	return uint(i), nil
}
