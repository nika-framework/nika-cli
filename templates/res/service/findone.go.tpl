package services

import (
	"context"

	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/dto"
	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/schema"
	"github.com/nika-framework/nika/common/mongodb/repository"
)

 
func (s *{{.TypeName}}Service) FindOne(ctx context.Context, input dto.FindOne{{.TypeName}}Dto) (*schema.{{.TypeName}}, error) {
	id, err := repository.ParseObjectID(input.ID)
	if err != nil {
		return nil, err
	}
	return s.repo.FindOneByID(ctx, id)
}
