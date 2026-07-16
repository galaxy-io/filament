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
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end -}}

{{- define "filament.namespace" -}}
{{- .Values.namespaceOverride | default .Release.Namespace -}}
{{- end -}}

{{- define "filament.selectorLabels" -}}
app.kubernetes.io/name: {{ include "filament.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}


{{- define "filament.controlPlane.fullname" -}}
{{- printf "%s-control-plane" (include "filament.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "filament.controlPlane.labels" -}}
{{ include "filament.labels" . }}
app.kubernetes.io/component: control-plane
{{- end -}}

{{- define "filament.controlPlane.selectorLabels" -}}
{{ include "filament.selectorLabels" . }}
app.kubernetes.io/component: control-plane
{{- end -}}

{{- define "filament.controlPlane.serviceAccountName" -}}
{{- include "filament.controlPlane.fullname" . -}}
{{- end -}}


{{- define "filament.worker.fullname" -}}
{{- printf "%s-worker" (include "filament.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "filament.worker.labels" -}}
{{ include "filament.labels" . }}
app.kubernetes.io/component: worker
{{- end -}}

{{- define "filament.worker.serviceAccountName" -}}
{{- .Values.controlPlane.dispatch.worker.serviceAccount | default (include "filament.worker.fullname" .) -}}
{{- end -}}


{{/* User's existingSecret, else the chart-created Secret. */}}
{{- define "filament.secretName" -}}
{{- .Values.existingSecret | default (printf "%s-secret" (include "filament.fullname" .)) -}}
{{- end -}}


{{- define "filament.server.fullname" -}}
{{- printf "%s-server" (include "filament.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "filament.server.labels" -}}
{{ include "filament.labels" . }}
app.kubernetes.io/component: server
{{- end -}}

{{- define "filament.server.selectorLabels" -}}
{{ include "filament.selectorLabels" . }}
app.kubernetes.io/component: server
{{- end -}}

{{- define "filament.server.serviceAccountName" -}}
{{- include "filament.server.fullname" . -}}
{{- end -}}
