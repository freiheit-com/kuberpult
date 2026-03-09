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
Package testutil provides utilities for anything that has only basic dependencies, especially not pkg/auth.
*/

package testutil

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/onokonem/sillyQueueServer/timeuuid"

	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"github.com/freiheit-com/kuberpult/pkg/uuid"
)

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

func MakeEnvConfigUpstream(upstream types.EnvName, argoCd *config.EnvironmentConfigArgoCd) config.EnvironmentConfig {
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
	return MakeArgoCdConfigDestination(concreteEnvName, "destination-name", "server")
}

func MakeArgoCdConfigDestination(concreteEnvName, destinationName, destinationServer string) *config.EnvironmentConfigArgoCd {
	return &config.EnvironmentConfigArgoCd{
		Destination: config.ArgoCdDestination{
			Name:                 destinationName,
			Server:               destinationServer,
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
	id := gen.arr[gen.count]
	gen.count += 1
	return id

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

// WrapTestRoutine is intended for tests, because normal logging via logger.FromContext()
// does not appear in tests by default.
// LogLevel should be either WARN, INFO, or ERROR
// NOTE: You need to call this for each spawned go-routine (if any).
func WrapTestRoutine(t *testing.T, ctx context.Context, logLevel string, inner func(ctx context.Context)) {
	err := os.Setenv("LOG_LEVEL", logLevel)
	if err != nil {
		t.Fatalf("failed to set LOG_LEVEL: %v", err)
	}
	err = logger.Wrap(ctx, func(ctx context.Context) error {
		inner(ctx)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to wrap logger: %v", err)
	}
}

// CmpDiff is exactly like cmp.Diff but with type safety
func CmpDiff[T any](expected, got T, opts ...cmp.Option) string {
	return cmp.Diff(expected, got, opts...)
}

func DiffOrFail[T any](t *testing.T, message string, expected, got T, opts ...cmp.Option) {
	t.Helper()
	if diff := CmpDiff(expected, got, opts...); diff != "" {
		t.Logf("%s: want:\n%v\n", message, expected)
		t.Logf("%s: got:\n%v\n", message, got)
		t.Errorf("%s: (-want +got):\n%s", message, diff)
	}
}
