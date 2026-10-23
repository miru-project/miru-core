package network

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/miru-project/miru-core/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rawLineServer starts a TCP listener that answers every request with a fixed
// 200 response and reports the raw request line it received over the channel.
// It exists to prove what actually goes on the wire (percent-encoding, header
// casing) rather than what a parsed http.Request looks like.
func rawLineServer(t *testing.T) (url string, lines <-chan string, close func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	linesCh := make(chan string, 8)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				line, _ := r.ReadString('\n')
				select {
				case linesCh <- line:
				default:
				}
				for {
					l, err := r.ReadString('\n')
					if err != nil || l == "\r\n" {
						break
					}
				}
				fmt.Fprint(c, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")
			}(c)
		}
	}()
	return "http://" + ln.Addr().String(), linesCh, func() { ln.Close() }
}

// TestTLSClientEncodesNonASCIIRequestURI proves both transports percent-encode
// non-ASCII bytes in the request line: Cloudflare answers 400 for raw
// multi-byte UTF-8 in the request target, and extensions interpolate raw
// keywords into search URLs.
func TestTLSClientEncodesNonASCIIRequestURI(t *testing.T) {
	Init()
	base, lines, stop := rawLineServer(t)
	defer stop()
	url := base + "/search?kw=日本語"

	// Plain fasthttp transport.
	_, err := Request[string](url, &RequestOptions{Headers: map[string]string{}}, ReadAll)
	require.NoError(t, err)
	select {
	case line := <-lines:
		assert.Contains(t, line, "kw=%E6%97%A5%E6%9C%AC%E8%AA%9E", "fasthttp request line must be percent-encoded: %q", line)
		assert.NotContains(t, line, "日本語")
	case <-time.After(2 * time.Second):
		t.Fatal("no request reached the server on the fasthttp path")
	}

	// Browser-impersonating tls-client transport.
	_, _, err = OpenWebview(url, &RequestOptions{Headers: map[string]string{}})
	require.NoError(t, err)
	select {
	case line := <-lines:
		assert.Contains(t, line, "kw=%E6%97%A5%E6%9C%AC%E8%AA%9E", "tls-client request line must be percent-encoded: %q", line)
		assert.NotContains(t, line, "日本語")
	case <-time.After(2 * time.Second):
		t.Fatal("no request reached the server on the tls-client path")
	}
}

// TestCallerUserAgentWins proves the User-Agent precedence: an explicit caller
// header beats the browser identity's UA on both transports, while a request
// without one gets a UA from the same browser family as the fingerprint.
func TestCallerUserAgentWins(t *testing.T) {
	Init()
	seenUA := make(chan string, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUA <- r.Header.Get("User-Agent")
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	callerUA := "MyExtensionUA/1.0"

	// Plain transport: caller header wins.
	_, err := Request[string](srv.URL, &RequestOptions{Headers: map[string]string{"User-Agent": callerUA}}, ReadAll)
	require.NoError(t, err)
	assert.Equal(t, callerUA, <-seenUA)

	// tls-client transport with a caller header: caller wins over the
	// profile's own UA.
	_, identity, err := OpenWebview(srv.URL, &RequestOptions{
		Headers:   map[string]string{"User-Agent": callerUA},
		TLSConfig: &TLSConfig{Profile: "chrome_150"},
	})
	require.NoError(t, err)
	assert.Equal(t, callerUA, <-seenUA)
	assert.Equal(t, callerUA, identity.UserAgent, "reported identity must reflect the UA actually sent")

	// tls-client transport without a caller header: the profile's own
	// coherent UA is used (fingerprint chrome_150, UA chrome_150).
	_, identity, err = OpenWebview(srv.URL, &RequestOptions{TLSConfig: &TLSConfig{Profile: "chrome_150"}})
	require.NoError(t, err)
	assert.Equal(t, coherentUserAgent("chrome_150"), <-seenUA)
	assert.Equal(t, coherentUserAgent("chrome_150"), identity.UserAgent)
}

// TestCoherentUserAgentForProfile pins the UA/fingerprint coherence rule: a
// non-curated tls-client profile must get a User-Agent from the same browser
// family, never a contradicting one.
func TestCoherentUserAgentForProfile(t *testing.T) {
	assert.Equal(t, defaultUserAgent, coherentUserAgent(""), "empty profile = tls-client default (Chrome)")
	assert.Equal(t, defaultUserAgent, coherentUserAgent("totally_unknown_profile"))
	assert.Contains(t, coherentUserAgent("firefox_133"), "Firefox",
		"firefox family profile must carry a Firefox UA")
	assert.Contains(t, coherentUserAgent("chrome_133"), "Chrome",
		"chrome family profile must carry a Chrome UA")
	assert.Contains(t, coherentUserAgent("safari_18_0"), "Safari",
		"safari family profile must carry a Safari UA")
}

// TestTLSClientEnforcesMaxBodySize proves the response body cap applies on the
// browser-impersonating transport too, not only on plain fasthttp.
func TestTLSClientEnforcesMaxBodySize(t *testing.T) {
	Init()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(make([]byte, 64))
	}))
	defer srv.Close()

	t.Cleanup(func() { SetMaxBodySize(DefaultMaxBodySize) })
	SetMaxBodySize(8)

	_, err := Request[string](srv.URL, &RequestOptions{Headers: map[string]string{}}, ReadAll)
	require.Error(t, err, "plain transport must enforce the body cap")
	assert.Contains(t, err.Error(), "too large")

	_, _, err = OpenWebview(srv.URL, &RequestOptions{Headers: map[string]string{}})
	require.Error(t, err, "browser-impersonating transport must enforce the body cap")
	assert.Contains(t, err.Error(), "too large")
}

// TestTLSClientRespectsProxyActivate proves the ProxyActivate toggle gates the
// browser-impersonating transport exactly like the fasthttp one: a configured
// proxy is used while activated and skipped once deactivated.
//
// The dead proxy is passed per-request (ProxyHost) instead of through the app
// setting because the ent client in this package is a singleton bound to the
// first test's temp DB, whose file is gone by the time later tests run — the
// ProxyActivate cache value is what proxyEnabled() actually reads.
func TestTLSClientRespectsProxyActivate(t *testing.T) {
	Init()
	t.Cleanup(func() {
		defer func() { _ = recover() }()
		_ = db.SetAppSetting("ProxyActivate", "true")
	})
	deadProxy := &RequestOptions{
		Headers:     map[string]string{},
		ProxyHost:   "127.0.0.1:1",
		ProxyScheme: "http",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// ProxyActivate=true (the package default) -> the dead proxy breaks the request.
	// The cache is what proxyEnabled() reads; the DB persist may be unavailable
	// (singleton client bound to a deleted temp file), which is fine here.
	_ = db.SetAppSetting("ProxyActivate", "true")
	require.True(t, proxyEnabled(), "precondition: proxy activated")
	_, _, err := OpenWebview(srv.URL, deadProxy)
	require.Error(t, err, "an active dead proxy must break the request")

	// ProxyActivate=false -> the proxy is skipped and the request succeeds.
	_ = db.SetAppSetting("ProxyActivate", "false")
	require.False(t, proxyEnabled(), "precondition: proxy deactivated")
	res, _, err := OpenWebview(srv.URL, deadProxy)
	require.NoError(t, err, "a deactivated proxy must be skipped")
	assert.Equal(t, 200, res.StatusCode)
}
