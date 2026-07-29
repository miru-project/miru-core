package network

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"path"
	"strings"

	"github.com/miru-project/miru-core/pkg/db"
	"github.com/valyala/fasthttp"
)

// Query parameters understood by the Proxy handler for the header/tls-aware
// streaming mode. They are prefixed with "__" to avoid colliding with a target
// URL's own query parameters.
const (
	// proxyURLParam carries the base64url(RawURLEncoding) encoded absolute
	// target URL. When present, it takes precedence over the wildcard path so a
	// target URL's own query string survives intact.
	proxyURLParam = "__u"
	// proxyHeadersParam carries base64url(json) of the request headers the proxy
	// must apply server-side (e.g. Referer/User-Agent that a browser cannot set
	// on a cross-origin fetch).
	proxyHeadersParam = "__mh"
	// proxyTLSParam ("1") routes the upstream fetch through the
	// browser-impersonating tls-client instead of the fasthttp client. Required
	// for CDNs (e.g. Cloudflare-fronted vault-*.owocdn.top) that block Go's TLS
	// fingerprint.
	proxyTLSParam = "__tls"
	// proxyProfileParam names the tls-client fingerprint profile (e.g.
	// "chrome_120"). Empty falls back to the tls-client default.
	proxyProfileParam = "__tlsp"
)

// encodeProxyHeaders serialises headers to base64url(json). Returns "" for an
// empty map.
func encodeProxyHeaders(headers map[string]string) string {
	if len(headers) == 0 {
		return ""
	}
	b, err := json.Marshal(headers)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// decodeProxyHeaders reverses encodeProxyHeaders.
func decodeProxyHeaders(enc string) map[string]string {
	out := map[string]string{}
	if enc == "" {
		return out
	}
	b, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(b, &out)
	return out
}

// BuildProxyURL converts a raw media/stream URL into an absolute proxy URL
// (host included) that, when requested by the player/reader, makes miru-core
// fetch the target server-side with the supplied headers applied. When
// tlsProfile is non-empty the fetch is routed through the browser-impersonating
// tls-client (required for TLS-fingerprinting CDNs).
//
// host must be the origin the player reaches miru-core at (scheme://host, e.g.
// "http://localhost:3000"). It is required: a relative "/proxy/..." path is not
// directly requestable by a browser player, and (for HLS) every rewritten child
// segment/key must point back at the same origin. The host is NOT validated for
// reachability -- the caller owns that (usually config.Global).
func BuildProxyURL(host, target string, headers map[string]string, tlsProfile string) string {
	v := url.Values{}
	v.Set(proxyURLParam, base64.RawURLEncoding.EncodeToString([]byte(target)))
	if enc := encodeProxyHeaders(headers); enc != "" {
		v.Set(proxyHeadersParam, enc)
	}
	if tlsProfile != "" {
		v.Set(proxyTLSParam, "1")
		v.Set(proxyProfileParam, tlsProfile)
	}

	// Preserve the target's file name in the path so players can sniff the
	// container/extension (e.g. .m3u8). It is otherwise ignored by the handler.
	name := "stream"
	if u, err := url.Parse(target); err == nil {
		if base := path.Base(u.Path); base != "" && base != "/" && base != "." {
			name = base
		}
	}
	return strings.TrimRight(host, "/") + "/proxy/" + name + "?" + v.Encode()
}

// BuildProxyPath is retained for callers that build server-relative paths (e.g.
// HLS child rewriting inside the proxy handler, which already knows the request
// origin). Prefer BuildProxyURL for anything handed to a client/player.
func BuildProxyPath(target string, headers map[string]string, tlsProfile string) string {
	v := url.Values{}
	v.Set(proxyURLParam, base64.RawURLEncoding.EncodeToString([]byte(target)))
	if enc := encodeProxyHeaders(headers); enc != "" {
		v.Set(proxyHeadersParam, enc)
	}
	if tlsProfile != "" {
		v.Set(proxyTLSParam, "1")
		v.Set(proxyProfileParam, tlsProfile)
	}
	name := "stream"
	if u, err := url.Parse(target); err == nil {
		if base := path.Base(u.Path); base != "" && base != "/" && base != "." {
			name = base
		}
	}
	return "/proxy/" + name + "?" + v.Encode()
}

// proxyViaTLS fetches targetURL through the tls-client (browser impersonation),
// applying extraHeaders, and writes the response back to ctx. When the response
// is an HLS manifest its child URIs are rewritten to go through this proxy too,
// so segments/keys inherit the same headers and tls fingerprint (otherwise the
// player would fetch them cross-origin and get 403).
func proxyViaTLS(ctx *fasthttp.RequestCtx, targetURL string, extraHeaders map[string]string, profile string) {
	headers := map[string]string{}
	// Forward Range so the player can seek within segments.
	if r := string(ctx.Request.Header.Peek("Range")); r != "" {
		headers["Range"] = r
	}
	for k, v := range extraHeaders {
		headers[k] = v
	}

	opt := &RequestOptions{
		Method:    string(ctx.Method()),
		Headers:   headers,
		TLSConfig: &TLSConfig{Profile: profile},
	}
	resp, err := FetchString(targetURL, opt, ReadAll)
	if err != nil {
		ctx.Error(err.Error(), fasthttp.StatusBadGateway)
		return
	}

	body := resp.Body
	contentType := resp.Headers["Content-Type"]
	if isM3U8(contentType, body) {
		origin := string(ctx.URI().Scheme()) + "://" + string(ctx.Host())
		body = rewriteM3U8(body, targetURL, origin, extraHeaders, profile)
		if contentType == "" {
			contentType = "application/vnd.apple.mpegurl"
		}
	}

	ctx.SetStatusCode(resp.StatusCode)
	if contentType != "" {
		ctx.SetContentType(contentType)
	}
	// Note: Content-Length/Content-Encoding are intentionally NOT propagated --
	// FetchString already decompressed the body, so fasthttp must recompute the
	// length from the (possibly rewritten) body we set below.
	for _, h := range []string{"Content-Range", "Accept-Ranges", "Cache-Control"} {
		if v := resp.Headers[h]; v != "" {
			ctx.Response.Header.Set(h, v)
		}
	}
	ctx.SetBodyString(body)
}

// isM3U8 reports whether a response looks like an HLS manifest, by content type
// or by the mandatory #EXTM3U first line.
func isM3U8(contentType, body string) bool {
	if strings.Contains(strings.ToLower(contentType), "mpegurl") {
		return true
	}
	return strings.HasPrefix(strings.TrimSpace(body), "#EXTM3U")
}

// rewriteM3U8 rewrites every child URI in an HLS manifest (variant playlists,
// media segments, and URI="..." attributes such as EXT-X-KEY/EXT-X-MEDIA/
// EXT-X-MAP) into an absolute proxy URL rooted at proxyOrigin, resolving
// relative URIs against baseURL. headers/profile are carried onto each child so
// segments and keys are fetched with the same mirror headers and tls
// fingerprint as the manifest.
func rewriteM3U8(manifest, baseURL, proxyOrigin string, headers map[string]string, profile string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return manifest
	}

	proxify := func(raw string) string {
		ref, err := url.Parse(strings.TrimSpace(raw))
		if err != nil {
			return raw
		}
		abs := base.ResolveReference(ref).String()
		return proxyOrigin + BuildProxyPath(abs, headers, profile)
	}

	lines := strings.Split(manifest, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			// Rewrite a URI="..." attribute if present (keys, alt renditions, maps).
			if idx := strings.Index(line, `URI="`); idx != -1 {
				start := idx + len(`URI="`)
				if end := strings.Index(line[start:], `"`); end != -1 {
					uri := line[start : start+end]
					lines[i] = line[:start] + proxify(uri) + line[start+end:]
				}
			}
			continue
		}
		// A bare non-comment line is a segment or variant-playlist URI.
		lines[i] = proxify(trimmed)
	}
	return strings.Join(lines, "\n")
}
// ProxyOrigin returns the scheme+host of the local stream proxy (e.g.
// "http://127.0.0.1:3000"). It is derived from the app's Core host setting
// and falls back to the default local development host when unset so tests
// and local runs work without configuration.
func ProxyOrigin() string {
	host, _ := db.GetAPPSetting("CoreHost")
	if host == "" {
		return "http://127.0.0.1:3000"
	}
	return "http://" + host
}

// IsProxyURL reports whether urlString looks like a miru-core proxy URL.
func IsProxyURL(urlString string) bool {
	u, err := url.Parse(urlString)
	if err != nil {
		return false
	}
	origin := ProxyOrigin()
	if origin == "" {
		return false
	}
	originURL, _ := url.Parse(origin)
	return u.Scheme == originURL.Scheme && u.Host == originURL.Host && strings.HasPrefix(u.Path, "/proxy/")
}

// ResolveProxyTarget extracts the real upstream target URL from a miru-core
// proxy URL. It returns the decoded target and true if successful, otherwise
// the original string and false.
func ResolveProxyTarget(urlString string) (string, bool) {
	if !IsProxyURL(urlString) {
		return urlString, false
	}
	u, err := url.Parse(urlString)
	if err != nil {
		return urlString, false
	}
	if enc := u.Query().Get(proxyURLParam); enc != "" {
		if decoded, err := base64.RawURLEncoding.DecodeString(enc); err == nil {
			return string(decoded), true
		}
	}
	// Legacy wildcard path fallback.
	target := u.Path
	target = strings.TrimPrefix(target, "/proxy/")
	target, _ = url.PathUnescape(target)
	if target == "" {
		return urlString, false
	}
	return target, true
}

// ProxyURLTargetName returns the file name component of the real upstream
// target hidden behind a proxy URL. For direct URLs it simply returns
// path.Base(urlString).
func ProxyURLTargetName(urlString string) string {
	target, ok := ResolveProxyTarget(urlString)
	if ok {
		return path.Base(target)
	}
	return path.Base(urlString)
}
