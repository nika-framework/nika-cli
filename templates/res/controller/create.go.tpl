package controllers

import (
	"fmt"
	"{{.ModulePath}}/src/{{.ModuleName}}/dto"
	res "{{.ModulePath}}/src/{{.ModuleName}}/response"

	"github.com/gin-gonic/gin"
	"github.com/nika-framework/nika/common/response"
	"github.com/nika-framework/nika/common/validator"
)

// Create a {{.TypeName}} godoc
//
// @Summary Create a {{.TypeName}}
// @Description Create a {{.TypeName}}
// @Tags {{.TypeName}}
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.Create{{.TypeName}}Dto true "{{.TypeName}} Data"
// @Success 201 {object} res.CreateResponse
// @Failure 400 {object} response.Error "Error code"
// @Failure 422 {object} response.Error "Validation error"
// @Router /api/{{.ModuleName}}s [post]
func (c *{{.TypeName}}Controller) CreateHandler(
	ctx *gin.Context,
) {
	var body dto.Create{{.TypeName}}Dto
	if !validator.BindAndValidate(ctx, &body) {
		return
	}
	user, err := c.service.Create(ctx,body)
	if err != nil {
		response.BadRequest(ctx, "CREATION_FAILED", err.Error())
		return
	}
	response.Create(
		ctx,
		res.CreateResponse{
			Success: true,
			Message:   "CREATE_SUCCESS_FULL",
			Data:    res.ToUserResponse(user),  
		},
	)
}

