package handler

import (
	"github.com/miru-project/miru-core/ext"
	"github.com/miru-project/miru-core/pkg/jsExtension"
	"github.com/miru-project/miru-core/pkg/result"
)

func HelloMiru() (*result.Result[any], error) {
	out := make(map[string]any)
	extMeta := make([]*jsExtension.Ext, 0)

	for _, cache := range jsExtension.ApiPkgCache {
		extMeta = append(extMeta, cache.Ext)
	}

	for _, cache := range jsExtension.ApiPkgCache {
		extMeta = append(extMeta, cache.Ext)
	}
	out["extensionMeta"] = extMeta

	return result.NewSuccessResult(out), nil
}

func GetAppSetting() (*result.Result[any], error) {
	// Get all settings
	settings, err := ext.GetAllSettings()
	if err != nil {
		return result.NewErrorResult("Failed to get settings", 500), err
	}

	return result.NewSuccessResult(settings), nil
}

func SetAppSetting(settings *[]ext.AppSettingJson) []error {

	if e := ext.SetAppSettings(settings); len(e) != 0 {
		return e
	}
	return nil
}
