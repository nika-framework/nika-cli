package services

import (
	"context"
	"errors"
	"time"

	"{{.ModulePath}}/src/{{.ModuleName}}/dto"
	"{{.ModulePath}}/src/{{.ModuleName}}/schema"
	"github.com/nika-framework/nika/common/sqldb/repository"
)

func (s *{{.TypeName}}Service) Update(ctx context.Context, param dto.FindOne{{.TypeName}}Dto, data dto.Update{{.TypeName}}Dto) (*schema.{{.TypeName}}, error) {
	model, err := s.repo.FindOneByID(ctx, param.ID)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, errors.New("NOT_FOUND")
	}

	changes := repository.Filter{}
	{{- range .Fields}}
	if data.{{.Name}} != nil {
		changes["{{.ColumnName}}"] = *data.{{.Name}}
	}
	{{- end}}
	if len(changes) == 0 {
		return model, nil
	}
	changes["updated_at"] = time.Now().UTC()
	if err := s.repo.UpdateOneByID(ctx, param.ID, changes); err != nil {
		return nil, err
	}
	return s.repo.FindOneByID(ctx, param.ID)
}