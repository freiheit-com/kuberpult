
package auth

import (
	"context"
	"reflect"
	"testing"
)

func TestAuthContextFunctions(t *testing.T) {
	tcs := []struct {
		Name         string
		Author       *User
		ExpectedUser *User
	}{
		{
			Name: "User is fully specified",
			Author: &User{
				Email: "new@test.com",
				Name:  "New",
			},
			ExpectedUser: &User{
				Email: "new@test.com",
				Name:  "New",
			},
		},
		{
			Name: "Name is not specified",
			Author: &User{
				Email: "new@test.com",
			},
			ExpectedUser: &User{
				Email: "new@test.com",
				Name:  "new@test.com",
			},
		},
		{
			Name:   "User is not specified",
			Author: nil,
			ExpectedUser: &User{
				Email: "local.user@freiheit.com",
				Name:  "defaultUser",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := ToContext(context.Background(), tc.Author)
			u := Extract(ctx)
			if !reflect.DeepEqual(u, tc.ExpectedUser) {
				t.Fatalf("Unexpected User was extracted from context.\nexpected: %#v \nrecieved: %#v \n", tc.ExpectedUser, u)
			}
		})
	}

}
