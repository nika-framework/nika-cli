package services

import (
	"context"

	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/dto"
	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/schema"
)

func (s *{{.TypeName}}Service) Create(ctx context.Context, input dto.Create{{.TypeName}}Dto) (*schema.{{.TypeName}}, error) {
	model := schema.New{{.TypeName}}()
	{{- range .Fields}} 
	model.{{.Name}} = input.{{.Name}}
	{{- end}}
	item,err := s.repo.Create(ctx, model);
	if  err != nil {
		return nil, err
	}
	return item, nil
}
