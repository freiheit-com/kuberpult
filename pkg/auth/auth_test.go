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
	"context"
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
			Name: "Email is not specified",
			Author: &User{
				Name: "my name",
			},
			ExpectedUser: &User{
				Email: "local.user@freiheit.com",
				Name:  "defaultUser",
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
			if u.Email != tc.ExpectedUser.Email {
				t.Fatalf("Unexpected Email was extracted from context.\nexpected: %#v \nrecieved: %#v \n", tc.ExpectedUser.Email, u.Email)
			}
			if u.Name != tc.ExpectedUser.Name {
				t.Fatalf("Unexpected Name was extracted from context.\nexpected: %#v \nrecieved: %#v \n", tc.ExpectedUser.Name, u.Name)
			}
		})
	}

}
