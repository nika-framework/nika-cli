package controllers

import (
	"github.com/gin-gonic/gin" 
	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/services" 
)

type {{.TypeName}}Controller struct {
	service  *services.{{.TypeName}}Service

	Create         func(*gin.Context) `route:"POST:/{{.ModuleName}}s"`
	FindOne        func(*gin.Context) `route:"GET:/{{.ModuleName}}s/:id"`
	Find           func(*gin.Context) `route:"GET:/{{.ModuleName}}s"`
	Update         func(*gin.Context) `route:"PATCH:/{{.ModuleName}}s/:id"`
	Delete         func(*gin.Context) `route:"DELETE:/{{.ModuleName}}s/:id"`
}

func New{{.TypeName}}Controller(
	service *services.{{.TypeName}}Service, 
) *{{.TypeName}}Controller {
	c := &{{.TypeName}}Controller{
		service:  service, 
	}
	c.Create = c.CreateHandler
	c.FindOne = c.FindOneHandler
	c.Find = c.FindHandler
	c.Update = c.UpdateHandler
	c.Delete = c.DeleteHandler
	return c
}
