package network

import (
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
	proxyClients = make(map[string]*fasthttp.Client)
	proxyMutex   sync.RWMutex
	tcpDialer    = &fasthttp.TCPDialer{
		Concurrency:      4096,
		DNSCacheDuration: 6 * time.Hour,
	}
)

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
	proxyMutex.Unlock()

	logger.Println("[Proxy] request to:", targetURL)
	return client, nil
}

func Proxy(ctx *fasthttp.RequestCtx) {
	req := &ctx.Request
	res := &ctx.Response

	targetURL := ctx.UserValue("path").(string)
	if targetURL == "" {
		ctx.Error("Empty target URL", fasthttp.StatusBadRequest)
		return
	}

	query := ctx.QueryArgs().String()
	if query != "" {
		if strings.Contains(targetURL, "?") {
			targetURL += "&" + query
		} else {
			targetURL += "?" + query
		}
	}

	req.SetRequestURI(targetURL)

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
