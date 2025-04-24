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

package testutil

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/config"
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
		ArgoCdConfigs:    nil,
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
		ArgoCdConfigs:    nil,
	}
}

func MakeDummyArgoCdConfig(concreteEnvName string) *config.EnvironmentConfigArgoCd {
	return &config.EnvironmentConfigArgoCd{
		Destination: config.ArgoCdDestination{
			Name:                 "destination-name",
			Server:               "server",
			Namespace:            nil,
			AppProjectNamespace:  nil,
			ApplicationNamespace: nil,
		},
		SyncWindows:              nil,
		ClusterResourceWhitelist: nil,
		ApplicationAnnotations:   nil,
		IgnoreDifferences:        nil,
		SyncOptions:              nil,
		ConcreteEnvName:          concreteEnvName,
	}
}

func MakeArgoCDConfigs(commonName, concreteName string, envNumber int) *config.ArgoCDConfigs {
	toReturn := config.ArgoCDConfigs{
		CommonEnvPrefix:      &commonName,
		ArgoCdConfigurations: make([]*config.EnvironmentConfigArgoCd, 0),
	}

	for i := 0; i < envNumber; i++ {
		toReturn.ArgoCdConfigurations = append(toReturn.ArgoCdConfigurations, MakeDummyArgoCdConfig(concreteName+"-"+strconv.Itoa(i)))
	}
	return &toReturn
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

func NewIncrementalUUIDGenerator() uuid.GenerateUUIDs {
	fakeGenBase := IncrementalUUIDBase{
		count: 0,
	}
	fakeGen := IncrementalUUID{
		gen: &fakeGenBase,
	}
	return fakeGen
}

type IncrementalUUID struct {
	gen *IncrementalUUIDBase
}

func (gen IncrementalUUID) Generate() string {
	return gen.gen.Generate()
}

// NOTE: FOR TESTING PURPOSES ONLY
/* We need this new generator because we need to perserve order
   and with the normal generator all of the uuids point to the
   same timestamp. Hence the new generator with 6 uuids that
   point to different timestamps 3 seconds appart
*/

type IncrementalUUIDBaseForPageSizeTest struct {
	count uint64
	arr   []string
}

func (gen *IncrementalUUIDBaseForPageSizeTest) Generate() string {
	uuid := gen.arr[gen.count]
	gen.count += 1
	return uuid

}

type IncrementalUUIDForPageSizeTest struct {
	gen *IncrementalUUIDBaseForPageSizeTest
}

func (gen IncrementalUUIDForPageSizeTest) Generate() string {
	return gen.gen.Generate()
}

func NewIncrementalUUIDGeneratorForPageSizeTest() uuid.GenerateUUIDs {
	fakeGenBase := IncrementalUUIDBaseForPageSizeTest{
		count: 0,
		arr: []string{
			"dbfee8cd-4f41-11ef-b76a-00e04c684024",
			"ddc9f32b-4f41-11ef-bb1b-00e04c684024",
			"df93c826-4f41-11ef-b685-00e04c684024",
			"e15d9a99-4f41-11ef-9ae5-00e04c684024",
			"e3276e62-4f41-11ef-8788-00e04c684024",
			"e4f13c8b-4f41-11ef-9735-00e04c684024",
			"e4f13c8b-4f41-11ef-9735-00e04c684025",
			"e4f13c8b-4f41-11ef-9735-00e04c684026",
		},
	}
	fakeGen := IncrementalUUIDForPageSizeTest{
		gen: &fakeGenBase,
	}
	return fakeGen
}
