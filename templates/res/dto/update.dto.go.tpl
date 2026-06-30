package dto

type Update{{.TypeName}}Dto struct {
	{{- range .Fields}}
	{{.Name}}  {{.Type}} `{{.UpdateTag}}`
	{{- end}}
}
