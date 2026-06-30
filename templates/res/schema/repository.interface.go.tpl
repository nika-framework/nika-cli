package schema

import (
	"github.com/sajadweb/nika/common/mongodb/repository"
)

type I{{.TypeName}}Repository interface {
	repository.IBaseRepository[{{.TypeName}}]
}
