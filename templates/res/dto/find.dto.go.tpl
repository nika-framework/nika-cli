package dto

type List{{.TypeName}}Dto struct {
	Page  int64  `form:"page" validate:"omitempty,min=0"`
	Count int64  `form:"count" validate:"omitempty,oneof=5 10 15 25 50 100"`
	Query string `form:"query" validate:"omitempty,max=100"`
}
