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

/*
Package testutilauth provides utilities for anything that depends on pkg/auth.
*/

package testutilauth

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/freiheit-com/kuberpult/pkg/auth"
)

func MakeTestContext() context.Context {
	u := auth.User{
		DexAuthContext: nil,
		Email:          "testmail@example.com",
		Name:           "test tester",
	}
	ctx := auth.WriteUserToContext(context.Background(), u)

	// for some specific calls we need to set the *incoming* grpc context
	// this happens when we call a function like `ProcessBatch` directly in the test
	ctx = metadata.NewIncomingContext(ctx, metadata.New(map[string]string{
		auth.HeaderUserEmail: auth.Encode64("myemail@example.com"),
		auth.HeaderUserName:  auth.Encode64("my name"),
	}))
	return ctx
}

func MakeTestContextDexEnabledUser(role string) context.Context {
	u := auth.User{
		Email:          "testmail@example.com",
		Name:           "test tester",
		DexAuthContext: &auth.DexAuthContext{Role: []string{role}},
	}
	ctx := auth.WriteUserToContext(context.Background(), u)
	ctx = metadata.NewIncomingContext(ctx, metadata.New(map[string]string{
		auth.HeaderUserEmail: auth.Encode64("myemail@example.com"),
		auth.HeaderUserName:  auth.Encode64("my name"),
		auth.HeaderUserRole:  auth.Encode64(role),
	}))
	return ctx
}

func MakeTestContextDexEnabled() context.Context {
	// Default user role.
	return MakeTestContextDexEnabledUser("developer")
}
