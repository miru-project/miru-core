package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Detail holds the schema definition for the Detail entity.
type Detail struct {
	ent.Schema
}

// Fields of the Detail.
func (Detail) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").
			Positive().
			Immutable().
			Comment("The ID of the detail record").
			StorageKey("id"),

		field.String("title").
			Optional().
			Nillable().
			NotEmpty().
			Comment("The title of the detail"),

		field.String("cover").
			Optional().
			Nillable().
			Comment("Cover image URL"),

		field.String("desc").
			Optional().
			Nillable().
			Comment("Description of the content"),

		field.String("detailUrl").
			NotEmpty().
			Comment("The Detail URL of the content"),

		field.String("package").
			NotEmpty().
			Comment("The package identifier"),

		field.JSON("downloaded", []string{}).
			Optional().
			Comment("List of downloaded episodes"),

		field.Text("episodes").
			Optional().
			Nillable().
			Comment("JSON encoded string of episodes"),

		field.Text("headers").
			Optional().
			Nillable().
			Comment("JSON encoded string of headers"),
	}
}

// Edges of the Detail.
func (Detail) Edges() []ent.Edge {
	return nil
}

// Indexes of the Detail.
func (Detail) Indexes() []ent.Index {
	return []ent.Index{
		// Create a unique index on the package and detailUrl field with replace on conflict strategy
		index.Fields("package", "detailUrl").
			Unique(),
	}
}
