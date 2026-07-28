package services

import (
	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/entity"
)

type {{.TypeName}}Service struct {
	repo entity.I{{.TypeName}}Repository
}

func New{{.TypeName}}Service(
	repo entity.I{{.TypeName}}Repository,
) *{{.TypeName}}Service {
	return &{{.TypeName}}Service{repo: repo}
}
