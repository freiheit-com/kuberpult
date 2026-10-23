# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright freiheit.com

# Selects all root app filter clusters that are set to true
{{- define "kuberpult.experimentalRootAppFilter" -}}
{{- $envs := list -}}
{{- range $k, $v := .Values.manifestRepoExport.rendering.experimentalRootAppFilter.envs -}}
{{- if $v -}}{{- $envs = append $envs $k -}}{{- end -}}
{{- end -}}
{{- sortAlpha $envs | join "," -}}
{{- end -}}
