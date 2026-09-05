package handler

import (
	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/download"
	golang "github.com/miru-project/miru-core/pkg/extension/golang"
	js "github.com/miru-project/miru-core/pkg/extension/js"
	"github.com/miru-project/miru-core/pkg/result"
	"github.com/miru-project/miru-core/pkg/torrent"
)

func HelloMiru() (*result.Result[any], error) {
	out := make(map[string]any)

	// Extension metaData: include BOTH JavaScript and Go/Scriggo extensions so
	// the frontend can list and use extensions from either runtime.
	out["extensionMeta"] = BuildExtensionMeta()

	// Download status
	out["downloadStatus"] = download.DownloadStatus()
	out["torrent"] = torrent.TorrentStatus()

	// History
	histories, _ := db.GetHistorysFiltered(nil, nil, 0, 0)
	out["history"] = histories

	return result.NewSuccessResult[any](out), nil
}

func GetAppSetting() (*result.Result[any], error) {
	// Get all settings
	settings, err := db.GetAllAPPSettings()
	if err != nil {
		return result.NewErrorResultAny("Failed to get settings", 500), err
	}

	return result.NewSuccessResult[any](settings), nil
}

func SetAppSettings(settings map[string]string) []error {

	if e := db.SetAppSettings(settings); len(e) != 0 {
		return e
	}
	return nil
}

func SetAppSetting(key string, value string) error {
	return db.SetAppSetting(key, value)
}

// BuildExtensionMeta assembles the extension list returned to the frontend.
// It merges the JavaScript extensions held in the JS runtime cache with the
// Go/Scriggo extensions discovered on disk, so the UI can list and drive
// extensions from either runtime. On a package-name collision the Go extension
// wins, mirroring endpoint.GetRuntime's resolution order (.go file checked
// before .js). Source contexts are stripped to keep the payload small.
//
// This is the single source of truth for the extension list: both the initial
// HelloMiru snapshot and every live ExtensionUpdate event must publish it.
// Publishing a runtime-local list instead makes the frontend's whole-list
// replace drop the other runtime's extensions.
func BuildExtensionMeta() []*js.Ext {
	extMeta := make([]*js.Ext, 0)
	seen := make(map[string]bool)

	for _, gExt := range golang.GetExtensions() {
		copy := *gExt
		copy.Context = nil
		extMeta = append(extMeta, &copy)
		seen[gExt.Pkg] = true
	}

	for _, cache := range js.ApiPkgCache.GetAll() {
		if seen[cache.Ext.Pkg] {
			continue
		}
		extCopy := *cache.Ext
		extCopy.Context = nil
		extMeta = append(extMeta, &extCopy)
	}

	return extMeta
}
