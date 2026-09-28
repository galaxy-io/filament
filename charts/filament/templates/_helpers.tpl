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
{{- .Values.controlPlane.serviceAccount.name | default (include "filament.controlPlane.fullname" .) -}}
{{- end -}}


{{- define "filament.worker.fullname" -}}
{{- printf "%s-worker" (include "filament.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "filament.worker.labels" -}}
{{ include "filament.labels" . }}
app.kubernetes.io/component: worker
{{- end -}}

{{- define "filament.worker.serviceAccountName" -}}
{{- .Values.controlPlane.dispatch.worker.serviceAccount.name | default (include "filament.worker.fullname" .) -}}
{{- end -}}


{{/* User's existingSecret, else the chart's own Secret. */}}
{{- define "filament.secretName" -}}
{{- .Values.existingSecret | default "filament-secret" -}}
{{- end -}}

{{/* Generated on first install, read back from the Secret on upgrades. */}}
{{- define "filament.generated" -}}
{{- $existing := lookup "v1" "Secret" (include "filament.namespace" .context) (include "filament.secretName" .context) -}}
{{- if and $existing (hasKey $existing.data .key) -}}
{{- index $existing.data .key | b64dec -}}
{{- else -}}
{{- randAlphaNum 32 -}}
{{- end -}}
{{- end -}}

{{/* Configured issuer, else the vendored Keycloak's. */}}
{{- define "filament.auth.keycloak.issuer" -}}
{{- if .Values.auth.keycloak.issuer -}}
{{- .Values.auth.keycloak.issuer -}}
{{- else if .Values.keycloak.enabled -}}
{{- required "keycloak.hostname is required when keycloak.enabled=true" .Values.keycloak.hostname | trimSuffix "/" -}}/realms/filament
{{- else -}}
{{- required "auth.keycloak.issuer is required when auth.type=keycloak" .Values.auth.keycloak.issuer -}}
{{- end -}}
{{- end -}}

{{- define "filament.auth.keycloak.clientSecret" -}}
{{- if .Values.auth.keycloak.clientSecret -}}{{ .Values.auth.keycloak.clientSecret }}
{{- else if .Values.keycloak.enabled -}}{{ include "filament.generated" (dict "context" . "key" "AUTH_CLIENT_SECRET") }}
{{- else -}}{{ required "auth.keycloak.clientSecret is required when auth.type=keycloak" .Values.auth.keycloak.clientSecret }}
{{- end -}}
{{- end -}}

{{- define "filament.auth.keycloak.adminUser" -}}
{{- if .Values.auth.keycloak.adminUser -}}{{ .Values.auth.keycloak.adminUser }}
{{- else if .Values.keycloak.enabled -}}{{ .Values.keycloak.adminUser }}
{{- end -}}
{{- end -}}

{{- define "filament.auth.keycloak.adminPassword" -}}
{{- if .Values.auth.keycloak.adminPassword -}}{{ .Values.auth.keycloak.adminPassword }}
{{- else if .Values.keycloak.enabled -}}{{ include "filament.generated" (dict "context" . "key" "AUTH_ADMIN_PASSWORD") }}
{{- end -}}
{{- end -}}

{{/* Secret the vendored provider mints its own admin token into during setup,
     named after its machine user. Empty against an external provider, where the
     token is supplied through the chart Secret instead. */}}
{{- define "filament.auth.mintedPatSecret" -}}
{{- if and .Values.auth.enabled (eq .Values.auth.type "zitadel") .Values.zitadel.enabled -}}
{{- $machine := dig "zitadel" "configmapConfig" "FirstInstance" "Org" "Machine" "Machine" (dict) .Values.zitadel -}}
{{- printf "%s-pat" (default "iam-admin" $machine.Username) -}}
{{- end -}}
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
{{- .Values.server.serviceAccount.name | default (include "filament.server.fullname" .) -}}
{{- end -}}
