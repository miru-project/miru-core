package network

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func Request(url string, option *RequestOptions) (string, error) {

	log.Println("Making request to:", url)

	if option.TlsSpoofConfig.Body != "" {
		return requestWithCycleTLS(url, option)
	}
	return request(url, option)
}

// Request with cycle TLS
func requestWithCycleTLS(url string, option *RequestOptions) (string, error) {
	client := cycletls.Init()
	defer client.Close()

	res, err := client.Do(url, option.TlsSpoofConfig, checkRequestMethod(option.Method))

	if err != nil {
		return "", err
	}

	return res.Body, nil
}

// Request with built-in http client
func request(url string, option *RequestOptions) (string, error) {

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
		return "", err
	}

	// Add headers if provided in options
	for key, value := range option.Headers {
		req.Header.Add(key, value)
	}

	client := &http.Client{}
	client.Transport = setupProxy(option)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Return response as string
	return string(body), nil
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

type RequestOptions struct {
	Headers        map[string]string `json:"headers"`
	Method         string            `json:"method"`
	Timeout        int               `json:"timeout"`
	ProxyHost      string            `json:"proxy_host"`
	ProxyScheme    string            `json:"proxy_scheme"`
	ProxyUserName  string            `json:"proxy_username"`
	ProxyPassword  string            `json:"proxy_password"`
	RequestBody    string            `json:"request_body"`
	RequestBodyRaw []byte            `json:"request_body_raw"`
	TlsSpoofConfig cycletls.Options  `json:"tls_spoof_config"`
}
