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
	"strings"

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
		allowedApps:    []string{"EnvironmentLock", "EnvironmentApplicationLock", "Deploy", "Undeploy", "EnvironmentFromApplication"},
		allowedActions: []string{"Create", "Delete"},
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
