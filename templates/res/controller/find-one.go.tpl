package controllers

import ( 
	"{{.ModulePath}}/src/{{.ModuleName}}/dto" 
	res "{{.ModulePath}}/src/{{.ModuleName}}/response"

	"github.com/gin-gonic/gin"
	"github.com/nika-framework/nika/common/response"
	"github.com/nika-framework/nika/common/validator"
)


// Find One {{.TypeName}} godoc
//
// @Summary Find a {{.TypeName}} By Id
// @Description Get {{.TypeName}} by ID
// @Tags {{.TypeName}}s
// @Produce json
// @Security BearerAuth
// @Param request path dto.FindOne{{.TypeName}}Dto true "{{.TypeName}} ID"
// @Success 200 {object} res.CreateResponse
// @Failure 400 {object} response.Error "Error code"
// @Failure 404 {object} response.Error "NotFound error"
// @Failure 422 {object} response.Error "Validation error"
// @Router /api/{{.ModuleName}}s/{id} [get]
func (c *{{.TypeName}}Controller) FindOneHandler(
	ctx *gin.Context,
) {
	var find{{.TypeName}}Dto dto.FindOne{{.TypeName}}Dto
	if !validator.BindAndValidateUri(ctx, &find{{.TypeName}}Dto) {
		return
	}
	
	{{.ModuleName}}, err := c.service.FindOne(ctx,find{{.TypeName}}Dto)
	if err != nil {
		response.BadRequest(ctx, "FIND_FAILED", err.Error())
		return
	}
	if {{.ModuleName}} == nil {
		response.NotFoundRequest(ctx, "FIND_FAILED", "NOT_FOND")
		return
	}

	response.Ok(
		ctx,
		res.CreateResponse{
			Success: true,
			Message: "FIND_SUCCESS",
			Data:    res.To{{.ModuleName}}Response({{.ModuleName}}),
		},
	)
}