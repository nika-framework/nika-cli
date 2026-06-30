package services

import (
	"context"

	"{{.ModulePath}}/src/{{.ModuleName}}/dto"
	"{{.ModulePath}}/src/{{.ModuleName}}/schema"
	"github.com/sajadweb/nika/common/mongodb/repository"
)

type {{.TypeName}}FindOneService struct {
	repo schema.I{{.TypeName}}Repository
}

func New{{.TypeName}}FindOneService(
	repo schema.I{{.TypeName}}Repository,
) *{{.TypeName}}FindOneService {
	return &{{.TypeName}}FindOneService{repo: repo}
}

func (s *{{.TypeName}}FindOneService) Run(ctx context.Context, input *dto.FindOne{{.TypeName}}Dto) (*schema.{{.TypeName}}, error) {
	id, err := repository.ParseObjectID(input.ID)
	if err != nil {
		return nil, err
	}
	return s.repo.FindOneByID(ctx, id)
}
