package network

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"time"

	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	logger "github.com/gofiber/fiber/v2/log"
	log "github.com/miru-project/miru-core/pkg/logger"
)

var defaultClient *http.Client

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
func Request[T StringOrBytes](url string, option *RequestOptions, readPreference func(*http.Response) ([]byte, error)) (T, error) {

	log.Println("Making request to:", url)

	if option.TlsSpoofConfig.Body != "" {
		o, e := requestWithCycleTLS(url, option)
		return T(o), e
	}

	return request[T](url, option, readPreference)
}

// Request with cycle TLS
func requestWithCycleTLS(requrl string, option *RequestOptions) (string, error) {
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

	res, err := client.Do(requrl, config, checkRequestMethod(option.Method))
	if err != nil {
		return "", err
	}

	jar.SetCookies(reqUrl, res.Cookies)

	return res.Body, nil
}

func cookieHeader(rawCookies string) []*http.Cookie {
	header := http.Header{}
	header.Add("Cookie", rawCookies)
	req := http.Request{Header: header}
	return req.Cookies()
}

// Request with built-in http client
func request[T StringOrBytes](url string, option *RequestOptions, readPreference func(*http.Response) ([]byte, error)) (T, error) {

	// create request body
	var requestBody io.Reader

	if option.RequestBody != "" {
		requestBody = bytes.NewBuffer([]byte(option.RequestBody))
	} else if option.RequestBodyRaw != nil {
		requestBody = bytes.NewBuffer(option.RequestBodyRaw)
	} else {
		requestBody = nil
	}

	// Create a new request
	req, err := http.NewRequest(checkRequestMethod(option.Method), url, requestBody)
	if err != nil {
		return T(""), err
	}

	// Add Cookie from cookiejar
	for _, value := range jar.Cookies(req.URL) {
		req.AddCookie(value)
	}

	// Parse cookie string from request header
	for _, value := range cookieHeader(option.Headers["Cookie"]) {
		req.AddCookie(value)
	}

	var client *http.Client

	if option.ProxyHost != "" {
		client = &http.Client{}
		client.Transport = setupProxy(option)
	} else {
		client = defaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return T(""), err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := readPreference(resp)
	if err != nil {
		return T(""), err
	}

	// Save Cookies
	resCookies := resp.Cookies()
	jar.SetCookies(req.URL, resCookies)

	var result T

	switch any(result).(type) {
	case string:
		result = any(string(body)).(T)
	case []byte:
		result = any(body).(T)
	}

	return result, nil
}

func checkRequestMethod(method string) string {
	switch method {
	case "GET", "POST", "PUT", "DELETE", "PATCH":
		return method
	default:
		return "GET"
	}
}

// setup proxy for http client
func setupProxy(option *RequestOptions) *http.Transport {

	transport := &http.Transport{}

	if option.ProxyHost != "" {
		transport.Proxy = http.ProxyURL(&url.URL{
			Scheme: option.ProxyScheme,
			Host:   option.ProxyHost,
			User:   url.UserPassword(option.ProxyUserName, option.ProxyPassword),
		})

	} else {

		transport.Proxy = http.ProxyFromEnvironment

	}
	return transport
}

func SaveFile(filePath string, data *[]byte) error {

	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Create the file
	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the data to file
	_, err = out.Write(*data)
	if err != nil {
		return err
	}

	return nil
}
func DeleteFile(filePath string) error {
	if err := os.Remove(filePath); err != nil {
		return err
	}
	return nil
}

// ReadAll reads the entire response body and returns it as a byte slice.
func ReadAll(res *http.Response) ([]byte, error) {
	return io.ReadAll(res.Body)
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
	TlsSpoofConfig cycletls.Options  `json:"tls_spoof_config"`
}

func dnsResolve() {
	addrs, err := net.LookupHost("www.google.com")
	if len(addrs) == 0 {
		logger.Error("Check dns failed", addrs, err)

		fn := func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "udp", "1.1.1.1:53")
		}

		net.DefaultResolver = &net.Resolver{
			Dial: fn,
		}

		addrs, err = net.LookupHost("www.google.com")
		logger.Info("Check cloudflare dns", addrs, err)
	} else {
		logger.Info("Check dns OK", addrs, err)
	}
}

func Init() {
	go dnsResolve()
	initCookieJar()

	defaultClient = &http.Client{
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}
