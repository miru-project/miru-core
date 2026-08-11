package torrent

import "strings"

// IsTorrentLink reports whether a link refers to a torrent resource: a magnet:
// URI or a .torrent file URL.
//
// This is the single source of truth for that classification. Both the Go
// (Scriggo) extension runtime and the gRPC watch/mirror proxy consult it, so
// the two never disagree about what counts as a torrent -- a torrent link is
// resolved server-side into a file tree rather than wrapped into a generic
// /proxy stream URL.
func IsTorrentLink(link string) bool {
	if link == "" {
		return false
	}
	if strings.HasPrefix(link, "magnet:") {
		return true
	}
	return strings.HasSuffix(strings.ToLower(link), ".torrent")
}
