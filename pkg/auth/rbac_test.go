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
			Permission: "p,Developer,Deploy,*,dev:development-d2,allow",
			WantPermission: &Permission{
				Role:        "Developer",
				Application: "Deploy",
				Action:      "*",
				Environment: "dev:development-d2",
			},
		},
		{
			Name:       "Invalid permission Application",
			Permission: "p,Developer,WRONG_APP,*,dev:development-d2,allow",
			WantError:  "invalid application WRONG_APP",
		},
		{
			Name:       "Invalid permission Action",
			Permission: "p,Developer,EnvironmentLock,WRONG_ACTION,dev:development-d2,allow",
			WantError:  "invalid action WRONG_ACTION",
		},
		{
			Name:       "Invalid permission Environment <ENVIRONMENT_GROUP:ENVIRONMENT>",
			Permission: "p,Developer,Deploy,*,dev:-foo,allow",
			WantError:  "invalid environment dev:-foo",
		},
		{
			Name:       "Invalid permission Environment <ENVIRONMENT>",
			Permission: "p,Developer,Deploy,*,-foo,allow",
			WantError:  "invalid environment -foo",
		},
		{
			Name:       "Invalid permission Empty Environment",
			Permission: "p,Developer,Deploy,*,,allow",
			WantError:  "invalid environment ",
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			permission, err := ValidateRbacPermission(tc.Permission)
			if err != nil {
				if diff := cmp.Diff(err.Error(), tc.WantError); diff != "" {
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
