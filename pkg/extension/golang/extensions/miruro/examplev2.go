// ==MiruExtension==
// @name         Miruro
// @version      v0.1.0
// @author       Miru
// @lang         all
// @license      MIT
// @icon         https://example.com/icon.png
// @package      examplev2
// @type         bangumi
// @webSite      https://example.com
// @nsfw         false
// @apiVersion   2
// ==/MiruExtension==

// Package examplev2 is a sample Miru Go (Scriggo) v2 extension. It is loaded
// by the shared-folder tests that exercise both runtimes reading from the same
// directory, and by the Golang runtime's LoadExtensions path.
package examplev2

import (
	runtime "github.com/miru-project/miru-core/pkg/extension/golang/runtime"
	sdk "github.com/miru-project/miru-core/pkg/extension/golang/sdk"
)

// Load runs once at startup. Like the JavaScript runtime's load() hook it takes
// no arguments; any cross-function state is seeded through sdk.SaveCache keyed
// by the extension's own package name.
func Load() {
	sdk.SaveCache("example.v2", "greeting", "hello from miruro")
}

// Latest returns the latest content. pkg is the package name, injected by the
// runtime as the first argument of every entry point (mirroring the JS layout).
func Latest(pkg string, page int) ([]sdk.ExtensionListItem, error) {
	return []sdk.ExtensionListItem{
		{Title: "Example Latest", URL: "https://example.com/latest"},
	}, nil
}

func Search(pkg, kw string, page int, filter string) ([]sdk.ExtensionListItem, error) {
	return []sdk.ExtensionListItem{
		{Title: "Example Search: " + kw, URL: "https://example.com/search"},
	}, nil
}

func Detail(pkg, url string) (*sdk.ExtensionDetail, error) {
	return &sdk.ExtensionDetail{Title: "Example Detail", URL: url}, nil
}

// Watch returns the V2 mirror list (proto.ExtensionWatch) -- NOT the final
// link. The frontend lets the user pick a mirror, then calls Mirror() to
// resolve it. This matches the JavaScript V2 watch() contract.
func Watch(pkg, url string) (*sdk.ExtensionWatch, error) {
	return &sdk.ExtensionWatch{URL: url}, nil
}

// Mirror resolves the chosen mirror (the URL the user picked from the Watch
// list) into the final watchable resource. Under the V2 layout Watch() only
// yields the list of mirrors; the actual stream link is produced here. This
// extension declares a bangumi type, so the resolved shape is an
// ExtensionBangumiWatchMirror. The Type field carries the CONTENT type
// (hls|mp4|torrent|magnet) -- never the extension type -- mirroring what V1
// watch() returned.
func Mirror(pkg, url string) (*runtime.ExtensionBangumiWatchMirror, error) {
	return &runtime.ExtensionBangumiWatchMirror{Type: runtime.HLS, URL: url}, nil
}
