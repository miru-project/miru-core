package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type ExtensionRepo struct {
	ent.Schema
}

// Fields of the ExtensionRepo
func (ExtensionRepo) Fields() []ent.Field {
	return []ent.Field{
		field.String("url").Unique().NotEmpty().Comment("The URL of the extension repository"),
		field.String("name").NotEmpty().Comment("The name of the extension repository"),
	}
}
