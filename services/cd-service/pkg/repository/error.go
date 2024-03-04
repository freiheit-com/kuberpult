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

import (
	"fmt"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
)

type CreateReleaseError struct {
	response api.CreateReleaseResponse
}

func (e *CreateReleaseError) Error() string {
	return e.response.String()
}

func (e *CreateReleaseError) Response() *api.CreateReleaseResponse {
	return &e.response
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

func GetCreateReleaseAlreadyExistsSame() *CreateReleaseError {
	response := api.CreateReleaseResponseAlreadyExistsSame{}
	return &CreateReleaseError{
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
		response: api.CreateReleaseResponse{
			Response: &api.CreateReleaseResponse_TooLong{
				TooLong: &response,
			},
		},
	}
}

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
