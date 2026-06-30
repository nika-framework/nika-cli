package services

import (
	"context"

	"{{.ModulePath}}/src/{{.ModuleName}}/dto"
	"{{.ModulePath}}/src/{{.ModuleName}}/schema"
	"github.com/sajadweb/nika/common/mongodb/repository"
)

type {{.TypeName}}DeleteService struct {
	repo schema.I{{.TypeName}}Repository
}

func New{{.TypeName}}DeleteService(
	repo schema.I{{.TypeName}}Repository,
) *{{.TypeName}}DeleteService {
	return &{{.TypeName}}DeleteService{repo: repo}
}

func (s *{{.TypeName}}DeleteService) Run(ctx context.Context, input *dto.FindOne{{.TypeName}}Dto) error {
	id, err := repository.ParseObjectID(input.ID)
	if err != nil {
		return err
	}
	return s.repo.DeleteByID(ctx, id)
}
