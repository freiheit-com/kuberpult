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

{{/*
Returns a sorted, comma-separated list of environment names that have bracket rendering enabled.
Returns an empty string when experimentalBrackets.enabled is false.
*/}}
{{- define "kuberpult.experimentalBracketsClusters" -}}
{{- if .Values.rollout.experimentalBrackets.enabled -}}
{{- $clusters := list -}}
{{- range $k, $v := .Values.rollout.experimentalBrackets.clusters -}}
{{- if $v -}}{{- $clusters = append $clusters $k -}}{{- end -}}
{{- end -}}
{{- sortAlpha $clusters | join "," -}}
{{- end -}}
{{- end -}}
