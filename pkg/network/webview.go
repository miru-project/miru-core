package network

import (
	"crypto/rand"
	"math/big"
	"strings"
	"sync"

	tls_profiles "github.com/bogdanfinn/tls-client/profiles"
	log "github.com/miru-project/miru-core/pkg/logger"
)

// webviewProfile is one coherent browser (webview) identity: a tls-client
// fingerprint profile together with the User-Agent and default header set that
// exact browser sends. The TLS ClientHello (what the server hashes into JA3 /
// JA4) is dictated by Profile; UserAgent and Headers must belong to the same
// browser so the whole request is internally consistent.
type webviewProfile struct {
	Profile   string
	UserAgent string
	Headers   map[string]string
	// ja3Hash / ja4 are the server-observed MD5(JA3) and JA4 this identity
	// produces, recorded against https://tls.peet.ws/api/all. They are the
	// ground truth tests verify a spoofed request against.
	ja3Hash string
	ja4     string
}

var chromeAcceptHeader = "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"

var webviewProfiles = []webviewProfile{
	{
		Profile:   "chrome_150",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36",
		Headers: map[string]string{
			"Accept":                    chromeAcceptHeader,
			"Accept-Language":           "en-US,en;q=0.9",
			"Sec-Ch-Ua":                 `"Chromium";v="150", "Google Chrome";v="150", "Not-A.Brand";v="99"`,
			"Sec-Ch-Ua-Mobile":          "?0",
			"Sec-Ch-Ua-Platform":        `"Windows"`,
			"Sec-Fetch-Dest":            "document",
			"Sec-Fetch-Mode":            "navigate",
			"Sec-Fetch-Site":            "none",
			"Sec-Fetch-User":            "?1",
			"Upgrade-Insecure-Requests": "1",
		},
		ja3Hash: "f984bd5bc7358922cde86ed4471a2e89",
		ja4:     "t13d1516h2_8daaf6152771_806a8c22fdea",
	},
	{
		Profile:   "firefox_148",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:148.0) Gecko/20100101 Firefox/148.0",
		Headers: map[string]string{
			"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
			"Accept-Language":           "en-US,en;q=0.5",
			"Sec-Fetch-Dest":            "document",
			"Sec-Fetch-Mode":            "navigate",
			"Sec-Fetch-Site":            "none",
			"Sec-Fetch-User":            "?1",
			"Upgrade-Insecure-Requests": "1",
		},
		ja3Hash: "992b82b242c18c86da84eff5bf3e3100",
		ja4:     "t13d1917h2_4d8ed5baf28e_3cbfd9057e0d",
	},
	{
		Profile:   "firefox_135",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:135.0) Gecko/20100101 Firefox/135.0",
		Headers: map[string]string{
			"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
			"Accept-Language":           "en-US,en;q=0.5",
			"Sec-Fetch-Dest":            "document",
			"Sec-Fetch-Mode":            "navigate",
			"Sec-Fetch-Site":            "none",
			"Sec-Fetch-User":            "?1",
			"Upgrade-Insecure-Requests": "1",
		},
		ja3Hash: "7704a11cf87dfcf33080b90ce11d5527",
		ja4:     "t13d1715h2_5b57614c22b0_a54fffd0eb61",
	},
	{
		Profile:   "safari_16_0",
		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Safari/605.1.15",
		Headers: map[string]string{
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"Accept-Language": "en-US,en;q=0.9",
		},
		ja3Hash: "773906b0efdefa24a7f2b8eb6985bf37",
		ja4:     "t13d2014h2_a09f3c656075_14788d8d241b",
	},
	{
		Profile:   "safari_ios_26_0",
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 26_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Mobile/15E148 Safari/604.1",
		Headers: map[string]string{
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"Accept-Language": "en-US,en;q=0.9",
		},
		ja3Hash: "ecdf4f49dd59effc439639da29186671",
		ja4:     "t13d2013h2_a09f3c656075_7f0f34a4126d",
	},
}

// webviewByProfile indexes webviewProfiles by tls-client profile name.
var webviewByProfile = func() map[string]webviewProfile {
	m := make(map[string]webviewProfile, len(webviewProfiles))
	for _, p := range webviewProfiles {
		m[p.Profile] = p
	}
	return m
}()

// familyUserAgents maps a browser family (chrome / firefox / safari) to the
// User-Agent of its newest curated profile, so a request whose TLS fingerprint
// comes from a non-curated tls-client profile can still carry a UA from the
// same browser family.
var familyUserAgents = func() map[string]string {
	m := make(map[string]string)
	for _, p := range webviewProfiles {
		family, _, _ := strings.Cut(p.Profile, "_")
		if _, ok := m[family]; !ok {
			m[family] = p.UserAgent
		}
	}
	return m
}()

// coherentUserAgent returns a User-Agent belonging to the same browser family
// as the given tls-client profile, so the UA never contradicts the JA3/JA4
// fingerprint actually sent. Curated webview profiles return their recorded
// UA; other profiles fall back to a same-family UA; unknown names get the
// default (Chrome) UA, which matches the tls-client default profile.
func coherentUserAgent(profile string) string {
	if p, ok := webviewByProfile[profile]; ok {
		return p.UserAgent
	}
	family, _, _ := strings.Cut(profile, "_")
	if ua, ok := familyUserAgents[family]; ok {
		return ua
	}
	return defaultUserAgent
}

// WebviewIdentity is the browser identity an OpenWebview call used. Callers
// receive it so later requests (or watch/mirror headers) can stay consistent
// with the same browser.
type WebviewIdentity struct {
	Profile   string `json:"profile"`
	UserAgent string `json:"userAgent"`
}

// defaultIdentity is chosen ONCE per install (crypto/rand over the profile
// table) and then stays fixed for the process lifetime: every request presents
// the same coherent browser (same TLS fingerprint, User-Agent and header set),
// which is what a real browser session looks like. A per-call random identity
// would hop fingerprints while reusing the same cookies -- more bot-like, not
// less -- and one flagged fingerprint would only burn a single install.
var (
	defaultIdentityOnce sync.Once
	defaultIdentity     WebviewIdentity
)

// DefaultWebviewIdentity returns this install's browser identity.
func DefaultWebviewIdentity() WebviewIdentity {
	defaultIdentityOnce.Do(func() {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(webviewProfiles))))
		if err != nil {
			log.Println("webview: crypto/rand failed, falling back to first profile:", err)
			n = big.NewInt(0)
		}
		p := webviewProfiles[n.Int64()]
		defaultIdentity = WebviewIdentity{Profile: p.Profile, UserAgent: p.UserAgent}
	})
	return defaultIdentity
}

// KnownWebviewFingerprint returns the JA3 hash and JA4 a real browser with the
// given profile produces (as recorded from a fingerprint probe service). ok is
// false for profiles outside the webview set.
func KnownWebviewFingerprint(profile string) (ja3Hash, ja4 string, ok bool) {
	p, ok := webviewByProfile[profile]
	if !ok {
		return "", "", false
	}
	return p.ja3Hash, p.ja4, true
}

// OpenWebview issues an HTTP request impersonating a real browser (webview):
// the TLS fingerprint (JA3/JA4), the User-Agent and the default browser
// headers all come from the same identity, so JA3/JA4-fencing CDNs
// (Cloudflare) see a genuine browser visit instead of a Go client.
//
// The identity is the install's fixed default (DefaultWebviewIdentity) unless
// option pins one via TLSConfig.Profile: curated webview profiles bring their
// full coherent header set, while any other valid tls-client profile name is
// honored as a raw fingerprint with a same-family User-Agent. Caller headers
// win over the identity defaults, and the User-Agent precedence is: caller
// header > TLSConfig.UserAgent > the pinned/default profile's own UA. Redirect
// / verify flags in TLSConfig are honored. The request always goes through
// the browser-impersonating tls-client transport. The identity actually used
// is returned alongside the response.
func OpenWebview(reqURL string, option *RequestOptions) (Response[string], WebviewIdentity, error) {
	if option == nil {
		option = &RequestOptions{}
	}

	// The install's fixed identity by default; a pinned profile selects its
	// own coherent identity. Caller headers are kept: clone the option and
	// merge the identity defaults underneath them.
	profile, ok := webviewByProfile[DefaultWebviewIdentity().Profile]
	if !ok {
		profile = webviewProfiles[0]
	}
	if option.TLSConfig != nil && option.TLSConfig.Profile != "" {
		if p, pinned := webviewByProfile[option.TLSConfig.Profile]; pinned {
			profile = p
		} else if _, raw := tls_profiles.MappedTLSClients[option.TLSConfig.Profile]; raw {
			// Not a curated webview profile but a valid tls-client
			// fingerprint: honor it without a recorded browser header set.
			profile = webviewProfile{Profile: option.TLSConfig.Profile}
		} else {
			log.Println("webview: unknown pinned profile", option.TLSConfig.Profile, "- using the install default identity")
		}
	}
	identity := WebviewIdentity{Profile: profile.Profile}

	merged := make(map[string]string, len(profile.Headers)+len(option.Headers)+1)
	for k, v := range profile.Headers {
		merged[k] = v
	}
	for k, v := range option.Headers {
		merged[k] = v
	}
	// User-Agent precedence: explicit caller header > TLSConfig.UserAgent >
	// the profile's own UA, so the header never contradicts the JA3/JA4
	// fingerprint unless the caller explicitly asked for it.
	userAgent := coherentUserAgent(identity.Profile)
	if option.TLSConfig != nil && option.TLSConfig.UserAgent != "" {
		userAgent = option.TLSConfig.UserAgent
	}
	if v, ok := option.Headers["User-Agent"]; ok && strings.TrimSpace(v) != "" {
		userAgent = v
	}
	merged["User-Agent"] = userAgent
	identity.UserAgent = userAgent

	wo := *option
	wo.Headers = merged
	wo.TLSConfig = &TLSConfig{
		Profile:            identity.Profile,
		UserAgent:          userAgent,
		DisableRedirect:    option.TLSConfig != nil && option.TLSConfig.DisableRedirect,
		InsecureSkipVerify: option.TLSConfig != nil && option.TLSConfig.InsecureSkipVerify,
	}

	res, err := requestWithTLSClient(reqURL, &wo)
	return res, identity, err
}
