package runtime

import (
	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/network"
)

// ExtensionSettingType is the UI control type of an extension setting. It is
// the Go counterpart of the proto ExtensionSettingType enum (input/radio/
// toggle) and the stored db_type values.
type ExtensionSettingType string

const (
	// SettingInput is a plain text/value input.
	SettingInput ExtensionSettingType = "input"
	// SettingRadio is a radio group of the setting's options.
	SettingRadio ExtensionSettingType = "radio"
	// SettingToggle is an on/off toggle.
	SettingToggle ExtensionSettingType = "toggle"
)

// ExtensionSetting is the typed definition of an extension setting. It is the
// Go counterpart of the JavaScript registerSetting({...}) object and the
// proto.ExtensionSetting message: an extension registers settings by value and
// the host persists them to the extension_settings table.
type ExtensionSetting struct {
	// Key is the unique identifier of the setting within the package.
	Key string
	// Title is the display name of the setting.
	Title string
	// Type is the UI control type (input/radio/toggle). An empty Type defaults
	// to SettingInput.
	Type ExtensionSettingType
	// Value is the current value of the setting.
	Value string
	// DefaultValue is the value used when no explicit value has been set.
	DefaultValue string
	// Description is an optional helper text shown under the setting.
	Description string
	// Options is the list of selectable values for radio settings.
	Options []string
}

// RegisterSetting registers a typed setting definition for an extension
// package. It is the Go counterpart of the JavaScript registerSetting().
// The setting is persisted by the host into the shared settings store; its
// value can later be read and written with GetSetting / SetSetting.
func RegisterSetting(setting ExtensionSetting, pkg string) error {
	typ := setting.Type
	if typ == "" {
		typ = SettingInput
	}
	var options any
	if len(setting.Options) > 0 {
		options = setting.Options
	}
	return db.RegisterSetting(map[string]any{
		"key":          setting.Key,
		"title":        setting.Title,
		"type":         string(typ),
		"value":        setting.Value,
		"defaultValue": setting.DefaultValue,
		"description":  setting.Description,
		"options":      options,
	}, pkg)
}

// GetSetting reads a previously registered setting value for the extension
// package, returned as a plain string (the stored Value). An empty string
// means the setting is unset or has no value.
func GetSetting(pkg, key string) (string, error) {
	setting, err := db.GetSetting(pkg, key)
	if err != nil {
		return "", err
	}
	if setting == nil || setting.Value == nil {
		return "", nil
	}
	return *setting.Value, nil
}

// SetSetting writes a setting value for a package. The key must have been
// registered with RegisterSetting first, mirroring db.SetSetting.
func SetSetting(pkg, key, value string) error {
	return db.SetSetting(pkg, key, value)
}

// GetCookies returns the cookies currently stored for a URL as a slice of
// "name=value" strings, matching the JavaScript getCookies() contract.
func GetCookies(url string) ([]string, error) {
	cookies, err := network.GetCookies(url)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(cookies))
	for _, c := range cookies {
		out = append(out, c.String())
	}
	return out, nil
}

// SetCookies stores the given cookies for a URL. Each entry is a cookie
// serialized as "name=value" (optionally with attributes), mirroring the
// JavaScript setCookies() contract.
func SetCookies(url string, cookies []string) error {
	return network.SetCookiesString(url, cookies)
}
