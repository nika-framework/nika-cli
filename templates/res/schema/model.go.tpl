package schema

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type {{.TypeName}} struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	{{- range .Fields}}
	{{.Name}}  {{.Type}} `{{.ModelTag}}`
	{{- end}}
	CreatedAt time.Time `bson:"created_at,omitempty" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at,omitempty" json:"updated_at"`
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
