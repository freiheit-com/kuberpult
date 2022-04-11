/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
package config

type ApplicationConfig struct {
	Owner string `json:"owner"`
}

type EnvironmentConfig struct {
	Upstream *EnvironmentConfigUpstream `json:"upstream,omitempty"`
	ArgoCd   *EnvironmentConfigArgoCd   `json:"argocd,omitempty"`
}

type EnvironmentConfigUpstream struct {
	Environment string `json:"environment"`
	Latest      bool   `json:"latest,omitempty"`
}

type AccessEntry struct {
	Group string `json:"group,omitempty"`
	Kind  string `json:"kind,omitempty"`
}

type EnvironmentConfigArgoCd struct {
	Destination              ArgoCdDestination        `json:"destination"`
	SyncWindows              []ArgoCdSyncWindow       `json:"syncWindows,omitempty"`
	ClusterResourceWhitelist []AccessEntry            `json:"accessList,omitempty"`
	ApplicationAnnotations   map[string]string        `json:"applicationAnnotations,omitempty"`
	IgnoreDifferences        []ArgoCdIgnoreDifference `json:"ignoreDifferences,omitempty"`
}

type ArgoCdDestination struct {
	Name      string `json:"name"`
	Server    string `json:"server"`
	Namespace string `json:"namespace,omitempty"`
}

type ArgoCdSyncWindow struct {
	Schedule string `json:"schedule,omitempty"`
	Duration string `json:"duration,omitempty"`
	Kind     string `json:"kind,omitempty"`
}

type ArgoCdIgnoreDifference struct {
	Group        string   `json:"group,omitempty"`
	Kind         string   `json:"kind"`
	Name         string   `json:"name,omitempty"`
	Namespace    string   `json:"namespace,omitempty"`
	JSONPointers []string `json:"jsonPointers"`
}
