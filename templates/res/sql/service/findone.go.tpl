package services

import (
	"context"

	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/dto"
	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/entity"
)

func (s *{{.TypeName}}Service) FindOne(ctx context.Context, input dto.FindOne{{.TypeName}}Dto) (*entity.{{.TypeName}}, error) {
	return s.repo.FindOneByID(ctx, input.ID)
}