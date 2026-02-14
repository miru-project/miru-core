package handler

import (
	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/download"
	"github.com/miru-project/miru-core/pkg/jsExtension"
	"github.com/miru-project/miru-core/pkg/result"
	"github.com/miru-project/miru-core/pkg/torrent"
)

func HelloMiru() (*result.Result[any], error) {
	out := make(map[string]any)

	// Extension metaData
	extMeta := make([]*jsExtension.Ext, 0)
	for _, cache := range jsExtension.ApiPkgCache.GetAll() {
		extCopy := *cache.Ext
		extCopy.Context = nil
		extMeta = append(extMeta, &extCopy)
	}
	out["extensionMeta"] = extMeta

	// Download status
	out["downloadStatus"] = download.DownloadStatus()
	out["torrent"] = torrent.TorrentStatus()

	// History
	histories, _ := db.GetHistorysFiltered(nil, nil, 0, 0)
	out["history"] = histories

	return result.NewSuccessResult(out), nil
}

func GetAppSetting() (*result.Result[any], error) {
	// Get all settings
	settings, err := db.GetAllAPPSettings()
	if err != nil {
		return result.NewErrorResult("Failed to get settings", 500, nil), err
	}

	return result.NewSuccessResult(settings), nil
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
