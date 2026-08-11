package golang

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/pkg/network"
)

// fetchExtSource is a Go extension compiled and run by the Scriggo runtime. It
// imports the runtime package exposed to extensions (via its full module path,
// which is unambiguous and avoids colliding with the Go standard library
// "runtime" package) and calls rt.Fetch exactly the way a real Go extension
// would, so the test exercises the full Scriggo path (compile -> load -> call
// by name).
//
// Check fetches https://tls.peet.ws/api/all, optionally with the given TLS
// profile (an empty string disables the TLS config), and returns an error if
// the server-observed fingerprint does not match what the requested profile
// should produce:
//   - no TLS config -> plain HTTP/1.1, no h2 ALPN (default fasthttp transport)
//   - chrome_133    -> an HTTP/2 (h2) ALPN negotiation (tls-client impersonation)
const fetchExtSource = `package fetchext

import (
	"encoding/json"
	"errors"

	rt "github.com/miru-project/miru-core/pkg/extension/golang/runtime"
)

func Load() {}

func Check(profile string) error {
	var tls *rt.TLSConfig
	if profile != "" {
		tls = &rt.TLSConfig{Profile: profile}
	}
	body, status, errStr := rt.Fetch("https://tls.peet.ws/api/all", "GET", nil, "", tls)
	if errStr != "" {
		return errors.New(errStr)
	}
	if status != 200 {
		return errors.New("unexpected status code")
	}
	var resp struct {
		HTTPVersion string ` + "`json:\"http_version\"`" + `
		TLS         struct {
			Extensions []struct {
				Name      string   ` + "`json:\"name\"`" + `
				Protocols []string ` + "`json:\"protocols\"`" + `
			} ` + "`json:\"extensions\"`" + `
		} ` + "`json:\"tls\"`" + `
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return err
	}
	var hasH2 bool
	for _, ext := range resp.TLS.Extensions {
		if ext.Name == "application_layer_protocol_negotiation (16)" {
			for _, p := range ext.Protocols {
				if p == "h2" {
					hasH2 = true
				}
			}
		}
	}
	if profile == "" {
		if resp.HTTPVersion != "HTTP/1.1" {
			return errors.New("without TLS config expected HTTP/1.1, got " + resp.HTTPVersion)
		}
		if hasH2 {
			return errors.New("without TLS config did not expect h2 ALPN")
		}
		return nil
	}
	if !hasH2 {
		return errors.New("with TLS profile " + profile + " expected h2 ALPN but got none")
	}
	return nil
}
`

// scriggoFetchRuntime writes the temporary Go extension (which calls
// runtime.Fetch) into ExtensionDir, loads it into a real Scriggo VM and returns
// the Runtime so individual functions can be invoked by name.
func scriggoFetchRuntime(t *testing.T) *Runtime {
	t.Helper()
	dir := t.TempDir()
	extPath := filepath.Join(dir, "fetchext.go")
	if err := os.WriteFile(extPath, []byte(fetchExtSource), 0644); err != nil {
		t.Fatalf("write extension source: %v", err)
	}
	ExtensionDir = dir

	rt := NewRuntime(NewScriggoVM(nil))
	if err := rt.LoadExtension(&extension.Extension{Name: "fetchext", Pkg: "fetchext"}); err != nil {
		t.Fatalf("LoadExtension failed: %v", err)
	}
	return rt
}

// networkAvailable reports whether the test environment can reach the public
// fetch probe host. The fetch tests depend on a live external endpoint
// (tls.peet.ws), so they are skipped when offline instead of failing.
func networkAvailable(t *testing.T) bool {
	t.Helper()
	conn, err := net.DialTimeout("tcp", "tls.peet.ws:443", 2*time.Second)
	if err != nil {
		t.Logf("network unreachable, skipping live fetch test: %v", err)
		return false
	}
	conn.Close()
	return true
}

// TestFetchWithoutTLS runs runtime.Fetch inside the Scriggo runtime with no TLS
// config and asserts the default (fasthttp) transport is used: HTTP/1.1 and no
// h2 ALPN negotiation (no browser impersonation).
func TestFetchWithoutTLS(t *testing.T) {
	if !networkAvailable(t) {
		t.Skip("requires network access to tls.peet.ws")
	}
	network.Init()
	rt := scriggoFetchRuntime(t)
	if _, err := rt.Call("Check", ""); err != nil {
		t.Fatalf("Check (no TLS): %v", err)
	}
}

// TestFetchWithTLS runs runtime.Fetch inside the Scriggo runtime with a TLS
// config and asserts the intended profile is actually applied: the server must
// observe an HTTP/2 (h2) ALPN negotiation, which only the chrome_133 fingerprint
// produces.
func TestFetchWithTLS(t *testing.T) {
	if !networkAvailable(t) {
		t.Skip("requires network access to tls.peet.ws")
	}
	network.Init()
	rt := scriggoFetchRuntime(t)
	if _, err := rt.Call("Check", "chrome_133"); err != nil {
		t.Fatalf("Check (chrome_133): %v", err)
	}
}
