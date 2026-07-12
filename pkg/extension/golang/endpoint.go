package golang

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

// ExtensionDir is the directory where Scriggo extensions are stored.
var ExtensionDir string

// ParseExtensionMetadata parses metadata from a Scriggo extension source file.
// It delegates to the shared, runtime-agnostic parser in the extension package.
func ParseExtensionMetadata(pkg string) (*extension.Extension, error) {
	extPath := filepath.Join(ExtensionDir, pkg+string(extension.LanguageGolang))
	source, err := os.ReadFile(extPath)
	if err != nil {
		return nil, fmt.Errorf("read extension source %s: %w", extPath, err)
	}
	return extension.ParseExtensionMetadata(string(source), pkg+string(extension.LanguageGolang))
}

// CreateFilter creates filter options for a package.
func CreateFilter(pkg string, filter string) (map[string]*proto.ExtensionFilter, error) {
	return nil, nil
}

// GetExtensionMeta returns the parsed metadata for a package.
func GetExtensionMeta(pkg string) (*extension.Extension, error) {
	return ParseExtensionMetadata(pkg)
}

// readExtensionPkgSource reads the source of an extension package from disk.
func readExtensionPkgSource(pkg string) ([]byte, error) {
	extPath := filepath.Join(ExtensionDir, pkg+".go")
	return os.ReadFile(extPath)
}

// callExtension compiles an extension package and invokes the named function by
// name using the local Scriggo build that supports calling a function by name.
//
// The extension source is compiled as-is: the package declaration, any imports
// and the // ==MiruExtension== metadata header are all valid Go that the
// compiler handles, so there is no need to strip anything. The first return
// value of the called function (index 0) is returned; if the function also
// returns an error (index 1) it is propagated to the caller.
func callExtension(pkg, fn string, args ...any) (any, error) {
	vm := NewScriggoVM(nil)
	src, err := readExtensionPkgSource(pkg)
	if err != nil {
		return nil, err
	}

	prog, err := vm.Compile(pkg+"_"+fn, string(src))
	if err != nil {
		return nil, fmt.Errorf("compile extension %s: %w", pkg, err)
	}

	res, err := prog.program.Call(fn, args...)
	if err != nil {
		return nil, withStackTrace("extension "+pkg+"."+fn, err)
	}
	if len(res) >= 2 {
		if e, ok := res[1].(error); ok && e != nil {
			return nil, e
		}
	}
	if len(res) == 0 {
		return nil, nil
	}
	return res[0], nil
}

// Search searches for content by keyword.
func Search(pkg string, page int, kw string, filter string) ([]*proto.ExtensionListItem, error) {
	res, err := callExtension(pkg, "Search", pkg, kw, page, filter)
	if err != nil {
		return nil, err
	}
	return toExtensionListItems(res), nil
}

// Latest returns the latest content for a package.
func Latest(pkg string, page int) ([]*proto.ExtensionListItem, error) {
	res, err := callExtension(pkg, "Latest", pkg, page)
	if err != nil {
		return nil, err
	}
	return toExtensionListItems(res), nil
}

// Detail returns detailed information about a content item.
func Detail(pkg string, url string) (*proto.ExtensionDetail, error) {
	res, err := callExtension(pkg, "Detail", pkg, url)
	if err != nil {
		return nil, err
	}
	return toDetail(res), nil
}

// Watch returns watch/stream information for a content item. The returned
// *extension.Extension carries the ApiVersion/WatchType used by callers to
// pick a response shape. The Golang backend is v2-only.
func Watch(pkg string, url string) (any, *extension.Extension, error) {
	res, err := callExtension(pkg, "Watch", pkg, url)
	if err != nil {
		return nil, nil, err
	}
	meta, err := GetExtensionMeta(pkg)
	if err != nil {
		return nil, nil, err
	}
	return toWatch(res), meta, nil
}

// Mirror returns mirror/alternative URLs for a content item.
func Mirror(pkg string, url string) (any, error) {
	res, err := callExtension(pkg, "Mirror", pkg, url)
	if err != nil {
		return nil, err
	}
	return toMirrors(res), nil
}

// SearchJSON searches for content and returns JSON-encoded results.
func SearchJSON(pkg string, page int, kw string, filter string) (string, error) {
	items, err := Search(pkg, page, kw, filter)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("marshal search results: %w", err)
	}
	return string(data), nil
}

// LatestJSON returns latest content as JSON.
func LatestJSON(pkg string, page int) (string, error) {
	items, err := Latest(pkg, page)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("marshal latest results: %w", err)
	}
	return string(data), nil
}

// DetailJSON returns detail as JSON.
func DetailJSON(pkg string, url string) (string, error) {
	d, err := Detail(pkg, url)
	if err != nil {
		return "", err
	}
	if d == nil {
		return "null", nil
	}
	data, err := json.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("marshal detail: %w", err)
	}
	return string(data), nil
}

// WatchJSON returns watch info as JSON.
func WatchJSON(pkg string, url string) (string, error) {
	w, _, err := Watch(pkg, url)
	if err != nil {
		return "", err
	}
	if w == nil {
		return "null", nil
	}
	data, err := json.Marshal(w)
	if err != nil {
		return "", fmt.Errorf("marshal watch: %w", err)
	}
	return string(data), nil
}

// MirrorJSON returns mirror info as JSON.
func MirrorJSON(pkg string, url string) (string, error) {
	m, err := Mirror(pkg, url)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("marshal mirrors: %w", err)
	}
	return string(data), nil
}

// ---------------------------------------------------------------------------
// Reflection-based conversion from script-defined extension return values to
// proto types.
//
// Extensions are written in a Go subset and return plain structs whose types
// are defined inside the compiled program; they are not the generated proto
// types. These converters read the returned values by field name and build the
// corresponding proto values. Field names are matched case-insensitively so
// that the script's "URL" maps onto proto's "Url", and the few structural
// differences between the two type families (for example the script's
// ExtensionDetail.Chapters and ExtensionEpisodeGroup.URLs) are handled
// explicitly below.
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

func toExtensionListItems(v any) []*proto.ExtensionListItem {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil
	}
	items := make([]*proto.ExtensionListItem, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		items = append(items, &proto.ExtensionListItem{
			Title:   strField(elem, "Title"),
			Url:     strField(elem, "URL"),
			Cover:   strField(elem, "Cover"),
			Update:  strField(elem, "Update"),
			Headers: mapStrField(elem, "Headers"),
		})
	}
	return items
}

func toEpisodeGroups(v any) []*proto.ExtensionEpisodeGroup {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil
	}
	groups := make([]*proto.ExtensionEpisodeGroup, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		urls := fieldByName(elem, "URLs")
		var episodes []*proto.ExtensionEpisode
		if urls.IsValid() && urls.Kind() == reflect.Slice {
			for j := 0; j < urls.Len(); j++ {
				u := urls.Index(j)
				// Extensions may supply episodes either as bare URL strings
				// (the original model, kept for backwards compatibility with
				// existing extensions such as example) or as structs carrying
				// both a Name and a URL. The latter is what makes episode text
				// appear in the UI; the former yields unlabelled episodes.
				if u.Kind() == reflect.String {
					episodes = append(episodes, &proto.ExtensionEpisode{Url: u.String()})
					continue
				}
				uv := u
				if uv.Kind() == reflect.Interface && !uv.IsNil() {
					uv = uv.Elem()
				}
				if uv.Kind() == reflect.Struct {
					episodes = append(episodes, &proto.ExtensionEpisode{
						Name: strField(uv, "Name"),
						Url:  strField(uv, "URL"),
					})
				}
			}
		}
		groups = append(groups, &proto.ExtensionEpisodeGroup{
			Title: strField(elem, "Title"),
			Urls:  episodes,
		})
	}
	return groups
}

func toDetail(v any) *proto.ExtensionDetail {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	title := strField(rv, "Title")
	cover := strField(rv, "Cover")
	desc := strField(rv, "Desc")
	if desc == "" {
		desc = strField(rv, "Description")
	}
	return &proto.ExtensionDetail{
		Title:    &title,
		Cover:    &cover,
		Desc:     &desc,
		Episodes: toEpisodeGroups(anyField(rv, "Chapters")),
		Headers:  mapStrField(rv, "Headers"),
	}
}

func toMirrorGroups(v any) []*proto.ExtensionMirrorGroup {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil
	}
	groups := make([]*proto.ExtensionMirrorGroup, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		groups = append(groups, &proto.ExtensionMirrorGroup{
			Title:   strField(elem, "Title"),
			Mirrors: toMirrors(anyField(elem, "Mirrors")),
		})
	}
	return groups
}

func toMirrors(v any) []*proto.ExtensionMirror {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil
	}
	mirrors := make([]*proto.ExtensionMirror, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		mirrors = append(mirrors, &proto.ExtensionMirror{
			Name:    strField(elem, "Name"),
			Url:     strField(elem, "URL"),
			Headers: mapStrField(elem, "Headers"),
		})
	}
	return mirrors
}

// toWatch converts a script ExtensionWatch into the proto equivalent. The script
// type carries its groups directly in the Groups field; when an extension only
// provides a top-level Title/URL (no groups), a single group is synthesised so
// the result is never empty, mirroring the previous behaviour.
func toWatch(v any) *proto.ExtensionWatch {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	groups := toMirrorGroups(anyField(rv, "Groups"))
	if len(groups) == 0 {
		title := strField(rv, "Title")
		url := strField(rv, "URL")
		if title != "" || url != "" {
			groups = []*proto.ExtensionMirrorGroup{
				{
					Title: title,
					Mirrors: []*proto.ExtensionMirror{
						{Name: title, Url: url},
					},
				},
			}
		}
	}
	return &proto.ExtensionWatch{Groups: groups}
}
