package response

import "{{.ModulePath}}/src/{{.ModuleName}}/schema"

func To{{.TypeName}}Response(item *schema.{{.TypeName}}) {{.TypeName}}Response {
	return {{.TypeName}}Response{
		ID: item.ID,
		{{- range .Fields}}
		{{.Name}}: item.{{.Name}},
		{{- end}}
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func MapListToResponse(data []schema.{{.TypeName}}, total int64, page int64, count int64) ListResponse {
	items := make([]{{.TypeName}}Response, 0, len(data))
	for i := range data {
		items = append(items, To{{.TypeName}}Response(&data[i]))
	}

	return ListResponse{
		Success: true,
		Message: "LIST_SUCCESS",
		Data:    items,
		Meta: Meta{
			Total: total,
			Page:  page,
			Count: count,
		},
	}
}