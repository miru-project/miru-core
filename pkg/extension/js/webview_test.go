package js

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/dop251/goja_nodejs/require"
	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/pkg/network"
)

// peetWebview mirrors the fields of https://tls.peet.ws/api/all the webview
// tests verify (same shape as the network package's expectations).
type peetWebview struct {
	HTTPVersion string `json:"http_version"`
	UserAgent   string `json:"user_agent"`
	TLS         struct {
		JA3Hash string `json:"ja3_hash"`
		JA4     string `json:"ja4"`
	} `json:"tls"`
}

// webviewNetworkAvailable reports whether the live fingerprint probe host is
// reachable (the LAN resolver here can take >5s to answer, hence the generous
// timeout), so tests skip instead of failing offline.
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

// newWebviewVM builds a goja VM inside a running event loop with the full host
// primitive set registered (registerFunction), exactly the way a JS extension
// sees it in AsyncCallBack.
func newWebviewVM(t *testing.T) (*goja.Runtime, *eventloop.EventLoop) {
	t.Helper()
	sharedRegistry = require.NewRegistry()
	vm := goja.New()
	loop := eventloop.NewEventLoop()
	loop.Start()
	t.Cleanup(func() { loop.Stop() })
	job := &Job{loop: loop}
	api := &ExtApi{
		Ext:     &extension.Extension{Pkg: "testjs"},
		service: &ExtBaseService{},
	}
	loop.RunOnLoop(func(vm *goja.Runtime) {
		api.registerFunction(vm, *job)
	})
	return vm, loop
}

// callOpenWebView invokes the global openWebView host primitive against
// https://tls.peet.ws/api/all with the given JS options object and returns the
// raw result envelope as JSON text.
func callOpenWebView(t *testing.T, loop *eventloop.EventLoop, options string) string {
	t.Helper()
	script := `
(async () => {
	const res = await openWebView("https://tls.peet.ws/api/all", ` + options + `);
	return JSON.stringify(res);
})()
`
	type result struct {
		text string
		err  error
	}
	done := make(chan result, 1)
	loop.RunOnLoop(func(vm *goja.Runtime) {
		v, err := vm.RunString(script)
		if err != nil {
			done <- result{err: err}
			return
		}
		obj, ok := v.(*goja.Object)
		if !ok {
			done <- result{err: fmt.Errorf("expected promise, got %T", v)}
			return
		}
		then, ok := goja.AssertFunction(obj.Get("then"))
		if !ok {
			done <- result{err: fmt.Errorf("value is not thenable")}
			return
		}
		_, err = then(obj, vm.ToValue(func(call goja.FunctionCall) goja.Value {
			done <- result{text: call.Argument(0).String()}
			return goja.Undefined()
		}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
			done <- result{err: fmt.Errorf("openWebView rejected: %v", call.Argument(0))}
			return goja.Undefined()
		}))
		if err != nil {
			done <- result{err: err}
		}
	})
	r := <-done
	if r.err != nil {
		t.Fatalf("openWebView: %v", r.err)
	}
	return r.text
}

// TestOpenWebViewRandomSpoofFingerprint is the live one-shot spoof check for
// the JS host primitive: one RANDOM identity, and the probe server must report
// exactly that browser (User-Agent echo + recorded JA3 hash / JA4 of the
// profile), proving the TLS ClientHello was impersonated, not the Go default.
func TestOpenWebViewRandomSpoofFingerprint(t *testing.T) {
	if !webviewNetworkAvailable(t) {
		t.Skip("requires network access to tls.peet.ws")
	}
	network.Init()
	vm, loop := newWebviewVM(t)
	_ = vm

	var envelope struct {
		Status   int    `json:"status"`
		Data     string `json:"data"`
		Identity struct {
			Profile   string `json:"profile"`
			UserAgent string `json:"userAgent"`
		} `json:"identity"`
	}
	if err := json.Unmarshal([]byte(callOpenWebView(t, loop, "undefined")), &envelope); err != nil {
		t.Fatalf("decode openWebView envelope: %v", err)
	}
	if envelope.Status != 200 {
		t.Fatalf("expected 200, got %d", envelope.Status)
	}

	var peet peetWebview
	if err := json.Unmarshal([]byte(envelope.Data), &peet); err != nil {
		t.Fatalf("decode peet.ws payload: %v", err)
	}
	if peet.UserAgent != envelope.Identity.UserAgent {
		t.Fatalf("server saw user agent %q, expected identity %q", peet.UserAgent, envelope.Identity.UserAgent)
	}
	ja3, ja4, ok := network.KnownWebviewFingerprint(envelope.Identity.Profile)
	if !ok {
		t.Fatalf("identity profile %q unknown", envelope.Identity.Profile)
	}
	if peet.TLS.JA3Hash != ja3 || peet.TLS.JA4 != ja4 {
		t.Fatalf("profile %s: server saw ja3_hash %q / ja4 %q, expected %q / %q",
			envelope.Identity.Profile, peet.TLS.JA3Hash, peet.TLS.JA4, ja3, ja4)
	}
	if peet.HTTPVersion != "h2" {
		t.Fatalf("profile %s: expected h2 impersonation, got http_version %q",
			envelope.Identity.Profile, peet.HTTPVersion)
	}
	t.Logf("spoofed identity: profile=%s ua=%q", envelope.Identity.Profile, envelope.Identity.UserAgent)
}

// TestOpenWebViewRuntimeSurface checks offline that both base runtimes expose
// openWebView: Miru.openWebView on V1 and V2, and ext.openWebView on V1.
func TestOpenWebViewRuntimeSurface(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		script, err := AssetsFS.ReadFile("assets/runtime_" + version + ".js")
		if err != nil {
			t.Fatalf("read runtime_%s.js: %v", version, err)
		}
		vm := goja.New()
		if _, err := vm.RunString(string(script)); err != nil {
			t.Fatalf("eval runtime_%s.js: %v", version, err)
		}
		miru, err := vm.RunString(`typeof Miru.openWebView === "function"`)
		if err != nil {
			t.Fatalf("runtime_%s: check Miru.openWebView: %v", version, err)
		}
		if !miru.ToBoolean() {
			t.Fatalf("runtime_%s: Miru.openWebView is not exposed", version)
		}
		if version == "v1" {
			ext, err := vm.RunString(`typeof (new Extension("https://example.com")).openWebView === "function"`)
			if err != nil {
				t.Fatalf("runtime_v1: check ext.openWebView: %v", err)
			}
			if !ext.ToBoolean() {
				t.Fatal("runtime_v1: ext.openWebView is not exposed")
			}
		}
	}
}
