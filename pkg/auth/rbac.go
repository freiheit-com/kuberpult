package auth

import (
	"fmt"
	"strings"
)

// Inits the RBAC Config struct
func initRbacConfig() RBACconfig {
	return RBACconfig{
		allowedApp:     []string{"EnvironmentLock", "EnvironmentApplicationLock", "Deploy", "Undeploy", "EnvironmentFromApp"},
		allowedActions: []string{"Create", "Delete"},
	}
}

// Stores the RBAC allowed Applications and Actions
type RBACconfig struct {
	allowedApp     []string
	allowedActions []string
}

func (c *RBACconfig) validateApp(app string) error {
	if app == "" {
		return fmt.Errorf("empty application value")
	}
	for _, a := range c.allowedApp {
		if a == app {
			return nil
		}
	}
	return fmt.Errorf("invalid application %s", app)
}

func (c *RBACconfig) validateAction(action string) error {
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

// Struct to store an RBAC permission.
type permission struct {
	Role        string
	Application string
	Environment string
	Action      string
}

func validateRbacPermission(line string, cfg RBACconfig) (p *permission, err error) {
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
	return &permission{
		Role:        c[1],
		Application: c[2],
		Action:      c[3],
		Environment: c[4],
	}, nil
}
