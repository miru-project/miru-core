package golang

import (
	"reflect"

	"github.com/miru-project/miru-core/pkg/extension/golang/runtime"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

// filterFromProto converts the proto.FilterSelection sent by the frontend
// into the strongly-typed runtime.Filter struct. Extensions receive a real
// custom object (typed []string values), not a raw JSON string or a
// map[string]any.
//
// This function lives in the golang package (not runtime) so the runtime
// package stays free of proto imports. Extension authors import only the
// sdk package and never touch proto types directly.
func filterFromProto(p *proto.FilterSelection) runtime.Filter {
	if p == nil || len(p.Selections) == 0 {
		return runtime.Filter{}
	}
	f := runtime.Filter{Selections: make(map[string]runtime.FilterSelection, len(p.Selections))}
	for name, sv := range p.Selections {
		if sv == nil {
			continue
		}
		vs := make([]string, 0, len(sv.Values))
		for _, v := range sv.Values {
			if v != "" {
				vs = append(vs, v)
			}
		}
		if len(vs) > 0 {
			f.Selections[name] = runtime.FilterSelection{Values: vs}
		}
	}
	return f
}

// toExtensionFilters converts a script CreateFilter return value -- a
// map[string]runtime.FilterDefinition -- into the proto representation.
// Non-map results yield nil.
func toExtensionFilters(v any) map[string]*proto.ExtensionFilter {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Map {
		return nil
	}
	out := make(map[string]*proto.ExtensionFilter, rv.Len())
	for _, k := range rv.MapKeys() {
		if k.Kind() != reflect.String {
			continue
		}
		elem := rv.MapIndex(k)
		// The author returns runtime.FilterDefinition values; decode each
		// concrete variant into the proto oneof.
		if fd, ok := elem.Interface().(runtime.FilterDefinition); ok {
			out[k.String()] = runtime.FilterDefinitionToProto(fd)
			continue
		}
		// Fallback for raw struct shapes (rare): emit an empty filter.
		out[k.String()] = &proto.ExtensionFilter{}
	}
	return out
}
