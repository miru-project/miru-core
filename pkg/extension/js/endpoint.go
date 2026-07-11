package js

import (
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/pkg/torrent"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

func GetExtensionMeta(pkg string) (*Ext, error) {
	api, err := getPkgFromCache(pkg)
	if err != nil {
		return nil, err
	}
	return api.Ext, nil
}

// Extension latest should contain V1 and V2 api
func Latest[T any](pkg string, page int) ([]*T, error) {
	api, e := getPkgFromCache(pkg)
	if e != nil {
		return nil, e
	}
	res, err := api.asyncCallBack(api, pkg, fmt.Sprintf(api.latestEval, page))
	if err != nil {
		return nil, err
	}
	return extension.UnmarshalList[T](res)
}

// Extension search should contain V1 and V2 api
func Search[T proto.ExtensionListItem](pkg string, page int, kw string, filter string) ([]*T, error) {
	api, e := getPkgFromCache(pkg)
	if e != nil {
		return nil, e
	}
	res, err := api.asyncCallBack(api, pkg, fmt.Sprintf(api.searchEval, kw, page, filter))
	if err != nil {
		return nil, err
	}
	return extension.UnmarshalList[T](res)
}

// handleMediaType handles magnet and torrent links for bangumi type
func handleMediaType(api *ExtApi, pkg string, o any) (any, error) {
	if api.Ext.WatchType != "bangumi" {
		return o, nil
	}
	obj, ok := o.(map[string]any)
	if !ok {
		return o, nil
	}
	vidType, ok := obj["type"].(string)
	if !ok {
		return obj, nil
	}

	switch vidType {
	case "magnet":
		urlInterface, ok := obj["url"]
		if !ok {
			return obj, nil
		}
		link, ok := urlInterface.(string)
		if !ok {
			return obj, nil
		}
		t, e := torrent.AddMagnet(link, "", pkg)
		if e != nil {
			return nil, e
		}
		obj["torrent"] = t
		return obj, nil

	case "torrent":
		urlInterface, ok := obj["url"]
		if !ok {
			return obj, nil
		}
		link, ok := urlInterface.(string)
		if !ok {
			return obj, nil
		}
		if oPath, _ := url.Parse(link); !oPath.IsAbs() {
			web, _ := url.Parse(api.Ext.Website)
			web.Path = filepath.Join(web.Path, link)
			link = web.String()
		}
		t, e := torrent.AddTorrent(link, "", pkg)
		if e != nil {
			return nil, e
		}
		obj["torrent"] = t
		return obj, nil

	default:
		return obj, nil
	}
}

// Extension watch should contain V1 and V2 api
func Watch(pkg string, watchLink string) (any, *extension.Extension, error) {
	api, e := getPkgFromCache(pkg)
	if e != nil {
		return nil, nil, e
	}

	o, e := api.asyncCallBack(api, pkg, fmt.Sprintf(api.watchEval, watchLink))
	if e != nil {
		return nil, nil, e
	}

	switch api.Ext.ApiVersion {
	case "2":
		return o, api.Ext, nil
	default:
		resolved, err := handleMediaType(api, pkg, o)
		return resolved, api.Ext, err
	}
}

func Detail[T proto.ExtensionDetail](pkg string, url string) (*T, error) {
	api, e := getPkgFromCache(pkg)
	if e != nil {
		return nil, e
	}
	res, err := api.asyncCallBack(api, pkg, fmt.Sprintf(api.detailEval, url))
	if err != nil {
		return nil, err
	}
	return extension.Unmarshal[T](res)
}

func Mirror(pkg string, watchUrl string) (any, error) {
	api, e := getPkgFromCache(pkg)
	if e != nil {
		return "", e
	}
	res, err := api.asyncCallBack(api, pkg, fmt.Sprintf(api.mirrorEval, watchUrl))
	if err != nil {
		return "", err
	}
	return handleMediaType(api, pkg, res)
}

// Unmarshal is retained as a package-level helper (delegating to the shared
// runtime-agnostic implementation) for backwards compatibility with callers
// inside this package and tests.
func Unmarshal[T any](input any) (*T, error) {
	return extension.Unmarshal[T](input)
}

// UnmarshalList is the list variant of Unmarshal.
func UnmarshalList[T any](input any) ([]*T, error) {
	return extension.UnmarshalList[T](input)
}

func CreateFilter(pkg string, filter string) (map[string]*proto.ExtensionFilter, error) {
	api, e := getPkgFromCache(pkg)
	if e != nil {
		return nil, e
	}
	if filter == "" {
		filter = "null"
	}
	res, err := api.asyncCallBack(api, pkg, fmt.Sprintf(api.createFilterEval, filter))
	if err != nil {
		return nil, err
	}
	decoded, err := Unmarshal[map[string]*proto.ExtensionFilter](res)
	if err != nil {
		return nil, err
	}
	return *decoded, nil
}
