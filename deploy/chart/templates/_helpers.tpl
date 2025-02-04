{{- define "cards-api.name" -}}
cards-api
{{- end -}}

{{- define "cards-api.labels" -}}
app.kubernetes.io/name: {{ include "cards-api.name" . }}
app.kubernetes.io/part-of: cauri
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
cauri.io/team: payments
{{- end -}}

{{- define "cards-api.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cards-api.name" . }}
{{- end -}}
