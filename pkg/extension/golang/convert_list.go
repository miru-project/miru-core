package golang

import (
	"reflect"

	"github.com/miru-project/miru-core/proto/generate/proto"
)

// ---------------------------------------------------------------------------
// Converters for the list-shaped extension results: search/latest list items,
// detail pages (with their episode groups) and the mirror groups a V2 watch
// returns. The few structural differences between the script and proto type
// families (for example the script's ExtensionDetail.Chapters and
// ExtensionEpisodeGroup.URLs) are handled explicitly here.
// ---------------------------------------------------------------------------

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
