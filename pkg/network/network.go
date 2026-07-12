package network

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	tls_profiles "github.com/bogdanfinn/tls-client/profiles"
	"github.com/klauspost/compress/zstd"
	"github.com/miru-project/miru-core/pkg/logger"
	"github.com/valyala/fasthttp"
	"go.nhat.io/cookiejar"
)

var defaultClient *fasthttp.Client

type Response[T StringOrBytes] struct {
	Res        *fasthttp.Response
	Body       T
	StatusCode int
	Headers    map[string]string
}

// Request makes an HTTP request and returns the response as type T.
//
// Parameters:
//
//	url:           The URL to send the request to.
//	option:        Pointer to RequestOptions struct containing headers, method, proxy, etc.
//	readPreference: Function to read and process the response body (e.g., io.ReadAll,io.Read,).
//
// Returns:
//
//	The response body as type T (string or []byte), and an error if any occurred.
func Request[T StringOrBytes](url string, option *RequestOptions, readPreference func(*fasthttp.Response) ([]byte, error)) (Response[T], error) {

	// When a TLS config is provided, route through the browser-impersonating
	// tls-client. Otherwise use the lightweight fasthttp client. Both share the
	// same persistent cookie jar (jar). The underlying helpers are non-generic
	// (string body); we adapt the result to T here.
	var zero T
	str, err := FetchString(url, option, readPreference)
	if err != nil {
		return Response[T]{}, err
	}
	var body T
	switch any(zero).(type) {
	case string:
		body = any(str.Body).(T)
	case []byte:
		body = any([]byte(str.Body)).(T)
	}
	return Response[T]{
		Res:        str.Res,
		Body:       body,
		StatusCode: str.StatusCode,
		Headers:    str.Headers,
	}, nil
}

// FetchString is the non-generic string specialization of Request. It exists so
// the Scriggo (Go) extension runtime can invoke it: Scriggo's reflect-based
// callable cannot instantiate generic functions, so the golang runtime's Fetch
// delegates here instead of calling Request[string].
func FetchString(url string, option *RequestOptions, readPreference func(*fasthttp.Response) ([]byte, error)) (Response[string], error) {
	if option.TLSConfig != nil {
		return requestWithTLSClient(url, option)
	}
	return request(url, option, readPreference)
}

// jarAdapter adapts the persistent *cookiejar.PersistentJar (which speaks
// net/http.Cookie) to the fhttp.CookieJar interface expected by tls-client,
// converting between the standard net/http.Cookie and fhttp.Cookie
// representations. The same underlying persistent jar is used by the fasthttp
// transport, so cookies are shared across both transports.
type jarAdapter struct {
	inner *cookiejar.PersistentJar
}

func (a *jarAdapter) Cookies(u *url.URL) []*fhttp.Cookie {
	nh := a.inner.Cookies(u)
	out := make([]*fhttp.Cookie, 0, len(nh))
	for _, c := range nh {
		fc := &fhttp.Cookie{}
		fc.Name = c.Name
		fc.Value = c.Value
		fc.Path = c.Path
		fc.Domain = c.Domain
		fc.Expires = c.Expires
		fc.MaxAge = c.MaxAge
		fc.Secure = c.Secure
		fc.HttpOnly = c.HttpOnly
		fc.SameSite = fhttp.SameSite(c.SameSite)
		out = append(out, fc)
	}
	return out
}

func (a *jarAdapter) SetCookies(u *url.URL, cookies []*fhttp.Cookie) {
	nh := make([]*http.Cookie, 0, len(cookies))
	for _, c := range cookies {
		nh = append(nh, &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Domain:   c.Domain,
			Expires:  c.Expires,
			MaxAge:   c.MaxAge,
			Secure:   c.Secure,
			HttpOnly: c.HttpOnly,
			SameSite: http.SameSite(c.SameSite),
		})
	}
	a.inner.SetCookies(u, nh)
}

// GetAllCookies satisfies the tls-client CookieJar interface. tls-client uses
// it only to seed its own internal store; we return empty because cookies are
// persisted through the shared persistent jar via SetCookies (the source of
// truth). tls-client still reads the correct per-request cookies through
// Cookies after a request is issued to a URL.
func (a *jarAdapter) GetAllCookies() map[string][]*fhttp.Cookie {
	return map[string][]*fhttp.Cookie{}
}

// requestWithTLSClient performs the request through bogdanfinn/tls-client, a
// browser-impersonating HTTP client. The persistent cookie jar is shared with
// the fasthttp client (via tlsJar) so cookies survive across both transports.
func requestWithTLSClient(requrl string, option *RequestOptions) (Response[string], error) {
	cfg := option.TLSConfig

	// Resolve the browser-impersonation profile. An empty profile falls back to
	// the tls-client default (Chrome).
	clientProfile, ok := tls_profiles.MappedTLSClients[cfg.Profile]
	if !ok {
		clientProfile = tls_profiles.DefaultClientProfile
	}

	options := []tls_client.HttpClientOption{
		tls_client.WithCookieJar(&jarAdapter{inner: jar}),
		tls_client.WithClientProfile(clientProfile),
		tls_client.WithTimeoutSeconds(getTimeoutSeconds(option)),
	}

	if proxy := getProxyURL(option); proxy != "" {
		options = append(options, tls_client.WithProxyUrl(proxy))
	}

	if cfg.DisableRedirect {
		options = append(options, tls_client.WithNotFollowRedirects())
	}

	if cfg.InsecureSkipVerify {
		options = append(options, tls_client.WithInsecureSkipVerify())
	}

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		return Response[string]{}, err
	}
	defer client.CloseIdleConnections()

	req, err := fhttp.NewRequest(checkRequestMethod(option.Method), requrl, readerForBody(option))
	if err != nil {
		return Response[string]{}, err
	}

	for k, v := range option.Headers {
		req.Header.Set(k, v)
	}
	if cfg.UserAgent != "" {
		req.Header.Set("User-Agent", cfg.UserAgent)
	}

	resp, err := client.Do(req)
	if err != nil {
		return Response[string]{}, err
	}
	defer resp.Body.Close()

	nhResp := netHTTPResponseFromFhttp(resp)
	body, err := readDecompressed(nhResp)
	if err != nil {
		return Response[string]{}, err
	}

	headers := make(map[string]string)
	for k, vs := range nhResp.Header {
		if len(vs) > 0 {
			headers[k] = vs[len(vs)-1]
		}
	}

	return Response[string]{
		Res:        nil,
		Body:       string(body),
		StatusCode: nhResp.StatusCode,
		Headers:    headers,
	}, nil
}

// netHTTPResponseFromFhttp converts a bogdanfinn/fhttp response into a standard
// net/http.Response so it can be consumed by the shared decompression helper.
func netHTTPResponseFromFhttp(resp *fhttp.Response) *http.Response {
	nh := &http.Response{
		StatusCode: resp.StatusCode,
		Header:     make(http.Header),
		Body:       resp.Body,
	}
	for k, vs := range resp.Header {
		for _, v := range vs {
			nh.Header.Add(k, v)
		}
	}
	return nh
}

// getTimeoutSeconds returns the configured timeout in seconds (falling back to
// the default 30s). option.Timeout is expressed in milliseconds.
func getTimeoutSeconds(option *RequestOptions) int {
	if option.Timeout > 0 {
		sec := option.Timeout / 1000
		if sec < 1 {
			sec = 1
		}
		return sec
	}
	return 30
}

// readerForBody returns a reader for the request body, or nil when empty.
func readerForBody(option *RequestOptions) io.Reader {
	if option.RequestBody != "" {
		return strings.NewReader(option.RequestBody)
	}
	if option.RequestBodyRaw != nil {
		return bytes.NewReader(option.RequestBodyRaw)
	}
	return nil
}

// readDecompressed reads and decompresses the tls-client/http response body
// according to the Content-Encoding header (gzip, deflate, br, zstd). When no
// encoding is present the raw body is returned.
func readDecompressed(resp *http.Response) ([]byte, error) {
	contentEncoding := resp.Header.Get("Content-Encoding")
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// Some servers advertise a Content-Encoding (e.g. "br") while actually
	// sending base64/plain text (miruro.tv does exactly this). Trusting the
	// header blindly and decoding invalid compressed bytes raises errors such
	// as "brotli: HUFFMAN_SPACE". So we only decompress when the bytes are
	// genuinely valid for that encoding; otherwise we return the raw body and
	// let the caller decode it itself.
	switch strings.ToLower(contentEncoding) {
	case "gzip":
		if gz, err := gzip.NewReader(bytes.NewReader(raw)); err == nil {
			if dec, err := io.ReadAll(gz); err == nil {
				return dec, nil
			}
		}
	case "deflate":
		if zr, err := zlib.NewReader(bytes.NewReader(raw)); err == nil {
			if dec, err := io.ReadAll(zr); err == nil {
				return dec, nil
			}
		}
	case "br":
		if dec, err := io.ReadAll(brotli.NewReader(bytes.NewReader(raw))); err == nil {
			return dec, nil
		}
	case "zstd":
		if dec, err := zstd.NewReader(bytes.NewReader(raw)); err == nil {
			out, err := io.ReadAll(dec)
			dec.Close()
			if err == nil {
				return out, nil
			}
		}
	}
	return raw, nil
}

func parseCookie(cookie string) map[string]string {
	cookieMap := make(map[string]string)
	if cookie == "" {
		return cookieMap
	}
	cookiePair := strings.Split(cookie, ";")
	for _, cookie := range cookiePair {
		cookiePair := strings.Split(cookie, "=")
		if len(cookiePair) == 2 {
			cookieMap[cookiePair[0]] = cookiePair[1]
		}
	}
	return cookieMap
}

func prepareRequest(req *fasthttp.Request, reqUrl string, option *RequestOptions) (*fasthttp.Client, error) {
	req.SetRequestURI(reqUrl)
	req.Header.SetMethod(checkRequestMethod(option.Method))

	// Set headers
	for k, v := range option.Headers {
		req.Header.Set(k, v)
	}

	// Body
	if option.RequestBody != "" {
		req.SetBodyString(option.RequestBody)
	} else if option.RequestBodyRaw != nil {
		req.SetBody(option.RequestBodyRaw)
	}

	u, _ := url.Parse(reqUrl)

	// Add Cookie from cookiejar
	for _, value := range jar.Cookies(u) {
		req.Header.SetCookie(value.Name, value.Value)
	}

	// Parse cookie string from request header
	reqCookie := option.Headers["Cookie"]
	for k, v := range parseCookie(reqCookie) {
		req.Header.SetCookie(k, v)
	}

	return PrepareProxy(option, reqUrl)
}

func saveFasthttpCookies(u *url.URL, res *fasthttp.Response) {
	cookies := res.Header.Cookies()
	// Save Cookies
	for _, cookie := range cookies {
		c := fasthttp.AcquireCookie()
		c.ParseBytes(cookie)

		hc := &http.Cookie{
			Name:     string(c.Key()),
			Value:    string(c.Value()),
			Domain:   string(c.Domain()),
			Path:     string(c.Path()),
			Expires:  c.Expire(),
			Secure:   c.Secure(),
			HttpOnly: c.HTTPOnly(),
		}
		jar.SetCookies(u, []*http.Cookie{hc})
		fasthttp.ReleaseCookie(c)
	}
}

// request performs the request through the lightweight fasthttp client. The
// persistent cookie jar is shared with the tls-client transport. It returns the
// body as a string (non-generic) so it can be invoked from the Scriggo
// playground, which cannot call generic functions via reflection.
func request(reqUrl string, option *RequestOptions, readPreference func(*fasthttp.Response) ([]byte, error)) (Response[string], error) {

	req := fasthttp.AcquireRequest()
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)

	client, err := prepareRequest(req, reqUrl, option)
	if err != nil {
		fasthttp.ReleaseResponse(res)
		return Response[string]{Res: res}, err
	}

	if option.Timeout > 0 {
		err = client.DoTimeout(req, res, time.Duration(option.Timeout)*time.Millisecond)
	} else {
		err = client.Do(req, res)
	}

	if err != nil {
		fasthttp.ReleaseResponse(res)
		return Response[string]{Res: res}, err
	}

	// Read the response body
	body, err := readPreference(res)
	if err != nil {
		fasthttp.ReleaseResponse(res)
		return Response[string]{Res: res}, err
	}

	u, _ := url.Parse(reqUrl)
	saveFasthttpCookies(u, res)

	statusCode := res.StatusCode()
	headers := make(map[string]string)
	res.Header.VisitAll(func(key, value []byte) {
		headers[string(key)] = string(value)
	})

	fasthttp.ReleaseResponse(res)
	res = nil

	return Response[string]{
		Res:        nil,
		Body:       string(body),
		StatusCode: statusCode,
		Headers:    headers,
	}, nil
}

func checkRequestMethod(method string) string {
	switch method {
	case "GET", "POST", "PUT", "DELETE", "PATCH":
		return method
	default:
		return "GET"
	}
}

const DefaultMaxBodySize = 10 << 20

var maxBodySize = DefaultMaxBodySize

func SetMaxBodySize(n int) {
	if n <= 0 {
		maxBodySize = DefaultMaxBodySize
		return
	}
	maxBodySize = n
}

// ReadAll reads the entire response body and returns it as a byte slice.
func ReadAll(res *fasthttp.Response) ([]byte, error) {
	if cl := res.Header.ContentLength(); cl > maxBodySize {
		return nil, fmt.Errorf("response body too large: %d bytes (limit %d)", cl, maxBodySize)
	}

	body := res.Body()
	if len(body) > maxBodySize {
		return nil, fmt.Errorf("response body too large: %d bytes (limit %d)", len(body), maxBodySize)
	}

	contentEncoding := string(res.Header.Peek("Content-Encoding"))
	switch contentEncoding {
	case "gzip":
		if dec, err := res.BodyGunzip(); err == nil {
			return dec, nil
		}
	case "deflate":
		if dec, err := res.BodyInflate(); err == nil {
			return dec, nil
		}
	case "br":
		// Only brotli-decode when the bytes are valid brotli. Some servers
		// advertise "br" while sending base64/plain text (miruro.tv); decoding
		// that raises "brotli: HUFFMAN_SPACE". Fall back to the raw body.
		if dec, err := io.ReadAll(brotli.NewReader(bytes.NewReader(body))); err == nil {
			return dec, nil
		}
	case "zstd":
		if dec, err := res.BodyUnzstd(); err == nil {
			return dec, nil
		}
	}
	return body, nil
}

// GetDecompressedReader returns an io.Reader that automatically decompresses the response body
// based on the Content-Encoding header. Supports gzip, deflate, brotli, and zstd.
func GetDecompressedReader(res *fasthttp.Response) (io.Reader, error) {
	contentEncoding := string(res.Header.Peek("Content-Encoding"))
	bodyStream := res.BodyStream()

	switch contentEncoding {
	case "gzip":
		return gzip.NewReader(bodyStream)
	case "deflate":
		return zlib.NewReader(bodyStream)
	case "br":
		return brotli.NewReader(bodyStream), nil
	case "zstd":
		decoder, err := zstd.NewReader(bodyStream)
		if err != nil {
			return nil, err
		}
		return decoder, nil
	default:
		return bodyStream, nil
	}
}

type StringOrBytes interface {
	~string | ~[]byte
}

type RequestOptions struct {
	Headers        map[string]string `json:"headers"`
	Method         string            `json:"method"`
	ProxyHost      string            `json:"proxy_host"`
	ProxyScheme    string            `json:"proxy_scheme"`
	ProxyUserName  string            `json:"proxy_username"`
	ProxyPassword  string            `json:"proxy_password"`
	RequestBody    string            `json:"request_body"`
	RequestBodyRaw []byte            `json:"request_body_raw"`
	Timeout        int               `json:"timeout"`
	// TLSConfig, when set, routes the request through a browser-impersonating
	// tls-client instead of the default fasthttp client. Presence of this field
	// (not any particular sub-field) is what triggers the tls-client path.
	TLSConfig *TLSConfig `json:"tls_config,omitempty"`
}

// TLSConfig configures the browser-impersonating tls-client transport. Profile
// is a named tls-client fingerprint such as "chrome_133" (see
// github.com/bogdanfinn/tls-client/profiles). An empty Profile falls back to the
// library default.
type TLSConfig struct {
	Profile            string `json:"profile"`
	UserAgent          string `json:"userAgent,omitempty"`
	DisableRedirect    bool   `json:"disableRedirect,omitempty"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty"`
}

func dnsResolve() {
	addrs, err := net.LookupHost("www.google.com")
	if len(addrs) != 0 && err == nil {
		logger.Println("Check dns OK", addrs, err)
		return
	}

	logger.Println("Check dns failed", addrs, err)
	fn := func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{}
		return d.DialContext(ctx, "udp", "1.1.1.1:53")
	}
	net.DefaultResolver = &net.Resolver{
		Dial: fn,
	}
}

func Init() {
	defaultClient = &fasthttp.Client{
		MaxIdemponentCallAttempts: 2,
		// Name:                      "Mozilla/5.0 (X11; Linux x86_64; rv:146.0) Gecko/20100101 Firefox/146.0",
		MaxIdleConnDuration: 90 * time.Second,
		ReadTimeout:         30 * time.Second,
		WriteTimeout:        30 * time.Second,
		Dial: func(addr string) (net.Conn, error) {
			return tcpDialer.DialTimeout(addr, 15*time.Second)
		},
		MaxConnsPerHost: 300,
		// TLSConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	go dnsResolve()
	initCookieJar()
}
