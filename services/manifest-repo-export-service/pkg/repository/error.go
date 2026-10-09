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

package repository

import (
	"google.golang.org/protobuf/proto"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
)

type CreateReleaseError struct {
	response api.CreateReleaseResponse
}

func (e *CreateReleaseError) Error() string {
	return e.response.String()
}

func (e *CreateReleaseError) Response() *api.CreateReleaseResponse {
	if e == nil {
		return nil
	}
	return &e.response
}

func (e *CreateReleaseError) Is(target error) bool {
	tgt, ok := target.(*CreateReleaseError)
	if !ok {
		return false
	}
	return proto.Equal(e.Response(), tgt.Response())
}

func GetCreateReleaseGeneralFailure(err error) *CreateReleaseError {
	response := api.CreateReleaseResponseGeneralFailure{
		Message: err.Error(),
	}
	return &CreateReleaseError{
		response: api.CreateReleaseResponse{
			Response: &api.CreateReleaseResponse_GeneralFailure{
				GeneralFailure: &response,
			},
		},
	}
}

type LockedError struct {
	EnvironmentApplicationLocks map[string]Lock
	EnvironmentLocks            map[string]Lock
	TeamLocks                   map[string]Lock
}

func (l *LockedError) String() string {
	return "locked"
}

func (l *LockedError) Error() string {
	return l.String()
}

var _ error = (*LockedError)(nil)
