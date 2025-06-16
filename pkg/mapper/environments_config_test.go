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

package mapper

import (
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"github.com/google/go-cmp/cmp/cmpopts"

	"testing"

	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/google/go-cmp/cmp"
)

func makeUpstreamLatest() *api.EnvironmentConfig_Upstream {
	f := true
	return &api.EnvironmentConfig_Upstream{
		Latest: &f,
	}
}

func makeUpstreamEnvironment(env types.EnvName) *api.EnvironmentConfig_Upstream {
	return &api.EnvironmentConfig_Upstream{
		Environment: types.StringPtr(env),
	}
}

var nameStagingDe types.EnvName = "staging-de"
var nameOtherDe types.EnvName = "other-de"
var nameDevDe types.EnvName = "dev-de"
var nameCanaryDe types.EnvName = "canary-de"
var nameProdDe types.EnvName = "prod-de"
var nameWhoKnowsDe types.EnvName = "whoknows-de"

var nameStagingFr types.EnvName = "staging-fr"
var nameDevFr types.EnvName = "dev-fr"
var nameProdFr types.EnvName = "prod-fr"

var nameDevGlobal types.EnvName = "dev-global"
var nameTestGlobal types.EnvName = "test-global"

var nameStaging types.EnvName = "staging"
var nameDev types.EnvName = "dev"
var nameProd types.EnvName = "prod"
var nameWhoKnows types.EnvName = "whoknows"
var nameTest types.EnvName = "test"
var nameCanary types.EnvName = "canary"

func makeEnv(envName types.EnvName, groupName types.EnvName, upstream *api.EnvironmentConfig_Upstream, distanceToUpstream uint32, priority api.Priority) *api.Environment {
	return &api.Environment{
		Name: string(envName),
		Config: &api.EnvironmentConfig{
			Upstream:         upstream,
			EnvironmentGroup: types.StringPtr(groupName),
		},
		DistanceToUpstream: distanceToUpstream,
		Priority:           priority, // we are 1 away from prod, hence pre-prod
	}
}

func TestMapEnvironmentsToGroup(t *testing.T) {
	tcs := []struct {
		Name           string
		InputEnvs      map[types.EnvName]config.EnvironmentConfig
		ExpectedResult []*api.EnvironmentGroup
	}{
		{
			Name: "One Environment is one Group",
			InputEnvs: map[types.EnvName]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: "",
						Latest:      true,
					},
					ArgoCd:           nil,
					EnvironmentGroup: types.StringPtr(nameDevDe),
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: string(nameDevDe),
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamLatest(), 0, api.Priority_YOLO),
					},
					DistanceToUpstream: 0,
					Priority:           api.Priority_YOLO,
				},
			},
		},
		{
			Name: "Two Environments are two Groups",
			InputEnvs: map[types.EnvName]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
					ArgoCd: nil,
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameDevDe),
					},
					ArgoCd: nil,
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: string(nameDevDe),
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
					},
					DistanceToUpstream: 0,
					Priority:           api.Priority_UPSTREAM,
				},
				{
					EnvironmentGroupName: string(nameStagingDe),
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStagingDe, makeUpstreamEnvironment(nameDevDe), 1, api.Priority_PROD),
					},
					DistanceToUpstream: 1,
					Priority:           api.Priority_PROD,
				},
			},
		},
		{
			// note that this is not a realistic example, we just want to make sure it does not crash!
			// some outputs may be nonsensical (like distanceToUpstream), but that's fine as long as it's stable!
			Name: "Two Environments with a loop",
			InputEnvs: map[types.EnvName]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameStagingDe),
					},
					ArgoCd: nil,
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameDevDe),
					},
					ArgoCd: nil,
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: string(nameDevDe),
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamEnvironment(nameStagingDe), 667, api.Priority_OTHER),
					},
					DistanceToUpstream: 667,
					Priority:           api.Priority_CANARY, // set according to observed output, again, we just want to make sure it doesn't crash
				},
				{
					EnvironmentGroupName: string(nameStagingDe),
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStagingDe, makeUpstreamEnvironment(nameDevDe), 668, api.Priority_OTHER),
					},
					Priority:           api.Priority_PROD, // set according to observed output, again, we just want to make sure it doesn't crash
					DistanceToUpstream: 668,
				},
			},
		},
		{
			// note that this is not a realistic example, we just want to make sure it does not crash!
			// some outputs may be nonsensical (like distanceToUpstream), but that's fine as long as it's stable!
			Name: "Two Environments with non exists upstream",
			InputEnvs: map[types.EnvName]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
					ArgoCd: nil,
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameWhoKnows),
					},
					ArgoCd: nil,
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: string(nameDevDe),
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamLatest(), 0, api.Priority_YOLO),
					},
					DistanceToUpstream: 0,
					Priority:           api.Priority_UPSTREAM, // set according to observed output, again, we just want to make sure it doesn't crash
				},
				{
					EnvironmentGroupName: string(nameStagingDe),
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStagingDe, makeUpstreamEnvironment(nameWhoKnows), 667, api.Priority_PROD),
					},
					DistanceToUpstream: 667,
					Priority:           api.Priority_PROD,
				},
			},
		},
		{
			Name: "Three Environments are three Groups",
			InputEnvs: map[types.EnvName]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
					ArgoCd: nil,
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameDevDe),
					},
					ArgoCd: nil,
				},
				nameProdDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameStagingDe),
					},
					ArgoCd: nil,
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: string(nameDevDe),
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
					},
					DistanceToUpstream: 0,
					Priority:           api.Priority_UPSTREAM,
				},
				{
					EnvironmentGroupName: string(nameStagingDe),
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStagingDe, makeUpstreamEnvironment(nameDevDe), 1, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 1,
					Priority:           api.Priority_PRE_PROD,
				},
				{
					EnvironmentGroupName: string(nameProdDe),
					Environments: []*api.Environment{
						makeEnv(nameProdDe, nameProdDe, makeUpstreamEnvironment(nameStagingDe), 2, api.Priority_PROD),
					},
					DistanceToUpstream: 2,
					Priority:           api.Priority_PROD,
				},
			},
		},
		{
			Name: "Four Environments in a row to ensure that Priority_UPSTREAM works",
			InputEnvs: map[types.EnvName]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameDevDe),
					},
				},
				nameProdDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameStagingDe),
					},
				},
				nameWhoKnowsDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameProdDe),
					},
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: string(nameDevDe),
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
					},
					DistanceToUpstream: 0,
					Priority:           api.Priority_UPSTREAM,
				},
				{
					EnvironmentGroupName: string(nameStagingDe),
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStagingDe, makeUpstreamEnvironment(nameDevDe), 1, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 1,
					Priority:           api.Priority_PRE_PROD,
				},
				{
					EnvironmentGroupName: string(nameProdDe),
					Environments: []*api.Environment{
						makeEnv(nameProdDe, nameProdDe, makeUpstreamEnvironment(nameStagingDe), 2, api.Priority_CANARY),
					},
					DistanceToUpstream: 2,
					Priority:           api.Priority_CANARY,
				},
				{
					EnvironmentGroupName: string(nameWhoKnowsDe),
					Environments: []*api.Environment{
						makeEnv(nameWhoKnowsDe, nameWhoKnowsDe, makeUpstreamEnvironment(nameProdDe), 3, api.Priority_PROD),
					},
					DistanceToUpstream: 3,
					Priority:           api.Priority_PROD,
				},
			},
		},
		{
			Name: "five in a chain should be u->o->pp->c->p",
			InputEnvs: map[types.EnvName]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
				},
				nameOtherDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameDevDe),
					},
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameOtherDe),
					},
				},
				nameCanaryDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameStagingDe),
					},
				},
				nameProdDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameCanaryDe),
					},
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: string(nameDevDe),
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
					},
					DistanceToUpstream: 0,
					Priority:           api.Priority_UPSTREAM,
				},
				{
					EnvironmentGroupName: string(nameOtherDe),
					Environments: []*api.Environment{
						makeEnv(nameOtherDe, nameOtherDe, makeUpstreamEnvironment(nameDevDe), 1, api.Priority_OTHER),
					},
					DistanceToUpstream: 1,
					Priority:           api.Priority_OTHER,
				},
				{
					EnvironmentGroupName: string(nameStagingDe),
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStagingDe, makeUpstreamEnvironment(nameOtherDe), 2, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 2,
					Priority:           api.Priority_PRE_PROD,
				},
				{
					EnvironmentGroupName: string(nameCanaryDe),
					Environments: []*api.Environment{
						makeEnv(nameCanaryDe, nameCanaryDe, makeUpstreamEnvironment(nameStagingDe), 3, api.Priority_CANARY),
					},
					DistanceToUpstream: 3,
					Priority:           api.Priority_CANARY,
				},
				{
					EnvironmentGroupName: string(nameProdDe),
					Environments: []*api.Environment{
						makeEnv(nameProdDe, nameProdDe, makeUpstreamEnvironment(nameCanaryDe), 4, api.Priority_PROD),
					},
					DistanceToUpstream: 4,
					Priority:           api.Priority_PROD,
				},
			},
		},
		{
			Name: "Two chains of environments, one d->s->c->p and one d->s->p should have both p as prod and both s as staging",
			InputEnvs: map[types.EnvName]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
				},
				nameDevFr: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameDevDe),
					},
				},
				nameStagingFr: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameDevFr),
					},
				},
				nameCanaryDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameStagingDe),
					},
				},
				nameProdDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameCanaryDe),
					},
				},
				nameProdFr: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameStagingFr),
					},
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: string(nameDevDe),
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
					},
					DistanceToUpstream: 0,
					Priority:           api.Priority_UPSTREAM,
				},
				{
					EnvironmentGroupName: string(nameDevFr),
					Environments: []*api.Environment{
						makeEnv(nameDevFr, nameDevFr, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
					},
					DistanceToUpstream: 0,
					Priority:           api.Priority_UPSTREAM,
				},
				{
					EnvironmentGroupName: string(nameStagingDe),
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStagingDe, makeUpstreamEnvironment(nameDevDe), 1, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 1,
					Priority:           api.Priority_PRE_PROD,
				},
				{
					EnvironmentGroupName: string(nameStagingFr),
					Environments: []*api.Environment{
						makeEnv(nameStagingFr, nameStagingFr, makeUpstreamEnvironment(nameDevFr), 1, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 1,
					Priority:           api.Priority_PRE_PROD,
				},
				{
					EnvironmentGroupName: string(nameCanaryDe),
					Environments: []*api.Environment{
						makeEnv(nameCanaryDe, nameCanaryDe, makeUpstreamEnvironment(nameStagingDe), 2, api.Priority_CANARY),
					},
					DistanceToUpstream: 2,
					Priority:           api.Priority_CANARY,
				},
				{
					EnvironmentGroupName: string(nameProdFr),
					Environments: []*api.Environment{
						makeEnv(nameProdFr, nameProdFr, makeUpstreamEnvironment(nameStagingFr), 2, api.Priority_PROD),
					},
					DistanceToUpstream: 2,

					Priority: api.Priority_CANARY,
				},
				{
					EnvironmentGroupName: string(nameProdDe),
					Environments: []*api.Environment{
						makeEnv(nameProdDe, nameProdDe, makeUpstreamEnvironment(nameCanaryDe), 3, api.Priority_PROD),
					},
					DistanceToUpstream: 3,
					Priority:           api.Priority_PROD,
				},
			},
		},
		{
			// this is a realistic example
			Name: "Three Groups with 2 envs each",
			InputEnvs: map[types.EnvName]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
					EnvironmentGroup: types.StringPtr(nameDev),
				},
				nameDevFr: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
					EnvironmentGroup: types.StringPtr(nameDev),
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameDevDe),
					},
					EnvironmentGroup: types.StringPtr(nameStaging),
				},
				nameStagingFr: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameDevFr),
					},
					EnvironmentGroup: types.StringPtr(nameStaging),
				},
				nameProdDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameStagingDe),
					},
					EnvironmentGroup: types.StringPtr(nameProd),
				},
				nameProdFr: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameStagingFr),
					},
					EnvironmentGroup: types.StringPtr(nameProd),
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: string(nameDev),
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDev, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
						makeEnv(nameDevFr, nameDev, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
					},
					DistanceToUpstream: 0,
					Priority:           api.Priority_UPSTREAM,
				},
				{
					EnvironmentGroupName: string(nameStaging),
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStaging, makeUpstreamEnvironment(nameDevDe), 1, api.Priority_PRE_PROD),
						makeEnv(nameStagingFr, nameStaging, makeUpstreamEnvironment(nameDevFr), 1, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 1,
					Priority:           api.Priority_PRE_PROD,
				},
				{
					EnvironmentGroupName: string(nameProd),
					Environments: []*api.Environment{
						makeEnv(nameProdDe, nameProd, makeUpstreamEnvironment(nameStagingDe), 2, api.Priority_PROD),
						makeEnv(nameProdFr, nameProd, makeUpstreamEnvironment(nameStagingFr), 2, api.Priority_PROD),
					},
					DistanceToUpstream: 2,
					Priority:           api.Priority_PROD,
				},
			},
		},
		{
			Name: "Environments with different environment priorities",
			/*
					dev-global <--- test-global <--- staging-de <--- canary-de <--- prod-de
					                              |
												  -- staging-fr <--- prod-fr

				    ^^^^^^^^^^      ^^^^^^^^^^^      ^^^^^^^^^^      ^^^^^^^^^      ^^^^^^^
					dev             test             staging         canary         prod
					prio: u         prio: o          prio: pp        prio: c        prio: p

			*/
			InputEnvs: map[types.EnvName]config.EnvironmentConfig{
				nameDevGlobal: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
					EnvironmentGroup: types.StringPtr(nameDev),
				},
				nameTestGlobal: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameDevGlobal),
					},
					EnvironmentGroup: types.StringPtr(nameTest),
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameTestGlobal),
					},
					EnvironmentGroup: types.StringPtr(nameStaging),
				},
				nameStagingFr: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameTestGlobal),
					},
					EnvironmentGroup: types.StringPtr(nameStaging),
				},
				nameCanaryDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameStagingDe),
					},
					EnvironmentGroup: types.StringPtr(nameCanary),
				},
				nameProdDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameCanaryDe),
					},
					EnvironmentGroup: types.StringPtr(nameProd),
				},
				nameProdFr: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: string(nameStagingFr),
					},
					EnvironmentGroup: types.StringPtr(nameCanary),
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: string(nameDev),
					Environments: []*api.Environment{
						makeEnv(nameDevGlobal, nameDev, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
					},
					DistanceToUpstream: 0,
					Priority:           api.Priority_UPSTREAM,
				},
				{
					EnvironmentGroupName: string(nameTest),
					Environments: []*api.Environment{
						makeEnv(nameTestGlobal, nameTest, makeUpstreamEnvironment(nameDevGlobal), 1, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 1,
					Priority:           api.Priority_OTHER,
				},
				{
					EnvironmentGroupName: string(nameStaging),
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStaging, makeUpstreamEnvironment(nameTestGlobal), 2, api.Priority_PRE_PROD),
						makeEnv(nameStagingFr, nameStaging, makeUpstreamEnvironment(nameTestGlobal), 2, api.Priority_CANARY),
					},
					DistanceToUpstream: 2,
					Priority:           api.Priority_PRE_PROD,
				},
				{
					EnvironmentGroupName: string(nameCanary),
					Environments: []*api.Environment{
						makeEnv(nameCanaryDe, nameCanary, makeUpstreamEnvironment(nameStagingDe), 3, api.Priority_CANARY),
						makeEnv(nameProdFr, nameCanary, makeUpstreamEnvironment(nameStagingFr), 3, api.Priority_PROD),
					},
					DistanceToUpstream: 3,
					Priority:           api.Priority_CANARY,
				},
				{
					EnvironmentGroupName: string(nameProd),
					Environments: []*api.Environment{
						makeEnv(nameProdDe, nameProd, makeUpstreamEnvironment(nameCanaryDe), 4, api.Priority_PROD),
					},
					DistanceToUpstream: 4,
					Priority:           api.Priority_PROD,
				},
			},
		},
	}
	for _, tc := range tcs {
		opts := cmpopts.IgnoreUnexported(api.EnvironmentGroup{}, api.Environment{}, api.EnvironmentConfig{}, api.EnvironmentConfig_Upstream{})
		t.Run(tc.Name, func(t *testing.T) {
			actualResult := MapEnvironmentsToGroups(tc.InputEnvs)
			if !cmp.Equal(tc.ExpectedResult, actualResult, opts) {
				t.Fatal("Output mismatch (-want +got):\n", cmp.Diff(tc.ExpectedResult, actualResult, opts))
			}
		})
	}
}
