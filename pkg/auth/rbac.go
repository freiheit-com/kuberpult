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
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/freiheit-com/kuberpult/pkg/valid"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	PermissionCreateLock                   = "CreateLock"
	PermissionDeleteLock                   = "DeleteLock"
	PermissionCreateRelease                = "CreateRelease"
	PermissionDeployRelease                = "DeployRelease"
	PermissionCreateUndeploy               = "CreateUndeploy"
	PermissionDeployUndeploy               = "DeployUndeploy"
	PermissionCreateEnvironment            = "CreateEnvironment"
	PermissionDeleteEnvironmentApplication = "DeleteEnvironmentApplication"
	PermissionDeployReleaseTrain           = "DeployReleaseTrain"
	// The default permission template.
	PermissionTemplate = "p,role:%s,%s,%s:%s,%s,allow"
)

// All static rbac information that is required to check authentication of a given user.
type RBACConfig struct {
	// Indicates if Dex is enabled.
	DexEnabled bool
	// The RBAC policies. The key is a permission or group, for example: "Developer, CreateLock, development:development, *, allow"
	Policy *RBACPolicies
}

// Inits the RBAC Config struct
func initPolicyConfig() policyConfig {
	return policyConfig{
		// List of allowed actions on the RBAC policy.
		allowedActions: []string{
			PermissionCreateLock,
			PermissionDeleteLock,
			PermissionCreateRelease,
			PermissionDeployRelease,
			PermissionCreateUndeploy,
			PermissionDeployUndeploy,
			PermissionCreateEnvironment,
			PermissionDeleteEnvironmentApplication,
			PermissionDeployReleaseTrain},
	}
}

// Stores the RBAC Policy allowed Applications and Actions.
// Only used for policy validation.
type policyConfig struct {
	allowedActions []string
}

func (c *policyConfig) validateAction(action string) error {
	for _, a := range c.allowedActions {
		if a == action {
			return nil
		}
	}
	return fmt.Errorf("invalid action %s", action)
}

func (c *policyConfig) validateEnvs(envs, action string) error {
	e := strings.Split(envs, ":")
	if len(e) != 2 || envs == "" {
		return fmt.Errorf("invalid environment %s", envs)
	}
	// The environment follows the format <ENVIRONMENT_GROUP:ENVIRONMENT>
	groupName := e[0]
	envName := e[1]
	// Validate environment group
	if !valid.EnvironmentName(groupName) && groupName != "*" {
		return fmt.Errorf("invalid environment group %s", envs)
	}
	// Actions that are environment independent need to follow the format <ENVIRONMENT_GROUP:*>.
	if isEnvironmentIndependent(action) {
		if envName == "*" {
			return nil
		}
		return fmt.Errorf("the action %s requires the environment * and got %s", action, envs)
	}
	// Validate environment
	if !valid.EnvironmentName(envName) && envName != "*" {
		return fmt.Errorf("invalid environment %s", envs)
	}
	return nil
}

func (c *policyConfig) validateApplication(app string) error {
	if app == "*" {
		return nil
	}
	if !valid.ApplicationName(app) {
		return fmt.Errorf("invalid application %s", app)
	}
	return nil
}

// Helper function to indicate is the format if the specified action
// is independent from the environment. If so, the following format needs to be
// followed <ENVIRONMENT_GROUP:*>.
func isEnvironmentIndependent(action string) bool {
	switch action {
	case PermissionCreateUndeploy, PermissionDeployUndeploy, PermissionCreateRelease:
		return true
	}
	return false
}

// Struct to store an RBAC permission.
type Permission struct {
	Role        string
	Application string
	Environment string
	Action      string
}
type RBACGroup struct {
	Group string
	Role  string
}

type RBACPolicies struct {
	Groups      map[string]RBACGroup
	Permissions map[string]Permission
}

func ValidateRbacPermission(line string) (p Permission, err error) {
	cfg := initPolicyConfig()
	// Verifies if all fields are specified
	c := strings.Split(line, ",")
	if len(c) != 6 {
		return p, fmt.Errorf("6 fields are expected but only %d were specified", len(c))
	}
	// Permission role
	if !strings.Contains(c[1], "role:") {
		return p, fmt.Errorf("the format for permissions expects the prefix `role:` for permissions")
	}
	role := c[1][5:]
	// Validates the permission action
	action := c[2]
	err = cfg.validateAction(action)
	if err != nil {
		return p, err
	}
	// Validate the permission environment
	environment := c[3]
	err = cfg.validateEnvs(environment, action)
	if err != nil {
		return p, err
	}
	// Validate the application names
	application := c[4]
	err = cfg.validateApplication(application)
	if err != nil {
		return p, err
	}
	return Permission{
		Role:        role,
		Action:      action,
		Environment: environment,
		Application: application,
	}, nil
}

func ReadRbacPolicy(dexEnabled bool, DexRbacPolicyPath string) (policy *RBACPolicies, err error) {
	if !dexEnabled {
		return nil, nil
	}

	file, err := os.Open(DexRbacPolicyPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	policy = &RBACPolicies{Permissions: map[string]Permission{}, Groups: map[string]RBACGroup{}}
	for scanner.Scan() {
		// Trim spaces from policy
		line := strings.ReplaceAll(scanner.Text(), " ", "")
		if line[0] == 'p' {
			p, err := ValidateRbacPermission(line)
			if err != nil {
				return nil, err
			}
			policy.Permissions[line] = p
		}
	}
	if len(policy.Permissions) == 0 {
		return nil, errors.New("dex.policy.error: dexRbacPolicy is required when \"KUBERPULT_DEX_ENABLED\" is true")
	}
	return policy, nil
}

type PermissionError struct {
	User        string
	Role        string
	Action      string
	Environment string
	Team        string
}

func (e PermissionError) Error() string {
	var msg = fmt.Sprintf(
		"%s The user '%s' with role '%s' is not allowed to perform the action '%s' on environment '%s'",
		codes.PermissionDenied.String(),
		e.User,
		e.Role,
		e.Action,
		e.Environment,
	)
	if e.Team != "" {
		msg += fmt.Sprintf(" for team '%s'", e.Team)
	}
	return msg
}

func (e PermissionError) GRPCStatus() *status.Status {
	return status.New(codes.PermissionDenied, e.Error())
}

// Checks user permissions on the RBAC policy.
func CheckUserPermissions(rbacConfig RBACConfig, user *User, env, team, envGroup, application, action string) error {
	// If the action is environment independent, the env format is <ENVIRONMENT_GROUP>:*
	if isEnvironmentIndependent(action) {
		env = "*"
	}
	// Check for all possible Wildcard combinations. Maximum of 8 combinations (2^3).
	for _, pEnvGroup := range []string{envGroup, "*"} {
		for _, pEnv := range []string{env, "*"} {
			for _, pApplication := range []string{application, "*"} {
				// Check if the permission exists on the policy.
				if rbacConfig.Policy == nil {
					return errors.New("the desired action can not be performed because Dex is enabled without any RBAC policies")
				}
				permissionsWanted := fmt.Sprintf(PermissionTemplate, user.DexAuthContext.Role, action, pEnvGroup, pEnv, pApplication)
				_, permissionsExist := rbacConfig.Policy.Permissions[permissionsWanted]
				if permissionsExist {
					return nil
				}
			}
		}
	}
	// The permission is not found. Return an error.
	return PermissionError{
		User:        user.Name,
		Role:        user.DexAuthContext.Role,
		Action:      action,
		Environment: env,
		Team:        team,
	}
}

// Helper function to parse the scopes
func ReadScopes(s string) (scopes []string) {
	replacer := strings.NewReplacer(" ", "")
	scopesTrim := replacer.Replace(s)
	return strings.Split(scopesTrim, ",")
}
