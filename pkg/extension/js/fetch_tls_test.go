package js

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/dop251/goja_nodejs/require"
	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/pkg/network"
)

// peetResponse is a minimal view of the https://tls.peet.ws/api/all response,
// exposing exactly the fields needed to prove which transport / TLS profile was
// used to make the request.
type peetResponse struct {
	HTTPVersion string `json:"http_version"`
	UserAgent   string `json:"user_agent"`
	TLS         struct {
		Extensions []struct {
			Name      string   `json:"name"`
			Protocols []string `json:"protocols"`
		} `json:"extensions"`
	} `json:"tls"`
}

func (p *peetResponse) alpnProtocols() []string {
	for _, ext := range p.TLS.Extensions {
		if ext.Name == "application_layer_protocol_negotiation (16)" {
			return ext.Protocols
		}
	}
	return nil
}

// fetchEnvelope is the shape returned by the JS __fetch helper: the raw server
// body is nested (escaped) inside the "data" field.
type fetchEnvelope struct {
	Data string `json:"data"`
}

// parsePeet decodes the __fetch envelope and returns the peet.ws payload.
func parsePeet(t *testing.T, raw string) *peetResponse {
	t.Helper()
	var env fetchEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("decode fetch envelope: %v", err)
	}
	var resp peetResponse
	if err := json.Unmarshal([]byte(env.Data), &resp); err != nil {
		t.Fatalf("decode peet.ws payload: %v", err)
	}
	return &resp
}

// newFetchVM builds a goja VM with the __fetch function registered, exactly the
// way a JS extension would see it: inside a running event loop and with a Job
// that has a live loop (initFetch's async channel requires it).
func newFetchVM(t *testing.T) (*goja.Runtime, *eventloop.EventLoop) {
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
		api.initFetch(vm, job)
	})
	return vm, loop
}

// requestPeet calls __fetch against https://tls.peet.ws/api/all, optionally
// with a tls_config option, and returns the parsed response. The script runs on
// the extension's event loop and the result is bridged back to the test goroutine.
func requestPeet(t *testing.T, loop *eventloop.EventLoop, tlsConfig string) *peetResponse {
	t.Helper()
	script := `
(async () => {
	const opts = ` + tlsConfig + `;
	const resp = await __fetch("https://tls.peet.ws/api/all", opts);
	return JSON.stringify(resp);
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
			done <- result{err: fmt.Errorf("fetch rejected: %v", call.Argument(0))}
			return goja.Undefined()
		}))
		if err != nil {
			done <- result{err: err}
		}
	})
	r := <-done
	if r.err != nil {
		t.Fatalf("fetch: %v", r.err)
	}
	return parsePeet(t, r.text)
}

// TestFetchWithoutTLS verifies the default (fasthttp) transport is used when no
// tls_config is supplied: it speaks HTTP/1.1 and does NOT advertise the h2 ALPN
// protocol (no browser impersonation).
func TestFetchWithoutTLS(t *testing.T) {
	network.Init()
	vm, loop := newFetchVM(t)
	_ = vm
	resp := requestPeet(t, loop, "undefined")

	if resp.HTTPVersion != "HTTP/1.1" {
		t.Fatalf("expected HTTP/1.1 without tls_config, got %q", resp.HTTPVersion)
	}
	for _, p := range resp.alpnProtocols() {
		if p == "h2" {
			t.Fatalf("did not expect h2 ALPN without tls_config, got %v", resp.alpnProtocols())
		}
	}
}

// TestFetchWithTLS verifies that supplying tls_config routes the request through
// the browser-impersonating tls-client and that the intended profile is actually
// applied: the server must observe an HTTP/2 (h2) ALPN negotiation, which only
// the chrome_133 fingerprint produces.
func TestFetchWithTLS(t *testing.T) {
	network.Init()
	vm, loop := newFetchVM(t)
	_ = vm
	const profile = "chrome_133"
	resp := requestPeet(t, loop, `{"tls_config": {"profile": "`+profile+`"}}`)

	var hasH2 bool
	for _, p := range resp.alpnProtocols() {
		if p == "h2" {
			hasH2 = true
		}
	}
	if !hasH2 {
		t.Fatalf("expected h2 ALPN negotiation for profile %q, got protocols %v (http_version=%q)",
			profile, resp.alpnProtocols(), resp.HTTPVersion)
	}
}
