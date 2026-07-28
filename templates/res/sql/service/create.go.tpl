package services

import (
	"context"

	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/dto"
	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/entity"
)

func (s *{{.TypeName}}Service) Create(ctx context.Context, input dto.Create{{.TypeName}}Dto) (*entity.{{.TypeName}}, error) {
	model := entity.New{{.TypeName}}()
	{{- range .Fields}} 
	model.{{.Name}} = input.{{.Name}}
	{{- end}}
	item,err := s.repo.Create(ctx, model);
	if  err != nil {
		return nil, err
	}
	return item, nil
}
