{{/* vim: set filetype=mustache: */}}

{{/*
Return the proper image name
*/}}
{{- define "ingressh.image" -}}
{{- $imageRoot := .Values.image -}}
{{- if not .Values.image.tag }}
    {{- $tag := (dict "tag" .Chart.AppVersion) -}}
    {{- $imageRoot := merge .Values.image $tag -}}
{{- end -}}
{{- include "common.images.image" (dict "imageRoot" $imageRoot "global" .Values.global) -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "ingressh.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
    {{ default (include "common.names.fullname" .) .Values.serviceAccount.name | trunc 63 | trimSuffix "-" }}
{{- else -}}
    {{ default "default" .Values.serviceAccount.name | trunc 63 | trimSuffix "-" }}
{{- end -}}
{{- end -}}
