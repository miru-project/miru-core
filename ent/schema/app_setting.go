package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// AppSetting holds the schema definition for the AppSetting entity.
type AppSetting struct {
	ent.Schema
}

// Fields of the AppSetting.
func (AppSetting) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").
			NotEmpty().
			Unique().
			Comment("Key").
			StorageKey("key"), // Use "key" as the primary key in the database
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
	return nil // No additional indexes are needed since "key" is unique
}
