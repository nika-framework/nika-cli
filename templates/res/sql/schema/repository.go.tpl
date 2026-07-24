package schema

import (
	"github.com/nika-framework/nika/common/sqldb"
	"github.com/nika-framework/nika/common/sqldb/repository"
)

type {{.TypeName}}Repository struct {
	*repository.BaseRepository[{{.TypeName}}, int64]
}

var _ I{{.TypeName}}Repository = (*{{.TypeName}}Repository)(nil)

func New{{.TypeName}}Repository(db *sqldb.DB) I{{.TypeName}}Repository {
	return &{{.TypeName}}Repository{
		BaseRepository: repository.NewBaseRepositoryWithDialect[{{.TypeName}}, int64](
			db.Conn,
			repository.Dialect(db.Driver()),
			"{{.TableName}}",
			"id",
			true,
		),
	}
}