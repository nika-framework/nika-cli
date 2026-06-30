package services

import (
	"{{.ModulePath}}/src/{{.ModuleName}}/schema"
)

type {{.TypeName}}CreateService struct {
	repo schema.I{{.TypeName}}Repository
}

func New{{.TypeName}}CreateService(
	repo schema.I{{.TypeName}}Repository,
) *{{.TypeName}}CreateService {
	return &{{.TypeName}}CreateService{repo: repo}
}
