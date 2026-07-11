package controllers

import ( 
	"{{.ModulePath}}/src/{{.ModuleName}}/dto" 

	"github.com/gin-gonic/gin"
	"github.com/nika-framework/nika/common/response"
	"github.com/nika-framework/nika/common/validator"
)

// Delete {{.TypeName}} godoc
//
// @Summary Delete a {{.TypeName}}
// @Description Delete a {{.TypeName}} for authentication
// @Tags {{.TypeName}}
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request path dto.FindOne{{.TypeName}}Dto true "{{.TypeName}} ID"
// @Failure 400 {object} response.Error "BadRequest error"
// @Failure 404 {object} response.Error "NotFound error"
// @Failure 422 {object} response.Error "Validation error"
// @Router /api/{{.ModuleName}}/{id} [delete]
func (c *{{.TypeName}}Controller) DeleteHandler(
	ctx *gin.Context,
) {
	var param dto.FindOne{{.TypeName}}Dto
	if !validator.BindAndValidateUri(ctx, &param) {
		return
	}

	err := c.service.Delete(ctx, param)
	if err != nil {
		response.NotFoundRequest(ctx, "DELETE_FAILED", err.Error())
		return
	}
	
	response.Ok(ctx, response.BoolResponse{
		Success: true,
		Message: "DELETE_SUCCESS",
	})
}

