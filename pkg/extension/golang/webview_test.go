package golang

import (
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"

	"github.com/miru-project/miru-core/pkg/extension"
	grt "github.com/miru-project/miru-core/pkg/extension/golang/runtime"
	"github.com/miru-project/miru-core/pkg/network"
)

// webviewExtSource is a Go extension compiled and run by the Scriggo runtime.
// It calls rt.OpenWebView exactly the way a real Go extension would, so the
// test exercises the full Scriggo path (compile -> load -> call by name),
// including the struct return and the newly registered package declarations.
//
// Probe fetches https://tls.peet.ws/api/all through a RANDOM webview identity
// and returns the raw result; the fingerprint assertions live in the Go test
// (the Scriggo playground cannot reach network.KnownWebviewFingerprint).
const webviewExtSource = `package webviewext

import (
	rt "github.com/miru-project/miru-core/pkg/extension/golang/runtime"
)

func Load() {}

func Probe() (rt.WebviewResponse, error) {
	res := rt.OpenWebView("https://tls.peet.ws/api/all", "GET", nil, "")
	return res, nil
}
`

// scriggoWebviewRuntime writes the temporary Go extension into ExtensionDir,
// loads it into a real Scriggo VM and returns the Runtime so Probe can be
// invoked by name.
func scriggoWebviewRuntime(t *testing.T) *Runtime {
	t.Helper()
	dir := t.TempDir()
	extPath := dir + "/webviewext.go"
	if err := os.WriteFile(extPath, []byte(webviewExtSource), 0644); err != nil {
		t.Fatalf("write extension source: %v", err)
	}
	ExtensionDir = dir

	rt := NewRuntime(NewScriggoVM(nil))
	if err := rt.LoadExtension(&extension.Extension{Name: "webviewext", Pkg: "webviewext"}); err != nil {
		t.Fatalf("LoadExtension failed: %v", err)
	}
	return rt
}

// webviewNetworkAvailable reports whether the live fingerprint probe host is
// reachable (the LAN resolver here can take >5s to answer, hence the generous
// timeout), so the test skips instead of failing offline.
func webviewNetworkAvailable(t *testing.T) bool {
	t.Helper()
	conn, err := net.DialTimeout("tcp", "tls.peet.ws:443", 15*time.Second)
	if err != nil {
		t.Logf("network unreachable, skipping live webview test: %v", err)
		return false
	}
	conn.Close()
	return true
}

// TestOpenWebViewRandomSpoofFingerprint runs OpenWebView inside the Scriggo
// runtime with a RANDOM identity and asserts the probe server observed exactly
// that browser: the echoed User-Agent matches the identity and the reported
// JA3 hash / JA4 match the recorded fingerprint of the picked profile (i.e.
// the TLS ClientHello was impersonated, not the Go default).
func TestOpenWebViewRandomSpoofFingerprint(t *testing.T) {
	if !webviewNetworkAvailable(t) {
		t.Skip("requires network access to tls.peet.ws")
	}
	network.Init()
	rt := scriggoWebviewRuntime(t)

	res, err := rt.Call("Probe")
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	wr, ok := res.(grt.WebviewResponse)
	if !ok {
		t.Fatalf("expected grt.WebviewResponse, got %T", res)
	}
	if wr.Err != "" {
		t.Fatalf("OpenWebView error: %s", wr.Err)
	}
	if wr.Status != 200 {
		t.Fatalf("expected 200, got %d", wr.Status)
	}
	if wr.Profile == "" || wr.UserAgent == "" {
		t.Fatalf("identity not returned: profile=%q ua=%q", wr.Profile, wr.UserAgent)
	}

	var peet struct {
		HTTPVersion string `json:"http_version"`
		UserAgent   string `json:"user_agent"`
		TLS         struct {
			JA3Hash string `json:"ja3_hash"`
			JA4     string `json:"ja4"`
		} `json:"tls"`
	}
	if err := json.Unmarshal([]byte(wr.Body), &peet); err != nil {
		t.Fatalf("decode peet.ws payload: %v", err)
	}
	if peet.UserAgent != wr.UserAgent {
		t.Fatalf("server saw user agent %q, expected identity %q", peet.UserAgent, wr.UserAgent)
	}
	ja3, ja4, ok := network.KnownWebviewFingerprint(wr.Profile)
	if !ok {
		t.Fatalf("identity profile %q unknown", wr.Profile)
	}
	if peet.TLS.JA3Hash != ja3 || peet.TLS.JA4 != ja4 {
		t.Fatalf("profile %s: server saw ja3_hash %q / ja4 %q, expected %q / %q",
			wr.Profile, peet.TLS.JA3Hash, peet.TLS.JA4, ja3, ja4)
	}
	if peet.HTTPVersion != "h2" {
		t.Fatalf("profile %s: expected h2 impersonation, got http_version %q", wr.Profile, peet.HTTPVersion)
	}
	t.Logf("spoofed identity: profile=%s ua=%q", wr.Profile, wr.UserAgent)
}
