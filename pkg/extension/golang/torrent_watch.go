package golang

import (
	"github.com/miru-project/miru-core/pkg/torrent"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

// resolveBangumiTorrent resolves a magnet:/torrent mirror link into the proto
// ExtensionBangumiWatchTorrent the frontend reads to render the file tree. It is
// a thin wrapper over the torrent pkg's shared Resolve, which is the SAME
// implementation the JavaScript runtime uses -- so a torrent resolved by either
// runtime yields an identical payload (info hash, metainfo detail, file tree).
//
// The var indirection (instead of calling torrent.Resolve directly) exists only
// to let unit tests substitute a fake resolver that returns a canned
// TorrentDetailResult without performing a real network download.
var resolveBangumiTorrent = func(pkg, link string) (*proto.ExtensionBangumiWatchTorrent, error) {
	return torrent.Resolve(link, pkg)
}

// resolveTorrentForWatch resolves the torrent (if any) for a bangumi watch whose
// content type is "torrent" or "magnet", attaching the resolved file-tree handle
// to the returned proto. Non-torrent content types leave w.Torrent nil so the
// caller (the gRPC proxy) can proxy the raw stream URL normally.
func resolveTorrentForWatch(pkg, link, contentType string, w *proto.ExtensionBangumiWatch) {
	if w == nil {
		return
	}
	if contentType != "torrent" && contentType != "magnet" {
		return
	}
	if link == "" {
		return
	}
	t, err := resolveBangumiTorrent(pkg, link)
	if err != nil {
		// Resolution failures must not drop the whole watch; leave Torrent nil
		// so the frontend can surface the error from the raw URL if needed.
		return
	}
	w.Torrent = t
}
