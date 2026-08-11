package runtime

import (
	"testing"

	"github.com/miru-project/miru-core/ent/enttest"
	"github.com/miru-project/miru-core/ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// settingsEnt spins up an in-memory ent client and points the global db handle
// at it, so the runtime setting helpers (which persist via db.RegisterSetting /
// GetSetting / SetSetting) can be exercised without a real database.
func settingsEnt(t *testing.T) {
	t.Helper()
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1&_pragma=foreign_keys(1)")
	t.Cleanup(func() { client.Close() })
	ext.SetEntClientForTest(client)
	t.Cleanup(func() { ext.SetEntClientForTest(nil) })
}

// TestRegisterSettingPersists verifies a typed setting definition is registered
// and its type is normalized/persisted so GetSetting can read it back later.
func TestRegisterSettingPersists(t *testing.T) {
	settingsEnt(t)

	const pkg = "settingstest"
	err := RegisterSetting(ExtensionSetting{
		Key:          "theme",
		Title:        "Theme",
		Type:         SettingRadio,
		Value:        "dark",
		DefaultValue: "dark",
		Options:      []string{"dark", "light"},
	}, pkg)
	require.NoError(t, err)

	v, err := GetSetting(pkg, "theme")
	require.NoError(t, err)
	assert.Equal(t, "dark", v, "registered value should be readable")
}

// TestSettingEmptyTypeDefaultsToInput verifies an extension setting with no
// explicit type defaults to a plain input control rather than an empty/unknown
// type.
func TestSettingEmptyTypeDefaultsToInput(t *testing.T) {
	settingsEnt(t)

	const pkg = "settingstest2"
	require.NoError(t, RegisterSetting(ExtensionSetting{
		Key:          "token",
		Title:        "Token",
		Value:        "secret",
		DefaultValue: "secret",
	}, pkg))

	v, err := GetSetting(pkg, "token")
	require.NoError(t, err)
	assert.Equal(t, "secret", v)
}

// TestSetSettingUpdatesValue verifies writing a registered setting overrides the
// previously stored value (and that SetSetting requires a prior registration).
func TestSetSettingUpdatesValue(t *testing.T) {
	settingsEnt(t)

	const pkg = "settingstest3"
	require.NoError(t, RegisterSetting(ExtensionSetting{
		Key:          "quality",
		Title:        "Quality",
		Value:        "1080p",
		DefaultValue: "1080p",
	}, pkg))

	require.NoError(t, SetSetting(pkg, "quality", "720p"))

	v, err := GetSetting(pkg, "quality")
	require.NoError(t, err)
	assert.Equal(t, "720p", v, "SetSetting must overwrite the previous value")

	// SetSetting on an unregistered key must fail rather than silently create.
	err = SetSetting(pkg, "missing", "x")
	assert.Error(t, err, "writing an unregistered setting key is an error")
}

// TestGetSettingUnknownReturnsError verifies reading an unregistered setting is
// reported as an error (the ent query returns "not found") rather than silently
// yielding an empty string, so extensions must register before reading.
func TestGetSettingUnknownReturnsError(t *testing.T) {
	settingsEnt(t)
	v, err := GetSetting("nope", "nope")
	assert.Error(t, err, "reading an unregistered setting must surface an error")
	assert.Equal(t, "", v)
}
