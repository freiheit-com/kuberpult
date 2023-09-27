{{- define "rollout-podAnnotations" }}
{{- if .Values.datadogTracing.enabled }}
apm.datadoghq.com/env: '{"DD_SERVICE":"kuberpult-rollout-service","DD_ENV":"{{ .Values.datadogTracing.environment }}","DD_VERSION":"{{ .Values.rollout.tag }}"}'
{{- end }}
{{- end }}
