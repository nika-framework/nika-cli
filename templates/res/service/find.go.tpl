package services

import (
	"context"

	"{{.ModulePath}}/src/{{.ModuleName}}/dto"
	"{{.ModulePath}}/src/{{.ModuleName}}/schema"
	"github.com/sajadweb/nika/common/mongodb/repository"
)

type {{.TypeName}}FindService struct {
	repo schema.I{{.TypeName}}Repository
}

func New{{.TypeName}}FindService(
	repo schema.I{{.TypeName}}Repository,
) *{{.TypeName}}FindService {
	return &{{.TypeName}}FindService{repo: repo}
}

func (s *{{.TypeName}}FindService) Run(ctx context.Context, input *dto.List{{.TypeName}}Dto) (*repository.PaginationResults, error) {
	return s.repo.Pages(ctx, nil, input.Page, input.Count)
}
