package golang

import (
	"testing"

	"github.com/miru-project/miru-core/ent/enttest"
	"github.com/miru-project/miru-core/ext"
	"github.com/miru-project/miru-core/pkg/event"
	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/pkg/network"
	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newApiExtSource is a Go extension exercising every API added for feature
// parity with the JavaScript runtime: settings (RegisterSetting/GetSetting/
// SetSetting), cookies (GetCookies/SetCookies), the FilterDefinition type
// (returned from CreateFilter), the zstd stdlib import, fmt (dev-log hijack)
// and Fetch (dev-network event).
const newApiExtSource = `package newapiext

import (
	"fmt"

	"github.com/klauspost/compress/zstd"
	sdk "github.com/miru-project/miru-core/pkg/extension/golang/sdk"
)

func Load() {}

func CreateFilter(pkg string, filter sdk.Filter) map[string]sdk.FilterDefinition {
	return map[string]sdk.FilterDefinition{
		"quality": sdk.NewMultiSelect("Quality", 1, 4, "1080p").
			Option("1080p", "1080p").
			Option("720p", "720p").
			Build(),
	}
}

func Register() error {
	return sdk.RegisterSetting(sdk.ExtensionSetting{
		Key:          "theme",
		Title:        "Theme",
		Type:         sdk.SettingRadio,
		Value:        "dark",
		DefaultValue: "dark",
		Options:      []string{"dark", "light"},
	}, "newapiext")
}

func WriteSetting(v string) error {
	return sdk.SetSetting("newapiext", "theme", v)
}

func ReadSetting() (string, error) {
	return sdk.GetSetting("newapiext", "theme")
}

func SaveCookie(name, val string) error {
	return sdk.SetCookies("https://example.com", []string{name + "=" + val})
}

func ReadCookie(name string) (string, error) {
	cookies, err := sdk.GetCookies("https://example.com")
	if err != nil {
		return "", err
	}
	prefix := name + "="
	for _, c := range cookies {
		if len(c) >= len(prefix) && c[:len(prefix)] == prefix {
			return c[len(prefix):], nil
		}
	}
	return "", nil
}

func ZstdRoundTrip(input string) (bool, error) {
	var enc *zstd.Encoder
	var err error
	if enc, err = zstd.NewWriter(nil); err != nil {
		return false, err
	}
	var dec *zstd.Decoder
	if dec, err = zstd.NewReader(nil); err != nil {
		return false, err
	}
	compressed := enc.EncodeAll([]byte(input), nil)
	out, err := dec.DecodeAll(compressed, nil)
	if err != nil {
		return false, err
	}
	return string(out) == input, nil
}

func Log() {
	fmt.Println("dev-log-msg")
	fmt.Printf("dev-log-msg-%s", "printf")
}
`

// apiExtRuntime writes the extension, loads it into a real Scriggo VM and
// returns the Runtime for calling functions by name.
func apiExtRuntime(t *testing.T) *Runtime {
	t.Helper()
	writeExtensionSource(t, "newapiext", newApiExtSource)
	rt := NewRuntime(NewScriggoVM(nil))
	if err := rt.LoadExtension(extForTest("newapiext")); err != nil {
		t.Fatalf("LoadExtension failed: %v", err)
	}
	return rt
}

// extForTest builds a minimal extension.Extension for the given pkg so a
// runtime can load it without metadata parsing.
func extForTest(pkg string) *extension.Extension {
	return &extension.Extension{Name: pkg, Pkg: pkg}
}

// TestApiSettings verifies an extension can register and read/write settings
// through the SDK (backed by the real ent client).
func TestApiSettings(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1&_pragma=foreign_keys(1)")
	defer client.Close()
	ext.SetEntClientForTest(client)
	defer ext.SetEntClientForTest(nil)

	rt := apiExtRuntime(t)

	v, err := rt.Call("Register")
	require.NoError(t, err)
	require.Nil(t, v, "Register failed: %v", v)

	v, err = rt.Call("ReadSetting")
	require.NoError(t, err)
	assert.Equal(t, "dark", v)

	_, err = rt.Call("WriteSetting", "light")
	require.NoError(t, err)

	v, err = rt.Call("ReadSetting")
	require.NoError(t, err)
	assert.Equal(t, "light", v)
}

// TestApiCookies verifies an extension can set and read cookies via the SDK.
func TestApiCookies(t *testing.T) {
	network.Init()
	rt := apiExtRuntime(t)

	_, err := rt.Call("SaveCookie", "session", "abc123")
	require.NoError(t, err)

	v, err := rt.Call("ReadCookie", "session")
	require.NoError(t, err)
	assert.Equal(t, "abc123", v)
}

// TestApiCreateFilter verifies the CreateFilter endpoint converts a Go
// map[string]sdk.FilterDefinition into the proto representation.
func TestApiCreateFilter(t *testing.T) {
	writeExtensionSource(t, "newapiext", newApiExtSource)
	filters, err := CreateFilter("newapiext", nil)
	require.NoError(t, err)
	require.NotNil(t, filters)
	f, ok := filters["quality"]
	require.True(t, ok, "expected quality filter")
	ms, ok := f.GetKind().(*proto.ExtensionFilter_MultiSelect)
	require.True(t, ok, "quality filter should be a multi-select")
	assert.Equal(t, "Quality", ms.MultiSelect.Title)
	assert.Equal(t, int32(1), ms.MultiSelect.Min)
	assert.Equal(t, int32(4), ms.MultiSelect.Max)
	assert.Equal(t, "1080p", ms.MultiSelect.Default[0])
	assert.Equal(t, "1080p", ms.MultiSelect.Options["1080p"].Label)
}

// TestApiZstd verifies extensions can use the zstd compression library.
func TestApiZstd(t *testing.T) {
	rt := apiExtRuntime(t)
	v, err := rt.Call("ZstdRoundTrip", "compress me please")
	require.NoError(t, err)
	assert.Equal(t, true, v)
}

// TestApiFmtDevLog verifies fmt.Print* from an extension emits a DevLog event
// (while still falling through to stdout).
func TestApiFmtDevLog(t *testing.T) {
	rt := apiExtRuntime(t)
	ch := event.GlobalBus.Subscribe()
	defer event.GlobalBus.Unsubscribe(ch)

	_, err := rt.Call("Log")
	require.NoError(t, err)

	var msgs []string
	for i := 0; i < 2; i++ {
		select {
		case e := <-ch:
			if e.Type == event.DevLog {
				msgs = append(msgs, e.Data.(*proto.DevLogEvent).Message)
			}
		default:
		}
	}
	require.Len(t, msgs, 2)
	assert.Contains(t, msgs[0], "dev-log-msg")
	assert.Contains(t, msgs[1], "dev-log-msg-printf")
}
