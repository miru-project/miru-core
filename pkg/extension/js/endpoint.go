package js

import (
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/pkg/result"
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

// Extension watch should contain V1 and V2 api.
//
// The extension returns the watch payload as the script produced it (e.g. a
// stream URL, or a magnet/torrent URL as a plain string). For bangumi
// extensions, handleMediaType resolves any magnet:/torrent link into a
// downloadable handle and attaches it as `torrent` so the frontend can read the
// file tree and decide which files to download -- the same behaviour as the
// Golang/Scriggo runtime. Non-torrent payloads are passed through unchanged.
func Watch(pkg string, watchLink string) (any, *extension.Extension, error) {
	api, e := getPkgFromCache(pkg)
	if e != nil {
		return nil, nil, e
	}

	o, e := api.asyncCallBack(api, pkg, fmt.Sprintf(api.watchEval, watchLink))
	if e != nil {
		return nil, nil, e
	}

	o, e = handleMediaType(api, pkg, o)
	if e != nil {
		return nil, nil, e
	}

	return o, api.Ext, nil
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
	// For bangumi extensions, resolve any magnet:/torrent mirror link into a
	// downloadable handle and attach it as `torrent`, exactly like Watch.
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

// handleMediaType resolves magnet:/torrent links embedded in a watch (v1) or
// mirror (v2) payload for bangumi extensions, mirroring the JavaScript runtime's
// handleMediaType behaviour and the Golang/Scriggo extension logic: it fetches the
// JSON the extension produced, parses it, resolves the torrent into a downloadable
// handle, and attaches it as `torrent` so the frontend can read the file tree and
// decide which files to download.
//
// The per-media `type` field is an enum limited to "magnet" / "torrent" -- any
// other value (mp4, hls, ...) is passed through untouched. The resolved torrent
// is shaped as *proto.ExtensionBangumiWatchTorrent so it round-trips through the
// shared mapstructure Unmarshal used by the gRPC Watch path and matches the
// Golang runtime's output exactly.
func handleMediaType(api *ExtApi, pkg string, o any) (any, error) {
	if api.Ext.WatchType != extension.WatchTypeBangumi {
		return o, nil
	}

	// A mirror (v2) may return an array of mirror objects; a watch (v1) a
	// single object. Process whichever shape we receive.
	if list, ok := o.([]any); ok {
		for _, item := range list {
			if m, ok := item.(map[string]any); ok {
				if _, err := resolveMediaType(api, pkg, m); err != nil {
					return nil, err
				}
			}
		}
		return o, nil
	}

	if obj, ok := o.(map[string]any); ok {
		return resolveMediaType(api, pkg, obj)
	}
	return o, nil
}

// resolveMediaType resolves the torrent on a single watch/mirror object in place
// and returns it. It is the faithful port of the JavaScript handleMediaType
// switch: only the "magnet" and "torrent" type values trigger resolution.
func resolveMediaType(api *ExtApi, pkg string, obj map[string]any) (any, error) {
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
		obj["torrent"] = toProtoBangumiTorrent(t)
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
		obj["torrent"] = toProtoBangumiTorrent(t)
		return obj, nil

	default:
		return obj, nil
	}
}

// toProtoBangumiTorrent converts a resolved torrent result into the proto
// ExtensionBangumiWatchTorrent shape the gRPC WatchResponse (and the frontend)
// expects, rebuilding the directory file tree from the metainfo so the frontend can
// pick which files to download.
func toProtoBangumiTorrent(res result.TorrentDetailResult) *proto.ExtensionBangumiWatchTorrent {
	out := &proto.ExtensionBangumiWatchTorrent{
		InfoHash: res.InfoHash,
		Files:    res.Files,
	}
	if detail := toProtoBangumiTorrentDetail(res.Detail); detail != nil {
		out.Detail = detail
	}
	return out
}

func toProtoBangumiTorrentDetail(info *metainfo.Info) *proto.ExtensionBangumiWatchTorrentDetail {
	if info == nil {
		return nil
	}
	d := &proto.ExtensionBangumiWatchTorrentDetail{
		PieceLength: optInt32(int32(info.PieceLength)),
		Pieces:      optStr(string(info.Pieces)),
		Name:        optStr(info.Name),
		NameUtf8:    optStr(info.NameUtf8),
	}
	if info.Length != 0 {
		d.Length = optInt64(info.Length)
	}
	if info.Source != "" {
		d.Source = optStr(info.Source)
	}
	if info.MetaVersion != 0 {
		d.MetaVersion = optInt32(int32(info.MetaVersion))
	}
	d.FileTree = toProtoBangumiFileTree(info)
	return d
}

// toProtoBangumiFileTree rebuilds the nested file tree from metainfo.Info.Files.
// A node may carry both a File (leaf) and a Dir (children), matching BEP 52
// torrents where a file can also have sub-directories.
func toProtoBangumiFileTree(info *metainfo.Info) *proto.ExtensionBangumiWatchTorrentFileTree {
	root := &proto.ExtensionBangumiWatchTorrentFileTree{}
	if info == nil {
		return root
	}
	for i := range info.Files {
		fi := &info.Files[i]
		segments := fi.BestPath()
		if len(segments) == 0 {
			continue
		}
		node := root
		for _, seg := range segments[:len(segments)-1] {
			if node.Dir == nil {
				node.Dir = make(map[string]*proto.ExtensionBangumiWatchTorrentFileTree)
			}
			child, ok := node.Dir[seg]
			if !ok {
				child = &proto.ExtensionBangumiWatchTorrentFileTree{}
				node.Dir[seg] = child
			}
			node = child
		}
		leaf := &proto.ExtensionBangumiWatchTorrentFileTreeFile{
			Length:     fi.Length,
			PiecesRoot: fi.PiecesRoot.String(),
		}
		if node.File != nil {
			// File already present at this path (unusual); keep it and also
			// register the leaf under a child node so nothing is lost.
			if node.Dir == nil {
				node.Dir = make(map[string]*proto.ExtensionBangumiWatchTorrentFileTree)
			}
			node.Dir[segments[len(segments)-1]] = &proto.ExtensionBangumiWatchTorrentFileTree{File: leaf}
		} else {
			node.File = leaf
		}
	}
	return root
}

func optStr(s string) *string { return &s }

func optInt32(v int32) *int32 { return &v }

func optInt64(v int64) *int64 { return &v }
