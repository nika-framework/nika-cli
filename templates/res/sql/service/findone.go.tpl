package services

import (
	"context"

	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/dto"
	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/schema"
)

func (s *{{.TypeName}}Service) FindOne(ctx context.Context, input dto.FindOne{{.TypeName}}Dto) (*schema.{{.TypeName}}, error) {
	return s.repo.FindOneByID(ctx, input.ID)
}