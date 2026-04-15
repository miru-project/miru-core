package jsExtension

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/go-viper/mapstructure/v2"
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

func Unmarshal[T any](input any) (*T, error) {
	var result T
	config := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   &result,
		TagName:  "json",
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return nil, err
	}
	err = decoder.Decode(input)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func UnmarshalList[T any](input any) ([]*T, error) {
	items, ok := input.([]any)
	if !ok {
		return nil, errors.New("input is not a list")
	}
	result := make([]*T, len(items))
	for i, item := range items {
		u, err := Unmarshal[T](item)
		if err != nil {
			return nil, err
		}
		result[i] = u
	}
	return result, nil
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
	return UnmarshalList[T](res)
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
	return UnmarshalList[T](res)
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
func Watch(pkg string, watchLink string) (any, error, *ExtApi) {
	api, e := getPkgFromCache(pkg)
	if e != nil {
		return nil, e, nil
	}

	o, e := api.asyncCallBack(api, pkg, fmt.Sprintf(api.watchEval, watchLink))
	if e != nil {
		return nil, e, api
	}

	switch api.Ext.ApiVersion {
	case "1":
		resolved, err := handleMediaType(api, pkg, o)
		return resolved, err, api
	// Every extension will be treat as V2 if the `@ApiVersion` is not "1"
	default:
		return o, nil, api
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
	return Unmarshal[T](res)
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
