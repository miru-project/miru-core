package network

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/miru-project/miru-core/pkg/db"
)

// peetTLS is the slice of the https://tls.peet.ws/api/all response the
// webview tests verify: the server-observed User-Agent, JA3 hash, JA4 and
// negotiated HTTP version.
type peetWebview struct {
	HTTPVersion string `json:"http_version"`
	UserAgent   string `json:"user_agent"`
	TLS         struct {
		JA3Hash string `json:"ja3_hash"`
		JA4     string `json:"ja4"`
	} `json:"tls"`
}

// webviewAvailable reports whether the live fingerprint probe service is
// actually answering (TCP connect alone can succeed while the server hangs),
// so tests skip instead of failing when the probe is down or rate-limiting.
func webviewAvailable(t *testing.T) bool {
	t.Helper()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://tls.peet.ws/api/all")
	if err != nil {
		t.Logf("fingerprint probe service unreachable, skipping live webview test: %v", err)
		return false
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Logf("fingerprint probe service returned %d, skipping live webview test", resp.StatusCode)
		return false
	}
	return true
}

// TestWebviewProfilesCoherent checks the identity table offline: every profile
// carries a User-Agent from its own browser family, a default header set, and
// a recorded JA3/JA4 fingerprint, with no two profiles sharing a fingerprint.
func TestWebviewProfilesCoherent(t *testing.T) {
	if len(webviewProfiles) < 2 {
		t.Fatalf("webview set must offer multiple identities, got %d", len(webviewProfiles))
	}
	seenJA3 := map[string]string{}
	for _, p := range webviewProfiles {
		if p.Profile == "" || p.UserAgent == "" {
			t.Fatalf("profile %q: empty profile name or user agent", p.Profile)
		}
		if len(p.Headers) == 0 || p.Headers["Accept"] == "" {
			t.Fatalf("profile %q: missing default Accept header", p.Profile)
		}
		ua := p.UserAgent
		switch {
		case strings.HasPrefix(p.Profile, "chrome_"):
			if !strings.Contains(ua, "Chrome/") || strings.Contains(ua, "Firefox/") {
				t.Fatalf("profile %q: user agent %q is not Chrome", p.Profile, ua)
			}
			if p.Headers["Sec-Ch-Ua"] == "" {
				t.Fatalf("profile %q: Chrome identity missing Sec-Ch-Ua", p.Profile)
			}
		case strings.HasPrefix(p.Profile, "firefox_"):
			if !strings.Contains(ua, "Firefox/") {
				t.Fatalf("profile %q: user agent %q is not Firefox", p.Profile, ua)
			}
			if p.Headers["Sec-Ch-Ua"] != "" {
				t.Fatalf("profile %q: Firefox identity must not send Sec-Ch-Ua", p.Profile)
			}
		case strings.HasPrefix(p.Profile, "safari"):
			if !strings.Contains(ua, "Safari/") {
				t.Fatalf("profile %q: user agent %q is not Safari", p.Profile, ua)
			}
		default:
			t.Fatalf("profile %q does not name a known browser family", p.Profile)
		}
		ja3, ja4, ok := KnownWebviewFingerprint(p.Profile)
		if !ok || ja3 == "" || ja4 == "" {
			t.Fatalf("profile %q: missing recorded JA3/JA4 fingerprint", p.Profile)
		}
		if other, dup := seenJA3[ja3]; dup {
			t.Fatalf("profiles %q and %q share JA3 hash %q", other, p.Profile, ja3)
		}
		seenJA3[ja3] = p.Profile
	}
	if _, _, ok := KnownWebviewFingerprint("not_a_profile"); ok {
		t.Fatal("unknown profile must not resolve")
	}
}

// TestDefaultWebviewIdentity checks that the per-install identity is stable
// across calls and always comes from the curated table.
func TestDefaultWebviewIdentity(t *testing.T) {
	first := DefaultWebviewIdentity()
	if _, ok := webviewByProfile[first.Profile]; !ok {
		t.Fatalf("default identity %q is not a known webview profile", first.Profile)
	}
	if first.UserAgent == "" {
		t.Fatalf("default identity %q has empty user agent", first.Profile)
	}
	for i := 0; i < 8; i++ {
		if got := DefaultWebviewIdentity(); got != first {
			t.Fatalf("default identity changed between calls: %v -> %v", first, got)
		}
	}
}

// TestOpenWebviewDefaultSpoofFingerprint is the live one-shot spoof check: it
// opens https://tls.peet.ws/api/all through the install's default browser
// identity and asserts the server observed exactly that browser - the echoed
// User-Agent matches the identity, and the reported JA3 hash / JA4 match the
// recorded fingerprint of the picked profile (i.e. the TLS ClientHello was
// really spoofed, not the Go default).
func TestOpenWebviewDefaultSpoofFingerprint(t *testing.T) {
	if !webviewAvailable(t) {
		t.Skip("requires network access to tls.peet.ws")
	}
	Init()
	clearProxySetting()

	res, identity, err := OpenWebview("https://tls.peet.ws/api/all", nil)
	if err != nil {
		t.Fatalf("OpenWebview: %v", err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if identity.Profile != DefaultWebviewIdentity().Profile {
		t.Fatalf("OpenWebview must use the per-install default identity: got %q, want %q",
			identity.Profile, DefaultWebviewIdentity().Profile)
	}

	var peet peetWebview
	if err := json.Unmarshal([]byte(res.Body), &peet); err != nil {
		t.Fatalf("decode peet.ws payload: %v", err)
	}

	if peet.UserAgent != identity.UserAgent {
		t.Fatalf("server saw user agent %q, expected identity %q", peet.UserAgent, identity.UserAgent)
	}
	ja3, ja4, ok := KnownWebviewFingerprint(identity.Profile)
	if !ok {
		t.Fatalf("identity profile %q unknown", identity.Profile)
	}
	if peet.TLS.JA3Hash != ja3 {
		t.Fatalf("profile %s: server saw ja3_hash %q, expected %q", identity.Profile, peet.TLS.JA3Hash, ja3)
	}
	if peet.TLS.JA4 != ja4 {
		t.Fatalf("profile %s: server saw ja4 %q, expected %q", identity.Profile, peet.TLS.JA4, ja4)
	}
	if peet.HTTPVersion != "h2" {
		t.Fatalf("profile %s: expected h2 impersonation, got http_version %q", identity.Profile, peet.HTTPVersion)
	}
	t.Logf("spoofed identity: profile=%s ua=%q ja3_hash=%s ja4=%s", identity.Profile, identity.UserAgent, ja3, ja4)
}

// clearProxySetting drops an app-level proxy left in the shared test DB by
// other tests in this package (one process, one DB), so the live webview
// requests dial directly. Safe when no DB is initialized.
func clearProxySetting() {
	defer func() { _ = recover() }()
	_ = db.SetAppSetting("Proxy", "")
}

// TestOpenWebviewPinnedProfile verifies a caller can pin one identity instead
// of a random pick and still gets the full coherent impersonation.
func TestOpenWebviewPinnedProfile(t *testing.T) {
	if !webviewAvailable(t) {
		t.Skip("requires network access to tls.peet.ws")
	}
	Init()
	clearProxySetting()

	const profile = "firefox_135"
	res, identity, err := OpenWebview("https://tls.peet.ws/api/all", &RequestOptions{
		TLSConfig: &TLSConfig{Profile: profile},
	})
	if err != nil {
		t.Fatalf("OpenWebview: %v", err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if identity.Profile != profile {
		t.Fatalf("pinned profile not honored: got %q", identity.Profile)
	}
	var peet peetWebview
	if err := json.Unmarshal([]byte(res.Body), &peet); err != nil {
		t.Fatalf("decode peet.ws payload: %v", err)
	}
	ja3, ja4, _ := KnownWebviewFingerprint(profile)
	if peet.UserAgent != identity.UserAgent || peet.TLS.JA3Hash != ja3 || peet.TLS.JA4 != ja4 {
		t.Fatalf("pinned profile %s not spoofed correctly: ua=%q ja3_hash=%q ja4=%q",
			profile, peet.UserAgent, peet.TLS.JA3Hash, peet.TLS.JA4)
	}
}
