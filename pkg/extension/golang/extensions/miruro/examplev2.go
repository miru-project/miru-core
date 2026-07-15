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

func Watch(pkg, url string) (*sdk.ExtensionWatch, error) {
	return &sdk.ExtensionWatch{URL: url}, nil
}

func Mirror(pkg, url string) ([]sdk.ExtensionMirror, error) {
	return []sdk.ExtensionMirror{{Name: "Mirror 1", URL: url}}, nil
}
