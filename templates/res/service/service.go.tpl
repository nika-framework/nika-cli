package services

import (
	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/schema"
)

type {{.TypeName}}Service struct {
	repo schema.I{{.TypeName}}Repository
}

func New{{.TypeName}}Service(
	repo schema.I{{.TypeName}}Repository,
) *{{.TypeName}}Service {
	return &{{.TypeName}}Service{repo: repo}
}
