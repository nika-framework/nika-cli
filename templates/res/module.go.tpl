package {{.ModuleName}}

import (
	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/controllers"
	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/schema"
	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/services"
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
		services.New{{.TypeName}}Service,
	}
}

func (m *{{.TypeName}}Module) Imports() []nika.Module {
	return []nika.Module{}
}

func (m *{{.TypeName}}Module) Exports() []interface{} {
	return []interface{}{}
}
