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

package db

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/freiheit-com/kuberpult/pkg/types"
)

func TestUpsertAppTeam(t *testing.T) {
	tcs := []struct {
		Name     string
		Initial  AppTeamMap
		TeamName string
		AppName  types.AppName
		Expected AppTeamMap
	}{
		{
			Name:     "add to nil map",
			Initial:  nil,
			TeamName: "team1",
			AppName:  "app1",
			Expected: AppTeamMap{"team1": {"app1"}},
		},
		{
			Name:     "app already in the right team is a no-op",
			Initial:  AppTeamMap{"team1": {"app1", "app2"}},
			TeamName: "team1",
			AppName:  "app1",
			Expected: AppTeamMap{"team1": {"app1", "app2"}},
		},
		{
			Name:     "moving an app keeps the other apps of the old team",
			Initial:  AppTeamMap{"team1": {"app1", "app2"}},
			TeamName: "team2",
			AppName:  "app1",
			Expected: AppTeamMap{"team1": {"app2"}, "team2": {"app1"}},
		},
		{
			Name:     "emptied team is removed",
			Initial:  AppTeamMap{"team1": {"app1"}},
			TeamName: "team2",
			AppName:  "app1",
			Expected: AppTeamMap{"team2": {"app1"}},
		},
		{
			Name:     "app listed under several teams ends up in exactly one",
			Initial:  AppTeamMap{"team1": {"app1"}, "team2": {"app1"}, "team3": {"app1", "app2"}},
			TeamName: "team4",
			AppName:  "app1",
			Expected: AppTeamMap{"team3": {"app2"}, "team4": {"app1"}},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			actual := UpsertAppTeam(tc.Initial, tc.TeamName, tc.AppName)
			if diff := cmp.Diff(tc.Expected, actual); diff != "" {
				t.Errorf("app team map mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestRemoveAppFromTeams(t *testing.T) {
	tcs := []struct {
		Name     string
		Initial  AppTeamMap
		AppName  types.AppName
		Expected AppTeamMap
	}{
		{
			Name:     "removing keeps the other apps of the team",
			Initial:  AppTeamMap{"team1": {"app1", "app2"}},
			AppName:  "app1",
			Expected: AppTeamMap{"team1": {"app2"}},
		},
		{
			Name:     "emptied team is removed",
			Initial:  AppTeamMap{"team1": {"app1"}, "team2": {"app2"}},
			AppName:  "app1",
			Expected: AppTeamMap{"team2": {"app2"}},
		},
		{
			Name:     "unknown app is a no-op",
			Initial:  AppTeamMap{"team1": {"app1"}},
			AppName:  "app-does-not-exist",
			Expected: AppTeamMap{"team1": {"app1"}},
		},
		{
			Name:     "app listed under several teams is removed everywhere",
			Initial:  AppTeamMap{"team1": {"app1"}, "team2": {"app1", "app2"}},
			AppName:  "app1",
			Expected: AppTeamMap{"team2": {"app2"}},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			actual := RemoveAppFromTeams(tc.Initial, tc.AppName)
			if diff := cmp.Diff(tc.Expected, actual); diff != "" {
				t.Errorf("app team map mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
