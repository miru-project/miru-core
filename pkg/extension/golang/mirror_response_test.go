package golang

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/pkg/extension/golang/sdk"
	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScriggoMirrorResponses verifies a Scriggo (Golang) V2 extension's
// Mirror() produces the SAME final per-type watch shapes as the JavaScript V1/V2
// mirror() step. Under the V2 contract:
//
//	Watch()  -> a list of mirrors (ExtensionWatch) for the user to pick from
//	Mirror() -> the final stream link, a per-type watch keyed by the @type
//
// The Scriggo author may return any of: a full per-type watch struct
// (ExtensionBangumiWatchMirror / ExtensionMangaWatchMirror / ExtensionFikushonWatchMirror /
// ExtensionAllMirror), a single ExtensionMirror object, or a bare URL string.
// The endpoint must resolve every one into the matching proto per-type watch
// ({bangumi,manga,fikushon,all}) exactly like the JS runtime.
//
// Each case compiles a tiny Scriggo program and drives it through the public
// Mirror() endpoint so it exercises the real converter + torrent path.
func TestScriggoMirrorResponses(t *testing.T) {
	const chosenURL = "https://stream.example.com/ep/1/index.m3u8"

	// Replace the real torrent resolver with a canned result so the torrent
	// cases below exercise the host's wiring (torrent resolved server-side
	// into w.Torrent) without performing an actual network download.
	origResolve := resolveBangumiTorrent
	name := "Resolved"
	resolveBangumiTorrent = func(pkg, link string) (*proto.ExtensionBangumiWatchTorrent, error) {
		return &proto.ExtensionBangumiWatchTorrent{
			InfoHash: "resolvedhash",
			Files:    []string{"resolved.mkv"},
			Detail:   &proto.ExtensionBangumiWatchTorrentDetail{Name: &name},
		}, nil
	}
	t.Cleanup(func() { resolveBangumiTorrent = origResolve })

	cases := []struct {
		name    string
		watch   extension.WatchType
		srcTmpl string // %q is replaced with url (defaults to chosenURL)
		url     string // per-case URL injected into %q; "" means chosenURL
		assert  func(t *testing.T, res any)
	}{
		// ---- bangumi -------------------------------------------------------
		{
			name:  "bangumi/full-struct",
			watch: extension.WatchTypeBangumi,
			srcTmpl: `
package example
import runtime "github.com/miru-project/miru-core/pkg/extension/golang/runtime"
func Mirror(pkg, url string) (*runtime.ExtensionBangumiWatchMirror, error) {
	return &runtime.ExtensionBangumiWatchMirror{Type: runtime.HLS, URL: %q}, nil
}`,
			assert: func(t *testing.T, res any) {
				w, ok := res.(*proto.ExtensionBangumiWatch)
				require.True(t, ok, "expected *proto.ExtensionBangumiWatch, got %T", res)
				// The per-type watch Type carries the CONTENT type, never the
				// extension type ("bangumi").
				assert.Equal(t, "hls", w.Type)
				assert.Equal(t, chosenURL, w.Url)
			},
		},
		{
			name:  "bangumi/bare-url",
			watch: extension.WatchTypeBangumi,
			srcTmpl: `
package example
func Mirror(pkg, url string) (string, error) {
	return %q, nil
}`,
			assert: func(t *testing.T, res any) {
				w, ok := res.(*proto.ExtensionBangumiWatch)
				require.True(t, ok, "expected *proto.ExtensionBangumiWatch, got %T", res)
				// A bare URL (default mirror) derives its content type from the
				// URL: an .m3u8 stream is "hls".
				assert.Equal(t, "hls", w.Type)
				assert.Equal(t, chosenURL, w.Url)
			},
		},
		{
			name:  "bangumi/single-mirror",
			watch: extension.WatchTypeBangumi,
			srcTmpl: `
package example
import sdk "github.com/miru-project/miru-core/pkg/extension/golang/sdk"
func Mirror(pkg, url string) (*sdk.ExtensionMirror, error) {
	return &sdk.ExtensionMirror{Name: "Source", URL: %q}, nil
}`,
			assert: func(t *testing.T, res any) {
				w, ok := res.(*proto.ExtensionBangumiWatch)
				require.True(t, ok, "expected *proto.ExtensionBangumiWatch, got %T", res)
				assert.Equal(t, "hls", w.Type)
				assert.Equal(t, chosenURL, w.Url)
			},
		},
		// ---- bangumi + torrent -------------------------------------------
		{
			// A torrent bangumi watch -- exactly the JS V1 watch() shape
			// { type, url }. The host does NOT resolve the torrent; it just
			// maps { type: "torrent", url } into the proto per-type watch and
			// hands it to the frontend, which resolves/streams the torrent
			// itself. (See runtime_v1.js's watch() contract.)
			name:  "bangumi/torrent-full-struct",
			watch: extension.WatchTypeBangumi,
			url:   "magnet:?xt=urn:btih:ABCDEF1234567890ABCDEF1234567890ABCDEF12&dn=Example Anime S01",
			srcTmpl: `
package example
import runtime "github.com/miru-project/miru-core/pkg/extension/golang/runtime"
func Mirror(pkg, url string) (*runtime.ExtensionBangumiWatchMirror, error) {
	return &runtime.ExtensionBangumiWatchMirror{
		Type: runtime.Torrent,
		URL:  %q,
	}, nil
}`,
			assert: func(t *testing.T, res any) {
				w, ok := res.(*proto.ExtensionBangumiWatch)
				require.True(t, ok, "expected *proto.ExtensionBangumiWatch, got %T", res)
				// Type carries the CONTENT type ("torrent"), never the
				// extension type ("bangumi"). No Torrent metadata is attached
				// by the host -- that is the frontend's job.
				assert.Equal(t, "torrent", w.Type)
				assert.Equal(t, "magnet:?xt=urn:btih:ABCDEF1234567890ABCDEF1234567890ABCDEF12&dn=Example Anime S01", w.Url)
			},
		},
		{
			// Same torrent carrier, but returned as a bare .torrent URL. The
			// endpoint must derive the "torrent" content type from the URL
			// (contentTypeFromURL -> torrent.IsTorrentLink) rather than a struct Type
			// field. A .torrent file link yields the "torrent" content type
			// (a bare "magnet:" link would yield "magnet").
			name:  "bangumi/torrent-bare-url",
			watch: extension.WatchTypeBangumi,
			url:   "https://example.com/anime/cool-show.torrent",
			srcTmpl: `
package example
func Mirror(pkg, url string) (string, error) {
	return %q, nil
}`,
			assert: func(t *testing.T, res any) {
				w, ok := res.(*proto.ExtensionBangumiWatch)
				require.True(t, ok, "expected *proto.ExtensionBangumiWatch, got %T", res)
				assert.Equal(t, "torrent", w.Type)
				assert.Equal(t, "https://example.com/anime/cool-show.torrent", w.Url)
			},
		},
		{
			// A magnet: link carries the distinct "magnet" content type
			// (contentTypeFromURL returns "magnet" for the magnet: scheme,
			// while a .torrent file URL yields "torrent"). The host still does
			// NOT resolve it -- the frontend resolves/streams the magnet.
			name:  "bangumi/magnet",
			watch: extension.WatchTypeBangumi,
			url:   "magnet:?xt=urn:btih:0011223344556677889900112233445566778899&dn=Cool+Show+S01",
			srcTmpl: `
package example
import runtime "github.com/miru-project/miru-core/pkg/extension/golang/runtime"
func Mirror(pkg, url string) (*runtime.ExtensionBangumiWatchMirror, error) {
	return &runtime.ExtensionBangumiWatchMirror{
		Type: runtime.Magnet,
		URL:  %q,
	}, nil
}`,
			assert: func(t *testing.T, res any) {
				w, ok := res.(*proto.ExtensionBangumiWatch)
				require.True(t, ok, "expected *proto.ExtensionBangumiWatch, got %T", res)
				assert.Equal(t, "magnet", w.Type)
				assert.Equal(t, "magnet:?xt=urn:btih:0011223344556677889900112233445566778899&dn=Cool+Show+S01", w.Url)
			},
		},
		// ---- bangumi + TLSConfig proxying -----------------------------------
		{
			// When an extension sets TLSConfig on the mirror, the backend
			// rewrites every URL (main + subtitles) into a proxy URL. The
			// raw upstream URL must NOT appear in the proto output.
			name:  "bangumi/tls-proxy",
			watch: extension.WatchTypeBangumi,
			srcTmpl: `
package example
import runtime "github.com/miru-project/miru-core/pkg/extension/golang/runtime"
func ptr(s string) *string { return &s }
func Mirror(pkg, url string) (*runtime.ExtensionBangumiWatchMirror, error) {
	return &runtime.ExtensionBangumiWatchMirror{
		Type: runtime.HLS,
		URL:  %q,
		Subtitles: []runtime.ExtensionBangumiWatchMirrorSubtitle{
			{Language: ptr("en"), Title: "English", URL: "https://cdn.example.com/sub/en.vtt"},
		},
		TLSConfig: &runtime.TLSConfig{Profile: "chrome_133"},
	}, nil
}`,
			assert: func(t *testing.T, res any) {
				w, ok := res.(*proto.ExtensionBangumiWatch)
				require.True(t, ok, "expected *proto.ExtensionBangumiWatch, got %T", res)
				assert.Equal(t, "hls", w.Type)
				// The main URL must be wrapped in a proxy URL.
				assert.NotEqual(t, chosenURL, w.Url,
					"main URL must be proxied when TLSConfig is set")
				assert.Contains(t, w.Url, "/proxy/",
					"proxied URL must contain /proxy/ path")
				assert.Contains(t, w.Url, "__tls=1",
					"proxied URL must include TLS flag")
				assert.Contains(t, w.Url, "__tlsp=chrome_133",
					"proxied URL must include the TLS profile")
				// Subtitle URLs must also be proxied.
				require.Len(t, w.Subtitles, 1)
				assert.NotEqual(t, "https://cdn.example.com/sub/en.vtt",
					w.Subtitles[0].Url,
					"subtitle URL must be proxied when TLSConfig is set")
				assert.Contains(t, w.Subtitles[0].Url, "/proxy/",
					"proxied subtitle URL must contain /proxy/ path")
			},
		},
		// ---- manga ---------------------------------------------------------
		{
			name:  "manga/full-struct",
			watch: extension.WatchTypeManga,
			srcTmpl: `
package example
import runtime "github.com/miru-project/miru-core/pkg/extension/golang/runtime"
func Mirror(pkg, url string) (*runtime.ExtensionMangaWatchMirror, error) {
	return &runtime.ExtensionMangaWatchMirror{URLs: []string{%q}}, nil
}`,
			assert: func(t *testing.T, res any) {
				w, ok := res.(*proto.ExtensionMangaWatch)
				require.True(t, ok, "expected *proto.ExtensionMangaWatch, got %T", res)
				require.Len(t, w.Urls, 1)
				assert.Equal(t, chosenURL, w.Urls[0])
			},
		},

		// ---- fikushon ------------------------------------------------------
		{
			name:  "fikushon/full-struct",
			watch: extension.WatchTypeFikushon,
			srcTmpl: `
package example
import runtime "github.com/miru-project/miru-core/pkg/extension/golang/runtime"
func Mirror(pkg, url string) (*runtime.ExtensionFikushonWatchMirror, error) {
	return &runtime.ExtensionFikushonWatchMirror{Title: "Ch.1", Content: []string{%q}}, nil
}`,
			assert: func(t *testing.T, res any) {
				w, ok := res.(*proto.ExtensionFikushonWatch)
				require.True(t, ok, "expected *proto.ExtensionFikushonWatch, got %T", res)
				require.Len(t, w.Content, 1)
				assert.Equal(t, chosenURL, w.Content[0])
			},
		},

		// ---- all -----------------------------------------------------------
		{
			name:  "all/full-struct",
			watch: extension.WatchTypeAll,
			srcTmpl: `
package example
import runtime "github.com/miru-project/miru-core/pkg/extension/golang/runtime"
func Mirror(pkg, url string) (*runtime.ExtensionAllMirror, error) {
	return &runtime.ExtensionAllMirror{
		Bangumi: &runtime.ExtensionBangumiWatchMirror{Type: runtime.HLS, URL: %q},
	}, nil
}`,
			assert: func(t *testing.T, res any) {
				w, ok := res.(*proto.ExtensionAllWatch)
				require.True(t, ok, "expected *proto.ExtensionAllWatch, got %T", res)
				require.NotNil(t, w.Bangumi)
				assert.Equal(t, "hls", w.Bangumi.Type)
				assert.Equal(t, chosenURL, w.Bangumi.Url)
			},
		},
		{
			name:  "all/bare-url",
			watch: extension.WatchTypeAll,
			srcTmpl: `
package example
func Mirror(pkg, url string) (string, error) {
	return %q, nil
}`,
			assert: func(t *testing.T, res any) {
				// For an "all" extension a single URL is exposed via the bangumi
				// member so the frontend still gets a usable resource.
				w, ok := res.(*proto.ExtensionAllWatch)
				require.True(t, ok, "expected *proto.ExtensionAllWatch, got %T", res)
				require.NotNil(t, w.Bangumi)
				assert.Equal(t, chosenURL, w.Bangumi.Url)
			},
		},
		{
			// An "all" extension that returns a bare .torrent URL. The host
			// must derive the "torrent" content type and nest it into the
			// Bangumi member of the all watch (not drop it).
			name:  "all/torrent-bare-url",
			watch: extension.WatchTypeAll,
			url:   "https://example.com/all/cool-show.torrent",
			srcTmpl: `
package example
func Mirror(pkg, url string) (string, error) {
	return %q, nil
}`,
			assert: func(t *testing.T, res any) {
				w, ok := res.(*proto.ExtensionAllWatch)
				require.True(t, ok, "expected *proto.ExtensionAllWatch, got %T", res)
				require.NotNil(t, w.Bangumi, "torrent must land in the Bangumi member")
				assert.Equal(t, "torrent", w.Bangumi.Type)
				assert.Equal(t, "https://example.com/all/cool-show.torrent", w.Bangumi.Url)
				// The host resolves the torrent server-side (shared with the
				// JS runtime) into a file-tree handle the frontend reads.
				require.NotNil(t, w.Bangumi.Torrent, "torrent should be resolved server-side")
				assert.Equal(t, "resolvedhash", w.Bangumi.Torrent.InfoHash)
			},
		},
		{
			// An "all" extension that returns a bare bangumi-shaped struct
			// (the SAME shape a single-type bangumi extension returns) with a
			// torrent link. Previously the content was dropped (the host only
			// handled ExtensionAllMirror for "all"); now it is nested into the
			// Bangumi member so the frontend gets { type: "torrent", url }.
			name:  "all/torrent-bangumi-struct",
			watch: extension.WatchTypeAll,
			url:   "magnet:?xt=urn:btih:AA11BB22CC33DD44EE55FF660011223344556677&dn=All+Show+S01",
			srcTmpl: `
package example
import runtime "github.com/miru-project/miru-core/pkg/extension/golang/runtime"
func Mirror(pkg, url string) (*runtime.ExtensionBangumiWatchMirror, error) {
	return &runtime.ExtensionBangumiWatchMirror{
		Type: runtime.Torrent,
		URL:  %q,
	}, nil
}`,
			assert: func(t *testing.T, res any) {
				w, ok := res.(*proto.ExtensionAllWatch)
				require.True(t, ok, "expected *proto.ExtensionAllWatch, got %T", res)
				require.NotNil(t, w.Bangumi, "torrent struct must land in the Bangumi member")
				assert.Equal(t, "torrent", w.Bangumi.Type)
				assert.Equal(t, "magnet:?xt=urn:btih:AA11BB22CC33DD44EE55FF660011223344556677&dn=All+Show+S01", w.Bangumi.Url)
				require.NotNil(t, w.Bangumi.Torrent, "torrent should be resolved server-side")
				assert.Equal(t, "resolvedhash", w.Bangumi.Torrent.InfoHash)
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Each case is its own extension so GetExtensionMeta reads the right
			// @type. Write it to a throwaway package under a temp ExtensionDir.
			dir := t.TempDir()
			pkg := "ext_" + sanitizeName(tc.name)
			// The per-case url (or chosenURL) is what gets injected into the
			// template's %q placeholder and passed to Mirror().
			caseURL := tc.url
			if caseURL == "" {
				caseURL = chosenURL
			}
			src := headerFor(tc.watch) + "\n" + fmtSrc(tc.srcTmpl, caseURL)
			require.NoError(t, os.WriteFile(filepath.Join(dir, pkg+".go"), []byte(src), 0o644))

			ExtensionDir = dir
			res, err := Mirror(pkg, caseURL)
			require.NoError(t, err)
			require.NotNil(t, res, "Mirror(%s) returned nil", tc.name)
			tc.assert(t, res)
		})
	}
}

// TestScriggoRuntimeMirrorTypeNames guards the rename of the Scriggo runtime
// per-type watch shapes from ExtensionXWatch to ExtensionXWatchMirror. If a
// future change reverts the name, this test fails loudly so the runtime keeps
// emitting "...Mirror" types. The proto wire types stay "...Watch" (e.g.
// proto.ExtensionBangumiWatch) -- only the internal Scriggo runtime types were
// renamed.
func TestScriggoRuntimeMirrorTypeNames(t *testing.T) {
	cases := []struct {
		value any
		want  string
	}{
		{sdk.ExtensionBangumiWatchMirror{}, "ExtensionBangumiWatchMirror"},
		{sdk.ExtensionBangumiWatchMirrorSubtitle{}, "ExtensionBangumiWatchMirrorSubtitle"},
		{sdk.ExtensionMangaWatchMirror{}, "ExtensionMangaWatchMirror"},
		{sdk.ExtensionFikushonWatchMirror{}, "ExtensionFikushonWatchMirror"},
		{sdk.ExtensionAllMirror{}, "ExtensionAllMirror"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, reflect.TypeOf(tc.value).Name(),
			"runtime mirror type name must end in 'Mirror'")
	}
}

// sanitizeName turns a test case name into a valid Go package identifier.
func sanitizeName(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

// headerFor builds the // ==MiruExtension== metadata block with the given type.
// The literal "@type" must only appear inside this header; doc comments
// elsewhere in a Scriggo source are parsed by ParseExtensionMetadata too.
func headerFor(wt extension.WatchType) string {
	const headerFmt = `// ==MiruExtension==
// @name         Test
// @version      v0.1.0
// @author       Miru
// @lang         all
// @license      MIT
// @icon         https://example.com/icon.png
// @package      test
// @type         %s
// @webSite      https://example.com
// @nsfw         false
// ==/MiruExtension==`
	return fmt.Sprintf(headerFmt, string(wt))
}

// fmtSrc substitutes the URL into the %q placeholder of the template.
func fmtSrc(tmpl, url string) string {
	return fmt.Sprintf(tmpl, url)
}
