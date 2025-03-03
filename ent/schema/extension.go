package schema

import "entgo.io/ent"

// Extension holds the schema definition for the Extension entity.
type Extension struct {
	ent.Schema
}

// Fields of the Extension.
func (Extension) Fields() []ent.Field {
	return nil
}

// Edges of the Extension.
func (Extension) Edges() []ent.Edge {
	return nil
}
