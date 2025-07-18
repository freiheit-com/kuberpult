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
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"google.golang.org/protobuf/proto"
)

type CreateReleaseError struct {
	response   api.CreateReleaseResponse
	innerError error
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

func (e *CreateReleaseError) Unwrap() error {
	// Return the inner error.
	return e.innerError
}

func GetCreateReleaseGeneralFailure(err error) *CreateReleaseError {
	response := api.CreateReleaseResponseGeneralFailure{
		Message: err.Error(),
	}
	return &CreateReleaseError{
		innerError: err,
		response: api.CreateReleaseResponse{
			Response: &api.CreateReleaseResponse_GeneralFailure{
				GeneralFailure: &response,
			},
		},
	}
}

func GetCreateReleaseAlreadyExistsSame() *CreateReleaseError {
	response := api.CreateReleaseResponseAlreadyExistsSame{}
	return &CreateReleaseError{
		innerError: nil,
		response: api.CreateReleaseResponse{
			Response: &api.CreateReleaseResponse_AlreadyExistsSame{
				AlreadyExistsSame: &response,
			},
		},
	}
}

func GetCreateReleaseAlreadyExistsDifferent(firstDifferingField api.DifferingField, diff string) *CreateReleaseError {
	response := api.CreateReleaseResponseAlreadyExistsDifferent{
		FirstDifferingField: firstDifferingField,
		Diff:                diff,
	}
	return &CreateReleaseError{
		innerError: nil,
		response: api.CreateReleaseResponse{
			Response: &api.CreateReleaseResponse_AlreadyExistsDifferent{
				AlreadyExistsDifferent: &response,
			},
		},
	}
}

func GetCreateReleaseTooOld() *CreateReleaseError {
	response := api.CreateReleaseResponseTooOld{}
	return &CreateReleaseError{
		innerError: nil,
		response: api.CreateReleaseResponse{
			Response: &api.CreateReleaseResponse_TooOld{
				TooOld: &response,
			},
		},
	}
}

func GetCreateReleaseAppNameTooLong(appName string, regExp string, maxLen uint32) *CreateReleaseError {
	response := api.CreateReleaseResponseAppNameTooLong{
		AppName: appName,
		RegExp:  regExp,
		MaxLen:  maxLen,
	}
	return &CreateReleaseError{
		innerError: nil,
		response: api.CreateReleaseResponse{
			Response: &api.CreateReleaseResponse_TooLong{
				TooLong: &response,
			},
		},
	}
}
func GetCreateReleaseMissingManifest(missingManifests []types.EnvName) *CreateReleaseError {
	missingManifestStr := types.EnvNamesToStrings(missingManifests)
	response := api.CreateReleaseResponseMissingManifest{
		MissingManifest: missingManifestStr,
	}
	return &CreateReleaseError{
		innerError: nil,
		response: api.CreateReleaseResponse{
			Response: &api.CreateReleaseResponse_MissingManifest{
				MissingManifest: &response,
			},
		},
	}
}

func GetCreateReleaseIsNoDownstream(noDownstream []types.EnvName) *CreateReleaseError {
	noDownstreamStr := types.EnvNamesToStrings(noDownstream)
	response := api.CreateReleaseResponseIsNoDownstream{
		NoDownstream: noDownstreamStr,
	}
	return &CreateReleaseError{
		innerError: nil,
		response: api.CreateReleaseResponse{
			Response: &api.CreateReleaseResponse_IsNoDownstream{
				IsNoDownstream: &response,
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

type TeamNotFoundErr struct {
	err error
}

func (e *TeamNotFoundErr) Error() string {
	return e.err.Error()
}

func (e *TeamNotFoundErr) Is(target error) bool {
	_, ok := target.(*TeamNotFoundErr)
	return ok
}
