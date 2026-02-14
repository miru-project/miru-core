package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Download holds the schema definition for the Download entity.
type Download struct {
	ent.Schema
}

// Fields of the Download.
func (Download) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").
			Positive().
			Immutable().
			Comment("The ID of the download record").
			StorageKey("id"),
		field.Strings("url").
			Comment("List of download URLs"),
		field.String("watchUrl").
			NotEmpty().
			Comment("The source watch URL"),
		field.String("detailUrl").
			NotEmpty().
			Comment("The Detail URL of the content"),
		field.JSON("headers", map[string]string{}).
			Optional().
			Default(map[string]string{}).
			Comment("Download URL headers"),
		field.String("package").
			NotEmpty().
			Comment("The extension package identifier"),
		field.Ints("progress").
			Optional().
			Comment("List of segment progress"),
		field.String("key").
			Unique().
			NotEmpty().
			Comment("Unique identifier (info hash for torrent, initial URL for others)"),
		field.String("title").
			NotEmpty().
			Comment("Title of the content"),
		field.String("media_type").
			NotEmpty().
			Comment("Media type (hls, mp4, torrent)"),
		field.String("status").
			NotEmpty().
			Comment("Current status of the download"),
		field.String("save_path").
			Optional().
			Comment("Final save path of the content"),
		field.Time("date").
			Default(time.Now).
			Comment("Date when the download entry was created/updated"),
	}
}

// Edges of the Download.
func (Download) Edges() []ent.Edge {
	return nil
}

// Indexes of the Download.
func (Download) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("package", "watchUrl", "detailUrl").
			Unique(),
	}
}
