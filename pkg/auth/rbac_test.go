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
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestValidateRbacPermission(t *testing.T) {
	tcs := []struct {
		Name           string
		Permission     string
		WantError      error
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
			WantError:  errMatcher{"invalid application VeryLongAppWithInvalidName"},
		},
		{
			Name:       "Invalid permission Action",
			Permission: "Developer,WRONG_ACTION,dev:development-d2,*,allow",
			WantError:  errMatcher{"invalid action WRONG_ACTION"},
		},
		{
			Name:       "Invalid permission Environment <ENVIRONMENT_GROUP:ENVIRONMENT>",
			Permission: "Developer,CreateLock,dev:-foo,*,allow",
			WantError:  errMatcher{"invalid environment dev:-foo"},
		},
		{
			Name:       "Invalid permission Environment <ENVIRONMENT>",
			Permission: "Developer,CreateLock,-foo,*,allow",
			WantError:  errMatcher{"invalid environment -foo"},
		},
		{
			Name:       "Invalid permission Empty Environment",
			Permission: "Developer,CreateLock,,*,allow",
			WantError:  errMatcher{"invalid environment "},
		},
		{
			Name:       "Invalid permission for Environment Independent action <ENVIRONMENT_GROUP:*>",
			Permission: "Developer,DeployUndeploy,dev:development-1,*,allow",
			WantError:  errMatcher{"the action DeployUndeploy requires the environment * and got dev:development-1"},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			permission, err := ValidateRbacPermission(tc.Permission)
			if diff := cmp.Diff(tc.WantError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			if diff := cmp.Diff(permission, tc.WantPermission, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("%s: unexpected result diff : %v", tc.Name, diff)
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
		team        string
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
			team:        "",
		},
		{
			Name:        "Environment independent works as expected",
			user:        &User{Name: "user", DexAuthContext: &DexAuthContext{Role: "Developer"}},
			env:         "production",
			envGroup:    "production",
			application: "app1",
			action:      PermissionCreateUndeploy,
			rbacConfig:  RBACConfig{DexEnabled: true, Policy: map[string]*Permission{"Developer,CreateUndeploy,production:*,*,allow": {Role: "Developer"}}},
			team:        "team",
		},
		{
			Name:        "User does not have permission: wrong environment/group",
			user:        &User{DexAuthContext: &DexAuthContext{Role: "Developer"}},
			env:         "production",
			envGroup:    "staging",
			application: "app1",
			action:      PermissionCreateLock,
			rbacConfig:  RBACConfig{DexEnabled: true, Policy: map[string]*Permission{"Developer,CreateLock,production:production,app1,allow": {Role: "Developer"}}},
			team:        "random-team",
			WantError: PermissionError{
				Role:        "Developer",
				Action:      "CreateLock",
				Environment: "production",
				Team:        "random-team",
			},
		},
		{
			Name:        "User does not have permission: wrong app",
			user:        &User{DexAuthContext: &DexAuthContext{Role: "Developer"}},
			env:         "production",
			envGroup:    "production",
			application: "app2",
			action:      PermissionCreateLock,
			team:        "other-team",
			rbacConfig:  RBACConfig{DexEnabled: true, Policy: map[string]*Permission{"Developer,CreateLock,production:production,app1,allow": {Role: "Developer"}}},
			WantError: PermissionError{
				Role:        "Developer",
				Action:      "CreateLock",
				Environment: "production",
				Team:        "other-team",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			err := CheckUserPermissions(tc.rbacConfig, tc.user, tc.env, tc.team, tc.envGroup, tc.application, tc.action)
			if diff := cmp.Diff(tc.WantError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("Error mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateRbacPermissionWildcards(t *testing.T) {
	tcs := []struct {
		Name        string
		permissions []string
		WantError   error
	}{
		{
			Name: "Check permission validation works for all wildcard combinations",
			permissions: []string{
				"Developer,CreateLock,production:production,app1,allow",
				"Developer,CreateLock,production:production,*,allow",
				"Developer,CreateLock,production:*,app1,allow",
				"Developer,CreateLock,production:*,*,allow",
				"Developer,CreateLock,*:production,app1,allow",
				"Developer,CreateLock,*:production,*,allow",
				"Developer,CreateLock,*:*,app1,allow",
				"Developer,CreateLock,*:*,*,allow",
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			// Test all wildcard possible combinations (2^8).
			for _, permission := range tc.permissions {
				_, err := ValidateRbacPermission(permission)
				if diff := cmp.Diff(tc.WantError, err, cmpopts.EquateErrors()); diff != "" {
					t.Errorf("Error mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestCheckUserPermissionsWildcards(t *testing.T) {
	tcs := []struct {
		Name        string
		user        *User
		env         string
		envGroup    string
		application string
		action      string
		policies    []map[string]*Permission
		WantError   error
	}{
		{
			Name:        "Check user permission works for all wildcard combinations",
			user:        &User{DexAuthContext: &DexAuthContext{Role: "Developer"}},
			env:         "production",
			envGroup:    "production",
			application: "app1",
			action:      PermissionCreateLock,
			policies: []map[string]*Permission{
				{"Developer,CreateLock,production:production,app1,allow": {Role: "Developer"}},
				{"Developer,CreateLock,production:production,*,allow": {Role: "Developer"}},
				{"Developer,CreateLock,production:*,app1,allow": {Role: "Developer"}},
				{"Developer,CreateLock,production:*,*,allow": {Role: "Developer"}},
				{"Developer,CreateLock,*:production,app1,allow": {Role: "Developer"}},
				{"Developer,CreateLock,*:production,*,allow": {Role: "Developer"}},
				{"Developer,CreateLock,*:*,app1,allow": {Role: "Developer"}},
				{"Developer,CreateLock,*:*,*,allow": {Role: "Developer"}},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			// Test all wildcard possible combinations (2^8).
			for _, policy := range tc.policies {
				rbacConfig := RBACConfig{DexEnabled: true, Policy: policy}
				err := CheckUserPermissions(rbacConfig, tc.user, tc.env, "", tc.envGroup, tc.application, tc.action)
				if diff := cmp.Diff(tc.WantError, err, cmpopts.EquateErrors()); diff != "" {
					t.Errorf("Error mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestReadScopes(t *testing.T) {
	tcs := []struct {
		Name         string
		ScopesString string
		WantScopes   []string
	}{
		{
			Name:         "Correctly parses the scopes string",
			ScopesString: "groups, emails, profile, openID",
			WantScopes:   []string{"groups", "emails", "profile", "openID"},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			scopes := ReadScopes(tc.ScopesString)
			if diff := cmp.Diff(tc.WantScopes, scopes); diff != "" {
				t.Errorf("Error mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
