package dto

type Update{{.TypeName}}Dto struct {
	{{- range .Fields}}
	{{.Name}} *{{.Type}} `json:"{{.ColumnName}}" validate:"omitempty"`
	{{- end}}
}