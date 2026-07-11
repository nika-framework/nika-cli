package response

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type {{.TypeName}}Response struct {
	ID        primitive.ObjectID `json:"id"`
	{{- range .Fields}}
	{{.Name}}  {{.Type}} `json:"{{.BsonName}}"`
	{{- end}}
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}
type CreateResponse struct {
	Success bool         `json:"success"`
	Message string       `json:"message"`
	Data    {{.TypeName}}Response `json:"data"`
}

type ListResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Data    []{{.TypeName}}Response `json:"data"`
	Meta    Meta           `json:"meta"`
}

type Meta struct {
	Total int64 `json:"total"`
	Page  int64 `json:"page"`
	Count int64 `json:"count"`
}
