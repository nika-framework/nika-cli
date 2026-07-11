package services

import (
	"{{.ModulePath}}/src/{{.ModuleName}}/schema"
)

type {{.TypeName}}Service struct {
	repo schema.I{{.TypeName}}Repository
}

func New{{.TypeName}}Service(
	repo schema.I{{.TypeName}}Repository,
) *{{.TypeName}}Service {
	return &{{.TypeName}}Service{repo: repo}
}
