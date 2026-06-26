{{/*
Expand the name of the chart.
*/}}
{{- define "ops-ai.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "ops-ai.fullname" -}}
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
Create the name of the service account to use
*/}}
{{- define "ops-ai.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- include "ops-ai.fullname" . }}-sa
{{- else }}
{{- .Values.serviceAccount.name | default "default" }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "ops-ai.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "ops-ai.labels" -}}
helm.sh/chart: {{ include "ops-ai.chart" . }}
{{ include "ops-ai.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "ops-ai.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ops-ai.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the StatefulSet
*/}}
{{- define "ops-ai.statefulset" -}}
{{- include "ops-ai.fullname" . }}
{{- end }}

{{/*
Create the database secret name
*/}}
{{- define "ops-ai.database.secretName" -}}
{{- if .Values.database.existingSecret }}
{{- .Values.database.existingSecret }}
{{- else }}
{{- include "ops-ai.fullname" . }}-db
{{- end }}
{{- end }}

{{/*
Create the Redis secret name
*/}}
{{- define "ops-ai.redis.secretName" -}}
{{- if .Values.redis.existingSecret }}
{{- .Values.redis.existingSecret }}
{{- else }}
{{- include "ops-ai.fullname" . }}-redis
{{- end }}
{{- end }}
