package runtime

import (
	"net/url"
	"path/filepath"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/miru-project/miru-core/pkg/result"
	"github.com/miru-project/miru-core/pkg/torrent"
)

// AddMagnet resolves a magnet: link the same way the JavaScript runtime's
// handleMediaType did: it adds the magnet to the host torrent client, waits for
// its metainfo, and returns the resolved Torrent (mirroring the proto
// ExtensionBangumiWatchTorrent so the gRPC WatchResponse can hand it to the
// frontend). The error is returned as a string (not the Go error interface)
// because Scriggo's playground cannot marshal an error value back from a host
// function call -- see Fetch for the same convention.
//
// title is the human-readable download name (may be empty; the torrent's own
// name is used when absent). pkg is the calling extension's package name.
func AddMagnet(magnet, title, pkg string) (Torrent, string) {
	res, err := torrent.AddMagnet(magnet, title, pkg)
	if err != nil {
		return Torrent{}, err.Error()
	}
	return toRuntimeTorrent(res), ""
}

// AddTorrent resolves a .torrent file URL the same way the JavaScript runtime's
// handleMediaType did. See AddMagnet for the full contract. When link is a
// relative path it is resolved against the extension's Website (mirroring the
// JS behaviour) before being fetched.
func AddTorrent(link, title, pkg, website string) (Torrent, string) {
	resolved := resolveTorrentURL(link, website)
	res, err := torrent.AddTorrent(resolved, title, pkg)
	if err != nil {
		return Torrent{}, err.Error()
	}
	return toRuntimeTorrent(res), ""
}

// resolveTorrentURL mirrors the JavaScript handleMediaType URL normalisation:
// an absolute link is returned unchanged, while a relative one is joined onto
// the extension's website origin.
func resolveTorrentURL(link, website string) string {
	if u, err := url.Parse(link); err == nil && u.IsAbs() {
		return link
	}
	if website == "" {
		return link
	}
	base, err := url.Parse(website)
	if err != nil {
		return link
	}
	base.Path = filepath.Join(base.Path, link)
	return base.String()
}

// toRuntimeTorrent converts the host torrent result into the SDK Torrent model,
// rebuilding the file tree from the flat metainfo file list so the frontend can
// read it and pick which files to download.
func toRuntimeTorrent(res result.TorrentDetailResult) Torrent {
	out := Torrent{
		InfoHash: res.InfoHash,
		Files:    res.Files,
	}
	if res.Detail != nil {
		detail := buildTorrentDetail(res.Detail)
		out.Detail = detail
	}
	return out
}

// buildTorrentDetail copies the optional metainfo fields into the SDK model and
// reconstructs the directory file tree from the flat Files slice.
func buildTorrentDetail(info *metainfo.Info) *TorrentDetail {
	if info == nil {
		return nil
	}
	d := &TorrentDetail{
		PieceLength: optInt32(int32(info.PieceLength)),
		Pieces:      optString(string(info.Pieces)),
		Name:        optString(info.Name),
		NameUtf8:    optString(info.NameUtf8),
	}
	if info.Length != 0 {
		d.Length = optInt64(info.Length)
	}
	if info.Source != "" {
		d.Source = optString(info.Source)
	}
	if info.MetaVersion != 0 {
		d.MetaVersion = optInt32(int32(info.MetaVersion))
	}
	d.FileTree = buildFileTree(info)
	return d
}

// buildFileTree rebuilds the nested FileTree from metainfo.Info.Files. A node
// may carry both a File (leaf) and a Dir (children), matching BEP 52 torrents
// where a file can also have sub-directories.
func buildFileTree(info *metainfo.Info) *TorrentFileTree {
	root := &TorrentFileTree{}
	for i := range info.Files {
		fi := &info.Files[i]
		segments := fi.BestPath()
		if len(segments) == 0 {
			continue
		}
		node := root
		for _, seg := range segments[:len(segments)-1] {
			if node.Dir == nil {
				node.Dir = make(map[string]*TorrentFileTree)
			}
			child, ok := node.Dir[seg]
			if !ok {
				child = &TorrentFileTree{}
				node.Dir[seg] = child
			}
			node = child
		}
		leaf := &TorrentFileTreeFile{
			Length:     fi.Length,
			PiecesRoot: fi.PiecesRoot.String(),
		}
		if node.File != nil {
			// File already present at this path (unusual); keep it and also
			// register the leaf under a child node so nothing is lost.
			if node.Dir == nil {
				node.Dir = make(map[string]*TorrentFileTree)
			}
			node.Dir[segments[len(segments)-1]] = &TorrentFileTree{File: leaf}
		} else {
			node.File = leaf
		}
	}
	return root
}

func optString(s string) *string { return &s }

func optInt32(v int32) *int32 { return &v }

func optInt64(v int64) *int64 { return &v }
