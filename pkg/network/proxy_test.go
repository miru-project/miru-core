package network

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/miru-project/miru-core/config"
	"github.com/miru-project/miru-core/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/proxy"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func initTestDB(t *testing.T) {
	t.Helper()
	tmpFile := t.TempDir() + "/test.db"
	config.Global.Database.Driver = "sqlite3"
	config.Global.Database.DBName = tmpFile
	db.Initialize()
	db.SetAppSetting("ProxyActivate", "true")
}

func stripScheme(raw string) string {
	return strings.TrimPrefix(strings.TrimPrefix(raw, "https://"), "http://")
}

// ---------------------------------------------------------------------------
// getProxyURL unit tests
// ---------------------------------------------------------------------------

func TestGetProxyURLFromOption(t *testing.T) {
	cases := []struct {
		name string
		opt  *RequestOptions
		want string
	}{
		{"default scheme", &RequestOptions{ProxyHost: "1.2.3.4:8080"}, "http://1.2.3.4:8080"},
		{"socks5 preserved", &RequestOptions{ProxyScheme: "socks5", ProxyHost: "1.2.3.4:1080"}, "socks5://1.2.3.4:1080"},
		{"socks4 preserved", &RequestOptions{ProxyScheme: "socks4", ProxyHost: "1.2.3.4:1080"}, "socks4://1.2.3.4:1080"},
		{"socks4a preserved", &RequestOptions{ProxyScheme: "socks4a", ProxyHost: "1.2.3.4:1080"}, "socks4a://1.2.3.4:1080"},
		{"socks5h preserved", &RequestOptions{ProxyScheme: "socks5h", ProxyHost: "1.2.3.4:1080"}, "socks5h://1.2.3.4:1080"},
		{"https preserved", &RequestOptions{ProxyScheme: "https", ProxyHost: "proxy.example.com:443"}, "https://proxy.example.com:443"},
		{"with creds", &RequestOptions{ProxyHost: "proxy.example.com:3128", ProxyUserName: "u", ProxyPassword: "p"}, "http://u:p@proxy.example.com:3128"},
		{"socks5 with creds", &RequestOptions{ProxyScheme: "socks5", ProxyHost: "1.2.3.4:1080", ProxyUserName: "user", ProxyPassword: "pass"}, "socks5://user:pass@1.2.3.4:1080"},
		{"nil option", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := getProxyURL(c.opt)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestProxySchemeSingleSourceOfTruth(t *testing.T) {
	schemes := []string{"http", "https", "socks4", "socks4a", "socks5", "socks5h"}
	for _, scheme := range schemes {
		t.Run(scheme, func(t *testing.T) {
			opt := &RequestOptions{ProxyScheme: scheme, ProxyHost: "1.2.3.4:9090"}
			got := getProxyURL(opt)
			gotScheme := strings.SplitN(got, "://", 2)[0]
			assert.Equal(t, scheme, gotScheme)
		})
	}
}

func TestGetProxyURLFromDBPreservesScheme(t *testing.T) {
	initTestDB(t)
	db.SetAppSetting("Proxy", "socks5://1.2.3.4:1080")
	got := getProxyURL(nil)
	assert.Equal(t, "socks5://1.2.3.4:1080", got)
}

// ---------------------------------------------------------------------------
// PrepareProxy dialer selection
// ---------------------------------------------------------------------------

func TestPrepareProxyHTTPDialer(t *testing.T) {
	initTestDB(t)
	opt := &RequestOptions{ProxyScheme: "http", ProxyHost: "127.0.0.1:1"}
	client, err := PrepareProxy(opt, "http://example.com")
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.NotNil(t, client.Dial)
}

func TestPrepareProxyHTTPSProxyDialer(t *testing.T) {
	// HTTPS proxy (TLS to the proxy itself) is handled through the default
	// HTTP CONNECT tunnel path. Verify it produces a non-nil Dial func.
	initTestDB(t)
	opt := &RequestOptions{ProxyScheme: "https", ProxyHost: "127.0.0.1:4443"}
	client, err := PrepareProxy(opt, "http://example.com")
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.NotNil(t, client.Dial)
}

func TestPrepareProxySOCKS4Dialer(t *testing.T) {
	initTestDB(t)
	opt := &RequestOptions{ProxyScheme: "socks4", ProxyHost: "127.0.0.1:1080"}
	client, err := PrepareProxy(opt, "http://example.com")
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.NotNil(t, client.Dial)
}

func TestPrepareProxySOCKS4ADialer(t *testing.T) {
	initTestDB(t)
	opt := &RequestOptions{ProxyScheme: "socks4a", ProxyHost: "127.0.0.1:1080"}
	client, err := PrepareProxy(opt, "http://example.com")
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.NotNil(t, client.Dial)
}

func TestPrepareProxySOCKS5Dialer(t *testing.T) {
	initTestDB(t)
	opt := &RequestOptions{ProxyScheme: "socks5", ProxyHost: "127.0.0.1:1080"}
	client, err := PrepareProxy(opt, "http://example.com")
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.NotNil(t, client.Dial)
}

func TestPrepareProxyCaching(t *testing.T) {
	initTestDB(t)
	opt := &RequestOptions{ProxyScheme: "socks5", ProxyHost: "127.0.0.1:1080"}
	c1, err := PrepareProxy(opt, "http://a.example.com")
	require.NoError(t, err)
	c2, err := PrepareProxy(opt, "http://b.example.com")
	require.NoError(t, err)
	assert.Same(t, c1, c2)
}

func TestNewSocks5FasthttpDialerInvalidURL(t *testing.T) {
	_, err := newSocks5FasthttpDialer("://invalid")
	assert.Error(t, err)
}

func TestPrepareRequestSetsHeaders(t *testing.T) {
	initTestDB(t)
	Init() // initialize cookie jar and default client
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	option := &RequestOptions{
		Method: "POST",
		Headers: map[string]string{
			"X-Custom": "test-value",
			"Cookie":   "session=abc123",
		},
		RequestBody: "body-data",
	}

	client, err := prepareRequest(req, "http://example.com/api", option)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "POST", string(req.Header.Method()))
	assert.Equal(t, "test-value", string(req.Header.Peek("X-Custom")))
	assert.Equal(t, "body-data", string(req.Body()))
}

func TestPrepareProxySOCKS4WithUserID(t *testing.T) {
	initTestDB(t)
	opt := &RequestOptions{
		ProxyScheme:   "socks4",
		ProxyHost:     "127.0.0.1:1080",
		ProxyUserName: "testuser",
	}
	client, err := PrepareProxy(opt, "http://example.com")
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.NotNil(t, client.Dial)
}

// ---------------------------------------------------------------------------
// SOCKS5 test server
// ---------------------------------------------------------------------------

type socks5TestServer struct {
	listener net.Listener
	ch       chan struct{}
}

func (s *socks5TestServer) Addr() string { return s.listener.Addr().String() }
func (s *socks5TestServer) Close() error { return s.listener.Close() }

func startSOCKS5Proxy(t *testing.T) (*socks5TestServer, <-chan struct{}) {
	t.Helper()
	ch := make(chan struct{}, 1)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &socks5TestServer{listener: listener, ch: ch}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go s.handleConn(conn)
		}
	}()
	return s, ch
}

func (s *socks5TestServer) handleConn(conn net.Conn) {
	defer conn.Close()

	// Greeting
	buf := make([]byte, 258)
	n, err := conn.Read(buf)
	if err != nil || n < 3 {
		return
	}
	conn.Write([]byte{0x05, 0x00})

	// Request
	n, err = conn.Read(buf)
	if err != nil || n < 4 {
		return
	}
	if buf[1] != 0x01 {
		conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	var targetAddr string
	switch buf[3] {
	case 0x01:
		if n < 10 {
			return
		}
		targetAddr = fmt.Sprintf("%d.%d.%d.%d:%d", buf[4], buf[5], buf[6], buf[7], uint16(buf[8])<<8|uint16(buf[9]))
	case 0x03:
		domainLen := int(buf[4])
		if n < 5+domainLen+2 {
			return
		}
		domain := string(buf[5 : 5+domainLen])
		port := uint16(buf[5+domainLen])<<8 | uint16(buf[5+domainLen+1])
		targetAddr = fmt.Sprintf("%s:%d", domain, port)
	default:
		conn.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	target, err := net.DialTimeout("tcp", targetAddr, 5*time.Second)
	if err != nil {
		conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer target.Close()

	localAddr := target.LocalAddr().(*net.TCPAddr)
	conn.Write([]byte{0x05, 0x00, 0x00, 0x01,
		byte(localAddr.IP.To4()[0]), byte(localAddr.IP.To4()[1]),
		byte(localAddr.IP.To4()[2]), byte(localAddr.IP.To4()[3]),
		byte(localAddr.Port >> 8), byte(localAddr.Port),
	})

	s.ch <- struct{}{}

	done := make(chan struct{})
	go func() {
		ioCopy(target, conn)
		close(done)
	}()
	ioCopy(conn, target)
	<-done
}

// ---------------------------------------------------------------------------
// SOCKS4 test server
// ---------------------------------------------------------------------------

type socks4TestServer struct {
	listener net.Listener
	ch       chan struct{}
}

func (s *socks4TestServer) Addr() string { return s.listener.Addr().String() }
func (s *socks4TestServer) Close() error { return s.listener.Close() }

func startSOCKS4Proxy(t *testing.T) (*socks4TestServer, <-chan struct{}) {
	t.Helper()
	ch := make(chan struct{}, 1)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &socks4TestServer{listener: listener, ch: ch}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go s.handleConn(conn)
		}
	}()
	return s, ch
}

func (s *socks4TestServer) handleConn(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil || n < 9 || buf[0] != 0x04 {
		return
	}
	port := uint16(buf[2])<<8 | uint16(buf[3])
	ip := net.IPv4(buf[4], buf[5], buf[6], buf[7])
	targetAddr := fmt.Sprintf("%s:%d", ip.String(), port)

	target, err := net.DialTimeout("tcp", targetAddr, 5*time.Second)
	if err != nil {
		conn.Write([]byte{0x00, 0x5B, 0, 0, 0, 0, 0, 0})
		return
	}
	defer target.Close()

	conn.Write([]byte{0x00, 0x5A, 0, 0, 0, 0, 0, 0})
	s.ch <- struct{}{}

	done := make(chan struct{})
	go func() {
		ioCopy(target, conn)
		close(done)
	}()
	ioCopy(conn, target)
	<-done
}

func ioCopy(dst, src net.Conn) {
	buf := make([]byte, 4096)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			dst.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

// ---------------------------------------------------------------------------
// HTTP CONNECT proxy test server
// ---------------------------------------------------------------------------

func startHTTPProxy(t *testing.T) (*httptest.Server, <-chan struct{}) {
	t.Helper()
	ch := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			ch <- struct{}{}
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "hijack not supported", 500)
				return
			}
			w.WriteHeader(http.StatusOK)
			clientConn, _, err := hijacker.Hijack()
			if err != nil {
				return
			}
			defer clientConn.Close()

			targetConn, err := net.DialTimeout("tcp", r.Host, 5*time.Second)
			if err != nil {
				return
			}
			defer targetConn.Close()

			done := make(chan struct{})
			go func() {
				ioCopy(targetConn, clientConn)
				close(done)
			}()
			ioCopy(clientConn, targetConn)
			<-done
		}
	}))
	return server, ch
}

// ---------------------------------------------------------------------------
// Integration: fasthttp transport
// ---------------------------------------------------------------------------

func TestFastHTTPThroughHTTPProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	proxyServer, proxyConn := startHTTPProxy(t)
	defer proxyServer.Close()

	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello")
	}))
	defer targetServer.Close()

	initTestDB(t)
	opt := &RequestOptions{
		ProxyScheme: "http",
		ProxyHost:   proxyServer.Listener.Addr().String(),
	}
	client, err := PrepareProxy(opt, targetServer.URL)
	require.NoError(t, err)

	req := fasthttp.AcquireRequest()
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)

	req.SetRequestURI(targetServer.URL)
	req.Header.SetMethod("GET")

	err = client.Do(req, res)
	require.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())

	select {
	case <-proxyConn:
	case <-time.After(5 * time.Second):
		t.Fatal("proxy did not receive connection in time")
	}
}

func TestFastHTTPThroughSOCKS5(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	socksServer, socksConn := startSOCKS5Proxy(t)
	defer socksServer.Close()

	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "fasthttp via socks5")
	}))
	defer targetServer.Close()

	initTestDB(t)
	opt := &RequestOptions{
		ProxyScheme: "socks5",
		ProxyHost:   socksServer.Addr(),
	}
	client, err := PrepareProxy(opt, targetServer.URL)
	require.NoError(t, err)

	req := fasthttp.AcquireRequest()
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)

	req.SetRequestURI(targetServer.URL)
	req.Header.SetMethod("GET")

	err = client.Do(req, res)
	require.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())
	assert.Contains(t, string(res.Body()), "fasthttp via socks5")

	select {
	case <-socksConn:
	case <-time.After(5 * time.Second):
		t.Fatal("SOCKS5 proxy did not receive connection in time")
	}
}

func TestFastHTTPThroughSOCKS4(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	socksServer, socksConn := startSOCKS4Proxy(t)
	defer socksServer.Close()

	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "fasthttp via socks4")
	}))
	defer targetServer.Close()

	initTestDB(t)
	opt := &RequestOptions{
		ProxyScheme: "socks4",
		ProxyHost:   socksServer.Addr(),
	}
	client, err := PrepareProxy(opt, targetServer.URL)
	require.NoError(t, err)

	req := fasthttp.AcquireRequest()
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)

	req.SetRequestURI(targetServer.URL)
	req.Header.SetMethod("GET")

	err = client.Do(req, res)
	require.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())
	assert.Contains(t, string(res.Body()), "fasthttp via socks4")

	select {
	case <-socksConn:
	case <-time.After(5 * time.Second):
		t.Fatal("SOCKS4 proxy did not receive connection in time")
	}
}

// ---------------------------------------------------------------------------
// Integration: tls_client transport
// ---------------------------------------------------------------------------

func TestTLSClientThroughHTTPProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	proxyServer, _ := startHTTPProxy(t)
	defer proxyServer.Close()

	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "tls-client via http proxy")
	}))
	defer targetServer.Close()

	initTestDB(t)
	Init()
	opt := &RequestOptions{
		ProxyScheme: "http",
		ProxyHost:   strings.TrimPrefix(proxyServer.URL, "http://"),
		TLSConfig:   &TLSConfig{Profile: "chrome_120"},
	}
	resp, err := FetchString(targetServer.URL, opt, ReadAll)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, resp.Body, "tls-client via http proxy")
}

func TestTLSClientThroughSOCKS5(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	socksServer, socksConn := startSOCKS5Proxy(t)
	defer socksServer.Close()

	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "tls-client via socks5")
	}))
	defer targetServer.Close()

	initTestDB(t)
	Init()
	opt := &RequestOptions{
		ProxyScheme: "socks5",
		ProxyHost:   socksServer.Addr(),
		TLSConfig:   &TLSConfig{Profile: "chrome_120"},
	}
	resp, err := FetchString(targetServer.URL, opt, ReadAll)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, resp.Body, "tls-client via socks5")

	select {
	case <-socksConn:
	case <-time.After(5 * time.Second):
		t.Fatal("SOCKS5 proxy did not receive connection in time")
	}
}

func TestTLSClientThroughSOCKS4(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	socksServer, socksConn := startSOCKS4Proxy(t)
	defer socksServer.Close()

	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "tls-client via socks4")
	}))
	defer targetServer.Close()

	initTestDB(t)
	Init()
	opt := &RequestOptions{
		ProxyScheme: "socks4",
		ProxyHost:   socksServer.Addr(),
		TLSConfig:   &TLSConfig{Profile: "chrome_120"},
	}
	resp, err := FetchString(targetServer.URL, opt, ReadAll)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, resp.Body, "tls-client via socks4")

	select {
	case <-socksConn:
	case <-time.After(5 * time.Second):
		t.Fatal("SOCKS4 proxy did not receive connection in time")
	}
}

// ---------------------------------------------------------------------------
// newSocks5FasthttpDialer e2e
// ---------------------------------------------------------------------------

func TestNewSocks5FasthttpDialerEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	socksServer, _ := startSOCKS5Proxy(t)
	defer socksServer.Close()

	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok")
	}))
	defer targetServer.Close()

	dialFunc, err := newSocks5FasthttpDialer("socks5://" + socksServer.Addr())
	require.NoError(t, err)

	client := &fasthttp.Client{
		Dial:            dialFunc,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		MaxConnsPerHost: 1,
	}

	req := fasthttp.AcquireRequest()
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)

	req.SetRequestURI(targetServer.URL)
	err = client.Do(req, res)
	require.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode())
	assert.Equal(t, "ok", string(res.Body()))
}

// ---------------------------------------------------------------------------
// golang.org/x/net/proxy.SOCKS5 direct verification
// ---------------------------------------------------------------------------

func TestSOCKS5ProxyDialerUnit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	socksServer, _ := startSOCKS5Proxy(t)
	defer socksServer.Close()

	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "dialed-through-socks5")
	}))
	defer targetServer.Close()

	dialer, err := proxy.SOCKS5("tcp", socksServer.Addr(), nil, &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 5 * time.Second,
	})
	require.NoError(t, err)

	conn, err := dialer.Dial("tcp", stripScheme(targetServer.URL))
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: " + stripScheme(targetServer.URL) + "\r\nConnection: close\r\n\r\n"))
	require.NoError(t, err)

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	require.NoError(t, err)
	assert.Contains(t, string(buf[:n]), "dialed-through-socks5")
}
