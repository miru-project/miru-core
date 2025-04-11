package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AppSetting holds the schema definition for the AppSetting entity.
type AppSetting struct {
	ent.Schema
}

// Fields of the AppSetting.
func (AppSetting) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").
			Positive().
			Immutable().
			StructTag(`json:"id,omitempty"`),
		field.String("key").
			NotEmpty().
			Comment("Key"),
		field.String("value").
			Comment("Value"),
	}
}

// Edges of the AppSetting.
func (AppSetting) Edges() []ent.Edge {
	return nil
}

// Indexes of the AppSetting.
func (AppSetting) Indexes() []ent.Index {
	return []ent.Index{
		// Create a unique index on key
		index.Fields("key").
			Unique(),
	}
}
