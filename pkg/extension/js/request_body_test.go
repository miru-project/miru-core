package js

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miru-project/miru-core/pkg/network"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

// TestMapJSBodyOptions covers the JS-conventional body option mapping at the
// host boundary: axios-style `data` (string sent raw, object JSON-encoded) and
// fetch-style `body`, with an explicit request_body always winning.
func TestMapJSBodyOptions(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
		want any // nil = key must stay absent
	}{
		{
			name: "data object json-encoded",
			in:   map[string]any{"data": map[string]any{"b": 1, "a": "x"}},
			want: `{"a":"x","b":1}`,
		},
		{
			name: "data string sent raw",
			in:   map[string]any{"data": "kw=naruto&page=1"},
			want: "kw=naruto&page=1",
		},
		{
			name: "data array json-encoded",
			in:   map[string]any{"data": []any{1, 2}},
			want: "[1,2]",
		},
		{
			name: "body string mapped",
			in:   map[string]any{"body": "raw-body"},
			want: "raw-body",
		},
		{
			name: "explicit request_body wins over data",
			in:   map[string]any{"data": map[string]any{"a": 1}, "request_body": "explicit"},
			want: "explicit",
		},
		{
			name: "no body keys untouched",
			in:   map[string]any{"method": "post", "headers": map[string]any{}},
			want: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mapJSBodyOptions(c.in)
			got, has := c.in["request_body"]
			if c.want == nil {
				if has {
					t.Fatalf("request_body unexpectedly set to %v", got)
				}
				return
			}
			if !has || got != c.want {
				t.Fatalf("request_body = %v (%T), want %v", got, got, c.want)
			}
		})
	}
}

// v1PostExtSource is a synthetic V1 extension (no real site) whose entry points
// POST through this.request with the JS-conventional body options, the way V1
// extensions in the official repo are written for GraphQL/form backends:
//   - search: method "post" (lowercase, as real extensions write it) + data object
//   - detail: data string (raw form body)
//
// __TARGET__ is replaced with the httptest server URL at boot.
const v1PostExtSource = `// ==MiruExtension==
// @name         V1 Post Fixture
// @version      v0.0.1
// @author       test
// @lang         en
// @license      MIT
// @package      v1post
// @type         bangumi
// @webSite      https://example.com
// ==/MiruExtension==

export default class extends Extension {
  async search(kw, page, filter) {
    const res = await this.request("", {
      method: "post",
      headers: { "Miru-Url": "__TARGET__/search", "Content-Type": "application/json" },
      data: { kw: kw, page: page },
    });
    return (res.items || []).map((item) => ({
      title: item.title,
      url: item.url,
      cover: item.cover,
    }));
  }

  async detail(url) {
    const res = await this.request("", {
      method: "POST",
      headers: { "Miru-Url": "__TARGET__/detail", "Content-Type": "application/x-www-form-urlencoded" },
      data: "id=" + encodeURIComponent(url),
    });
    return { title: res.title, url: url, cover: "", desc: "" };
  }
}
`

// TestV1RequestDataBody drives the REAL V1 runtime (InitRuntime -> Search /
// Detail) against an httptest server and asserts the JS body options reach the
// wire: the axios-style data object becomes the JSON POST body (this is what
// broke GraphQL-based V1 extensions like ani.gogo / komiic, whose search
// returned nothing because the server got an empty body), the lowercase
// "post" method is honored, and a data string is sent raw.
func TestV1RequestDataBody(t *testing.T) {
	network.Init()

	var gotMethod string
	var gotBody []byte
	var gotDetailBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"title":"hit-naruto","url":"/d/1","cover":"c.jpg"}]}`))
	})
	mux.HandleFunc("/detail", func(w http.ResponseWriter, r *http.Request) {
		gotDetailBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"title":"detail-ok"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Boot the real runtime with the fixture (t.TempDir keeps the fixture out
	// of the repo tree; the extension itself is synthetic).
	ApiPkgCache.Remove("v1post")
	dir := t.TempDir()
	src := strings.ReplaceAll(v1PostExtSource, "__TARGET__", srv.URL)
	if err := os.WriteFile(filepath.Join(dir, "v1post.js"), []byte(src), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	InitRuntime(dir, AssetsFS)
	api := waitForPkg(t, "v1post")
	if errText := api.Ext.Error; errText != "" {
		t.Fatalf("fixture failed to load: %s", errText)
	}
	t.Cleanup(func() {
		ApiPkgCache.Remove("v1post")
		deleteInstanceState("v1post")
	})

	items, err := Search[proto.ExtensionListItem]("v1post", 1, "naruto", nil)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("search: server saw method %q, want POST (lowercase method must be honored)", gotMethod)
	}
	var sent struct {
		Kw   string `json:"kw"`
		Page int    `json:"page"`
	}
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("search: server got body %q: %v", gotBody, err)
	}
	if sent.Kw != "naruto" || sent.Page != 1 {
		t.Fatalf("search: server got body %q, want {\"kw\":\"naruto\",\"page\":1}", gotBody)
	}
	if len(items) != 1 || items[0].Title != "hit-naruto" {
		t.Fatalf("search returned %+v, want one item hit-naruto", items)
	}

	detail, err := Detail("v1post", "abc-1")
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if string(gotDetailBody) != "id=abc-1" {
		t.Fatalf("detail: server got body %q, want raw data string id=abc-1", gotDetailBody)
	}
	if detail.Title == nil || *detail.Title != "detail-ok" {
		t.Fatalf("detail returned %+v, want title detail-ok", detail)
	}
}
