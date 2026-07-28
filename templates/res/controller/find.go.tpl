package controllers

import ( 
	"{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/dto" 
	res "{{.ModulePath}}/{{.SrcImport}}/{{.ModuleName}}/response"

	"github.com/gin-gonic/gin"
	"github.com/nika-framework/nika/common/response"
	"github.com/nika-framework/nika/common/validator"
)


// List {{.TypeName}}  godoc
//
// @Summary List {{.TypeName}}s
// @Description Get list of all {{.TypeName}}s
// @Tags {{.TypeName}}s
// @Produce json
// @Security BearerAuth
// @Param request query dto.List{{.TypeName}}Dto true "Pagination and Search Query"
// @Success 200 {object} res.ListResponse
// @Failure 400 {object} response.Error "Error code"
// @Failure 422 {object} response.Error "Validation error"
// @Router /api/{{.ModuleName}}s [get]
func (c *{{.TypeName}}Controller) FindHandler(
	ctx *gin.Context,
) {

	var dto dto.List{{.TypeName}}Dto
	if !validator.BindAndValidateQuery(ctx, &dto) { 
		return
	}
	if dto.Count == 0 {
		dto.Count = 5
	}
	{{.ModuleName}}s, err := c.service.Find(ctx,dto)
	if err != nil {
		response.BadRequest(ctx, "LIST_FAILED", "")
		return
	}
	response.Ok(ctx, res.MapListToResponse({{.ModuleName}}s.Data, {{.ModuleName}}s.Total, dto.Page, dto.Count))
}
