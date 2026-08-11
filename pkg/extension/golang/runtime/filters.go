package runtime

import (
	"github.com/miru-project/miru-core/proto/generate/proto"
)

// FilterOption pairs a stable option key with a display label. Authors
// typically write a key (e.g. "1080p") that the host sends back unchanged
// in a FilterSelection, and a label (e.g. "1080p" or "Full HD") that the
// UI shows to the user.
type FilterOption struct {
	Label string
}

// SelectFilter is a single-value filter: the user picks exactly one option.
// Default is the pre-selected option key ("" if no default).
//
// Use SelectFilter for radio-button-style choices (one value, fixed list).
type SelectFilter struct {
	Title   string
	Default string
	Options map[string]FilterOption
}

// MultiSelectFilter is a multi-value filter: the user picks between Min and
// Max options (inclusive). Default is the list of pre-selected option keys.
//
// Use MultiSelectFilter for chip/badge-style choices (e.g. genres, tags,
// quality levels, where the user can pick any combination).
type MultiSelectFilter struct {
	Title   string
	Min     int32
	Max     int32
	Default []string
	Options map[string]FilterOption
}

// RangeFilter is a numeric range filter with inclusive Min/Max bounds and
// separate DefaultMin/DefaultMax. There is no Options map; the UI typically
// renders a slider or two number inputs.
//
// Use RangeFilter for numeric bounds (e.g. year range, rating range).
type RangeFilter struct {
	Title                  string
	Min, Max               int32
	DefaultMin, DefaultMax int32
}

// FilterDefinition is the typed union of the three filter variants. Exactly
// one of its three fields is non-nil; consumers switch on which one.
//
// Concrete shapes (SelectFilter / MultiSelectFilter / RangeFilter) are what
// an extension's CreateFilter entry point returns. The host converts each
// to the matching proto.ExtensionFilter oneof variant.
type FilterDefinition struct {
	Select      *SelectFilter
	MultiSelect *MultiSelectFilter
	Range       *RangeFilter
}

// Title returns the human-readable title of whichever variant is set.
func (f FilterDefinition) Title() string {
	switch {
	case f.Select != nil:
		return f.Select.Title
	case f.MultiSelect != nil:
		return f.MultiSelect.Title
	case f.Range != nil:
		return f.Range.Title
	}
	return ""
}

// SelectFilterBuilder provides a fluent API to construct SelectFilter without the
// verbose map[string]FilterOption{...} literal. Each Option call adds one
// entry; Build returns the SelectFilter inside a FilterDefinition.
type SelectFilterBuilder struct {
	f SelectFilter
}

// NewSelect starts building a SelectFilter with the given title and (optional)
// default key. Default of "" means no pre-selection.
//
//	sdk.NewSelect("Type", "all").
//	    Option("all",     "All").
//	    Option("manga",   "Manga").
//	    Option("bangumi", "Anime").
//	    Build()
func NewSelect(title, def string) *SelectFilterBuilder {
	return &SelectFilterBuilder{f: SelectFilter{Title: title, Default: def}}
}

// Option adds a key/label pair to the SelectFilter. Calling Option again
// with the same key overwrites the previous label.
func (b *SelectFilterBuilder) Option(key, label string) *SelectFilterBuilder {
	if b.f.Options == nil {
		b.f.Options = make(map[string]FilterOption)
	}
	b.f.Options[key] = FilterOption{Label: label}
	return b
}

// Build returns the constructed SelectFilter wrapped in a FilterDefinition.
func (b *SelectFilterBuilder) Build() FilterDefinition {
	return FilterDefinition{Select: &b.f}
}

// MultiSelectBuilder builds a MultiSelectFilter.
type MultiSelectBuilder struct {
	f MultiSelectFilter
}

// NewMultiSelect starts building a MultiSelectFilter with the given title,
// Min/Max selection bounds, and pre-selected defaults.
//
//	sdk.NewMultiSelect("Quality", 1, 4).
//	    Option("1080p", "1080p").
//	    Option("720p",  "720p").
//	    Default("1080p").
//	    Build()
func NewMultiSelect(title string, min, max int32, defaults ...string) *MultiSelectBuilder {
	return &MultiSelectBuilder{f: MultiSelectFilter{
		Title:   title,
		Min:     min,
		Max:     max,
		Default: append([]string(nil), defaults...),
	}}
}

// Option adds a key/label pair to the MultiSelectFilter.
func (b *MultiSelectBuilder) Option(key, label string) *MultiSelectBuilder {
	if b.f.Options == nil {
		b.f.Options = make(map[string]FilterOption)
	}
	b.f.Options[key] = FilterOption{Label: label}
	return b
}

// Build returns the constructed MultiSelectFilter wrapped in a FilterDefinition.
func (b *MultiSelectBuilder) Build() FilterDefinition {
	return FilterDefinition{MultiSelect: &b.f}
}

// RangeBuilder builds a RangeFilter.
type RangeBuilder struct {
	f RangeFilter
}

// NewRange starts building a RangeFilter with the given title, Min/Max bounds,
// and DefaultMin/DefaultMax pre-selected values.
//
//	sdk.NewRange("Year", 1990, 2025, 2000, 2025).Build()
func NewRange(title string, min, max, defMin, defMax int32) *RangeBuilder {
	return &RangeBuilder{f: RangeFilter{
		Title: title, Min: min, Max: max,
		DefaultMin: defMin, DefaultMax: defMax,
	}}
}

// Build returns the constructed RangeFilter wrapped in a FilterDefinition.
func (b *RangeBuilder) Build() FilterDefinition {
	return FilterDefinition{Range: &b.f}
}

// FilterDefinitionToProto converts a FilterDefinition into the proto oneof
// representation used on the wire (proto.ExtensionFilter). It is the single
// place the typed filter variants become the protobuf message; the endpoint
// calls this for every entry returned by an extension's CreateFilter.
func FilterDefinitionToProto(fd FilterDefinition) *proto.ExtensionFilter {
	out := &proto.ExtensionFilter{}
	switch {
	case fd.Select != nil:
		s := fd.Select
		opts := map[string]*proto.FilterOption{}
		for k, v := range s.Options {
			opts[k] = &proto.FilterOption{Label: v.Label}
		}
		out.Kind = &proto.ExtensionFilter_Select{
			Select: &proto.SelectFilter{
				Title:   s.Title,
				Default: s.Default,
				Options: opts,
			},
		}
	case fd.MultiSelect != nil:
		m := fd.MultiSelect
		opts := map[string]*proto.FilterOption{}
		for k, v := range m.Options {
			opts[k] = &proto.FilterOption{Label: v.Label}
		}
		out.Kind = &proto.ExtensionFilter_MultiSelect{
			MultiSelect: &proto.MultiSelectFilter{
				Title:   m.Title,
				Min:     m.Min,
				Max:     m.Max,
				Default: m.Default,
				Options: opts,
			},
		}
	case fd.Range != nil:
		r := fd.Range
		out.Kind = &proto.ExtensionFilter_Range{
			Range: &proto.RangeFilter{
				Title:      r.Title,
				Min:        r.Min,
				Max:        r.Max,
				DefaultMin: r.DefaultMin,
				DefaultMax: r.DefaultMax,
			},
		}
	}
	return out
}