{{/*
Logging configuration helpers
*/}}

{{/*
Get logging configuration based on environment
*/}}
{{- define "cloudflare-dns-operator.loggingConfig" -}}
{{- $global := .Values.logging -}}
{{- $env := .Values.config.environment | default "production" -}}
{{- $envConfig := index .Values.logging.environments $env | default dict -}}
{{- $config := mergeOverwrite $global $envConfig -}}
{{- toYaml $config -}}
{{- end -}}

{{/*
Generate logging environment variables
*/}}
{{- define "cloudflare-dns-operator.loggingEnvVars" -}}
{{- $global := .Values.logging -}}
{{- $env := .Values.config.environment | default "production" -}}
{{- $envConfig := index .Values.logging.environments $env | default dict -}}
{{- $config := mergeOverwrite $global $envConfig -}}
- name: LOG_LEVEL
  value: {{ $config.level | quote }}
- name: LOG_FORMAT
  value: {{ $config.format | quote }}
- name: LOG_DEVELOPMENT
  value: {{ $config.development | quote }}
{{- if $config.sampling.enabled }}
- name: LOG_SAMPLING_ENABLED
  value: {{ $config.sampling.enabled | quote }}
- name: LOG_SAMPLING_INITIAL
  value: {{ $config.sampling.initial | quote }}
- name: LOG_SAMPLING_THEREAFTER
  value: {{ $config.sampling.thereafter | quote }}
{{- end }}
{{- end -}}

{{/*
Generate logging outputs as JSON array
*/}}
{{- define "cloudflare-dns-operator.loggingOutputs" -}}
{{- $global := .Values.logging -}}
{{- $env := .Values.config.environment | default "production" -}}
{{- $envConfig := index .Values.logging.environments $env | default dict -}}
{{- $config := mergeOverwrite $global $envConfig -}}
{{- $config.outputs | toJson -}}
{{- end -}}
