package response

import "time"

type {{.TypeName}}Response struct {
	ID        int64 `json:"id"`
	{{- range .Fields}}
	{{.Name}} {{.Type}} `json:"{{.ColumnName}}"`
	{{- end}}
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateResponse struct {
	Success bool               `json:"success"`
	Message string             `json:"message"`
	Data    {{.TypeName}}Response `json:"data"`
}

type ListResponse struct {
	Success bool                 `json:"success"`
	Message string               `json:"message"`
	Data    []{{.TypeName}}Response `json:"data"`
	Meta    Meta                 `json:"meta"`
}

type Meta struct {
	Total int64 `json:"total"`
	Page  int64 `json:"page"`
	Count int64 `json:"count"`
}