package network

import (
	"encoding/base64"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/logger"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
	"golang.org/x/net/http/httpproxy"
)

func getProxyURL(option *RequestOptions) string {
	if option != nil && option.ProxyHost != "" {
		u := url.URL{
			Scheme: option.ProxyScheme,
			Host:   option.ProxyHost,
		}
		if u.Scheme == "" {
			u.Scheme = "http"
		}
		if option.ProxyUserName != "" {
			if option.ProxyPassword != "" {
				u.User = url.UserPassword(option.ProxyUserName, option.ProxyPassword)
			} else {
				u.User = url.User(option.ProxyUserName)
			}
		}
		return u.String()
	}
	proxy, _ := db.GetAPPSetting("Proxy")
	return proxy
}

var (
	proxyClients      = make(map[string]*fasthttp.Client)
	proxyClientTimers = make(map[string]*time.Timer)
	proxyMutex        sync.RWMutex
	tcpDialer         = &fasthttp.TCPDialer{
		Concurrency:      4096,
		DNSCacheDuration: 6 * time.Hour,
	}
	maxProxyClients = 8
	proxyLastAccess = make(map[string]time.Time)
)

func evictOldestProxyClient() {
	proxyMutex.Lock()
	defer proxyMutex.Unlock()
	if len(proxyClients) < maxProxyClients {
		return
	}
	var oldestKey string
	var oldestTime time.Time
	for k, t := range proxyLastAccess {
		if oldestKey == "" || t.Before(oldestTime) {
			oldestKey = k
			oldestTime = t
		}
	}
	if oldestKey != "" {
		delete(proxyClients, oldestKey)
		delete(proxyLastAccess, oldestKey)
		if t, ok := proxyClientTimers[oldestKey]; ok {
			t.Stop()
			delete(proxyClientTimers, oldestKey)
		}
	}
}

func PrepareProxy(option *RequestOptions, targetURL string) (*fasthttp.Client, error) {
	proxy := getProxyURL(option)
	enableProxy, _ := db.GetAPPSetting("ProxyActivate")
	if proxy == "" || enableProxy == "false" {
		logger.Println("request to:", targetURL)
		return defaultClient, nil
	}

	proxyMutex.RLock()
	client, ok := proxyClients[proxy]
	proxyMutex.RUnlock()
	if ok {
		proxyMutex.Lock()
		proxyLastAccess[proxy] = time.Now()
		proxyMutex.Unlock()
		logger.Println("[Proxy] request to:", targetURL)
		return client, nil
	}

	link, err := url.Parse(proxy)
	if err != nil {
		return nil, err
	}

	var dialFunc fasthttp.DialFunc
	switch link.Scheme {
	case "socks4", "socks4a":
		protocol := SOCKS4
		if link.Scheme == "socks4a" {
			protocol = SOCKS4A
		}
		user := ""
		if link.User != nil {
			user = link.User.Username()
		}
		dialFunc = FasthttpDialer(protocol, link.Host, user, 15*time.Second)
	case "socks5":
		d := fasthttpproxy.Dialer{Timeout: 15 * time.Second, ConnectTimeout: 15 * time.Second,
			TCPDialer: fasthttp.TCPDialer{
				Concurrency:      4096,
				DNSCacheDuration: 6 * time.Hour,
			}, Config: httpproxy.Config{HTTPProxy: proxy, HTTPSProxy: proxy}}
		dialFunc, _ = d.GetDialFunc(false)

	// http and https proxy
	default:
		d := fasthttpproxy.Dialer{Timeout: 15 * time.Second, ConnectTimeout: 15 * time.Second,
			TCPDialer: fasthttp.TCPDialer{
				Concurrency:      4096,
				DNSCacheDuration: 6 * time.Hour,
			}, Config: httpproxy.Config{HTTPProxy: proxy, HTTPSProxy: proxy}}
		dialFunc, _ = d.GetDialFunc(false)
	}

	evictOldestProxyClient()

	client = &fasthttp.Client{
		MaxIdemponentCallAttempts: 2,
		MaxIdleConnDuration:       90 * time.Second,
		ReadTimeout:               30 * time.Second,
		WriteTimeout:              30 * time.Second,
		Dial:                      dialFunc,
		MaxConnsPerHost:           300,
	}

	proxyMutex.Lock()
	proxyClients[proxy] = client
	proxyLastAccess[proxy] = time.Now()
	proxyMutex.Unlock()

	logger.Println("[Proxy] request to:", targetURL)
	return client, nil
}

func Proxy(ctx *fasthttp.RequestCtx) {
	req := &ctx.Request
	res := &ctx.Response

	// Resolve the target URL. Prefer the base64url __u query param (the
	// header/tls-aware streaming mode built by BuildProxyPath) so a target's own
	// query string survives; fall back to the legacy wildcard path.
	targetURL := ""
	if enc := string(ctx.QueryArgs().Peek(proxyURLParam)); enc != "" {
		if dec, err := base64.RawURLEncoding.DecodeString(enc); err == nil {
			targetURL = string(dec)
		}
	}
	if targetURL == "" {
		if p, ok := ctx.UserValue("path").(string); ok {
			targetURL = p
		}
		if decoded, err := url.PathUnescape(targetURL); err == nil {
			targetURL = decoded
		}
	}

	if targetURL == "" {
		ctx.Error("Empty target URL", fasthttp.StatusBadRequest)
		return
	}

	// Extra headers the client wants applied server-side (e.g. a Referer the
	// browser cannot set on a cross-origin fetch).
	extraHeaders := decodeProxyHeaders(string(ctx.QueryArgs().Peek(proxyHeadersParam)))

	// When requested, route the upstream fetch through the browser-impersonating
	// tls-client (required for TLS-fingerprinting CDNs) and rewrite HLS manifests
	// so segments stay proxied.
	if string(ctx.QueryArgs().Peek(proxyTLSParam)) == "1" {
		proxyViaTLS(ctx, targetURL, extraHeaders, string(ctx.QueryArgs().Peek(proxyProfileParam)))
		return
	}

	req.Header.Del("Host")
	req.SetRequestURI(targetURL)

	// Apply any explicitly requested headers before forwarding.
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	// Inject Anilist Token if target is Anilist and no auth header is present
	if strings.Contains(targetURL, "anilist.co") && string(req.Header.Peek("Authorization")) == "" {
		if token, err := db.GetAPPSetting("anilist_token"); err == nil && token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	client, err := PrepareProxy(nil, targetURL)
	if err != nil {
		ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
		return
	}
	if err := client.Do(req, res); err != nil {
		ctx.Error(err.Error(), fasthttp.StatusBadGateway)
		return
	}
}
