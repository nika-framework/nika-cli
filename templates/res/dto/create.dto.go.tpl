package dto

type Create{{.TypeName}}Dto struct {
	{{- range .Fields}}
	{{.Name}}  {{.Type}} `{{.CreateTag}}`
	{{- end}}
}
