package jsExtension

import (
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/miru-project/miru-core/pkg/torrent"
)

// Extension latest should contain V1 and V2 api
func Latest(pkg string, page int) (any, error) {
	api, e := getPkgFromCache(pkg)
	if e != nil {
		return nil, e
	}
	return api.asyncCallBack(api, pkg, fmt.Sprintf(api.latestEval, page))
}

// Extension search should contain V1 and V2 api
func Search(pkg string, page int, kw string, filter string) (any, error) {
	api, e := getPkgFromCache(pkg)
	if e != nil {
		return nil, e
	}
	return api.asyncCallBack(api, pkg, fmt.Sprintf(api.searchEval, kw, page, filter))

}

// Extension watch should contain V1 and V2 api
func Watch(pkg string, link string) (any, error) {

	api, e := getPkgFromCache(pkg)
	if e != nil {
		return nil, e
	}

	o, e := api.asyncCallBack(api, pkg, fmt.Sprintf(api.watchEval, link))
	if e != nil {
		return nil, e
	}
	if api.Ext.WatchType != "bangumi" {
		return o, nil
	}
	obj := o.(map[string]any)
	vidType := obj["type"].(string)
	switch vidType {

	case "magnet":
		t, e := torrent.AddMagnet(link)
		if e != nil {
			return nil, e
		}
		obj["torrent"] = t
		return obj, nil

	case "torrent":
		if o, _ := url.Parse(link); !o.IsAbs() {
			web, _ := url.Parse(api.Ext.Website)
			web.Path = filepath.Join(web.Path, link)
			link = web.String()
		}
		t, e := torrent.AddTorrent(link)
		if e != nil {
			return nil, e
		}
		obj["torrent"] = t
		return obj, nil

	default:
		return obj, nil
	}

}

func Detail(pkg string, url string) (any, error) {
	api, e := getPkgFromCache(pkg)
	if e != nil {
		return nil, e
	}
	return api.asyncCallBack(api, pkg, fmt.Sprintf(api.detailEval, url))
}
