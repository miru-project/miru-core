package runtime

// FilterSelection holds the selected option keys for a single filter.
//
// This custom type wraps a typed []string so Go extension authors never deal
// with raw JSON, map[string]any, or proto types directly. The proto-to-Filter
// conversion lives in golang/endpoint.go (outside runtime to keep it proto-free).
type FilterSelection struct {
	// Values are the selected option keys (>= 1 for an active selection).
	Values []string
}

// Filter is the strongly-typed representation of the filter selection the
// frontend sends to Search / CreateFilter. It is a custom object (not a
// proto type, not a map[string]any) so extension authors get a concrete,
// documented type whose values are typed []string.
//
// It is keyed by the filter names declared in CreateFilter. Use the companion
// helper functions (HasSelection / FirstSelection / SelectionsOf) to read
// individual filters by name.
//
// Constructing a Filter:
//
//	f := runtime.NewFilterSelection().
//	    Select("type", "manga").
//	    SelectMany("language", "ja", "en").
//	    Build()
type Filter struct {
	// Selections maps a filter name (as returned by CreateFilter) to its
	// selected option keys.
	Selections map[string]FilterSelection
}

// FilterSelectionBuilder provides a fluent API to construct a Filter, avoiding
// verbose map[string]FilterSelection{…} literals.
type FilterSelectionBuilder struct {
	f Filter
}

// NewFilterSelection returns a FilterSelectionBuilder to construct a Filter
// with chained calls:
//
//	NewFilterSelection().Select("type", "manga").Build()
func NewFilterSelection() *FilterSelectionBuilder {
	return &FilterSelectionBuilder{f: Filter{Selections: make(map[string]FilterSelection)}}
}

// Select adds a single-value selection for name. Calling Select again
// overwrites the previous selection for that name.
func (b *FilterSelectionBuilder) Select(name string, value string) *FilterSelectionBuilder {
	b.f.Selections[name] = FilterSelection{Values: []string{value}}
	return b
}

// SelectMany adds a multi-value selection for name (0 or more values).
func (b *FilterSelectionBuilder) SelectMany(name string, values ...string) *FilterSelectionBuilder {
	b.f.Selections[name] = FilterSelection{Values: values}
	return b
}

// Build returns the constructed Filter.
func (b *FilterSelectionBuilder) Build() Filter {
	return b.f
}

// HasSelection reports whether the named filter has a non-empty selection.
func HasSelection(f Filter, name string) bool {
	s, ok := f.Selections[name]
	return ok && len(s.Values) > 0
}

// FirstSelection returns the first selected value for a filter ("" if unset).
// Use it for single-select filters (max == 1).
func FirstSelection(f Filter, name string) string {
	s, ok := f.Selections[name]
	if !ok || len(s.Values) == 0 {
		return ""
	}
	return s.Values[0]
}

// SelectionsOf returns the selected values for a filter (nil if unset).
// Use it for multi-select filters (max > 1).
func SelectionsOf(f Filter, name string) []string {
	if s, ok := f.Selections[name]; ok {
		return s.Values
	}
	return nil
}
