package runtime

import (
	"os"
	"strings"

	"github.com/miru-project/miru-core/config"
	"github.com/miru-project/miru-core/pkg/network"
)

// TLSConfig is the TLS/impersonation configuration that a Scriggo (Go)
// extension can build directly and pass to Fetch, e.g.
//
//	&runtime.TLSConfig{Profile: "chrome_133"}
//
// It is a concrete struct (not a type alias) so the Scriggo Go-subset compiler
// can resolve and construct it without pulling the host "network" package into
// the extension playground. Fetch converts it to the underlying network.TLSConfig.
type TLSConfig struct {
	// Profile is a tls-client fingerprint name, e.g. "chrome_133". An empty
	// Profile falls back to the tls-client library default.
	Profile string `json:"profile"`
	// UserAgent overrides the browser-impersonation User-Agent when non-empty.
	UserAgent string `json:"userAgent,omitempty"`
	// DisableRedirect stops the client from following redirects.
	DisableRedirect bool `json:"disableRedirect,omitempty"`
	// InsecureSkipVerify disables TLS certificate verification.
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// toNetwork converts a runtime.TLSConfig into the network-layer TLSConfig that
// performs the actual browser-impersonating request.
func (c *TLSConfig) toNetwork() *network.TLSConfig {
	if c == nil {
		return nil
	}
	return &network.TLSConfig{
		Profile:            c.Profile,
		UserAgent:          c.UserAgent,
		DisableRedirect:    c.DisableRedirect,
		InsecureSkipVerify: c.InsecureSkipVerify,
	}
}

// Fetch performs a single HTTP request and returns the raw response body
// together with its HTTP status code. On failure errStr is non-empty and holds
// the error message; an empty errStr means success.
//
// It is deliberately GENERIC and site-agnostic: the caller supplies the URL, the
// request method, the exact request headers, an optional request body, and an
// optional TLS configuration. The host holds NO API-specific knowledge here --
// no domain, no hard-coded headers, no protocol. Every site-specific concern
// (which domain, which headers, request signing, response decoding) lives
// inside the extension that calls Fetch.
//
// This thin primitive has to live in the host (rather than the extension)
// because of two hard limits of the Scriggo Go-subset that runs extensions:
//
//  1. Scriggo cannot emit interface-method calls, so an extension cannot call
//     the underlying http client / resp.Body.Close itself (the compiler aborts
//     with "internal error: not implemented").
//  2. Defeating TLS-fingerprint bot walls (Cloudflare) needs a browser JA3/h2
//     fingerprint from the tls-client "profiles" package, which is only
//     reachable through that same interface.
//
// Both un-emittable pieces are confined to this one primitive; the extension
// reaches it as a plain function call, which Scriggo supports.
//
// The error is returned as a string (not the Go error interface) on purpose:
// Scriggo's playground cannot marshal a Go error interface value back from a
// called host function, so returning error would panic at the call boundary.
//
// When tls is non-nil the request is routed through the browser-impersonating
// tls-client (using the configured Profile, e.g. "chrome_133"; an empty Profile
// falls back to the library default). When tls is nil the request goes through
// the default fasthttp client. Both transports share the same persistent cookie
// jar, so cookies survive across calls and across both transports.
func Fetch(url, method string, headers map[string]string, body string, tls *TLSConfig) (respBody string, status int, errStr string) {
	opts := &network.RequestOptions{
		Headers: headers,
		Method:  method,
	}
	if body != "" {
		opts.RequestBody = body
	}
	if tls != nil {
		opts.TLSConfig = tls.toNetwork()
	}

	// Delegate to the non-generic FetchString: Scriggo's reflect-based callable
	// cannot invoke a generic function, so the request must be performed through
	// a non-generic entry point.
	res, err := network.FetchString(url, opts, network.ReadAll)
	if err != nil {
		return "", 0, err.Error()
	}
	return res.Body, res.StatusCode, ""
}

// ProxyURL converts a raw media/stream URL into a host-relative proxy path that
// the miru backend will fetch server-side on behalf of the client. The target
// URL, the per-request headers (e.g. a Referer the browser cannot set on a
// cross-origin fetch because it is a forbidden header), and an optional
// tls-client fingerprint profile are all encoded into the returned URL, so the
// client simply requests that URL from the backend's /proxy endpoint.
//
// This exists as a host primitive for the same reason Fetch does: the backend
// proxy URL format (network.BuildProxyURL) lives in the host, and the Scriggo
// Go-subset that runs extensions cannot reference the host "network" package
// directly. The extension instead calls this plain function and hands the
// result back as a mirror/latest/detail/media URL.
//
// The returned value is an ABSOLUTE url (host included) rooted at the miru-core
// origin, because a browser player cannot request a relative "/proxy/..." path
// and (for HLS) every rewritten child segment/key must point back at the same
// origin. The host comes from the backend configuration (config.Global); a
// sane fallback is used if config has not been loaded.
//
// When tlsProfile is non-empty the backend routes the upstream fetch through the
// browser-impersonating tls-client -- required for TLS-fingerprinting CDNs
// (e.g. Cloudflare-fronted Miruro source CDNs) that block Go's crypto/tls
// ClientHello.
func ProxyURL(target string, headers map[string]string, tlsProfile string) string {
	return network.BuildProxyURL(proxyOrigin(), target, headers, tlsProfile)
}

// proxyOrigin returns the origin the player reaches miru-core at, derived from
// the loaded config. It falls back to a local default so extensions can still
// build usable (browser-resolvable) proxy URLs when config is not yet loaded
// (e.g. unit tests, or a deployment that sets the real host via MIRU_HOST).
func proxyOrigin() string {
	if h := strings.TrimSpace(os.Getenv("MIRU_HOST")); h != "" {
		return h
	}
	addr := config.Global.Address
	if addr == "" {
		addr = "127.0.0.1"
	}
	port := config.Global.Port
	if port == "" {
		port = "3000"
	}
	return "http://" + addr + ":" + port
}

// ExtensionListItem represents a search result item.
type ExtensionListItem struct {
	Title   string            `json:"title"`
	URL     string            `json:"url"`
	Cover   string            `json:"cover"`
	Update  string            `json:"update"`
	Image   string            `json:"image,omitempty"`
	Type    string            `json:"type,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// ExtensionDetail represents detailed content information.
type ExtensionDetail struct {
	Title       string                  `json:"title"`
	URL         string                  `json:"url"`
	Cover       string                  `json:"cover"`
	Image       string                  `json:"image,omitempty"`
	Type        string                  `json:"type,omitempty"`
	Description string                  `json:"description,omitempty"`
	Desc        string                  `json:"desc,omitempty"`
	Chapters    []ExtensionEpisodeGroup `json:"chapters,omitempty"`
	Headers     map[string]string       `json:"headers,omitempty"`
}

// ExtensionEpisodeGroup represents a group of episodes.
type ExtensionEpisodeGroup struct {
	Title string   `json:"title"`
	URLs  []string `json:"urls,omitempty"`
}

// ExtensionWatch represents watch/stream information.
type ExtensionWatch struct {
	Title  string                 `json:"title"`
	URL    string                 `json:"url"`
	Type   string                 `json:"type"`
	Pages  []string               `json:"pages,omitempty"`
	Groups []ExtensionMirrorGroup `json:"groups,omitempty"`
}

// ExtensionMirrorGroup represents a group of mirrors.
type ExtensionMirrorGroup struct {
	Title   string            `json:"title"`
	Mirrors []ExtensionMirror `json:"mirrors,omitempty"`
}

// ExtensionMirror represents a mirror/alternative URL.
type ExtensionMirror struct {
	Name    string            `json:"name"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// ExtensionMangaWatchMirror is the per-type watch shape for manga extensions. A Golang
// extension that declares @type manga returns this from its Watch entry point.
type ExtensionMangaWatchMirror struct {
	URLs    []string          `json:"urls"`
	Headers map[string]string `json:"headers,omitempty"`
}

// ExtensionFikushonWatchMirror is the per-type watch shape for novel/fiction
// (fikushon) extensions. A Golang extension that declares @type fikushon returns
// this from its Watch entry point.
type ExtensionFikushonWatchMirror struct {
	Content  []string `json:"content"`
	Title    string   `json:"title"`
	Subtitle string   `json:"subtitle,omitempty"`
}

// ExtensionBangumiWatchMirrorSubtitle is a single subtitle track for a bangumi watch.
type ExtensionBangumiWatchMirrorSubtitle struct {
	Language *string `json:"language,omitempty"`
	Title    string  `json:"title"`
	URL      string  `json:"url"`
}

// BangumiWatchType is the CONTENT type of a bangumi (anime) stream/mirror --
// hls, mp4, torrent, or magnet. It mirrors the dart ExtensionWatchBangumiType
// enum and the V1 watch() vocabulary, and is never the extension type
// ("bangumi"). The host uses it to pick a player/handler uniformly across V1
// watch() and V2 mirror().
type BangumiWatchType string

const (
	HLS     BangumiWatchType = "hls"
	MP4     BangumiWatchType = "mp4"
	Torrent BangumiWatchType = "torrent"
	Magnet  BangumiWatchType = "magnet"
)

// ExtensionBangumiWatchMirror is the per-type watch shape for bangumi (anime) video
// extensions. A Golang extension that declares @type bangumi returns this from
// its Watch entry point.
//
// When URL is a magnet: or .torrent link the host resolves it (mirroring the
// JavaScript handleMediaType behaviour) and fills Torrent; an author may also
// resolve one explicitly via sdk.AddMagnet / sdk.AddTorrent and set Torrent
// themselves. The frontend reads Torrent to decide which files to download.
//
// Type carries the CONTENT type (BangumiWatchType), never the extension type.
type ExtensionBangumiWatchMirror struct {
	Type       BangumiWatchType                      `json:"type"`
	URL        string                                `json:"url"`
	Subtitles  []ExtensionBangumiWatchMirrorSubtitle `json:"subtitles,omitempty"`
	Headers    map[string]string                     `json:"headers,omitempty"`
	AudioTrack string                                `json:"audioTrack,omitempty"`
	Torrent    *TorrentHandle                        `json:"torrent,omitempty"`
}

// TorrentFileTreeFile is a single file node in a torrent's file tree.
type TorrentFileTreeFile struct {
	Length     int64  `json:"length"`
	PiecesRoot string `json:"piecesRoot"`
}

// TorrentFileTree is a node in a torrent's file tree: either a file, a directory
// (dir), or both (a file that also has sub-directories).
type TorrentFileTree struct {
	File *TorrentFileTreeFile        `json:"file,omitempty"`
	Dir  map[string]*TorrentFileTree `json:"dir,omitempty"`
}

// TorrentDetail carries the resolved torrent metainfo the frontend needs to
// render the file tree and pick files to download. All fields are optional to
// match the proto ExtensionBangumiWatchTorrentDetail message.
type TorrentDetail struct {
	PieceLength *int32           `json:"pieceLength,omitempty"`
	Pieces      *string          `json:"pieces,omitempty"`
	Name        *string          `json:"name,omitempty"`
	NameUtf8    *string          `json:"nameUtf8,omitempty"`
	Length      *int64           `json:"length,omitempty"`
	Source      *string          `json:"source,omitempty"`
	MetaVersion *int32           `json:"metaVersion,omitempty"`
	FileTree    *TorrentFileTree `json:"fileTree,omitempty"`
}

// TorrentHandle is the resolved torrent handle an extension (or the host)
// attaches to a bangumi watch. It mirrors the proto ExtensionBangumiWatchTorrent
// message so the gRPC WatchResponse can carry it straight to the frontend. The
// JSON tag stays "torrent" so the proto field mapping is unchanged.
type TorrentHandle struct {
	InfoHash string         `json:"infoHash"`
	Detail   *TorrentDetail `json:"detail,omitempty"`
	Files    []string       `json:"files,omitempty"`
}

// ExtensionAllMirror bundles the three per-type watch shapes (manga, fikushon and
// bangumi) behind a single "all" extension type. A Golang extension that
// declares @type all returns this from its Watch entry point so the client can
// render any of the three media kinds from one watch call.
type ExtensionAllMirror struct {
	Manga    *ExtensionMangaWatchMirror    `json:"manga,omitempty"`
	Fikushon *ExtensionFikushonWatchMirror `json:"fikushon,omitempty"`
	Bangumi  *ExtensionBangumiWatchMirror  `json:"bangumi,omitempty"`
}
