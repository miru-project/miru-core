package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// History holds the schema definition for the History entity.
type History struct {
	ent.Schema
}

// Fields of the History.
func (History) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").
			Positive().
			Immutable().
			Comment("The ID of the history record").
			StorageKey("id"),

		field.String("package").
			NotEmpty().
			Comment("The package identifier"),

		field.String("url").
			NotEmpty().
			Comment("The URL of the content"),

		field.String("cover").
			Optional().
			Nillable().
			Comment("Cover image URL"),

		field.String("type").
			NotEmpty().
			Comment("Type of content (stored as string representation of enum)"),

		field.Int("episode_group_id").
			Comment("ID of the episode group"),

		field.Int("episode_id").
			Comment("ID of the episode"),

		field.String("title").
			NotEmpty().
			Comment("Title of the content"),

		field.String("episode_title").
			NotEmpty().
			Comment("Title of the episode"),

		field.String("progress").
			NotEmpty().
			Comment("Current progress in the content"),

		field.String("total_progress").
			NotEmpty().
			Comment("Total progress available"),

		field.Time("date").
			Default(time.Now).
			Comment("Date when the history entry was created/updated"),
	}
}

// Edges of the History.
func (History) Edges() []ent.Edge {
	return nil
}

// Indexes of the History.
func (History) Indexes() []ent.Index {
	return []ent.Index{
		// Create a unique index on the package field with replace on conflict strategy
		index.Fields("package").
			Unique(),
	}
}
