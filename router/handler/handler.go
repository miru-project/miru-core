package handler

import (
	"github.com/miru-project/miru-core/ext"
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
	return result.NewSuccessResult(out), nil
}

func GetAppSetting() (*result.Result[any], error) {
	// Get all settings
	settings, err := ext.GetAllSettings()
	if err != nil {
		return result.NewErrorResult("Failed to get settings", 500, nil), err
	}

	return result.NewSuccessResult(settings), nil
}

func SetAppSetting(settings *[]ext.AppSettingJson) []error {

	if e := ext.SetAppSettings(settings); len(e) != 0 {
		return e
	}
	return nil
}
