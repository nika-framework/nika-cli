package controllers

import (
	"github.com/gin-gonic/gin"
	"{{.ModulePath}}/src/{{.ModuleName}}/dto"
	"{{.ModulePath}}/src/{{.ModuleName}}/services"
	"github.com/sajadweb/nika/common/response"
	"github.com/sajadweb/nika/common/validator"
)

type {{.TypeName}}Controller struct {
	createService  *services.{{.TypeName}}CreateService
	findOneService *services.{{.TypeName}}FindOneService
	findService    *services.{{.TypeName}}FindService
	deleteService  *services.{{.TypeName}}DeleteService
	create         func(*gin.Context) `route:"POST:/{{.ModuleName}}s"`
	findOne        func(*gin.Context) `route:"GET:/{{.ModuleName}}s/:id"`
	find           func(*gin.Context) `route:"GET:/{{.ModuleName}}s"`
	delete         func(*gin.Context) `route:"DELETE:/{{.ModuleName}}s/:id"`
}

func New{{.TypeName}}Controller(
	createService *services.{{.TypeName}}CreateService,
	findOneService *services.{{.TypeName}}FindOneService,
	findService *services.{{.TypeName}}FindService,
	deleteService *services.{{.TypeName}}DeleteService,
) *{{.TypeName}}Controller {
	c := &{{.TypeName}}Controller{
		createService:  createService,
		findOneService: findOneService,
		findService:    findService,
		deleteService:  deleteService,
	}
	c.create = func(ctx *gin.Context) {
		var input dto.Create{{.TypeName}}Dto
		if !validator.BindAndValidate(ctx, &input) {
			return
		}
		result, err := c.createService.Run(ctx, &input)
		if err != nil {
			response.BadRequest(ctx, err.Error())
			return
		}
		response.Create(ctx, result)
	}
	c.findOne = func(ctx *gin.Context) {
		var input dto.FindOne{{.TypeName}}Dto
		if !validator.BindAndValidateUri(ctx, &input) {
			return
		}
		result, err := c.findOneService.Run(ctx, &input)
		if err != nil {
			response.NotFoundRequest(ctx, err.Error())
			return
		}
		response.Ok(ctx, result)
	}
	c.find = func(ctx *gin.Context) {
		var input dto.List{{.TypeName}}Dto
		if !validator.BindAndValidateQuery(ctx, &input) {
			return
		}
		result, err := c.findService.Run(ctx, &input)
		if err != nil {
			response.BadRequest(ctx, err.Error())
			return
		}
		response.Ok(ctx, result)
	}
	c.delete = func(ctx *gin.Context) {
		var input dto.FindOne{{.TypeName}}Dto
		if !validator.BindAndValidateUri(ctx, &input) {
			return
		}
		err := c.deleteService.Run(ctx, &input)
		if err != nil {
			response.NotFoundRequest(ctx, err.Error())
			return
		}
		response.OkByMsg(ctx, "deleted successfully")
	}
	return c
}
