package auth

import (
	"testing"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/google/go-cmp/cmp"
)

func TestValidateRbacPermission(t *testing.T) {
	devEnvironmentGroup := "dev"
	tcs := []struct {
		Name           string
		Permission     string
		configs        map[string]config.EnvironmentConfig
		WantError      string
		WantPermission *permission
	}{
		{
			Name:       "Validating RBAC works as expected",
			Permission: "p,Developer,Deploy,*,dev:development-d2,allow",
			configs: map[string]config.EnvironmentConfig{
				"development-d2": {
					EnvironmentGroup: &devEnvironmentGroup,
				},
			},
			WantPermission: &permission{
				Role:        "Developer",
				Application: "Deploy",
				Action:      "*",
				Environment: "dev:development-d2",
			},
		},
		{
			Name:       "Invalid permission Application",
			Permission: "p,Developer,WRONG_APP,*,dev:development-d2,allow",
			configs:    map[string]config.EnvironmentConfig{},
			WantError:  "invalid application WRONG_APP",
		},
		{
			Name:       "Invalid permission Action",
			Permission: "p,Developer,EnvironmentLock,WRONG_ACTION,dev:development-d2,allow",
			configs:    map[string]config.EnvironmentConfig{},
			WantError:  "invalid action WRONG_ACTION",
		},
		{
			Name:       "Invalid permission Environment",
			Permission: "p,Developer,Deploy,*,dev:development-WRONG,allow",
			configs: map[string]config.EnvironmentConfig{
				"development-d2": {
					EnvironmentGroup: &devEnvironmentGroup,
				},
			},
			WantError: "invalid environment dev:development-WRONG",
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			cfg := initRbacConfig(tc.configs)
			permission, err := validateRbacPermission(tc.Permission, cfg)
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
