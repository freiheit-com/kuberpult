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

package mapper

import (
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/google/go-cmp/cmp"
	"testing"
)

func makeUpstreamLatest() *api.EnvironmentConfig_Upstream {
	f := true
	return &api.EnvironmentConfig_Upstream{
		Latest: &f,
	}
}

func makeUpstreamEnvironment(env string) *api.EnvironmentConfig_Upstream {
	return &api.EnvironmentConfig_Upstream{
		Environment: &env,
	}
}

var nameStagingDe = "staging-de"
var nameDevDe = "dev-de"
var nameCanaryStagingDe = "canary-staging-de"
var nameProdDe = "prod-de"
var nameWhoKnowsDe = "whoknows-de"

var nameStagingFr = "staging-fr"
var nameDevFr = "dev-fr"
var nameProdFr = "prod-fr"
var nameWhoKnowsFr = "whoknows-fr"

var nameStaging = "staging"
var nameDev = "dev"
var nameProd = "prod"
var nameWhoKnows = "whoknows"

func makeEnv(envName string, groupName string, upstream *api.EnvironmentConfig_Upstream, distanceToUpstream uint32, priority api.Priority) *api.Environment {
	return &api.Environment{
		Name: envName,
		Config: &api.EnvironmentConfig{
			Upstream:         upstream,
			EnvironmentGroup: &groupName,
		},
		Locks:              map[string]*api.Lock{},
		Applications:       map[string]*api.Environment_Application{},
		DistanceToUpstream: distanceToUpstream,
		Priority:           priority, // we are 1 away from prod, hence pre-prod
	}
}

func TestMapEnvironmentsToGroup(t *testing.T) {
	tcs := []struct {
		Name           string
		InputEnvs      map[string]config.EnvironmentConfig
		ExpectedResult []*api.EnvironmentGroup
	}{
		{
			Name: "One Environment is one Group",
			InputEnvs: map[string]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: "",
						Latest:      true,
					},
					ArgoCd:           nil,
					EnvironmentGroup: &nameDevDe,
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: nameDevDe,
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamLatest(), 0, api.Priority_PROD),
					},
					DistanceToUpstream: 0,
				},
			},
		},
		{
			Name: "Two Environments are two Groups",
			InputEnvs: map[string]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
					ArgoCd: nil,
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameDevDe,
					},
					ArgoCd: nil,
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: nameDevDe,
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamLatest(), 0, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 0,
				},
				{
					EnvironmentGroupName: nameStagingDe,
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStagingDe, makeUpstreamEnvironment(nameDevDe), 1, api.Priority_PROD),
					},
					DistanceToUpstream: 1,
				},
			},
		},
		{
			// note that this is not a realistic example, we just want to make sure it does not crash!
			// some outputs may be nonsensical (like distanceToUpstream), but that's fine as long as it's stable!
			Name: "Two Environments with a loop",
			InputEnvs: map[string]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameStagingDe,
					},
					ArgoCd: nil,
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameDevDe,
					},
					ArgoCd: nil,
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: nameDevDe,
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamEnvironment(nameStagingDe), 667, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 667,
				},
				{
					EnvironmentGroupName: nameStagingDe,
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStagingDe, makeUpstreamEnvironment(nameDevDe), 668, api.Priority_PROD),
					},
					DistanceToUpstream: 668,
				},
			},
		},
		{
			// note that this is not a realistic example, we just want to make sure it does not crash!
			// some outputs may be nonsensical (like distanceToUpstream), but that's fine as long as it's stable!
			Name: "Two Environments with non exists upstream",
			InputEnvs: map[string]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
					ArgoCd: nil,
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameWhoKnows,
					},
					ArgoCd: nil,
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: nameDevDe,
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
					},
					DistanceToUpstream: 0,
				},
				{
					EnvironmentGroupName: nameStagingDe,
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStagingDe, makeUpstreamEnvironment(nameWhoKnows), 667, api.Priority_PROD),
					},
					DistanceToUpstream: 667,
				},
			},
		},
		{
			Name: "Three Environments are three Groups",
			InputEnvs: map[string]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
					ArgoCd: nil,
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameDevDe,
					},
					ArgoCd: nil,
				},
				nameProdDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameStagingDe,
					},
					ArgoCd: nil,
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: nameDevDe,
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
					},
					DistanceToUpstream: 0,
				},
				{
					EnvironmentGroupName: nameStagingDe,
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStagingDe, makeUpstreamEnvironment(nameDevDe), 1, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 1,
				},
				{
					EnvironmentGroupName: nameProdDe,
					Environments: []*api.Environment{
						makeEnv(nameProdDe, nameProdDe, makeUpstreamEnvironment(nameStagingDe), 2, api.Priority_PROD),
					},
					DistanceToUpstream: 2,
				},
			},
		},
		{
			Name: "Four Environments in a row to ensure that Priority_UPSTREAM works",
			InputEnvs: map[string]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameDevDe,
					},
				},
				nameProdDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameStagingDe,
					},
				},
				nameWhoKnowsDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameProdDe,
					},
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: nameDevDe,
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
					},
					DistanceToUpstream: 0,
				},
				{
					EnvironmentGroupName: nameStagingDe,
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStagingDe, makeUpstreamEnvironment(nameDevDe), 1, api.Priority_OTHER),
					},
					DistanceToUpstream: 1,
				},
				{
					EnvironmentGroupName: nameProdDe,
					Environments: []*api.Environment{
						makeEnv(nameProdDe, nameProdDe, makeUpstreamEnvironment(nameStagingDe), 2, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 2,
				},
				{
					EnvironmentGroupName: nameWhoKnowsDe,
					Environments: []*api.Environment{
						makeEnv(nameWhoKnowsDe, nameWhoKnowsDe, makeUpstreamEnvironment(nameProdDe), 3, api.Priority_PROD),
					},
					DistanceToUpstream: 3,
				},
			},
		},
		{
			Name: "Two chains of environments, one d->cs->s->p and one d->s->p should have both p as prod and both s as staging",
			InputEnvs: map[string]config.EnvironmentConfig{
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
				nameCanaryStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameDevDe,
					},
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameCanaryStagingDe,
					},
				},
				nameStagingFr: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameDevFr,
					},
				},
				nameProdDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameStagingDe,
					},
				},
				nameProdFr: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameStagingFr,
					},
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: nameDevDe,
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
					},
					DistanceToUpstream: 0,
				},
				{
					EnvironmentGroupName: nameDevFr,
					Environments: []*api.Environment{
						makeEnv(nameDevFr, nameDevFr, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
					},
					DistanceToUpstream: 0,
				},
				{
					EnvironmentGroupName: nameCanaryStagingDe,
					Environments: []*api.Environment{
						makeEnv(nameCanaryStagingDe, nameCanaryStagingDe, makeUpstreamEnvironment(nameDevDe), 1, api.Priority_OTHER),
					},
					DistanceToUpstream: 1,
				},
				{
					EnvironmentGroupName: nameStagingFr,
					Environments: []*api.Environment{
						makeEnv(nameStagingFr, nameStagingFr, makeUpstreamEnvironment(nameDevFr), 1, api.Priority_OTHER),
					},
					DistanceToUpstream: 1,
				},
				{
					EnvironmentGroupName: nameProdFr,
					Environments: []*api.Environment{
						makeEnv(nameProdFr, nameProdFr, makeUpstreamEnvironment(nameStagingFr), 2, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 2,
				},
				{
					EnvironmentGroupName: nameStagingDe,
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStagingDe, makeUpstreamEnvironment(nameCanaryStagingDe), 2, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 2,
				},
				{
					EnvironmentGroupName: nameProdDe,
					Environments: []*api.Environment{
						makeEnv(nameProdDe, nameProdDe, makeUpstreamEnvironment(nameStagingDe), 3, api.Priority_PROD),
					},
					DistanceToUpstream: 3,
				},
			},
		},
		{
			// this is a realistic example
			Name: "Three Groups with 2 envs each",
			InputEnvs: map[string]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
					EnvironmentGroup: &nameDev,
				},
				nameDevFr: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
					EnvironmentGroup: &nameDev,
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameDevDe,
					},
					EnvironmentGroup: &nameStaging,
				},
				nameStagingFr: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameDevFr,
					},
					EnvironmentGroup: &nameStaging,
				},
				nameProdDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameStagingDe,
					},
					EnvironmentGroup: &nameProd,
				},
				nameProdFr: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameStagingFr,
					},
					EnvironmentGroup: &nameProd,
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: nameDev,
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDev, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
						makeEnv(nameDevFr, nameDev, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
					},
					DistanceToUpstream: 0,
				},
				{
					EnvironmentGroupName: nameStaging,
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStaging, makeUpstreamEnvironment(nameDevDe), 1, api.Priority_PRE_PROD),
						makeEnv(nameStagingFr, nameStaging, makeUpstreamEnvironment(nameDevFr), 1, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 1,
				},
				{
					EnvironmentGroupName: nameProd,
					Environments: []*api.Environment{
						makeEnv(nameProdDe, nameProd, makeUpstreamEnvironment(nameStagingDe), 2, api.Priority_PROD),
						makeEnv(nameProdFr, nameProd, makeUpstreamEnvironment(nameStagingFr), 2, api.Priority_PROD),
					},
					DistanceToUpstream: 2,
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
