/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
package valid

import (
	"regexp"
	"strings"
)

var (
	applicationNameRx = regexp.MustCompile(`\A[a-z0-9]+(?:-[a-z0-9]+)*\z`)
	envNameRx         = regexp.MustCompile(`\A[a-z0-9]{1,15}\z`)
)

// {application}-{environemnt} should be a valid dns name
func EnvironmentName(env string) bool {
	return envNameRx.MatchString(env)
}
func ApplicationName(name string) bool {
	return len(name) < 40 && applicationNameRx.MatchString(name)
}

// Lock names must be valid file names
func LockId(lockId string) bool {
	return len(lockId) < 100 && len(lockId) > 1 && lockId != ".." && lockId != "." && !strings.ContainsAny(lockId, "/")
}
