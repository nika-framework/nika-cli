package entity

import "time"

type {{.TypeName}} struct {
	ID        int64 `db:"id" json:"id"`
	{{- range .Fields}}
	{{.Name}} {{.Type}} `{{.ModelTag}}`
	{{- end}}
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

func New{{.TypeName}}() *{{.TypeName}} {
	now := time.Now().UTC()
	return &{{.TypeName}}{
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (m *{{.TypeName}}) TouchUpdated() {
	m.UpdatedAt = time.Now().UTC()
}