
package valid

import (
	"regexp"
	"strings"
)

var (
	applicationNameRx = regexp.MustCompile(`\A[a-z0-9]+(?:-[a-z0-9]+)*\z`)
	teamNameRx        = regexp.MustCompile(`\A[a-z0-9]+(?:-[a-z0-9]+)*\z`)
	envNameRx         = regexp.MustCompile(`\A[a-z0-9]+(?:-[a-z0-9]+)*\z`)
)

// {application}-{environemnt} should be a valid dns name
func EnvironmentName(env string) bool {
	return len(env) < 21 && envNameRx.MatchString(env)
}
func ApplicationName(name string) bool {
	return len(name) < 40 && applicationNameRx.MatchString(name)
}

func TeamName(name string) bool {
	return len(name) < 21 && teamNameRx.MatchString(name)
}

// Lock names must be valid file names
func LockId(lockId string) bool {
	return len(lockId) < 100 && len(lockId) > 1 && lockId != ".." && lockId != "." && !strings.ContainsAny(lockId, "/")
}
