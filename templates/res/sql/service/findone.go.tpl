package services

import (
	"context"

	"{{.ModulePath}}/src/{{.ModuleName}}/dto"
	"{{.ModulePath}}/src/{{.ModuleName}}/schema"
)

func (s *{{.TypeName}}Service) FindOne(ctx context.Context, input dto.FindOne{{.TypeName}}Dto) (*schema.{{.TypeName}}, error) {
	return s.repo.FindOneByID(ctx, input.ID)
}