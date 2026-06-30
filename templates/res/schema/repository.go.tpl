package schema

import (
	"context"

	"github.com/sajadweb/nika/common/mongodb/repository"
	"go.mongodb.org/mongo-driver/mongo"
)

type {{.TypeName}}Repository struct {
	*repository.BaseRepository[{{.TypeName}}]
}

var _ I{{.TypeName}}Repository = (*{{.TypeName}}Repository)(nil)

func New{{.TypeName}}Repository(
	db *mongo.Database,
) I{{.TypeName}}Repository {
	return &{{.TypeName}}Repository{
		BaseRepository: repository.NewBaseRepository[{{.TypeName}}](
			db.Collection("{{.CollectionName}}"),
		),
	}
}
