package services

import (
	"context"

	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/dto"
	"github.com/nika-framework/nika/common/mongodb/repository"
)
 

func (s *{{.TypeName}}Service) Find(ctx context.Context, input dto.List{{.TypeName}}Dto) (*repository.PaginationResult, error) {
	return s.repo.Pages(ctx, nil, input.Page, input.Count)
}
