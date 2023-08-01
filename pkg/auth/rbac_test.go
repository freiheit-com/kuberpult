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

package auth

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestValidateRbacPermission(t *testing.T) {
	tcs := []struct {
		Name           string
		Permission     string
		WantError      string
		WantPermission *Permission
	}{
		{
			Name:       "Validating RBAC works as expected",
			Permission: "Developer,CreateUndeploy,dev:*,*,allow",
			WantPermission: &Permission{
				Role:        "Developer",
				Action:      "CreateUndeploy",
				Application: "*",
				Environment: "dev:*",
			},
		},
		{
			Name:       "Invalid permission Application",
			Permission: "Developer,CreateLock,dev:development-d2,VeryLongAppWithInvalidName,allow",
			WantError:  "invalid application VeryLongAppWithInvalidName",
		},
		{
			Name:       "Invalid permission Action",
			Permission: "Developer,WRONG_ACTION,dev:development-d2,*,allow",
			WantError:  "invalid action WRONG_ACTION",
		},
		{
			Name:       "Invalid permission Environment <ENVIRONMENT_GROUP:ENVIRONMENT>",
			Permission: "Developer,CreateLock,dev:-foo,*,allow",
			WantError:  "invalid environment dev:-foo",
		},
		{
			Name:       "Invalid permission Environment <ENVIRONMENT>",
			Permission: "Developer,CreateLock,-foo,*,allow",
			WantError:  "invalid environment -foo",
		},
		{
			Name:       "Invalid permission Empty Environment",
			Permission: "Developer,CreateLock,,*,allow",
			WantError:  "invalid environment ",
		},
		{
			Name:       "Invalid permission for Environment Independent action <ENVIRONMENT_GROUP:*>",
			Permission: "Developer,DeployUndeploy,dev:development-1,*,allow",
			WantError:  "the action DeployUndeploy requires the environment * and got dev:development-1",
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			permission, err := ValidateRbacPermission(tc.Permission)
			if err != nil {
				if diff := cmp.Diff(tc.WantError, err.Error()); diff != "" {
					t.Errorf("Error mismatch (-want +got):\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(permission, tc.WantPermission); diff != "" {
					t.Errorf("got %v, want %v, diff (-want +got) %s", permission, tc.WantPermission, diff)
				}
			}
		})
	}
}
