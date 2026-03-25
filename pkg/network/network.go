package network

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	"github.com/miru-project/miru-core/pkg/logger"
	"github.com/valyala/fasthttp"
)

var defaultClient *fasthttp.Client

type Response[T StringOrBytes] struct {
	Res  *fasthttp.Response
	Body T
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

	if option.TlsSpoofConfig.Body != "" {
		o, e := requestWithCycleTLS[T](url, option)
		return o, e
	}

	return request[T](url, option, readPreference)
}

// Request with cycle TLS
func requestWithCycleTLS[T StringOrBytes](requrl string, option *RequestOptions) (Response[T], error) {
	client := cycletls.Init()
	defer client.Close()
	config := option.TlsSpoofConfig
	reqUrl, _ := url.Parse(requrl)

	// Set cycleTls headers from header
	config.Headers = option.Headers

	// Set cookie from cookiejar
	if _, e := config.Headers["Cookie"]; !e {
		config.Headers["Cookie"] = getHeadersFromJar(reqUrl)
	} else {
		config.Headers["Cookie"] += "; " + getHeadersFromJar(reqUrl)
	}

	config.Proxy = getProxyURL(option)

	res, err := client.Do(requrl, config, checkRequestMethod(option.Method))
	if err != nil {
		return Response[T]{}, err
	}

	jar.SetCookies(reqUrl, res.Cookies)

	return Response[T]{
		Res:  &fasthttp.Response{},
		Body: any(res.Body).(T),
	}, nil
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

// Request with built-in http client
func request[T StringOrBytes](reqUrl string, option *RequestOptions, readPreference func(*fasthttp.Response) ([]byte, error)) (Response[T], error) {

	req := fasthttp.AcquireRequest()
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)

	client, err := prepareRequest(req, reqUrl, option)
	if err != nil {
		return Response[T]{Res: res}, err
	}

	if option.Timeout > 0 {
		err = client.DoTimeout(req, res, time.Duration(option.Timeout)*time.Millisecond)
	} else {
		err = client.Do(req, res)
	}

	if err != nil {
		return Response[T]{Res: res}, err
	}

	// Read the response body
	body, err := readPreference(res)
	if err != nil {
		return Response[T]{Res: res}, err
	}

	u, _ := url.Parse(reqUrl)
	saveFasthttpCookies(u, res)

	var result T

	switch any(result).(type) {
	case string:
		result = any(string(body)).(T)
	case []byte:
		result = any(body).(T)
	}

	return Response[T]{
		Res:  res,
		Body: result,
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

// ReadAll reads the entire response body and returns it as a byte slice.
func ReadAll(res *fasthttp.Response) ([]byte, error) {
	// check if compressed
	contentEncoding := string(res.Header.Peek("Content-Encoding"))
	switch contentEncoding {
	case "gzip":
		return res.BodyGunzip()
	case "deflate":
		return res.BodyInflate()
	case "br":
		return res.BodyUnbrotli()
	default:
		return res.Body(), nil
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
	TlsSpoofConfig cycletls.Options  `json:"tls_spoof_config"`
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
