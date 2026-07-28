package services

import (
	"context"

	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/dto"
	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/schema"
	"github.com/nika-framework/nika/common/sqldb/repository"
)

func (s *{{.TypeName}}Service) Find(ctx context.Context, input dto.List{{.TypeName}}Dto) (*repository.PaginationResult[schema.{{.TypeName}}], error) {
	return s.repo.Pages(ctx, nil, input.Page, input.Count)
}