package {{.ModuleName}}

import (
	"{{.ModulePath}}/src/{{.ModuleName}}/controllers"
	"{{.ModulePath}}/src/{{.ModuleName}}/schema"
	"{{.ModulePath}}/src/{{.ModuleName}}/services"
	"github.com/nika-framework/nika"
)

type {{.TypeName}}Module struct{}

func New{{.TypeName}}Module() *{{.TypeName}}Module {
	return &{{.TypeName}}Module{}
}

func (m *{{.TypeName}}Module) Controllers() []interface{} {
	return []interface{}{
		controllers.New{{.TypeName}}Controller,
	}
}

func (m *{{.TypeName}}Module) Providers() []interface{} {
	return []interface{}{
		schema.New{{.TypeName}}Repository,
		services.New{{.TypeName}}CreateService,
		services.New{{.TypeName}}FindOneService,
		services.New{{.TypeName}}FindService,
		services.New{{.TypeName}}DeleteService,
	}
}

func (m *{{.TypeName}}Module) Imports() []nika.Module {
	return []nika.Module{}
}
