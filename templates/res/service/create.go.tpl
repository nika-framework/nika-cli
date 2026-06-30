package services

import (
	"context"

	"{{.ModulePath}}/src/{{.ModuleName}}/dto"
	"{{.ModulePath}}/src/{{.ModuleName}}/schema"
)

func (s *{{.TypeName}}CreateService) Run(ctx context.Context, input *dto.Create{{.TypeName}}Dto) (*schema.{{.TypeName}}, error) {
	model := schema.New{{.TypeName}}()
	{{- range .Fields}}
	model.{{.Name}} = input.{{.Name}}
	{{- end}}
	if err := s.repo.Create(ctx, model); err != nil {
		return nil, err
	}
	return model, nil
}
