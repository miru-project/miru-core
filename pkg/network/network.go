package network

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"

	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

var jar, _ = cookiejar.New(nil)

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

func getHeadersFromJar(jar *cookiejar.Jar, url *url.URL) string {
	cookies := jar.Cookies(url)
	var cookieStrs []string
	for _, cookie := range cookies {
		cookieStrs = append(cookieStrs, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(cookieStrs, "; ")
}

// Request with cycle TLS
func requestWithCycleTLS(requrl string, option *RequestOptions) (string, error) {
	client := cycletls.Init()
	defer client.Close()
	config := option.TlsSpoofConfig
	reqUrl, _ := url.Parse(requrl)

	// Set cookie from cookiejar
	if _, e := config.Headers["Cookie"]; !e {
		config.Headers["Cookie"] = getHeadersFromJar(jar, reqUrl)
	} else {
		config.Headers["Cookie"] += "; " + getHeadersFromJar(jar, reqUrl)
	}

	// Set cycleTls headers from header
	config.Headers = option.Headers

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

	client := &http.Client{}
	client.Transport = setupProxy(option)

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
