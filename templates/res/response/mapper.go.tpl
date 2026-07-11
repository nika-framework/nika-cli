package response

import (
	"{{.ModulePath}}/src/{{.ModuleName}}/schema"
	"github.com/nika-framework/nika/common/mongodb/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func To{{.TypeName}}Response(
	item *schema.{{.TypeName}},
) {{.TypeName}}Response {
	return {{.TypeName}}Response{
		ID: item.ID,
{{- range .Fields}}
		{{.Name}}: item.{{.Name}},
{{- end}}
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func MapListToResponse(data []map[string]any, total int64, page int64, count int64) ListResponse {
	{{.ModuleName}}s := make([]{{.TypeName}}Response, 0, len(data))
	for _, item := range data {
		var id primitive.ObjectID
		if oid, ok := item["_id"].(primitive.ObjectID); ok {
			id = oid
		}

		{{.ModuleName}}s = append({{.ModuleName}}s, {{.TypeName}}Response{
			ID: id,
{{- range .Fields}}
			{{.Name}}: repository.GetSafeString(item, "{{.BsonName}}"),
{{- end}}
			CreatedAt: repository.GetSafeDate(item, "created_at"),
			UpdatedAt: repository.GetSafeDate(item, "updated_at"),
		})
	}

	return ListResponse{
		Success: true,
		Message: "LIST_SUCCESS",
		Data:    {{.ModuleName}}s,
		Meta: Meta{
			Total: total,
			Page:  page,
			Count: count,
		},
	}
}