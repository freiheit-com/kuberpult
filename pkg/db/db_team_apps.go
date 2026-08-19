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
	"slices"
	"strings"

	"github.com/freiheit-com/kuberpult/pkg/types"
)

// removeAppFromTeams removes the given app, and returns the modified map
func removeAppFromTeams(appTeams TeamToAppsMap, appName types.AppName) TeamToAppsMap {
	for team, apps := range appTeams {
		idx := slices.Index(apps, appName)
		if idx < 0 {
			continue
		}
		apps = slices.Delete(apps, idx, idx+1)
		if len(apps) == 0 {
			delete(appTeams, team)
		} else {
			appTeams[team] = apps
		}
	}
	return appTeams
}

// upsertAppTeam adds the given app, if it's not already in, and returns the modified map
func upsertAppTeam(appTeams TeamToAppsMap, teamName string, appName types.AppName) TeamToAppsMap {
	if appTeams == nil {
		appTeams = make(TeamToAppsMap)
	}
	if slices.Contains(appTeams[teamName], appName) {
		return appTeams
	}
	appTeams = removeAppFromTeams(appTeams, appName)
	appTeams[teamName] = append(appTeams[teamName], appName)
	return appTeams
}

// appsTeamsFromMap is the inverse of appsTeamsToMap: one entry per app, sorted by app name.
func appsTeamsFromMap(appTeams TeamToAppsMap) []AppWithTeam {
	result := make([]AppWithTeam, 0, len(appTeams))
	for team, apps := range appTeams {
		for _, appName := range apps {
			result = append(result, AppWithTeam{AppName: appName, TeamName: team})
		}
	}
	slices.SortFunc(result, func(a, b AppWithTeam) int {
		return strings.Compare(string(a.AppName), string(b.AppName))
	})
	return result
}

func appsTeamsToMap(appsWithTeam []AppWithTeam) TeamToAppsMap {
	teamByApp := make(map[types.AppName]string, len(appsWithTeam))
	for _, v := range appsWithTeam {
		teamByApp[v.AppName] = v.TeamName // last occurrence wins
	}
	result := make(TeamToAppsMap)
	for app, team := range teamByApp {
		result[team] = append(result[team], app)
	}
	for team := range result {
		slices.Sort(result[team])
	}
	return result
}
