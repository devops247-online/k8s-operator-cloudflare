{{/*
Expand the name of the chart.
*/}}
{{- define "cloudflare-dns-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "cloudflare-dns-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "cloudflare-dns-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "cloudflare-dns-operator.labels" -}}
helm.sh/chart: {{ include "cloudflare-dns-operator.chart" . }}
{{ include "cloudflare-dns-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "cloudflare-dns-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cloudflare-dns-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "cloudflare-dns-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "cloudflare-dns-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
SLO configuration helpers
*/}}

{{/*
Get SLO configuration based on environment
*/}}
{{- define "cloudflare-dns-operator.sloConfig" -}}
{{- $global := .Values.slo -}}
{{- $env := .Values.config.environment | default "production" -}}
{{- $envConfig := index .Values.slo.environments $env | default dict -}}
{{- $config := mergeOverwrite $global $envConfig -}}
{{- toYaml $config -}}
{{- end -}}

{{/*
Get SLO targets based on environment
*/}}
{{- define "cloudflare-dns-operator.sloTargets" -}}
{{- $global := .Values.slo -}}
{{- $env := .Values.config.environment | default "production" -}}
{{- $envConfig := index .Values.slo.environments $env | default dict -}}
{{- $config := mergeOverwrite $global $envConfig -}}
{{- toYaml $config.targets -}}
{{- end -}}

{{/*
Reliability configuration helpers
*/}}

{{/*
Get Reliability configuration based on environment
*/}}
{{- define "cloudflare-dns-operator.reliabilityConfig" -}}
{{- $global := .Values.reliability -}}
{{- $env := .Values.config.environment | default "production" -}}
{{- $envConfig := index .Values.reliability.environments $env | default dict -}}
{{- $config := mergeOverwrite $global $envConfig -}}
{{- toYaml $config -}}
{{- end -}}

{{/*
Get circuit breaker config based on environment
*/}}
{{- define "cloudflare-dns-operator.circuitBreakerConfig" -}}
{{- $global := .Values.reliability -}}
{{- $env := .Values.config.environment | default "production" -}}
{{- $envConfig := index .Values.reliability.environments $env | default dict -}}
{{- $config := mergeOverwrite $global $envConfig -}}
{{- toYaml $config.circuitBreaker -}}
{{- end -}}

{{/*
Get retry config based on environment
*/}}
{{- define "cloudflare-dns-operator.retryConfig" -}}
{{- $global := .Values.reliability -}}
{{- $env := .Values.config.environment | default "production" -}}
{{- $envConfig := index .Values.reliability.environments $env | default dict -}}
{{- $config := mergeOverwrite $global $envConfig -}}
{{- toYaml $config.retry -}}
{{- end -}}

{{/*
Get rate limiter config based on environment
*/}}
{{- define "cloudflare-dns-operator.rateLimiterConfig" -}}
{{- $global := .Values.reliability -}}
{{- $env := .Values.config.environment | default "production" -}}
{{- $envConfig := index .Values.reliability.environments $env | default dict -}}
{{- $config := mergeOverwrite $global $envConfig -}}
{{- toYaml $config.rateLimiter -}}
{{- end -}}

{{/*
Generate SLO environment variables
*/}}
{{- define "cloudflare-dns-operator.sloEnvVars" -}}
{{- $global := .Values.slo -}}
{{- $env := .Values.config.environment | default "production" -}}
{{- $envConfig := index .Values.slo.environments $env | default dict -}}
{{- $config := mergeOverwrite $global $envConfig -}}
- name: SLO_AVAILABILITY_TARGET
  value: {{ $config.targets.availability | quote }}
- name: SLO_SUCCESS_RATE_TARGET
  value: {{ $config.targets.successRate | quote }}
- name: SLO_LATENCY_P95_TARGET
  value: {{ $config.targets.latencyP95 | quote }}
- name: SLO_LATENCY_P99_TARGET
  value: {{ $config.targets.latencyP99 | quote }}
- name: SLO_THROUGHPUT_MIN_TARGET
  value: {{ $config.targets.throughputMin | quote }}
- name: SLO_ERROR_BUDGET_WINDOW_DAYS
  value: {{ $config.errorBudget.windowDays | quote }}
- name: SLO_ERROR_BUDGET_WARNING_THRESHOLD
  value: {{ $config.errorBudget.alertThresholds.warning | quote }}
- name: SLO_ERROR_BUDGET_CRITICAL_THRESHOLD
  value: {{ $config.errorBudget.alertThresholds.critical | quote }}
- name: SLO_PAGE_ALERTS_ENABLED
  value: {{ $config.alerting.pageAlerts.enabled | quote }}
- name: SLO_TICKET_ALERTS_ENABLED
  value: {{ $config.alerting.ticketAlerts.enabled | quote }}
{{- end -}}

{{/*
Generate Reliability environment variables
*/}}
{{- define "cloudflare-dns-operator.reliabilityEnvVars" -}}
{{- $global := .Values.reliability -}}
{{- $env := .Values.config.environment | default "production" -}}
{{- $envConfig := index .Values.reliability.environments $env | default dict -}}
{{- $config := mergeOverwrite $global $envConfig -}}
- name: CIRCUIT_BREAKER_ENABLED
  value: {{ $config.circuitBreaker.enabled | quote }}
- name: CIRCUIT_BREAKER_NAME
  value: {{ $config.circuitBreaker.name | quote }}
- name: CIRCUIT_BREAKER_MAX_REQUESTS
  value: {{ $config.circuitBreaker.maxRequests | quote }}
- name: CIRCUIT_BREAKER_INTERVAL
  value: {{ $config.circuitBreaker.interval | quote }}
- name: CIRCUIT_BREAKER_TIMEOUT
  value: {{ $config.circuitBreaker.timeout | quote }}
- name: CIRCUIT_BREAKER_FAILURE_RATE_THRESHOLD
  value: {{ $config.circuitBreaker.failureRateThreshold | quote }}
- name: CIRCUIT_BREAKER_MINIMUM_REQUESTS
  value: {{ $config.circuitBreaker.minimumRequests | quote }}
- name: RETRY_ENABLED
  value: {{ $config.retry.enabled | quote }}
- name: RETRY_MAX_ATTEMPTS
  value: {{ $config.retry.maxAttempts | quote }}
- name: RETRY_INITIAL_DELAY
  value: {{ $config.retry.initialDelay | quote }}
- name: RETRY_MAX_DELAY
  value: {{ $config.retry.maxDelay | quote }}
- name: RETRY_MULTIPLIER
  value: {{ $config.retry.multiplier | quote }}
- name: RETRY_JITTER
  value: {{ $config.retry.jitter | quote }}
- name: RETRY_MAX_JITTER
  value: {{ $config.retry.maxJitter | quote }}
- name: RATE_LIMITER_ENABLED
  value: {{ $config.rateLimiter.enabled | quote }}
- name: RATE_LIMITER_TYPE
  value: {{ $config.rateLimiter.type | quote }}
- name: RATE_LIMITER_REQUESTS_PER_SECOND
  value: {{ $config.rateLimiter.requestsPerSecond | quote }}
- name: RATE_LIMITER_BURST_SIZE
  value: {{ $config.rateLimiter.burstSize | quote }}
- name: RATE_LIMITER_WINDOW_SIZE
  value: {{ $config.rateLimiter.windowSize | quote }}
- name: RATE_LIMITER_BACKOFF_STRATEGY
  value: {{ $config.rateLimiter.backoffStrategy | quote }}
- name: RELIABILITY_METRICS_ENABLED
  value: {{ $config.metrics.enabled | quote }}
{{- end -}}
