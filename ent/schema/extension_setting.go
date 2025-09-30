package schema

import (
	"context"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/miru-project/miru-core/ent/hook"
)

// ExtensionSetting holds the schema definition for the ExtensionSetting entity.
type ExtensionSetting struct {
	ent.Schema
}

// Fields of the ExtensionSetting.
func (ExtensionSetting) Fields() []ent.Field {
	return []ent.Field{
		field.String("package").
			NotEmpty().
			Comment("Package name of the extension"),
		field.String("title").
			NotEmpty().
			Comment("Title"),
		field.String("key").
			NotEmpty().
			Comment("Key"),
		field.String("value").
			Optional().
			Nillable().
			Comment("Value"),
		field.String("default_value").
			NotEmpty().
			Nillable().
			Default("").
			Comment("Default value"),
		field.Enum("db_type").
			Values("input", "radio", "toggle").
			Default("input").
			Comment("Type stored as string: input, radio, toggle"),
		field.String("description").
			Optional().
			Nillable().
			Comment("Description"),
		field.String("options").
			Optional().
			Nillable().
			Comment("Options (JSON or comma-separated)"),
	}
}

// Hooks ensures `value` falls back to `default_value` when nil.
func (ExtensionSetting) Hooks() []ent.Hook {
	return []ent.Hook{
		hook.On(func(next ent.Mutator) ent.Mutator {
			return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
				if v, ok := m.Field("value"); !ok || v == nil {
					if dv, ok := m.Field("default_value"); ok && dv != nil {
						m.SetField("value", dv)
					}
				}
				return next.Mutate(ctx, m)
			})
		}, ent.OpCreate|ent.OpUpdate),
	}
}

// Edges of the ExtensionSetting.
func (ExtensionSetting) Edges() []ent.Edge {
	return nil
}

// Indexes of the ExtensionSetting.
func (ExtensionSetting) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("package", "key").
			Unique().
			StorageKey("idx_extension_setting_package_key"),
	}
}
