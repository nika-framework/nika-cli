package controllers

import ( 
	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/dto" 

	"github.com/gin-gonic/gin"
	"github.com/nika-framework/nika/common/response"
	"github.com/nika-framework/nika/common/validator"
)

 
// Update {{.TypeName}} godoc
//
// @Summary Update {{.TypeName}}
// @Description Update {{.TypeName}} for authentication
// @Tags {{.TypeName}}s
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.Update{{.TypeName}}Dto true "{{.TypeName}} Data"
// @Param request path dto.FindOne{{.TypeName}}Dto true "{{.TypeName}} ID"
// @Success 202 {object} response.Error "Success response"
// @Failure 400 {object} response.Error "Error code"
// @Failure 404 {object} response.Error "NotFound error"
// @Failure 422 {object} response.Error "Validation error"
// @Router /api/{{.ModuleName}}s/{id} [patch]
func (c *{{.TypeName}}Controller) UpdateHandler(ctx *gin.Context) {
	var body dto.Update{{.TypeName}}Dto
	var param dto.FindOne{{.TypeName}}Dto
	if !validator.BindAndValidate(ctx, &body) {
		return
	}
	if !validator.BindAndValidateUri(ctx, &param) {
		return
	}

	{{.ModuleName}}, err := c.service.Update(ctx, param, body)
	if err != nil {
		response.BadRequest(ctx, "FIND_FAILED", err.Error())
		return
	}

	if {{.ModuleName}} != nil {
		response.Ok(ctx, response.BoolResponse{
			Success: true,
			Message: "UPDATE_SUUCESS",
		})
		return
	}

	response.NotFoundRequest(ctx, "FIND_FAILED", "Not_Found")

}