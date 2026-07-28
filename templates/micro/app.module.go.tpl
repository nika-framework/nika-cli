package src

import (
	"github.com/nika-framework/nika"
)

// AppModule is the root module of the {{.AppName}} service.
//
// `nika g res <module> -a {{.AppName}}` appends generated modules to Imports()
// below, exactly as it does for an HTTP app — a microservice is the same module
// tree behind a different transport.
type AppModule struct{}

func NewAppModule() *AppModule {
	return &AppModule{}
}

func (m *AppModule) Controllers() []interface{} {
	return []interface{}{
		NewAppController(),
	}
}

func (m *AppModule) Providers() []interface{} {
	return []interface{}{}
}

func (m *AppModule) Exports() []interface{} {
	return []interface{}{}
}

func (m *AppModule) Imports() []nika.Module {
	return []nika.Module{}
}
