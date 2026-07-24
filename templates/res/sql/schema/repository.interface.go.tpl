package schema

import "github.com/nika-framework/nika/common/sqldb/repository"

type I{{.TypeName}}Repository interface {
	repository.IBaseRepository[{{.TypeName}}, int64]
}