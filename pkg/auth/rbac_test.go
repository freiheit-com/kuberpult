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
		WantPermission *permission
	}{
		{
			Name:       "Validating RBAC works as expected",
			Permission: "p,Developer,Deploy,*,dev:development-d2,allow",
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
			WantError:  "invalid application WRONG_APP",
		},
		{
			Name:       "Invalid permission Action",
			Permission: "p,Developer,EnvironmentLock,WRONG_ACTION,dev:development-d2,allow",
			WantError:  "invalid action WRONG_ACTION",
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			cfg := initRbacConfig()
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
