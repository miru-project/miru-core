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
	if api.Ext.WatchType != "bangumi" {
		return o, nil, api
	}
	obj, ok := o.(map[string]any)
	if !ok {
		return nil, errors.New("Malformed watch response"), api
	}
	vidType := obj["type"].(string)
	switch vidType {

	case "magnet":
		link := obj["url"].(string)
		t, e := torrent.AddMagnet(link, "", pkg)
		if e != nil {
			return nil, e, api
		}
		obj["torrent"] = t
		return obj, nil, api

	case "torrent":
		link := obj["url"].(string)
		if o, _ := url.Parse(link); !o.IsAbs() {
			web, _ := url.Parse(api.Ext.Website)
			web.Path = filepath.Join(web.Path, link)
			link = web.String()
		}
		t, e := torrent.AddTorrent(link, "", pkg)
		if e != nil {
			return nil, e, api
		}
		obj["torrent"] = t
		return obj, nil, api

	default:
		return obj, nil, api
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
