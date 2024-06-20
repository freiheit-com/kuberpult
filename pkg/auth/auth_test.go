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

Copyright freiheit.com*/

package auth

import (
	"context"
	"testing"
)

func TestToFromContext(t *testing.T) {
	tcs := []struct {
		Name         string
		Author       User
		ExpectedUser User
	}{
		{
			Name: "User is fully specified",
			Author: User{
				Email: "new@test.com",
				Name:  "New",
			},
			ExpectedUser: User{
				Email: "new@test.com",
				Name:  "New",
			},
		},
		{
			Name: "Name is not specified",
			Author: User{
				Email: "new@test.com",
			},
			ExpectedUser: User{
				Email: "new@test.com",
				Name:  "",
			},
		},
		{
			Name: "Email is not specified",
			Author: User{
				Name: "my name",
			},
			ExpectedUser: User{
				Email: "",
				Name:  "my name",
			},
		},
		{
			Name:   "User is not specified",
			Author: User{},
			ExpectedUser: User{
				Email: "",
				Name:  "",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := WriteUserToContext(context.Background(), tc.Author)
			u, err := ReadUserFromContext(ctx)
			if err != nil {
				t.Fatalf("Unexpected error: %#v \n", err)
			}
			if u.Email != tc.ExpectedUser.Email {
				t.Fatalf("Unexpected Email was extracted from context.\nexpected: %#v \nrecieved: %#v \n", tc.ExpectedUser.Email, u.Email)
			}
			if u.Name != tc.ExpectedUser.Name {
				t.Fatalf("Unexpected Name was extracted from context.\nexpected: %#v \nrecieved: %#v \n", tc.ExpectedUser.Name, u.Name)
			}
		})
	}
}
func TestGetUserOrDefault(t *testing.T) {
	tcs := []struct {
		Name         string
		OptionalUser *User
		DefaultUser  User
		ExpectedUser User
	}{
		{
			Name: "User is fully specified",
			OptionalUser: &User{
				Email: "a1",
				Name:  "a2",
			},
			DefaultUser: User{
				Email: "d1",
				Name:  "d2",
			},
			ExpectedUser: User{
				Email: "a1",
				Name:  "a2",
			},
		},
		{
			Name: "User name empty",
			OptionalUser: &User{
				Email: "a1",
				Name:  "",
			},
			DefaultUser: User{
				Email: "d1",
				Name:  "d2",
			},
			ExpectedUser: User{
				Email: "a1",
				Name:  "a1",
			},
		},
		{
			Name:         "User is empty",
			OptionalUser: nil,
			DefaultUser: User{
				Email: "d1",
				Name:  "d2",
			},
			ExpectedUser: User{
				Email: "d1",
				Name:  "d2",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			actualUser := GetUserOrDefault(tc.OptionalUser, tc.DefaultUser)
			if actualUser.Email != tc.ExpectedUser.Email {
				t.Fatalf("Unexpected Email was extracted from context.\nexpected: %#v \nrecieved: %#v \n", tc.ExpectedUser.Email, actualUser.Email)
			}
			if actualUser.Name != tc.ExpectedUser.Name {
				t.Fatalf("Unexpected Email was extracted from context.\nexpected: %#v \nrecieved: %#v \n", tc.ExpectedUser.Name, actualUser.Name)
			}
		})
	}
}
