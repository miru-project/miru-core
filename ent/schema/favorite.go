package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Favorite holds the schema definition for the Favorite entity.
type Favorite struct {
	ent.Schema
}

// Fields of the Favorite.
func (Favorite) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").
			Positive().
			Comment("The ID of the favorite"),

		field.String("package").
			NotEmpty().
			Comment("The package identifier"),

		field.String("url").
			NotEmpty().
			Comment("The URL of the content"),

		field.String("type").
			NotEmpty().
			Comment("Type of content"),

		field.String("title").
			NotEmpty().
			Comment("Title of the content"),

		field.String("cover").
			Optional().
			Nillable().
			Comment("Cover image URL"),

		field.Time("date").
			Default(time.Now).
			Comment("Date when the favorite was created/updated"),
	}
}

// Edges of the Favorite.
func (Favorite) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("group", FavoriteGroup.Type).
			Ref("favorites"),
	}
}

// Indexes of the Favorite.
func (Favorite) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("package", "url").
			Unique(),
	}
}
