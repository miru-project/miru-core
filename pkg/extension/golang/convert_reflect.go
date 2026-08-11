package golang

import (
	"reflect"
	"strings"
)

// ---------------------------------------------------------------------------
// Generic reflection helpers shared by every script-value converter.
//
// Extensions are written in a Go subset and return plain structs whose types
// are defined inside the compiled program; they are not the generated proto
// types. These helpers read those values by field name, matching
// case-insensitively so the script's "URL" maps onto proto's "Url".
// ---------------------------------------------------------------------------

func fieldByName(rv reflect.Value, name string) reflect.Value {
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return reflect.Value{}
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	lower := strings.ToLower(name)
	t := rv.Type()
	for i := 0; i < t.NumField(); i++ {
		if strings.ToLower(t.Field(i).Name) == lower {
			return rv.Field(i)
		}
	}
	return reflect.Value{}
}

func strField(rv reflect.Value, name string) string {
	f := fieldByName(rv, name)
	if !f.IsValid() || f.Kind() != reflect.String {
		return ""
	}
	return f.String()
}

func anyField(rv reflect.Value, name string) any {
	f := fieldByName(rv, name)
	if !f.IsValid() {
		return nil
	}
	return f.Interface()
}

// mapStrField reads a map[string]string field by name (e.g. a mirror's
// Headers). Templates return nil when the field is absent, so extensions that
// don't set headers simply produce an empty map.
func mapStrField(rv reflect.Value, name string) map[string]string {
	f := fieldByName(rv, name)
	if !f.IsValid() || f.Kind() != reflect.Map {
		return nil
	}
	out := make(map[string]string, f.Len())
	for _, k := range f.MapKeys() {
		if k.Kind() != reflect.String {
			continue
		}
		v := f.MapIndex(k)
		if v.Kind() == reflect.String {
			out[k.String()] = v.String()
		}
	}
	return out
}

// strSliceField reads a []string field by name (e.g. a manga watch's URLs).
// Templates return nil when the field is absent, so extensions that don't set
// the slice simply produce an empty slice.
func strSliceField(rv reflect.Value, name string) []string {
	f := fieldByName(rv, name)
	if !f.IsValid() || f.Kind() != reflect.Slice {
		return nil
	}
	out := make([]string, 0, f.Len())
	for i := 0; i < f.Len(); i++ {
		if s := f.Index(i); s.Kind() == reflect.String {
			out = append(out, s.String())
		}
	}
	return out
}

// optionalStrField reads a string field by name and returns it as a *string, or
// nil when the field is absent or empty. proto marks these fields `optional`, so
// an empty value must be omitted rather than sent as "".
func optionalStrField(rv reflect.Value, name string) *string {
	f := fieldByName(rv, name)
	if !f.IsValid() || f.Kind() != reflect.String || f.String() == "" {
		return nil
	}
	s := f.String()
	return &s
}

// derefStruct normalises a possibly-pointer/interface value to its underlying
// struct Value, returning an invalid Value when the value is nil or not a
// struct. It is the shared prelude for the per-type watch converters.
func derefStruct(v any) reflect.Value {
	if v == nil {
		return reflect.Value{}
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return reflect.Value{}
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	return rv
}
