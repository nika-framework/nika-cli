package dto

type FindOne{{.TypeName}}Dto struct {
	ID string `uri:"id" example:"6a345422b0d00690d26f8c15" validate:"required,objectid,len=24"`
}
