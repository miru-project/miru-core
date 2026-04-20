package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Track holds the schema definition for the Track entity.
type Track struct {
	ent.Schema
}

// Fields of the Track.
func (Track) Fields() []ent.Field {
	return []ent.Field{
		field.String("tracking_id").
			Comment("Tracking ID from provider"),
		field.Text("data").
			Comment("Track Detail JSON data"),
		field.String("media_type").
			Comment("Media Type (movie/tv)"),
		field.Enum("provider").
			Values("anilist", "tmdb", "myanimelist", "kitsu").
			Comment("Tracking Provider"),
	}
}

// Edges of the Track.
func (Track) Edges() []ent.Edge {
	return nil
}

// Indexes of the Track.
func (Track) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("provider", "tracking_id").Unique(),
	}
}
