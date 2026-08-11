package torrent

import (
	"github.com/anacrolix/torrent/metainfo"
	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/miru-project/miru-core/pkg/result"
)

// Resolve downloads (or fetches the metainfo of) a magnet:/torrent link and
// converts the resolved result into the proto shape the gRPC Watch/Mirror
// responses (and the frontend) expect. It is the single shared implementation
// used by BOTH the JavaScript and the Go/Scriggo extension runtimes, so that a
// torrent resolved by either runtime yields an identical ExtensionBangumiWatchTorrent
// (info hash, metainfo detail, and the directory file tree) for the frontend to
// read and pick files from.
//
// The per-media `type` field is an enum limited to "magnet" / "torrent" -- any
// other value is the caller's responsibility (handled before reaching here).
func Resolve(link string, pkg string) (*proto.ExtensionBangumiWatchTorrent, error) {
	var (
		res result.TorrentDetailResult
		err error
	)
	switch {
	case len(link) >= 7 && link[:7] == "magnet:":
		res, err = AddMagnet(link, "", pkg)
	default:
		res, err = AddTorrent(link, "", pkg)
	}
	if err != nil {
		return nil, err
	}
	return toProtoBangumiTorrent(res), nil
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
