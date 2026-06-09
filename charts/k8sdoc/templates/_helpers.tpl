{{/*
Expand the name of the chart.
*/}}
{{- define "k8sdoc.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "k8sdoc.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{- define "k8sdoc.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "k8sdoc.labels" -}}
helm.sh/chart: {{ include "k8sdoc.chart" . }}
{{ include "k8sdoc.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "k8sdoc.selectorLabels" -}}
app.kubernetes.io/name: {{ include "k8sdoc.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "k8sdoc.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "k8sdoc.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/* Ollama service URL — auto-resolves in-cluster when ollama.enabled */}}
{{- define "k8sdoc.ollamaURL" -}}
{{- if .Values.k8sdoc.ollamaBaseURL }}
{{- .Values.k8sdoc.ollamaBaseURL }}
{{- else if .Values.ollama.enabled }}
{{- printf "http://%s-ollama:11434" (include "k8sdoc.fullname" .) }}
{{- else }}
http://localhost:11434
{{- end }}
{{- end }}
