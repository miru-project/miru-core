package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Tracker holds the schema definition for the Tracker entity.
type Tracker struct {
	ent.Schema
}

// Fields of the Tracker.
func (Tracker) Fields() []ent.Field {
	return []ent.Field{
		field.String("tracker_id").
			NotEmpty().
			Comment("The external tracker ID"),

		field.Enum("provider").
			Values("anilist", "myanimelist", "kitsu", "tmdb").
			Comment("The provider of the tracker"),

		field.String("status").
			NotEmpty().
			Comment("Status of the tracking"),

		field.Int("score").
			Optional().
			Nillable().
			Comment("Score given by the user"),

		field.Int("progress").
			Comment("Progress of the media"),

		field.Int("total_progress").
			Optional().
			Nillable().
			Comment("Total progress of the media (total episodes/chapters)"),

		field.Int64("start_date").
			Optional().
			Nillable().
			Comment("Start date in unix timestamp"),

		field.Int64("finish_date").
			Optional().
			Nillable().
			Comment("Finish date in unix timestamp"),
	}
}

// Edges of the Tracker.
func (Tracker) Edges() []ent.Edge {
	return []ent.Edge{
		// Details are the masters
		edge.From("details", Detail.Type).
			Ref("trackers"),
	}
}

// Indexes of the Tracker.
func (Tracker) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tracker_id", "provider").
			Unique(),
	}
}
