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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
			if diff := cmp.Diff(permission, tc.WantPermission, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("%s: unexpected result diff : %v", tc.Name, diff)
			}
			if tc.WantError != "" {
				if diff := cmp.Diff(tc.WantError, err.Error(), cmpopts.EquateErrors()); diff != "" {
					t.Errorf("Error mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestCheckUserPermissions(t *testing.T) {
	tcs := []struct {
		Name        string
		rbacConfig  RBACConfig
		user        *User
		env         string
		envGroup    string
		application string
		action      string
		WantError   error
	}{
		{
			Name:        "Check user permission works as expected",
			user:        &User{DexAuthContext: &DexAuthContext{Role: "Developer"}},
			env:         "production",
			envGroup:    "production",
			application: "app1",
			action:      PermissionCreateLock,
			rbacConfig:  RBACConfig{DexEnabled: true, Policy: map[string]*Permission{"Developer,CreateLock,production:production,app1,allow": {Role: "Developer"}}},
		},
		{
			Name:        "Application Wildcard works as expected",
			user:        &User{DexAuthContext: &DexAuthContext{Role: "Developer"}},
			env:         "production",
			envGroup:    "production",
			application: "app1",
			action:      PermissionCreateLock,
			rbacConfig:  RBACConfig{DexEnabled: true, Policy: map[string]*Permission{"Developer,CreateLock,production:production,*,allow": {Role: "Developer"}}},
		},
		{
			Name:        "Environment independent works as expected",
			user:        &User{DexAuthContext: &DexAuthContext{Role: "Developer"}},
			env:         "production",
			envGroup:    "production",
			application: "app1",
			action:      PermissionCreateUndeploy,
			rbacConfig:  RBACConfig{DexEnabled: true, Policy: map[string]*Permission{"Developer,CreateUndeploy,production:*,*,allow": {Role: "Developer"}}},
		},
		{
			Name:        "User does not have permission: wrong environment/group",
			user:        &User{DexAuthContext: &DexAuthContext{Role: "Developer"}},
			env:         "production",
			envGroup:    "production",
			application: "app1",
			action:      PermissionCreateLock,
			rbacConfig:  RBACConfig{DexEnabled: true, Policy: map[string]*Permission{"Developer,CreateLock,staging:staging,app1,allow": {Role: "Developer"}}},
			WantError:   status.Errorf(codes.PermissionDenied, fmt.Sprintf("user does not have permissions for: %s", "Developer,CreateLock,production:production,app1,allow")),
		},
		{
			Name:        "User does not have permission: wrong app",
			user:        &User{DexAuthContext: &DexAuthContext{Role: "Developer"}},
			env:         "production",
			envGroup:    "production",
			application: "app2",
			action:      PermissionCreateLock,
			rbacConfig:  RBACConfig{DexEnabled: true, Policy: map[string]*Permission{"Developer,CreateLock,production:production,app1,allow": {Role: "Developer"}}},
			WantError:   status.Errorf(codes.PermissionDenied, fmt.Sprintf("user does not have permissions for: %s", "Developer,CreateLock,production:production,app2,allow")),
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			err := CheckUserPermissions(tc.rbacConfig, tc.user, tc.env, tc.envGroup, tc.application, tc.action)
			if diff := cmp.Diff(tc.WantError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("Error mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
