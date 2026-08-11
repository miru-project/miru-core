// ==MiruExtension==
// @name         Example
// @version      v0.1.0
// @author       Miru
// @lang         all
// @license      MIT
// @icon         https://example.com/icon.png
// @package      example
// @type         all
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

import (
	runtime "github.com/miru-project/miru-core/pkg/extension/golang/runtime"
	sdk "github.com/miru-project/miru-core/pkg/extension/golang/sdk"
)

// Search searches for content by keyword.
func Search(pkg, kw string, page int, filter sdk.Filter) ([]sdk.ExtensionListItem, error) {
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

// CreateFilter returns the search filters for this package, so the search UI
// can render selectable options. The filter argument carries the current
// selection as a strongly-typed sdk.Filter (the golang runtime decodes the
// frontend's JSON before calling this), so extensions read typed filter
// values via sdk.HasSelection / sdk.SelectionsOf / sdk.FirstSelection.
func CreateFilter(pkg string, filter sdk.Filter) map[string]sdk.FilterDefinition {
	return map[string]sdk.FilterDefinition{
		"type": sdk.NewSelect("Type", "all").
			Option("all", "All").
			Option("manga", "Manga").
			Option("bangumi", "Anime").
			Build(),
		"language": sdk.NewSelect("Language", "en").
			Option("en", "English").
			Option("ja", "Japanese").
			Build(),
	}
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

// Watch returns watch/stream information for a content item. This example
// declares the combined "all" extension type, so it returns the
// ExtensionAllMirror carrying a manga page list, a fikushon (novel) chapter, and
// a bangumi (anime) stream -- all from a single watch call.
func Watch(pkg, url string) (*sdk.ExtensionAllMirror, error) {
	return &sdk.ExtensionAllMirror{
		Manga: &runtime.ExtensionMangaWatchMirror{
			URLs: []string{
				"https://example.com/manga/1.jpg",
				"https://example.com/manga/2.jpg",
			},
		},
		Fikushon: &runtime.ExtensionFikushonWatchMirror{
			Title: "Chapter 1",
			Content: []string{
				"Paragraph one of the novel.",
				"Paragraph two of the novel.",
			},
		},
		Bangumi: &runtime.ExtensionBangumiWatchMirror{
			Type: runtime.HLS,
			URL:  url,
		},
	}, nil
}

// Mirror resolves the chosen source into the final watchable resource. Because the
// Golang (Scriggo) V2 runtime treats Watch() as a list of mirrors for the user
// to pick from and Mirror() as the step that yields the actual stream link
// (mirroring the JavaScript V2 layout), this returns the resolved per-type watch
// for the selected mirror rather than a list of candidates. The example declares
// an "all" type, so the resolved shape is bundled inside ExtensionAllMirror.
func Mirror(pkg, url string) (*sdk.ExtensionAllMirror, error) {
	return &sdk.ExtensionAllMirror{
		Bangumi: &runtime.ExtensionBangumiWatchMirror{
			Type: runtime.HLS,
			URL:  url,
		},
	}, nil
}
