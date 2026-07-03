{{- define "filament.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "filament.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "filament.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "filament.labels" -}}
helm.sh/chart: {{ include "filament.chart" . }}
{{ include "filament.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "filament.selectorLabels" -}}
app.kubernetes.io/name: {{ include "filament.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}


{{- define "filament.control.fullname" -}}
{{- printf "%s-control" (include "filament.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "filament.control.labels" -}}
{{ include "filament.labels" . }}
app.kubernetes.io/component: control
{{- end -}}

{{- define "filament.control.selectorLabels" -}}
{{ include "filament.selectorLabels" . }}
app.kubernetes.io/component: control
{{- end -}}

{{- define "filament.control.serviceAccountName" -}}
{{- include "filament.control.fullname" . -}}
{{- end -}}


{{- define "filament.worker.fullname" -}}
{{- printf "%s-worker" (include "filament.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "filament.worker.labels" -}}
{{ include "filament.labels" . }}
app.kubernetes.io/component: worker
{{- end -}}

{{- define "filament.worker.serviceAccountName" -}}
{{- .Values.control.dispatch.worker.serviceAccount | default (include "filament.worker.fullname" .) -}}
{{- end -}}


{{/* Postgres DSN secret: user's existingSecret, else the chart-created one. */}}
{{- define "filament.pg.secretName" -}}
{{- $es := .Values.persistence.postgresql.dsn.existingSecret -}}
{{- if $es.name -}}
{{- $es.name -}}
{{- else -}}
{{- printf "%s-db" (include "filament.control.fullname" .) -}}
{{- end -}}
{{- end -}}

{{- define "filament.pg.secretKey" -}}
{{- $es := .Values.persistence.postgresql.dsn.existingSecret -}}
{{- if $es.name -}}
{{- $es.key | default "dsn" -}}
{{- else -}}
dsn
{{- end -}}
{{- end -}}
