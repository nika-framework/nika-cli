package services

import (
	"context"

	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/dto"
	"github.com/nika-framework/nika/common/mongodb/repository"
)

 

func (s *{{.TypeName}}Service) Delete(ctx context.Context, input dto.FindOne{{.TypeName}}Dto) error {
	id, err := repository.ParseObjectID(input.ID)
	if err != nil {
		return err
	}
	return s.repo.DeleteByID(ctx, id)
}
