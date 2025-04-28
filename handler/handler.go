package handler

import (
	"github.com/miru-project/miru-core/ext"
	"github.com/miru-project/miru-core/pkg/result"
)

func HelloMiru() (*result.Result[any], error) {

	return result.NewSuccessResult("Hello Miru!!"), nil
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
