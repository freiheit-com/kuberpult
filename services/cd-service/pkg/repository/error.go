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
