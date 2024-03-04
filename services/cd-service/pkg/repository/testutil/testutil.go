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

package testutil

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/onokonem/sillyQueueServer/timeuuid"

	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/uuid"
	"google.golang.org/grpc/metadata"
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

func MakeTestContextDexEnabled() context.Context {
	// Default user role.
	return MakeTestContextDexEnabledUser("developer")
}

func MakeTestContextDexEnabledUser(role string) context.Context {
	u := auth.User{
		Email:          "testmail@example.com",
		Name:           "test tester",
		DexAuthContext: &auth.DexAuthContext{Role: role},
	}
	ctx := auth.WriteUserToContext(context.Background(), u)
	ctx = metadata.NewIncomingContext(ctx, metadata.New(map[string]string{
		auth.HeaderUserEmail: auth.Encode64("myemail@example.com"),
		auth.HeaderUserName:  auth.Encode64("my name"),
		auth.HeaderUserRole:  auth.Encode64(role),
	}))
	return ctx
}

func MakeEnvConfigLatest(argoCd *config.EnvironmentConfigArgoCd) config.EnvironmentConfig {
	return MakeEnvConfigLatestWithGroup(argoCd, nil)
}

func MakeEnvConfigLatestWithGroup(argoCd *config.EnvironmentConfigArgoCd, envGroup *string) config.EnvironmentConfig {
	return config.EnvironmentConfig{
		Upstream: &config.EnvironmentConfigUpstream{
			Environment: "",
			Latest:      true,
		},
		ArgoCd:           argoCd,
		EnvironmentGroup: envGroup,
	}
}

func MakeEnvConfigUpstream(upstream string, argoCd *config.EnvironmentConfigArgoCd) config.EnvironmentConfig {
	return config.EnvironmentConfig{
		Upstream: &config.EnvironmentConfigUpstream{
			Latest:      false,
			Environment: upstream,
		},
		ArgoCd:           argoCd,
		EnvironmentGroup: nil,
	}
}

type TestGenerator struct {
	Time time.Time
}

func (t TestGenerator) Generate() string {
	return timeuuid.UUIDFromTime(t.Time).String()
}

type IncrementalUUIDBase struct {
	count uint64
}

func (gen *IncrementalUUIDBase) Generate() string {
	ret := "00000000-0000-0000-0000-" + strings.Repeat("0", (12-len(fmt.Sprint(gen.count)))) + fmt.Sprint(gen.count)
	gen.count++
	return ret
}

type IncrementalUUID struct {
	gen *IncrementalUUIDBase
}

func (gen IncrementalUUID) Generate() string {
	return gen.gen.Generate()
}

func NewIncrementalUUIDGenerator() uuid.GenerateUUIDs {
	fakeGenBase := IncrementalUUIDBase{
		count: 0,
	}
	fakeGen := IncrementalUUID{
		gen: &fakeGenBase,
	}
	return fakeGen
}
