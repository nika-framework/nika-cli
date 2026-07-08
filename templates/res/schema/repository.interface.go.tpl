package schema

import (
	"github.com/nika-framework/nika/common/mongodb/repository"
)

type I{{.TypeName}}Repository interface {
	repository.IBaseRepository[{{.TypeName}}]
}
