package torrent

import "testing"

func TestIsTorrentLink(t *testing.T) {
	if !IsTorrentLink("https://x/2143519.torrent") {
		t.Error("expected .torrent link to be detected")
	}
	if !IsTorrentLink("magnet:?xt=urn:btih:abc") {
		t.Error("expected magnet link to be detected")
	}
	if IsTorrentLink("") {
		t.Error("empty url should not be a torrent link")
	}
	if IsTorrentLink("https://x/2143519.mp4") {
		t.Error("mp4 should not be a torrent link")
	}
	// Case-insensitive suffix match: an uppercase extension is still a torrent.
	if !IsTorrentLink("https://x/Some.Show.TORRENT") {
		t.Error("expected uppercase .TORRENT to be detected")
	}
}
