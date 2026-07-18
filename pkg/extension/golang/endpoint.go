package golang

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/pkg/extension/golang/runtime"
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
// pick a response shape.
//
// The Golang backend is not locked to a single watch shape: it routes the watch
// result to the right proto message by the *concrete struct the extension
// returned*. A legacy extension that returns the V2 ExtensionWatch shape stays
// on the generic V2 path, while an extension that returns one of the per-type
// shapes (manga/fikushon/bangumi/all) gets that oneof variant. This lets Golang
// support the per-type watches (and the combined "all" type) without breaking
// extensions that still emit the V2 ExtensionWatch.
func Watch(pkg string, url string) (any, *extension.Extension, error) {
	res, err := callExtension(pkg, "Watch", pkg, url)
	if err != nil {
		return nil, nil, err
	}
	meta, err := GetExtensionMeta(pkg)
	if err != nil {
		return nil, nil, err
	}
	// Mirror the JavaScript handleMediaType: for bangumi (or the bangumi member
	// of an "all" extension) resolve a magnet:/torrent URL into a Torrent and
	// attach it so the frontend can read the file tree. The extension may also
	// have resolved one itself (via sdk.AddMagnet / sdk.AddTorrent); in that
	// case we leave its Torrent untouched.
	resolveBangumiTorrent(pkg, meta, res)
	return toWatchValue(res), meta, nil
}

// resolveBangumiTorrent attaches a resolved Torrent to any bangumi-shaped watch
// result whose URL is a magnet:/torrent link and that does not already carry a
// Torrent. It handles both a top-level *ExtensionBangumiWatch and the bangumi
// member of a *ExtensionAllWatch.
func resolveBangumiTorrent(pkg string, meta *extension.Extension, res any) {
	if res == nil {
		return
	}
	rv := reflect.ValueOf(res)
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return
	}
	switch rv.Type().Name() {
	case "ExtensionBangumiWatch":
		attachTorrentIfNeeded(pkg, meta, rv)
	case "ExtensionAllWatch":
		bangumi := fieldByName(rv, "Bangumi")
		if bangumi.IsValid() && bangumi.Kind() == reflect.Ptr && !bangumi.IsNil() {
			attachTorrentIfNeeded(pkg, meta, bangumi.Elem())
		}
	}
}

// attachTorrentIfNeeded resolves a magnet:/torrent URL on a bangumi watch struct
// and sets its Torrent field when the field is currently nil.
func attachTorrentIfNeeded(pkg string, meta *extension.Extension, rw reflect.Value) {
	torrentField := fieldByName(rw, "Torrent")
	if torrentField.IsValid() && !torrentField.IsNil() {
		return // author already resolved one
	}
	urlField := fieldByName(rw, "URL")
	if !urlField.IsValid() || urlField.Kind() != reflect.String {
		return
	}
	link := urlField.String()
	if !isTorrentLink(link) {
		return
	}
	website := ""
	if meta != nil {
		website = meta.Website
	}
	var (
		t   runtime.Torrent
		err string
	)
	if strings.HasPrefix(link, "magnet:") {
		t, err = runtime.AddMagnet(link, "", pkg)
	} else {
		t, err = runtime.AddTorrent(link, "", pkg, website)
	}
	if err != "" {
		return // leave the raw link for the frontend to resolve
	}
	if torrentField.IsValid() && torrentField.CanSet() {
		torrentField.Set(reflect.ValueOf(&t))
	}
}

// isTorrentLink reports whether a watch URL refers to a torrent resource that
// the host should resolve: a magnet: link or a .torrent file URL.
func isTorrentLink(link string) bool {
	if link == "" {
		return false
	}
	if strings.HasPrefix(link, "magnet:") {
		return true
	}
	return strings.HasSuffix(strings.ToLower(link), ".torrent")
}

// toWatchValue converts a script Watch return value into the appropriate proto
// watch message. It inspects the concrete (script) type the extension returned:
//
//   - *ExtensionMangaWatch    -> proto.ExtensionMangaWatch
//   - *ExtensionFikushonWatch -> proto.ExtensionFikushonWatch
//   - *ExtensionBangumiWatch  -> proto.ExtensionBangumiWatch
//   - *ExtensionAllWatch      -> proto.ExtensionAllWatch
//   - anything else (incl. the V2 *ExtensionWatch) -> proto.ExtensionWatch
//
// The match is by the script type's name so the generic V2 shape keeps using
// the legacy V2 conversion while the per-type shapes get their own.
func toWatchValue(res any) any {
	if res == nil {
		return toWatch(res)
	}
	rv := reflect.ValueOf(res)
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return toWatch(res)
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return toWatch(res)
	}
	switch rv.Type().Name() {
	case "ExtensionMangaWatch":
		return toMangaWatch(res)
	case "ExtensionFikushonWatch":
		return toFikushonWatch(res)
	case "ExtensionBangumiWatch":
		return toBangumiWatch(res)
	case "ExtensionAllWatch":
		return toAllWatch(res)
	default:
		return toWatch(res)
	}
}

// Mirror returns mirror/alternative URLs for a content item.
func Mirror(pkg string, url string) (any, error) {
	res, err := callExtension(pkg, "Mirror", pkg, url)
	if err != nil {
		return nil, err
	}
	return toMirrors(res), nil
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

// int64Field reads an int-like field by name and returns it as int64 (0 when
// absent or non-numeric).
func int64Field(rv reflect.Value, name string) int64 {
	f := fieldByName(rv, name)
	if !f.IsValid() || !f.CanInt() && !f.CanUint() {
		return 0
	}
	return f.Int()
}

// optInt32Field reads an int-like field by name and returns it as a *int32, or
// nil when absent. proto marks these fields `optional`.
func optInt32Field(rv reflect.Value, name string) *int32 {
	f := fieldByName(rv, name)
	if !f.IsValid() || !f.CanInt() && !f.CanUint() {
		return nil
	}
	v := int32(f.Int())
	return &v
}

// optInt64Field reads an int-like field by name and returns it as a *int64, or
// nil when absent. proto marks these fields `optional`.
func optInt64Field(rv reflect.Value, name string) *int64 {
	f := fieldByName(rv, name)
	if !f.IsValid() || !f.CanInt() && !f.CanUint() {
		return nil
	}
	v := f.Int()
	return &v
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

// toMangaWatch converts a script ExtensionMangaWatch into the proto equivalent.
func toMangaWatch(v any) *proto.ExtensionMangaWatch {
	rv := derefStruct(v)
	if !rv.IsValid() {
		return nil
	}
	return &proto.ExtensionMangaWatch{
		Urls:    strSliceField(rv, "URLs"),
		Headers: mapStrField(rv, "Headers"),
	}
}

// toFikushonWatch converts a script ExtensionFikushonWatch into the proto
// equivalent.
func toFikushonWatch(v any) *proto.ExtensionFikushonWatch {
	rv := derefStruct(v)
	if !rv.IsValid() {
		return nil
	}
	return &proto.ExtensionFikushonWatch{
		Content:  strSliceField(rv, "Content"),
		Title:    strField(rv, "Title"),
		Subtitle: optionalStrField(rv, "Subtitle"),
	}
}

// toBangumiWatch converts a script ExtensionBangumiWatch into the proto
// equivalent, descending into its subtitles. Torrent resolution is a frontend
// concern, so no torrent metadata is carried here.
func toBangumiWatch(v any) *proto.ExtensionBangumiWatch {
	rv := derefStruct(v)
	if !rv.IsValid() {
		return nil
	}
	return &proto.ExtensionBangumiWatch{
		Type:       strField(rv, "Type"),
		Url:        strField(rv, "URL"),
		Subtitles:  toBangumiSubtitles(anyField(rv, "Subtitles")),
		Headers:    mapStrField(rv, "Headers"),
		AudioTrack: optionalStrField(rv, "AudioTrack"),
		Torrent:    toProtoTorrent(anyField(rv, "Torrent")),
	}
}

// toProtoTorrent converts a runtime Torrent (as returned by sdk.AddMagnet /
// sdk.AddTorrent, or auto-resolved by the host) into the proto
// ExtensionBangumiWatchTorrent the gRPC WatchResponse carries to the frontend.
func toProtoTorrent(v any) *proto.ExtensionBangumiWatchTorrent {
	rv := derefStruct(v)
	if !rv.IsValid() {
		return nil
	}
	out := &proto.ExtensionBangumiWatchTorrent{
		InfoHash: strField(rv, "InfoHash"),
		Files:    strSliceField(rv, "Files"),
	}
	if detail := toProtoTorrentDetail(anyField(rv, "Detail")); detail != nil {
		out.Detail = detail
	}
	return out
}

func toProtoTorrentDetail(v any) *proto.ExtensionBangumiWatchTorrentDetail {
	rv := derefStruct(v)
	if !rv.IsValid() {
		return nil
	}
	detail := &proto.ExtensionBangumiWatchTorrentDetail{
		PieceLength: optInt32Field(rv, "PieceLength"),
		Pieces:      optionalStrField(rv, "Pieces"),
		Name:        optionalStrField(rv, "Name"),
		NameUtf8:    optionalStrField(rv, "NameUtf8"),
		Length:      optInt64Field(rv, "Length"),
		Source:      optionalStrField(rv, "Source"),
		MetaVersion: optInt32Field(rv, "MetaVersion"),
	}
	if tree := toProtoFileTree(anyField(rv, "FileTree")); tree != nil {
		detail.FileTree = tree
	}
	return detail
}

func toProtoFileTree(v any) *proto.ExtensionBangumiWatchTorrentFileTree {
	rv := derefStruct(v)
	if !rv.IsValid() {
		return nil
	}
	tree := &proto.ExtensionBangumiWatchTorrentFileTree{}
	if file := toProtoFileTreeFile(anyField(rv, "File")); file != nil {
		tree.File = file
	}
	if dir := anyField(rv, "Dir"); dir != nil {
		dirRV := reflect.ValueOf(dir)
		if dirRV.Kind() == reflect.Map && dirRV.Type().Key().Kind() == reflect.String {
			out := make(map[string]*proto.ExtensionBangumiWatchTorrentFileTree, dirRV.Len())
			for _, key := range dirRV.MapKeys() {
				child := toProtoFileTree(dirRV.MapIndex(key).Interface())
				if child != nil {
					out[key.String()] = child
				}
			}
			tree.Dir = out
		}
	}
	return tree
}

func toProtoFileTreeFile(v any) *proto.ExtensionBangumiWatchTorrentFileTreeFile {
	rv := derefStruct(v)
	if !rv.IsValid() {
		return nil
	}
	return &proto.ExtensionBangumiWatchTorrentFileTreeFile{
		Length:     int64Field(rv, "Length"),
		PiecesRoot: strField(rv, "PiecesRoot"),
	}
}

func toBangumiSubtitles(v any) []*proto.ExtensionBangumiWatchSubtitle {
	rv := derefStruct(v)
	if !rv.IsValid() || rv.Kind() != reflect.Slice {
		return nil
	}
	out := make([]*proto.ExtensionBangumiWatchSubtitle, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem := derefStruct(rv.Index(i).Interface())
		if !elem.IsValid() {
			continue
		}
		out = append(out, &proto.ExtensionBangumiWatchSubtitle{
			Language: optionalStrField(elem, "Language"),
			Title:    strField(elem, "Title"),
			Url:      strField(elem, "URL"),
		})
	}
	return out
}

// toAllWatch converts a script ExtensionAllWatch into the proto equivalent,
// delegating each member to its own converter.
func toAllWatch(v any) *proto.ExtensionAllWatch {
	rv := derefStruct(v)
	if !rv.IsValid() {
		return nil
	}
	return &proto.ExtensionAllWatch{
		Manga:    toMangaWatch(anyField(rv, "Manga")),
		Fikushon: toFikushonWatch(anyField(rv, "Fikushon")),
		Bangumi:  toBangumiWatch(anyField(rv, "Bangumi")),
	}
}
