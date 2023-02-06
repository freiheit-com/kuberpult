
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
