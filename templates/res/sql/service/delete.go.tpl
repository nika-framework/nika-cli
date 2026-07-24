package services

import (
	"context"

	"{{.ModulePath}}/src/{{.ModuleName}}/dto"
)

func (s *{{.TypeName}}Service) Delete(ctx context.Context, input dto.FindOne{{.TypeName}}Dto) error {
	return s.repo.DeleteByID(ctx, input.ID)
}