{{/*
Tracing configuration helpers
*/}}

{{/*
Get tracing configuration based on environment
*/}}
{{- define "cloudflare-dns-operator.tracingConfig" -}}
{{- $global := .Values.tracing -}}
{{- $env := .Values.config.environment | default "production" -}}
{{- $envConfig := index .Values.tracing.environments $env | default dict -}}
{{- $config := mergeOverwrite $global $envConfig -}}
{{- toYaml $config -}}
{{- end -}}

{{/*
Generate tracing environment variables
*/}}
{{- define "cloudflare-dns-operator.tracingEnvVars" -}}
{{- $global := .Values.tracing -}}
{{- $env := .Values.config.environment | default "production" -}}
{{- $envConfig := index .Values.tracing.environments $env | default dict -}}
{{- $config := mergeOverwrite $global $envConfig -}}
- name: TRACING_ENABLED
  value: {{ $config.enabled | quote }}
{{- if $config.enabled }}
- name: TRACING_SERVICE_NAME
  value: {{ $config.serviceName | default (include "cloudflare-dns-operator.fullname" .) | quote }}
- name: TRACING_SERVICE_VERSION
  value: {{ $config.serviceVersion | default .Chart.AppVersion | quote }}
- name: TRACING_ENVIRONMENT
  value: {{ $config.environment | quote }}
- name: TRACING_EXPORTER_TYPE
  value: {{ $config.exporter.type | quote }}
{{- if $config.exporter.endpoint }}
- name: TRACING_EXPORTER_ENDPOINT
  value: {{ $config.exporter.endpoint | quote }}
{{- end }}
- name: TRACING_EXPORTER_TIMEOUT
  value: {{ $config.exporter.timeout | quote }}
- name: TRACING_EXPORTER_INSECURE
  value: {{ $config.exporter.insecure | quote }}
{{- if $config.exporter.compression }}
- name: TRACING_EXPORTER_COMPRESSION
  value: {{ $config.exporter.compression | quote }}
{{- end }}
- name: TRACING_SAMPLING_TYPE
  value: {{ $config.sampling.type | quote }}
- name: TRACING_SAMPLING_RATIO
  value: {{ $config.sampling.ratio | quote }}
{{- end }}
{{- end -}}

{{/*
Generate tracing resource attributes as environment variables
*/}}
{{- define "cloudflare-dns-operator.tracingResourceAttributesEnvVars" -}}
{{- $global := .Values.tracing -}}
{{- $env := .Values.config.environment | default "production" -}}
{{- $envConfig := index .Values.tracing.environments $env | default dict -}}
{{- $config := mergeOverwrite $global $envConfig -}}
{{- if and $config.enabled $config.resourceAttributes }}
{{- range $key, $value := $config.resourceAttributes }}
- name: TRACING_RESOURCE_{{ $key | upper | replace "." "_" | replace "-" "_" }}
  value: {{ $value | quote }}
{{- end }}
{{- end }}
{{- end -}}

{{/*
Generate tracing exporter headers as environment variables
*/}}
{{- define "cloudflare-dns-operator.tracingHeadersEnvVars" -}}
{{- $global := .Values.tracing -}}
{{- $env := .Values.config.environment | default "production" -}}
{{- $envConfig := index .Values.tracing.environments $env | default dict -}}
{{- $config := mergeOverwrite $global $envConfig -}}
{{- if and $config.enabled $config.exporter.headers }}
{{- range $key, $value := $config.exporter.headers }}
- name: TRACING_HEADER_{{ $key | upper | replace "-" "_" }}
  value: {{ $value | quote }}
{{- end }}
{{- end }}
{{- end -}}
