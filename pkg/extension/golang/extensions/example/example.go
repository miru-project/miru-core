// ==MiruExtension==
// @name         Example
// @version      v0.1.0
// @author       Miru
// @lang         all
// @license      MIT
// @icon         https://example.com/icon.png
// @package      example
// @type         bangumi
// @webSite      https://example.com
// @nsfw         false
// ==/MiruExtension==

// Package example is the canonical Miru Go extension. It is authored the
// ordinary way: a freshly `go mod init`'d project `go get`s the SDK and imports
// it directly,
//
//	import sdk "github.com/miru-project/miru-core/pkg/extension/golang/sdk"
//
// and refers to the model types explicitly as sdk.ExtensionListItem,
// sdk.ExtensionDetail, and so on. This same file serves both roles:
//
//   - as a normal Go package you can import and call directly (the "native"
//     stage), and
//   - as the source the host compiles with the Scriggo VM and drives by entry
//     point name (the "VM" stage).
//
// Because the extension imports the SDK itself, the host compiles it as-is --
// no source rewriting or injected imports are performed.
package example

import sdk "github.com/miru-project/miru-core/pkg/extension/golang/sdk"

// Search searches for content by keyword.
func Search(pkg, kw string, page int, filter string) ([]sdk.ExtensionListItem, error) {
	results := []sdk.ExtensionListItem{
		{
			Title:  "Example Result 1",
			URL:    "https://example.com/1",
			Cover:  "https://example.com/1.jpg",
			Update: "2024-01-01",
			Image:  "https://example.com/1.jpg",
			Type:   "manga",
		},
		{
			Title:  "Example Result 2",
			URL:    "https://example.com/2",
			Cover:  "https://example.com/2.jpg",
			Update: "2024-01-02",
			Image:  "https://example.com/2.jpg",
			Type:   "bangumi",
		},
	}
	return results, nil
}

// Latest returns the latest content for a package.
func Latest(pkg string, page int) ([]sdk.ExtensionListItem, error) {
	results := []sdk.ExtensionListItem{
		{
			Title:  "Latest Example 1",
			URL:    "https://example.com/latest/1",
			Cover:  "https://example.com/latest/1.jpg",
			Update: "2024-01-03",
			Image:  "https://example.com/latest/1.jpg",
			Type:   "manga",
		},
	}
	return results, nil
}

// Detail returns detailed information about a content item.
func Detail(pkg, url string) (*sdk.ExtensionDetail, error) {
	return &sdk.ExtensionDetail{
		Title: "Example Detail",
		Desc:  "Example description",
		URL:   url,
	}, nil
}

// Watch returns watch/stream information for a content item.
func Watch(pkg, url string) (*sdk.ExtensionWatch, error) {
	return &sdk.ExtensionWatch{
		Title: "Example Watch",
		URL:   url,
		Groups: []sdk.ExtensionMirrorGroup{
			{
				Title: "Group 1",
				Mirrors: []sdk.ExtensionMirror{
					{
						Name: "Mirror 1",
						URL:  url,
					},
				},
			},
		},
	}, nil
}

// Mirror returns mirror/alternative URLs for a content item.
func Mirror(pkg, url string) ([]sdk.ExtensionMirror, error) {
	return []sdk.ExtensionMirror{
		{
			Name: "Mirror 1",
			URL:  url,
		},
	}, nil
}
