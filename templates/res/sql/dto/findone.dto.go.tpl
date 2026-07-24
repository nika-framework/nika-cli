package dto

type FindOne{{.TypeName}}Dto struct {
	ID int64 `uri:"id" example:"1" validate:"required,min=1"`
}