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

package config

type EnvironmentConfig struct {
	Upstream         *EnvironmentConfigUpstream `json:"upstream,omitempty"`
	ArgoCd           *EnvironmentConfigArgoCd   `json:"argocd,omitempty"`
	EnvironmentGroup *string                    `json:"environmentGroup,omitempty"`
}

type EnvironmentConfigUpstream struct {
	Environment string `json:"environment,omitempty"`
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
	SyncOptions              []string                 `json:"syncOptions,omitempty"`
}

// ArgoCdDestination
// Namespace takes precedence over AppProjectNamespace and ApplicationNamespace. To use the latter attributes, omit the
// Namespace attribute.
type ArgoCdDestination struct {
	Name                 string  `json:"name"`
	Server               string  `json:"server"`
	Namespace            *string `json:"namespace,omitempty"`
	AppProjectNamespace  *string `json:"appProjectNamespace,omitempty"`
	ApplicationNamespace *string `json:"applicationNamespace,omitempty"`
}

type ArgoCdSyncWindow struct {
	Schedule string   `json:"schedule,omitempty"`
	Duration string   `json:"duration,omitempty"`
	Kind     string   `json:"kind,omitempty"`
	Apps     []string `json:"applications,omitempty"`
}

type ArgoCdIgnoreDifference struct {
	Group                 string   `json:"group,omitempty"`
	Kind                  string   `json:"kind"`
	Name                  string   `json:"name,omitempty"`
	Namespace             string   `json:"namespace,omitempty"`
	JSONPointers          []string `json:"jsonPointers,omitempty"`
	JqPathExpressions     []string `json:"jqPathExpressions,omitempty"`
	ManagedFieldsManagers []string `json:"managedFieldsManagers,omitempty"`
}
