{{/*
Expand the name of the chart.
*/}}
{{- define "loopers.name" -}}
{{- default .Chart.Name .Values.nameOverride | truncate 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "loopers.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | truncate 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | truncate 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | truncate 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "loopers.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | truncate 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "loopers.labels" -}}
helm.sh/chart: {{ include "loopers.chart" . }}
{{ include "loopers.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "loopers.selectorLabels" -}}
app.kubernetes.io/name: {{ include "loopers.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
