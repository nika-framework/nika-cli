package services

import (
	"context"
	"errors"
	"time"

	"{{.ModulePath}}/src/{{.ModuleName}}/dto"
	"{{.ModulePath}}/src/{{.ModuleName}}/schema"
	"github.com/nika-framework/nika/common/mongodb/repository"
	"go.mongodb.org/mongo-driver/bson"
)

func (s *{{.TypeName}}Service) Update(ctx context.Context, param *dto.FindOne{{.TypeName}}Dto,  input *dto.Update{{.TypeName}}Dto) (*schema.{{.TypeName}}, error) {
	id, err := repository.ParseObjectID(param.ID)
	if err != nil {
		return nil, err
	}

	modelFind, _ :=s.repository.FindOneByID(ctx, id)
	if modelFind == nil {
		return nil, errors.New("NOT_FOUND")
	}

	model := bson.M{}
	{{- range .Fields}} 
	if data.{{.Name}} != nil { 
		model["{{.BsonName}}"] = *data.{{.Name}}
	} 
	{{- end}}
	model["updated_at"] = time.Now().UTC()
	err1 := s.repository.UpdateOneByID(ctx, id, model)
	if err1 != nil {
		return nil, err1
	}
	return modelFind, nil
}
