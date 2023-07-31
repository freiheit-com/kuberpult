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

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/valid"
)

// All static rbac information that is required to check authentication of a given user.
type RBACConfig struct {
	// Indicates if Dex is enabled.
	DexEnabled bool
	// The RBAC policy. Key is for example "p,Developer,EnvironmentLock,Create,production,allow"
	Policy map[string]*Permission
}

// Inits the RBAC Config struct
func initPolicyConfig() policyConfig {
	return policyConfig{
		allowedApps:    []string{"EnvironmentLock", "EnvironmentApplicationLock", "Deploy", "Undeploy", "EnvironmentFromApplication", "*"},
		allowedActions: []string{"Create", "Delete", "*"},
	}
}

// Stores the RBAC Policy allowed Applications and Actions.
// Only used for policy validation.
type policyConfig struct {
	allowedApps    []string
	allowedActions []string
}

func (c *policyConfig) validateApp(app string) error {
	if app == "" {
		return fmt.Errorf("empty application value")
	}
	for _, a := range c.allowedApps {
		if a == app {
			return nil
		}
	}
	return fmt.Errorf("invalid application %s", app)
}

func (c *policyConfig) validateAction(action string) error {
	if action == "*" {
		return nil
	}
	for _, a := range c.allowedActions {
		if a == action {
			return nil
		}
	}
	return fmt.Errorf("invalid action %s", action)
}

func (c *policyConfig) validateEnvs(envs string) error {
	e := strings.Split(envs, ":")
	// Invalid format
	if len(e) > 2 || envs == "" {
		return fmt.Errorf("invalid environment %s", envs)
	}
	// Validate <ENVIRONMENT_GROUP:ENVIRONMENT>
	if len(e) == 2 {
		if !valid.EnvironmentName(e[0]) {
			return fmt.Errorf("invalid environment group %s", envs)
		}
		if !valid.EnvironmentName(e[1]) {
			return fmt.Errorf("invalid environment %s", envs)
		}
	}
	// Validate <ENVIRONMENT>
	if len(e) == 1 {
		if !valid.EnvironmentName(e[0]) {
			return fmt.Errorf("invalid environment %s", envs)
		}
	}
	return nil
}

// Struct to store an RBAC permission.
type Permission struct {
	Role        string
	Application string
	Environment string
	Action      string
}

func ValidateRbacPermission(line string) (p *Permission, err error) {
	cfg := initPolicyConfig()
	// Verifies if all fields are specified
	c := strings.Split(line, ",")
	if len(c) != 6 {
		return nil, fmt.Errorf("6 fields are expected but only %d were specified", len(c))
	}
	// Validates the permission app
	err = cfg.validateApp(c[2])
	if err != nil {
		return nil, err
	}
	// Validate the permission action
	err = cfg.validateAction(c[3])
	if err != nil {
		return nil, err
	}
	// Validate the environment names
	err = cfg.validateEnvs(c[4])
	if err != nil {
		return nil, err
	}
	return &Permission{
		Role:        c[1],
		Application: c[2],
		Action:      c[3],
		Environment: c[4],
	}, nil
}

func ReadRbacPolicy(dexEnabled bool) (policy map[string]*Permission, err error) {
	if !dexEnabled {
		return nil, nil
	}

	file, err := os.Open("policy.csv")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	policy = map[string]*Permission{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// Trim spaces from policy
		line := strings.ReplaceAll(scanner.Text(), " ", "")
		p, err := ValidateRbacPermission(line)
		if err != nil {
			return nil, err
		}
		policy[line] = p
	}
	if len(policy) == 0 {
		return nil, errors.New("dex.policy.error: dexRbacPolicy is required when \"KUBERPULT_DEX_ENABLED\" is true")
	}
	return policy, nil
}

func recursionPermission(iter int, replace []string) [][]string {
	allOptions := [][]string{}
	recurseReturned := [][]string{}
	if iter < len(replace)-1 {
		recurseReturned = recursionPermission(iter+1, replace)
	} else {
		replaceGeneral := make([]string, len(replace))
		copy(replaceGeneral, replace)
		replaceGeneral[iter] = "*"
		return [][]string{replaceGeneral, replace}
	}
	for i := range recurseReturned {
		allOptions = append(allOptions, recurseReturned[i])
		replaceGeneric := make([]string, len(recurseReturned[i]))
		copy(replaceGeneric, recurseReturned[i])
		replaceGeneric[iter] = "*"
		allOptions = append(allOptions, replaceGeneric)
	}
	return allOptions
}

func CheckUserPermissions(rbacConfig RBACConfig, user *User, env, envGroup, application, action string) error {
	if !rbacConfig.DexEnabled {
		return nil
	}
	permissionsWanted := fmt.Sprintf("p,%s,%s,%s,%s:%s,allow", user.DexAuthContext.Role, application, action, env, envGroup)
	_, permissionsExist := rbacConfig.Policy[permissionsWanted]
	if permissionsExist {
		return nil
	}

	// check generics
	generics := recursionPermission(0, []string{application, action, env, envGroup})
	for _, currentGeneric := range generics {
		permissionsWantedGeneric := fmt.Sprintf("p,%s,%s,%s,%s:%s,allow", user.DexAuthContext.Role, currentGeneric[0], currentGeneric[1], currentGeneric[2], currentGeneric[3])
		_, permissionsExist = rbacConfig.Policy[permissionsWantedGeneric]
		if permissionsExist {
			return nil
		}
	}
	return status.Errorf(codes.PermissionDenied, fmt.Sprintf("user does not have permissions for: %s", permissionsWanted))
}
