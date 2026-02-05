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
	"time"

	"github.com/freiheit-com/kuberpult/pkg/types"
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
	groupNameRx       = regexp.MustCompile(EnvNameRegExp)
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
func EnvironmentName(env types.EnvName) bool {
	return len(env) < 21 && envNameRx.MatchString(string(env))
}
func GroupName(env string) bool {
	return len(env) < 21 && groupNameRx.MatchString(env)
}
func ApplicationName(name types.AppName) bool {
	return len(name) <= MaxAppNameLen && applicationNameRx.MatchString(string(name))
}

func TeamName(name string) bool {
	// we use the team name in render.go as label and annotations which both have a limit of 63 characters:
	return len(name) < 63 && teamNameRx.MatchString(name)
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

func ReadEnvVarWithDefault(envName string, defaultValue string) string {
	envValue, ok := os.LookupEnv(envName)
	if !ok {
		return defaultValue
	}
	return envValue
}

// ReadEnvVarDurationWithDefault returns the defaultValue if the env var is not set
// ReadEnvVarDurationWithDefault returns an error if it is set but cannot be parsed
func ReadEnvVarDurationWithDefault(envName string, defaultValue time.Duration) (time.Duration, error) {
	envValue, ok := os.LookupEnv(envName)
	if !ok {
		return defaultValue, nil
	}
	duration, err := time.ParseDuration(envValue)
	if err != nil {
		return 0, fmt.Errorf("could not parse environment variable '%s=%s' invalid duration",
			envName, envValue)
	}
	return duration, nil
}

func ReadEnvVarUInt(envName string) (uint, error) {
	envValue, err := ReadEnvVar(envName)
	if err != nil {
		return 0, err
	}
	i, err := strconv.ParseUint(envValue, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("could not convert environment variable '%s=%s' to unsigned int", envName, envValue)
	}
	return uint(i), nil
}

func ReadEnvVarUIntWithDefault(envName string, defaultValue uint) (uint, error) {
	envValue, err := ReadEnvVar(envName)
	if err != nil {
		return defaultValue, nil
	}
	i, err := strconv.ParseUint(envValue, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("could not convert environment variable '%s=%s' to unsigned int", envName, envValue)
	}
	return uint(i), nil
}

func ReadEnvVarInt(envName string) (int, error) {
	envValue, err := ReadEnvVar(envName)
	if err != nil {
		return 0, err
	}
	i, err := strconv.ParseInt(envValue, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("could not convert environment variable '%s=%s' to int", envName, envValue)
	}
	return int(i), nil
}

func ReadEnvVarIntWithDefault(envName string, defaultValue int) (int, error) {
	envValue, err := ReadEnvVar(envName)
	if err != nil {
		return defaultValue, nil
	}
	i, err := strconv.ParseInt(envValue, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("could not convert environment variable '%s=%s' to int", envName, envValue)
	}
	return int(i), nil
}

func ReadEnvVarBool(envName string) (bool, error) {
	envValue, err := ReadEnvVar(envName)
	if err != nil {
		return false, err
	}
	return envValue == "true", nil
}

func ReadEnvVarBoolWithDefault(envName string, defaultVal bool) bool {
	envValue, err := ReadEnvVar(envName)
	if err != nil {
		return defaultVal
	}
	return envValue == "true"
}

func ReadEnvVarAsList(envName string, separator string) ([]string, error) {
	envValue, err := ReadEnvVar(envName)
	if err != nil {
		return nil, err
	}
	if envValue == "" {
		return []string{}, nil
	}

	raws := strings.Split(envValue, ",")
	results := make([]string, 0)
	for _, s := range raws {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" { // skip empty string
			results = append(results, trimmed)
		}
	}
	return results, nil
}
