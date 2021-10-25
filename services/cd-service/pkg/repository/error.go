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
package repository

import "fmt"

type InternalError struct {
	inner error
}

func (i *InternalError) String() string {
	return fmt.Sprintf("repository internal: %s", i.inner)
}

func (i *InternalError) Unwrap() error {
	return i.inner
}

func (i *InternalError) Error() string {
	return i.String()
}

var _ error = (*InternalError)(nil)

type LockedError struct {
	EnvironmentApplicationLocks map[string]Lock
	EnvironmentLocks            map[string]Lock
}

func (l *LockedError) String() string {
	return fmt.Sprintf("locked")
}

func (l *LockedError) Error() string {
	return l.String()
}

var _ error = (*LockedError)(nil)
