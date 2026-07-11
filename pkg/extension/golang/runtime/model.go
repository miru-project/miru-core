package runtime

import (
	"io"

	tls_client "github.com/bogdanfinn/tls-client"
	tls_profiles "github.com/bogdanfinn/tls-client/profiles"
)

// Fetch performs a single browser-impersonating HTTP GET and returns the raw
// response body together with its HTTP status code.
//
// It is deliberately GENERIC and site-agnostic: the caller supplies the URL,
// a TLS-impersonation profile name (e.g. "chrome_110"), and the exact request
// headers to send. The host holds NO API-specific knowledge here -- no domain,
// no hard-coded headers, no protocol. Every site-specific concern (which
// domain, which headers, request signing, response decoding) lives inside the
// extension that calls Fetch.
//
// This thin primitive has to live in the host (rather than the extension)
// because of two hard limits of the Scriggo Go-subset that runs extensions:
//
//  1. Scriggo cannot emit interface-method calls, so an extension cannot call
//     HttpClient.Get / resp.Body.Close itself (the compiler aborts with
//     "internal error: not implemented").
//  2. Defeating TLS-fingerprint bot walls (Cloudflare) needs a browser JA3/h2
//     fingerprint from the tls-client "profiles" package, which is only
//     reachable through that same interface.
//
// Both un-emittable pieces are confined to this one generic function; the
// extension reaches it as a plain function call, which Scriggo supports.
//
// profile is looked up in the tls-client profile table; an unknown or empty
// profile falls back to the library default. headers is a simple
// map[string]string (one value per header) so it is trivial to build from the
// Scriggo subset.
func Fetch(url string, profile string, headers map[string]string) (string, int, error) {
	clientProfile, ok := tls_profiles.MappedTLSClients[profile]
	if !ok {
		clientProfile = tls_profiles.DefaultClientProfile
	}

	defaultHeaders := make(map[string][]string, len(headers))
	for k, v := range headers {
		defaultHeaders[k] = []string{v}
	}

	client, err := tls_client.NewHttpClient(
		tls_client.NewNoopLogger(),
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithNotFollowRedirects(),
		tls_client.WithCookieJar(tls_client.NewCookieJar()),
		tls_client.WithClientProfile(clientProfile),
		tls_client.WithDefaultHeaders(defaultHeaders),
	)
	if err != nil {
		return "", 0, err
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, err
	}
	return string(body), resp.StatusCode, nil
}

// ExtensionListItem represents a search result item.
type ExtensionListItem struct {
	Title  string            `json:"title"`
	URL    string            `json:"url"`
	Cover  string            `json:"cover"`
	Update string            `json:"update"`
	Image  string            `json:"image,omitempty"`
	Type   string            `json:"type,omitempty"`
}

// ExtensionDetail represents detailed content information.
type ExtensionDetail struct {
	Title       string                  `json:"title"`
	URL         string                  `json:"url"`
	Cover       string                  `json:"cover"`
	Image       string                  `json:"image,omitempty"`
	Type        string                  `json:"type,omitempty"`
	Description string                  `json:"description,omitempty"`
	Desc        string                  `json:"desc,omitempty"`
	Chapters    []ExtensionEpisodeGroup `json:"chapters,omitempty"`
}

// ExtensionEpisodeGroup represents a group of episodes.
type ExtensionEpisodeGroup struct {
	Title string   `json:"title"`
	URLs  []string `json:"urls,omitempty"`
}

// ExtensionWatch represents watch/stream information.
type ExtensionWatch struct {
	Title  string                `json:"title"`
	URL    string                `json:"url"`
	Type   string                `json:"type"`
	Pages  []string              `json:"pages,omitempty"`
	Groups []ExtensionMirrorGroup `json:"groups,omitempty"`
}

// ExtensionMirrorGroup represents a group of mirrors.
type ExtensionMirrorGroup struct {
	Title   string              `json:"title"`
	Mirrors []ExtensionMirror   `json:"mirrors,omitempty"`
}

// ExtensionMirror represents a mirror/alternative URL.
type ExtensionMirror struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}
