package schema

import (
	"context"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/miru-project/miru-core/ent/hook"
)

type ExtensionRepoSetting struct {
	ent.Schema
}

// Fields of the ExtensionRepo
func (ExtensionRepoSetting) Fields() []ent.Field {
	return []ent.Field{
		field.String("link").Unique().NotEmpty().Comment("The URL of the extension repository"),
		field.String("name").NotEmpty().Comment("The name of the extension repository"),
	}
}

func (ExtensionRepoSetting) Hooks() []ent.Hook {
	return []ent.Hook{
		hook.On(func(next ent.Mutator) ent.Mutator {
			return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
				return next.Mutate(ctx, m)

			})
		}, ent.OpCreate|ent.OpUpdate),
	}
}

// func (ExtensionRepoSetting) Indexes() []ent.Index {
// 	return []ent.Index{
// 		index.Fields("url").
// 			Unique().
// 			StorageKey("idx_extension_repo_url"),
// 	}
// }
