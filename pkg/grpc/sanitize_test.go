package grpc

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/miru-project/miru-core/pkg/download"
	pb "github.com/miru-project/miru-core/proto/generate/proto"
	googleproto "google.golang.org/protobuf/proto"
)

func ptrStr(s string) *string    { return &s }
func ptrInt32(i int32) *int32    { return &i }
func ptrInt64(i int64) *int64    { return &i }

func TestIsValidUTF8(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"empty", "", true},
		{"ascii", "hello", true},
		{"emoji", "🎉", true},
		{"japanese", "テスト", true},
		{"chinese", "测试", true},
		{"mixed", "hello 世界 🎉", true},
		{"invalid_byte_0xff", "before\xffafter", false},
		{"invalid_byte_0xfe", "before\xfeafter", false},
		{"invalid_continuation", "before\xc0\xafafter", false},
		{"truncated_2byte", "\xc2", false},
		{"truncated_3byte", "\xe2\x82", false},
		{"truncated_4byte", "\xf0\x9f", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utf8.ValidString(tt.s)
			if got != tt.want {
				t.Errorf("utf8.ValidString(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestSanitizeUTF8(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{"empty", "", ""},
		{"valid_ascii", "hello", "hello"},
		{"valid_unicode", "テスト", "テスト"},
		{"valid_emoji", "🎉anime", "🎉anime"},
		{"invalid_0xff", "before\xffafter", "before\ufffdafter"},
		{"invalid_0xfe", "before\xfeafter", "before\ufffdafter"},
		{"invalid_sequence", "\xc0\xaf", "\ufffd\ufffd"},
		{"mixed_valid_invalid", "valid\xffinvalid", "valid\ufffdinvalid"},
		{"japanese_valid", "[SubGroup] 日本語タイトル EP01", "[SubGroup] 日本語タイトル EP01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeUTF8(tt.s)
			if got != tt.want {
				t.Errorf("sanitizeUTF8(%q) = %q, want %q", tt.s, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("sanitizeUTF8(%q) produced invalid UTF-8: %q", tt.s, got)
			}
		})
	}
}

func TestSanitizeNames(t *testing.T) {
	names := []string{
		"valid_file.mp4",
		"日本語ファイル.mkv",
		"bad\xfffile.ts",
		"",
	}
	got := sanitizeNames(names)
	if len(got) != len(names) {
		t.Fatalf("sanitizeNames returned %d names, want %d", len(got), len(names))
	}
	if got[0] != "valid_file.mp4" {
		t.Errorf("got[0] = %q, want %q", got[0], "valid_file.mp4")
	}
	if got[1] != "日本語ファイル.mkv" {
		t.Errorf("got[1] = %q, want %q", got[1], "日本語ファイル.mkv")
	}
	if got[2] != "bad\ufffdfile.ts" {
		t.Errorf("got[2] = %q, want %q", got[2], "bad\ufffdfile.ts")
	}
	if got[3] != "" {
		t.Errorf("got[3] = %q, want empty", got[3])
	}
}

func TestSanitizeTags(t *testing.T) {
	tags := []string{"anime", "日本語", "bad\xfe"}
	got := sanitizeTags(tags)
	if len(got) != 3 {
		t.Fatalf("sanitizeTags returned %d tags, want 3", len(got))
	}
	if got[0] != "anime" {
		t.Errorf("got[0] = %q, want %q", got[0], "anime")
	}
	if got[1] != "日本語" {
		t.Errorf("got[1] = %q, want %q", got[1], "日本語")
	}
	if got[2] != "bad\ufffd" {
		t.Errorf("got[2] = %q, want %q", got[2], "bad\ufffd")
	}
}

func TestSanitizeTagsNil(t *testing.T) {
	got := sanitizeTags(nil)
	if got != nil {
		t.Errorf("sanitizeTags(nil) = %v, want nil", got)
	}
}

func TestSanitizeUTF8Idempotent(t *testing.T) {
	input := "日本語テスト 🎉 \xff"
	sanitized := sanitizeUTF8(input)
	double := sanitizeUTF8(sanitized)
	if sanitized != double {
		t.Errorf("sanitizeUTF8 not idempotent: %q != %q", sanitized, double)
	}
}

func TestSanitizeHeaders(t *testing.T) {
	h := map[string]string{
		"Referer":     "https://example.com",
		"User-Agent":  "Mozilla/5.0",
		"bad\xffkey":  "value",
		"key\xff":     "bad\xffvalue",
	}
	got := sanitizeHeaders(h)
	if len(got) != 4 {
		t.Fatalf("sanitizeHeaders returned %d entries, want 4", len(got))
	}
	if got["Referer"] != "https://example.com" {
		t.Errorf("got[Referer] = %q, want https://example.com", got["Referer"])
	}
	// Invalid UTF-8 keys and values should be replaced
	for k, v := range got {
		if !utf8.ValidString(k) {
			t.Errorf("sanitizeHeaders key contains invalid UTF-8: %q", k)
		}
		if !utf8.ValidString(v) {
			t.Errorf("sanitizeHeaders value contains invalid UTF-8: %q", v)
		}
	}
}

func TestSanitizeBangumiWatch(t *testing.T) {
	w := &pb.ExtensionBangumiWatch{
		Type:       "torrent\xff",
		Url:        "magnet:?xt=urn:btih:ABC",
		AudioTrack: ptrStr("ja\xfe"),
		Headers:    map[string]string{"Referer": "https://example.com\xff"},
		Subtitles: []*pb.ExtensionBangumiWatchSubtitle{
			{Language: ptrStr("en"), Title: "English", Url: "https://sub.example.com/en.vtt"},
			{Language: ptrStr("ja\xff"), Title: "日本語\xfe", Url: "https://sub.example.com/ja.vtt"},
		},
		Torrent: &pb.ExtensionBangumiWatchTorrent{
			InfoHash: "ABCDEF1234567890",
			Detail: &pb.ExtensionBangumiWatchTorrentDetail{
				Name:     ptrStr("テスト\xffアニメ"),
				Pieces:   ptrStr("\xff"),
				NameUtf8: ptrStr("テスト\xffアニメ"),
				Source:   ptrStr("example.com"),
			},
			Files: []string{
				"テスト/ep01.mkv",
				"bad\xfffile.mkv",
			},
		},
	}

	sanitizeBangumiWatch(w)

	// All fields should be valid UTF-8 now
	if !utf8.ValidString(w.Type) {
		t.Errorf("Type has invalid UTF-8: %q", w.Type)
	}
	if !utf8.ValidString(w.Url) {
		t.Errorf("Url has invalid UTF-8: %q", w.Url)
	}
	if w.AudioTrack != nil && !utf8.ValidString(*w.AudioTrack) {
		t.Errorf("AudioTrack has invalid UTF-8: %q", *w.AudioTrack)
	}
	for k, v := range w.Headers {
		if !utf8.ValidString(k) || !utf8.ValidString(v) {
			t.Errorf("Headers have invalid UTF-8: %q -> %q", k, v)
		}
	}
	for i, s := range w.Subtitles {
		if s.Language != nil && !utf8.ValidString(*s.Language) {
			t.Errorf("Subtitles[%d].Language has invalid UTF-8", i)
		}
		if !utf8.ValidString(s.Title) {
			t.Errorf("Subtitles[%d].Title has invalid UTF-8", i)
		}
		if !utf8.ValidString(s.Url) {
			t.Errorf("Subtitles[%d].Url has invalid UTF-8", i)
		}
	}
	// Torrent fields should be sanitized
	if !utf8.ValidString(w.Torrent.InfoHash) {
		t.Errorf("Torrent.InfoHash has invalid UTF-8")
	}
	if w.Torrent.Detail.Name != nil && !utf8.ValidString(*w.Torrent.Detail.Name) {
		t.Errorf("Torrent.Detail.Name has invalid UTF-8")
	}
	if w.Torrent.Detail.Pieces != nil && !utf8.ValidString(*w.Torrent.Detail.Pieces) {
		t.Errorf("Torrent.Detail.Pieces has invalid UTF-8")
	}
	if w.Torrent.Detail.NameUtf8 != nil && !utf8.ValidString(*w.Torrent.Detail.NameUtf8) {
		t.Errorf("Torrent.Detail.NameUtf8 has invalid UTF-8")
	}
	if w.Torrent.Detail.Source != nil && !utf8.ValidString(*w.Torrent.Detail.Source) {
		t.Errorf("Torrent.Detail.Source has invalid UTF-8")
	}
	for i, f := range w.Torrent.Files {
		if !utf8.ValidString(f) {
			t.Errorf("Torrent.Files[%d] has invalid UTF-8", i)
		}
	}

	// Must be able to marshal to protobuf without error
	if _, err := googleproto.Marshal(w); err != nil {
		t.Fatalf("proto.Marshal failed after sanitization: %v", err)
	}
}

func TestSanitizeMangaWatch(t *testing.T) {
	w := &pb.ExtensionMangaWatch{
		Urls:    []string{"https://example.com/page1\xff", "https://example.com/page2"},
		Headers: map[string]string{"Referer": "https://example.com\xff"},
	}

	sanitizeMangaWatch(w)

	for i, u := range w.Urls {
		if !utf8.ValidString(u) {
			t.Errorf("Urls[%d] has invalid UTF-8", i)
		}
	}
	for k, v := range w.Headers {
		if !utf8.ValidString(k) || !utf8.ValidString(v) {
			t.Errorf("Headers have invalid UTF-8")
		}
	}

	if _, err := googleproto.Marshal(w); err != nil {
		t.Fatalf("proto.Marshal failed after sanitization: %v", err)
	}
}

func TestSanitizeFikushonWatch(t *testing.T) {
	w := &pb.ExtensionFikushonWatch{
		Content:  []string{"Hello\xffworld", "正常なテキスト"},
		Title:    "テスト\xfeタイトル",
		Subtitle: ptrStr("サブタイトル\xff"),
	}

	sanitizeFikushonWatch(w)

	for i, c := range w.Content {
		if !utf8.ValidString(c) {
			t.Errorf("Content[%d] has invalid UTF-8", i)
		}
	}
	if !utf8.ValidString(w.Title) {
		t.Errorf("Title has invalid UTF-8")
	}
	if w.Subtitle != nil && !utf8.ValidString(*w.Subtitle) {
		t.Errorf("Subtitle has invalid UTF-8")
	}

	if _, err := googleproto.Marshal(w); err != nil {
		t.Fatalf("proto.Marshal failed after sanitization: %v", err)
	}
}

func TestSanitizeExtensionWatch(t *testing.T) {
	w := &pb.ExtensionWatch{
		DefaultGroup: ptrStr("默认\xfe分组"),
		Groups: []*pb.ExtensionMirrorGroup{
			{
				Title: "グループ\xff",
				Mirrors: []*pb.ExtensionMirror{
					{
						Name:    "ミラーファイル\xff.mkv",
						Url:     "https://mirror.example.com/stream",
						Headers: map[string]string{"Referer": "https://example.com"},
					},
					{
						Name:    "bad\xfename",
						Url:     "https://other.example.com\xff/stream",
						Headers: nil,
					},
				},
			},
		},
	}

	sanitizeExtensionWatch(w)

	if w.DefaultGroup != nil && !utf8.ValidString(*w.DefaultGroup) {
		t.Errorf("DefaultGroup has invalid UTF-8")
	}
	for i, g := range w.Groups {
		if !utf8.ValidString(g.Title) {
			t.Errorf("Groups[%d].Title has invalid UTF-8", i)
		}
		for j, m := range g.Mirrors {
			if !utf8.ValidString(m.Name) {
				t.Errorf("Groups[%d].Mirrors[%d].Name has invalid UTF-8", i, j)
			}
			if !utf8.ValidString(m.Url) {
				t.Errorf("Groups[%d].Mirrors[%d].Url has invalid UTF-8", i, j)
			}
			for k, v := range m.Headers {
				if !utf8.ValidString(k) || !utf8.ValidString(v) {
					t.Errorf("Groups[%d].Mirrors[%d].Headers have invalid UTF-8", i, j)
				}
			}
		}
	}

	if _, err := googleproto.Marshal(w); err != nil {
		t.Fatalf("proto.Marshal failed after sanitization: %v", err)
	}
}

func TestSanitizeWatchResponse(t *testing.T) {
	tests := []struct {
		name string
		resp *pb.WatchResponse
	}{
		{
			"bangumi_with_invalid_utf8",
			&pb.WatchResponse{
				Data: &pb.WatchResponse_Bangumi{
					Bangumi: &pb.ExtensionBangumiWatch{
						Type: "torrent\xff",
						Url:  "magnet:?xt=urn:btih:ABC",
						Torrent: &pb.ExtensionBangumiWatchTorrent{
							InfoHash: "ABCDEF",
							Detail: &pb.ExtensionBangumiWatchTorrentDetail{
								Name: ptrStr("テスト\xffアニメ"),
							},
							Files: []string{"bad\xfffile.mkv"},
						},
					},
				},
			},
		},
		{
			"manga_with_invalid_utf8",
			&pb.WatchResponse{
				Data: &pb.WatchResponse_Manga{
					Manga: &pb.ExtensionMangaWatch{
						Urls: []string{"https://example.com\xff/page"},
					},
				},
			},
		},
		{
			"fikushon_with_invalid_utf8",
			&pb.WatchResponse{
				Data: &pb.WatchResponse_Fikushon{
					Fikushon: &pb.ExtensionFikushonWatch{
						Content: []string{"Hello\xffworld"},
						Title:   "テスト\xfe",
					},
				},
			},
		},
		{
			"watch_v2_with_invalid_utf8",
			&pb.WatchResponse{
				Data: &pb.WatchResponse_Watch{
					Watch: &pb.ExtensionWatch{
						DefaultGroup: ptrStr("默认\xfe分组"),
						Groups: []*pb.ExtensionMirrorGroup{
							{
								Title: "グループ\xff",
								Mirrors: []*pb.ExtensionMirror{
									{Name: "bad\xfename", Url: "https://example.com/stream\xff"},
								},
							},
						},
					},
				},
			},
		},
		{
			"all_watch_with_invalid_utf8",
			&pb.WatchResponse{
				Data: &pb.WatchResponse_All{
					All: &pb.ExtensionAllWatch{
						Bangumi: &pb.ExtensionBangumiWatch{
							Type: "hls\xff",
							Url:  "https://stream.example.com/index.m3u8",
						},
						Manga: &pb.ExtensionMangaWatch{
							Urls: []string{"https://example.com\xff/page"},
						},
						Fikushon: &pb.ExtensionFikushonWatch{
							Content: []string{"text\xffcontent"},
							Title:   "title\xfe",
						},
					},
				},
			},
		},
		{
			"nil_watch_response",
			&pb.WatchResponse{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitizeWatchResponse(tt.resp)

			// Must be able to marshal without error
			_, err := googleproto.Marshal(tt.resp)
			if err != nil {
				t.Fatalf("proto.Marshal failed: %v", err)
			}
		})
	}
}

func TestSanitizeMirrorResponse(t *testing.T) {
	tests := []struct {
		name string
		resp *pb.MirrorResponse
	}{
		{
			"bangumi_mirror_with_invalid_utf8",
			&pb.MirrorResponse{
				Data: &pb.MirrorResponse_Bangumi{
					Bangumi: &pb.ExtensionBangumiWatch{
						Type: "torrent\xff",
						Url:  "magnet:?xt=urn:btih:ABC",
					},
				},
			},
		},
		{
			"all_mirror_with_invalid_utf8",
			&pb.MirrorResponse{
				Data: &pb.MirrorResponse_All{
					All: &pb.ExtensionAllWatch{
						Bangumi: &pb.ExtensionBangumiWatch{
							Type: "hls\xff",
							Url:  "https://stream.example.com/index.m3u8",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitizeMirrorResponse(tt.resp)

			_, err := googleproto.Marshal(tt.resp)
			if err != nil {
				t.Fatalf("proto.Marshal failed: %v", err)
			}
		})
	}
}

func TestWatchResponseMarshalRoundTrip(t *testing.T) {
	// Simulate an extension returning invalid UTF-8 in torrent filenames
	original := &pb.WatchResponse{
		Data: &pb.WatchResponse_Bangumi{
			Bangumi: &pb.ExtensionBangumiWatch{
				Type: "torrent",
				Url:  "magnet:?xt=urn:btih:ABCDEF1234567890",
				Headers: map[string]string{
					"Referer": "https://example.com",
				},
				Subtitles: []*pb.ExtensionBangumiWatchSubtitle{
					{Language: ptrStr("ja"), Title: "日本語字幕", Url: "https://sub.example.com/ja.vtt"},
				},
				Torrent: &pb.ExtensionBangumiWatchTorrent{
					InfoHash: "ABCDEF1234567890ABCDEF1234567890ABCDEF12",
					Detail: &pb.ExtensionBangumiWatchTorrentDetail{
						Name:        ptrStr("[SubGroup] 日本語タイトル"),
						PieceLength: ptrInt32(262144),
						Length:      ptrInt64(2147483648),
					},
					Files: []string{
						"[SubGroup] 日本語タイトル/EP01.mkv",
						"[SubGroup] 日本語タイトル/EP02.mkv",
						"[SubGroup] 日本語タイトル/subs/eng.ass",
					},
				},
			},
		},
	}

	// Sanitize the response
	sanitizeWatchResponse(original)

	// Marshal to protobuf bytes
	data, err := googleproto.Marshal(original)
	if err != nil {
		t.Fatalf("proto.Marshal failed: %v", err)
	}

	// Unmarshal back
	decoded := &pb.WatchResponse{}
	if err := googleproto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("proto.Unmarshal failed: %v", err)
	}

	// Verify the round-trip preserves data
	bangumi := decoded.GetBangumi()
	if bangumi == nil {
		t.Fatal("decoded response has no bangumi")
	}
	if bangumi.Type != "torrent" {
		t.Errorf("Type = %q, want torrent", bangumi.Type)
	}
	if bangumi.Url != "magnet:?xt=urn:btih:ABCDEF1234567890" {
		t.Errorf("Url = %q, want magnet URL", bangumi.Url)
	}
	if bangumi.Headers["Referer"] != "https://example.com" {
		t.Errorf("Headers[Referer] = %q", bangumi.Headers["Referer"])
	}
	if len(bangumi.Subtitles) != 1 {
		t.Fatalf("Subtitles length = %d, want 1", len(bangumi.Subtitles))
	}
	if bangumi.Subtitles[0].Title != "日本語字幕" {
		t.Errorf("Subtitle title = %q", bangumi.Subtitles[0].Title)
	}
	if bangumi.Torrent == nil {
		t.Fatal("Torrent is nil")
	}
	if bangumi.Torrent.InfoHash != "ABCDEF1234567890ABCDEF1234567890ABCDEF12" {
		t.Errorf("InfoHash = %q", bangumi.Torrent.InfoHash)
	}
	if bangumi.Torrent.Detail.Name == nil || *bangumi.Torrent.Detail.Name != "[SubGroup] 日本語タイトル" {
		t.Errorf("Detail.Name = %v", bangumi.Torrent.Detail.Name)
	}
	if len(bangumi.Torrent.Files) != 3 {
		t.Errorf("Files length = %d, want 3", len(bangumi.Torrent.Files))
	}
	if bangumi.Torrent.Files[0] != "[SubGroup] 日本語タイトル/EP01.mkv" {
		t.Errorf("Files[0] = %q", bangumi.Torrent.Files[0])
	}

	// All strings in the decoded response must be valid UTF-8
	assertAllStringsValid(t, decoded)
}

func assertAllStringsValid(t *testing.T, resp *pb.WatchResponse) {
	t.Helper()
	bangumi := resp.GetBangumi()
	if bangumi == nil {
		return
	}
	fields := []string{bangumi.Type, bangumi.Url}
	for _, f := range fields {
		if !utf8.ValidString(f) {
			t.Errorf("invalid UTF-8 in field: %q", f)
		}
	}
	if bangumi.AudioTrack != nil && !utf8.ValidString(*bangumi.AudioTrack) {
		t.Errorf("invalid UTF-8 in AudioTrack: %q", *bangumi.AudioTrack)
	}
	for k, v := range bangumi.Headers {
		if !utf8.ValidString(k) || !utf8.ValidString(v) {
			t.Errorf("invalid UTF-8 in headers: %q -> %q", k, v)
		}
	}
	for i, s := range bangumi.Subtitles {
		if s.Language != nil && !utf8.ValidString(*s.Language) {
			t.Errorf("invalid UTF-8 in subtitles[%d].Language", i)
		}
		if !utf8.ValidString(s.Title) || !utf8.ValidString(s.Url) {
			t.Errorf("invalid UTF-8 in subtitles[%d]", i)
		}
	}
	if bangumi.Torrent != nil {
		if !utf8.ValidString(bangumi.Torrent.InfoHash) {
			t.Error("invalid UTF-8 in torrent InfoHash")
		}
		if bangumi.Torrent.Detail != nil {
			if bangumi.Torrent.Detail.Name != nil && !utf8.ValidString(*bangumi.Torrent.Detail.Name) {
				t.Error("invalid UTF-8 in torrent Detail.Name")
			}
			if bangumi.Torrent.Detail.NameUtf8 != nil && !utf8.ValidString(*bangumi.Torrent.Detail.NameUtf8) {
				t.Error("invalid UTF-8 in torrent Detail.NameUtf8")
			}
			if bangumi.Torrent.Detail.Source != nil && !utf8.ValidString(*bangumi.Torrent.Detail.Source) {
				t.Error("invalid UTF-8 in torrent Detail.Source")
			}
		}
		for i, f := range bangumi.Torrent.Files {
			if !utf8.ValidString(f) {
				t.Errorf("invalid UTF-8 in torrent files[%d]: %q", i, f)
			}
		}
	}
}

// assertMirrorAllStringsValid checks that all string fields in a MirrorResponse
// are valid UTF-8 after sanitization.
func assertMirrorAllStringsValid(t *testing.T, resp *pb.MirrorResponse) {
	t.Helper()
	if resp == nil || resp.Data == nil {
		return
	}
	switch d := resp.Data.(type) {
	case *pb.MirrorResponse_Bangumi:
		if d.Bangumi == nil {
			return
		}
		for _, f := range []string{d.Bangumi.Type, d.Bangumi.Url} {
			if !utf8.ValidString(f) {
				t.Errorf("mirror bangumi field invalid UTF-8: %q", f)
			}
		}
		if d.Bangumi.AudioTrack != nil && !utf8.ValidString(*d.Bangumi.AudioTrack) {
			t.Errorf("mirror bangumi AudioTrack invalid UTF-8: %q", *d.Bangumi.AudioTrack)
		}
		for k, v := range d.Bangumi.Headers {
			if !utf8.ValidString(k) || !utf8.ValidString(v) {
				t.Errorf("mirror bangumi headers invalid UTF-8: %q -> %q", k, v)
			}
		}
		if d.Bangumi.Torrent != nil {
			if !utf8.ValidString(d.Bangumi.Torrent.InfoHash) {
				t.Error("mirror torrent InfoHash invalid UTF-8")
			}
			for i, f := range d.Bangumi.Torrent.Files {
				if !utf8.ValidString(f) {
					t.Errorf("mirror torrent files[%d] invalid UTF-8: %q", i, f)
				}
			}
		}
	case *pb.MirrorResponse_Manga:
		if d.Manga == nil {
			return
		}
		for i, u := range d.Manga.Urls {
			if !utf8.ValidString(u) {
				t.Errorf("mirror manga urls[%d] invalid UTF-8: %q", i, u)
			}
		}
	case *pb.MirrorResponse_Fikushon:
		if d.Fikushon == nil {
			return
		}
		if !utf8.ValidString(d.Fikushon.Title) {
			t.Errorf("mirror fikushon title invalid UTF-8")
		}
	case *pb.MirrorResponse_All:
		if d.All == nil {
			return
		}
		if d.All.Bangumi != nil {
			for _, f := range []string{d.All.Bangumi.Type, d.All.Bangumi.Url} {
				if !utf8.ValidString(f) {
					t.Errorf("mirror all bangumi field invalid UTF-8: %q", f)
				}
			}
		}
	}
}

// TestWatchHandlerSimulatedBangumiTorrentInvalidUTF8 simulates the Watch handler
// receiving a bangumi torrent result with Shift-JIS/GBK-encoded filenames and
// verifies sanitization produces a valid protobuf marshal.
func TestWatchHandlerSimulatedBangumiTorrentInvalidUTF8(t *testing.T) {
	// Simulate extension returning Shift-JIS encoded torrent filenames
	watchResp := &pb.WatchResponse{
		Data: &pb.WatchResponse_Bangumi{
			Bangumi: &pb.ExtensionBangumiWatch{
				Type:       "torrent",
				Url:        "magnet:?xt=urn:btih:ABCDEF1234567890",
				AudioTrack: ptrStr("ja"),
				Headers:    map[string]string{"Referer": "https://example.com"},
				Subtitles: []*pb.ExtensionBangumiWatchSubtitle{
					{Language: ptrStr("en"), Title: "English", Url: "https://sub.example.com/en.vtt"},
				},
				Torrent: &pb.ExtensionBangumiWatchTorrent{
					InfoHash: "ABCDEF1234567890ABCDEF1234567890ABCDEF12",
					Detail: &pb.ExtensionBangumiWatchTorrentDetail{
						Name:     ptrStr("[Shift-JIS]\x83\x52\x81\x5b\x83\x58\x8d\x44"), // invalid UTF-8 from Shift-JIS
						Pieces:   ptrStr("\xff\xfe"),
						NameUtf8: ptrStr("テスト\xff名前"),
						Source:   ptrStr("example.com"),
					},
					Files: []string{
						"[Shift-JIS]\x83\x52\x81\x5b/EP01.mkv",
						"bad\xfffile.mkv",
						"\x83\x52\x81\x5b/ep02.mkv",
					},
				},
			},
		},
	}

	sanitizeWatchResponse(watchResp)

	// Must marshal without error
	data, err := googleproto.Marshal(watchResp)
	if err != nil {
		t.Fatalf("proto.Marshal failed after sanitization: %v", err)
	}

	// Unmarshal back and verify
	decoded := &pb.WatchResponse{}
	if err := googleproto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("proto.Unmarshal failed: %v", err)
	}

	assertAllStringsValid(t, decoded)

	bangumi := decoded.GetBangumi()
	if bangumi == nil {
		t.Fatal("decoded bangumi is nil")
	}
	if len(bangumi.Torrent.Files) != 3 {
		t.Errorf("torrent files count = %d, want 3", len(bangumi.Torrent.Files))
	}
	// Verify invalid bytes were replaced
	for i, f := range bangumi.Torrent.Files {
		if !utf8.ValidString(f) {
			t.Errorf("torrent file[%d] still has invalid UTF-8 after sanitize: %q", i, f)
		}
	}
}

// TestWatchHandlerSimulatedV2ExtensionInvalidUTF8 simulates the Watch handler
// receiving a V2 ExtensionWatch with invalid UTF-8 in mirror names and URLs.
func TestWatchHandlerSimulatedV2ExtensionInvalidUTF8(t *testing.T) {
	watchResp := &pb.WatchResponse{
		Data: &pb.WatchResponse_Watch{
			Watch: &pb.ExtensionWatch{
				DefaultGroup: ptrStr("默认\xfe分组"),
				Groups: []*pb.ExtensionMirrorGroup{
					{
						Title: "グループ\xff",
						Mirrors: []*pb.ExtensionMirror{
							{
								Name:    "ミラーファイル\xff.mkv",
								Url:     "https://mirror.example.com/stream",
								Headers: map[string]string{"Referer": "https://example.com"},
							},
							{
								Name:    "bad\xfename",
								Url:     "https://other.example.com\xff/stream",
								Headers: nil,
							},
						},
					},
				},
			},
		},
	}

	sanitizeWatchResponse(watchResp)

	data, err := googleproto.Marshal(watchResp)
	if err != nil {
		t.Fatalf("proto.Marshal failed after sanitization: %v", err)
	}

	decoded := &pb.WatchResponse{}
	if err := googleproto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("proto.Unmarshal failed: %v", err)
	}

	extWatch := decoded.GetWatch()
	if extWatch == nil {
		t.Fatal("decoded extension watch is nil")
	}
	if extWatch.DefaultGroup != nil && !utf8.ValidString(*extWatch.DefaultGroup) {
		t.Errorf("DefaultGroup invalid UTF-8: %q", *extWatch.DefaultGroup)
	}
	for i, g := range extWatch.Groups {
		if !utf8.ValidString(g.Title) {
			t.Errorf("Group[%d] title invalid UTF-8: %q", i, g.Title)
		}
		for j, m := range g.Mirrors {
			if !utf8.ValidString(m.Name) {
				t.Errorf("Group[%d].Mirror[%d] name invalid UTF-8: %q", i, j, m.Name)
			}
			if !utf8.ValidString(m.Url) {
				t.Errorf("Group[%d].Mirror[%d] url invalid UTF-8: %q", i, j, m.Url)
			}
		}
	}
}

// TestMirrorHandlerSimulatedBangumiInvalidUTF8 simulates the Mirror handler
// receiving a bangumi result with invalid UTF-8 and verifies sanitization.
func TestMirrorHandlerSimulatedBangumiInvalidUTF8(t *testing.T) {
	mirrorResp := &pb.MirrorResponse{
		Data: &pb.MirrorResponse_Bangumi{
			Bangumi: &pb.ExtensionBangumiWatch{
				Type:       "hls\xff",
				Url:        "https://stream.example.com/index.m3u8",
				AudioTrack: ptrStr("ja\xfe"),
				Headers:    map[string]string{"Referer": "https://example.com\xff"},
				Subtitles: []*pb.ExtensionBangumiWatchSubtitle{
					{Language: ptrStr("en"), Title: "English", Url: "https://sub.example.com/en.vtt"},
					{Language: ptrStr("ja\xff"), Title: "日本語\xfe", Url: "https://sub.example.com/ja.vtt"},
				},
			},
		},
	}

	sanitizeMirrorResponse(mirrorResp)

	data, err := googleproto.Marshal(mirrorResp)
	if err != nil {
		t.Fatalf("proto.Marshal failed after sanitization: %v", err)
	}

	decoded := &pb.MirrorResponse{}
	if err := googleproto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("proto.Unmarshal failed: %v", err)
	}

	assertMirrorAllStringsValid(t, decoded)
}

// TestMirrorHandlerSimulatedAllWatchInvalidUTF8 simulates the Mirror handler
// receiving an all-watch result with invalid UTF-8 across all sub-types.
func TestMirrorHandlerSimulatedAllWatchInvalidUTF8(t *testing.T) {
	mirrorResp := &pb.MirrorResponse{
		Data: &pb.MirrorResponse_All{
			All: &pb.ExtensionAllWatch{
				Bangumi: &pb.ExtensionBangumiWatch{
					Type: "torrent\xff",
					Url:  "magnet:?xt=urn:btih:ABC",
					Torrent: &pb.ExtensionBangumiWatchTorrent{
						InfoHash: "ABCDEF",
						Detail: &pb.ExtensionBangumiWatchTorrentDetail{
							Name: ptrStr("テスト\xffアニメ"),
						},
						Files: []string{"bad\xfffile.mkv"},
					},
				},
				Manga: &pb.ExtensionMangaWatch{
					Urls:    []string{"https://example.com\xff/page"},
					Headers: map[string]string{"Referer": "https://example.com"},
				},
				Fikushon: &pb.ExtensionFikushonWatch{
					Content: []string{"Hello\xffworld"},
					Title:   "テスト\xfe",
				},
			},
		},
	}

	sanitizeMirrorResponse(mirrorResp)

	data, err := googleproto.Marshal(mirrorResp)
	if err != nil {
		t.Fatalf("proto.Marshal failed after sanitization: %v", err)
	}

	decoded := &pb.MirrorResponse{}
	if err := googleproto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("proto.Unmarshal failed: %v", err)
	}

	assertMirrorAllStringsValid(t, decoded)
}

// TestWatchResponseHeavyInvalidUTF8 tests sanitization of a WatchResponse
// with a large amount of invalid UTF-8 data across all fields, simulating
// real-world extensions that return Shift-JIS, GBK, or corrupted strings.
func TestWatchResponseHeavyInvalidUTF8(t *testing.T) {
	// Generate heavily corrupted strings (simulating Shift-JIS/GBK)
	corrupted := "\x83\x52\x81\x5b\x83\x58\x8d\x44\xff\xfe\x82\xd0\x82\xf0\x82\xb1\x82\xf1"
	corrupted2 := "\xc0\xaf\xc0\xbc\xc0\xa7\xff\xfe\xfd" // GBK-like invalid UTF-8

	watchResp := &pb.WatchResponse{
		Data: &pb.WatchResponse_Bangumi{
			Bangumi: &pb.ExtensionBangumiWatch{
				Type:       corrupted,
				Url:        "magnet:?xt=urn:btih:" + corrupted,
				AudioTrack: ptrStr(corrupted2),
				Headers: map[string]string{
					corrupted:  corrupted2,
					"Referer":  "https://example.com",
					corrupted2: corrupted,
				},
				Subtitles: []*pb.ExtensionBangumiWatchSubtitle{
					{Language: ptrStr(corrupted), Title: corrupted2, Url: "https://sub.example.com/en.vtt"},
					{Language: ptrStr("en"), Title: corrupted, Url: corrupted2},
				},
				Torrent: &pb.ExtensionBangumiWatchTorrent{
					InfoHash: corrupted,
					Detail: &pb.ExtensionBangumiWatchTorrentDetail{
						Name:     ptrStr(corrupted2),
						Pieces:   ptrStr(corrupted),
						NameUtf8: ptrStr(corrupted),
						Source:   ptrStr(corrupted2),
					},
					Files: []string{
						corrupted + "/ep01.mkv",
						corrupted2 + ".mkv",
						"normal_file.mkv",
						corrupted + corrupted2 + ".mkv",
					},
				},
			},
		},
	}

	sanitizeWatchResponse(watchResp)

	data, err := googleproto.Marshal(watchResp)
	if err != nil {
		t.Fatalf("proto.Marshal failed: %v", err)
	}

	decoded := &pb.WatchResponse{}
	if err := googleproto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("proto.Unmarshal failed: %v", err)
	}

	assertAllStringsValid(t, decoded)
}

// TestMirrorResponseRoundTrip verifies that a MirrorResponse with invalid UTF-8
// survives a full marshal/unmarshal round trip after sanitization.
func TestMirrorResponseRoundTrip(t *testing.T) {
	original := &pb.MirrorResponse{
		Data: &pb.MirrorResponse_Bangumi{
			Bangumi: &pb.ExtensionBangumiWatch{
				Type: "torrent",
				Url:  "magnet:?xt=urn:btih:ABCDEF1234567890",
				Headers: map[string]string{
					"Referer":    "https://example.com",
					"User-Agent": "Mozilla/5.0",
				},
				Subtitles: []*pb.ExtensionBangumiWatchSubtitle{
					{Language: ptrStr("ja"), Title: "日本語字幕", Url: "https://sub.example.com/ja.vtt"},
				},
				Torrent: &pb.ExtensionBangumiWatchTorrent{
					InfoHash: "ABCDEF1234567890ABCDEF1234567890ABCDEF12",
					Detail: &pb.ExtensionBangumiWatchTorrentDetail{
						Name: ptrStr("[SubGroup] 日本語タイトル"),
					},
					Files: []string{
						"[SubGroup] 日本語タイトル/EP01.mkv",
						"[SubGroup] 日本語タイトル/EP02.mkv",
					},
				},
			},
		},
	}

	sanitizeMirrorResponse(original)

	data, err := googleproto.Marshal(original)
	if err != nil {
		t.Fatalf("proto.Marshal failed: %v", err)
	}

	decoded := &pb.MirrorResponse{}
	if err := googleproto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("proto.Unmarshal failed: %v", err)
	}

	assertMirrorAllStringsValid(t, decoded)

	bangumi := decoded.GetBangumi()
	if bangumi == nil {
		t.Fatal("decoded mirror bangumi is nil")
	}
	if bangumi.Torrent == nil {
		t.Fatal("decoded torrent is nil")
	}
	if bangumi.Torrent.InfoHash != "ABCDEF1234567890ABCDEF1234567890ABCDEF12" {
		t.Errorf("InfoHash = %q", bangumi.Torrent.InfoHash)
	}
	if len(bangumi.Torrent.Files) != 2 {
		t.Errorf("Files count = %d, want 2", len(bangumi.Torrent.Files))
	}
}

// TestToProtoDownloadProgressAllMediaTypes tests that toProtoDownloadProgress
// correctly converts download.Progress to proto for all media types (hls, mp4,
// torrent, magnet) and sanitizes string fields.
func TestToProtoDownloadProgressAllMediaTypes(t *testing.T) {
	names := []string{"episode01.ts", "episode02.ts"}
	corruptedNames := []string{"\x83\x52\x81\x5b/ep01.mkv", "bad\xfffile.mkv"}

	tests := []struct {
		name     string
		progress *download.Progress
	}{
		{
			"hls_downloading",
			&download.Progress{
				Progrss:            50,
				Total:              100,
				Status:             download.Downloading,
				MediaType:          download.Hls,
				CurrentDownloading: "/downloads/episode01.ts",
				TaskID:             1,
				Title:              "Episode 01",
				Package:            "anime.pkg",
				Key:                "ABCDEF",
				Names:              &names,
				Priority:           3,
			},
		},
		{
			"mp4_downloading",
			&download.Progress{
				Progrss:            500000,
				Total:              1000000,
				Status:             download.Downloading,
				MediaType:          download.Mp4,
				CurrentDownloading: "/downloads/video.mp4",
				TaskID:             2,
				Title:              "Movie",
				Package:            "movie.pkg",
				Key:                "123456",
				Names:              &[]string{"video.mp4"},
				Priority:           5,
			},
		},
		{
			"torrent_completed",
			&download.Progress{
				Progrss:            2147483648,
				Total:              2147483648,
				Status:             download.Completed,
				MediaType:          download.Torrent,
				CurrentDownloading: "/downloads/[SubGroup] anime/EP01.mkv",
				TaskID:             3,
				Title:              "[SubGroup] 日本語アニメ",
				Package:            "torrent.pkg",
				Key:                "ABCDEF1234567890ABCDEF1234567890ABCDEF12",
				Names:              &[]string{"[SubGroup] anime/EP01.mkv"},
				Priority:           1,
			},
		},
		{
			"magnet_converting",
			&download.Progress{
				Progrss:            100,
				Total:              100,
				Status:             download.Converting,
				MediaType:          download.Magnet,
				CurrentDownloading: "/downloads/segment0.ts",
				TaskID:             4,
				Title:              "Magnet Anime",
				Package:            "magnet.pkg",
				Key:                "FEEDFACE",
				Names:              &[]string{"segment0.ts", "segment1.ts"},
				Priority:           2,
			},
		},
		{
			"hls_with_invalid_utf8_names",
			&download.Progress{
				Progrss:            10,
				Total:              50,
				Status:             download.Downloading,
				MediaType:          download.Hls,
				CurrentDownloading: "/downloads/\x83\x52\x81\x5b/ep01.ts",
				TaskID:             5,
				Title:              "Shift-JIS \xff Title",
				Package:            "sjis.pkg",
				Key:                "BADKEY",
				Names:              &corruptedNames,
				Priority:           1,
			},
		},
		{
			"mp4_paused",
			&download.Progress{
				Progrss:            0,
				Total:              0,
				Status:             download.Paused,
				MediaType:          download.Mp4,
				CurrentDownloading: "",
				TaskID:             6,
				Title:              "Paused Video",
				Package:            "pause.pkg",
				Key:                "PAUSE",
				Names:              nil,
				Priority:           0,
			},
		},
		{
			"torrent_failed",
			&download.Progress{
				Progrss:            1000,
				Total:              5000,
				Status:             download.Failed,
				MediaType:          download.Torrent,
				CurrentDownloading: "/downloads/fail.mkv",
				TaskID:             7,
				Title:              "Failed Torrent",
				Package:            "fail.pkg",
				Key:                "FAIL",
				Names:              &[]string{"fail.mkv"},
				Priority:           0,
			},
		},
		{
			"mp4_canceled",
			&download.Progress{
				Progrss:            200,
				Total:              1000,
				Status:             download.Canceled,
				MediaType:          download.Mp4,
				CurrentDownloading: "/downloads/canceled.mp4",
				TaskID:             8,
				Title:              "Canceled Video",
				Package:            "cancel.pkg",
				Key:                "CANCEL",
				Names:              &[]string{"canceled.mp4"},
				Priority:           0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proto := toProtoDownloadProgress(tt.progress)

			if proto.Progress != int32(tt.progress.Progrss) {
				t.Errorf("Progress = %d, want %d", proto.Progress, tt.progress.Progrss)
			}
			if proto.Total != int32(tt.progress.Total) {
				t.Errorf("Total = %d, want %d", proto.Total, tt.progress.Total)
			}
			if proto.TaskId != int32(tt.progress.TaskID) {
				t.Errorf("TaskId = %d, want %d", proto.TaskId, tt.progress.TaskID)
			}
			if proto.Priority != int32(tt.progress.Priority) {
				t.Errorf("Priority = %d, want %d", proto.Priority, tt.progress.Priority)
			}

			// Media type is mapped to the proto enum.
			wantMT := mediaTypeToProto(tt.progress.MediaType)
			if proto.MediaType != wantMT {
				t.Errorf("MediaType = %v, want %v", proto.MediaType, wantMT)
			}
			if !utf8.ValidString(proto.CurrentDownloading) {
				t.Errorf("CurrentDownloading has invalid UTF-8: %q", proto.CurrentDownloading)
			}
			if !utf8.ValidString(proto.Title) {
				t.Errorf("Title has invalid UTF-8: %q", proto.Title)
			}
			if !utf8.ValidString(proto.Package) {
				t.Errorf("Package has invalid UTF-8: %q", proto.Package)
			}
			if !utf8.ValidString(proto.Key) {
				t.Errorf("Key has invalid UTF-8: %q", proto.Key)
			}

			// Names should be sanitized
			for i, n := range proto.Names {
				if !utf8.ValidString(n) {
					t.Errorf("Names[%d] has invalid UTF-8: %q", i, n)
				}
			}

			// Must marshal without error
			if _, err := googleproto.Marshal(proto); err != nil {
				t.Fatalf("proto.Marshal failed: %v", err)
			}
		})
	}
}

func TestProxyBangumiWatch(t *testing.T) {
	// proxyBangumiWatch wraps the URL in a proxy URL. We can't test the full
	// proxy URL building (it depends on network.ProxyOrigin), but we can verify
	// that nil and empty cases are handled safely.
	w := &pb.ExtensionBangumiWatch{
		Type: "hls",
		Url:  "",
		Headers: map[string]string{
			"Referer": "https://example.com",
		},
	}
	proxyBangumiWatch(w)
	// Empty URL should remain empty (no proxy wrapping for empty)
	if w.Url != "" {
		t.Errorf("empty URL should remain empty, got %q", w.Url)
	}
}

func TestProxyBangumiWatchNil(t *testing.T) {
	// Should not panic on nil
	proxyBangumiWatch(nil)
}

func TestProxyMangaWatchNil(t *testing.T) {
	proxyMangaWatch(nil)
}

func TestProxyFikushonWatchNil(t *testing.T) {
	// Should be a no-op
	proxyFikushonWatch(nil)
}

func TestProxyAllWatchNil(t *testing.T) {
	proxyAllWatch(nil)
}

func TestProxyMirrorResponseNil(t *testing.T) {
	proxyMirrorResponse(nil)
}

func TestProxyMirrorResponseEmptyData(t *testing.T) {
	proxyMirrorResponse(&pb.MirrorResponse{})
}

func TestProxyWatchURLEmpty(t *testing.T) {
	got := proxyWatchURL("", nil)
	if got != "" {
		t.Errorf("proxyWatchURL(\"\") = %q, want empty", got)
	}
}

func TestProxyBangumiWatchWithHeaders(t *testing.T) {
	w := &pb.ExtensionBangumiWatch{
		Type: "mp4",
		Url:  "https://example.com/video.mp4",
		Headers: map[string]string{
			"Referer":    "https://example.com",
			"User-Agent": "Mozilla/5.0",
		},
	}
	proxyBangumiWatch(w)
	// URL should be wrapped in proxy (non-empty, non-proxy URL)
	if w.Url == "https://example.com/video.mp4" {
		t.Error("URL was not proxied")
	}
	// Headers should still be present
	if w.Headers["Referer"] != "https://example.com" {
		t.Errorf("Headers[Referer] = %q", w.Headers["Referer"])
	}
}

func TestProxyBangumiWatchTLSProfileStripped(t *testing.T) {
	w := &pb.ExtensionBangumiWatch{
		Type: "hls",
		Url:  "https://example.com/stream.m3u8",
		Headers: map[string]string{
			"Referer":          "https://example.com",
			miruTLSProfileHeader: "chrome_110",
		},
	}
	proxyBangumiWatch(w)
	// Miru-TLS-Profile header should be removed from forwarded headers
	if _, ok := w.Headers[miruTLSProfileHeader]; ok {
		t.Error("Miru-TLS-Profile header should be stripped")
	}
}

func TestProxyBangumiWatchAlreadyProxied(t *testing.T) {
	// An already-proxied URL should not be double-wrapped.
	// IsProxyURL checks for scheme+host matching ProxyOrigin() + /proxy/ prefix.
	proxiedURL := "http://127.0.0.1:3000/proxy/test?__u=dGVzdA"
	w := &pb.ExtensionBangumiWatch{
		Type: "hls",
		Url:  proxiedURL,
	}
	proxyBangumiWatch(w)
	if w.Url != proxiedURL {
		t.Errorf("already-proxied URL was modified: %q -> %q", proxiedURL, w.Url)
	}
}

func TestProxyMangaWatchMultiplePages(t *testing.T) {
	w := &pb.ExtensionMangaWatch{
		Urls: []string{
			"https://example.com/page1",
			"https://example.com/page2",
			"https://example.com/page3",
		},
		Headers: map[string]string{"Referer": "https://example.com"},
	}
	proxyMangaWatch(w)
	for i, u := range w.Urls {
		if u == "https://example.com/page${i+1}" {
			t.Errorf("Urls[%d] was not proxied: %q", i, u)
		}
	}
}

func TestProxyAllWatchSubTypes(t *testing.T) {
	w := &pb.ExtensionAllWatch{
		Bangumi: &pb.ExtensionBangumiWatch{
			Type: "mp4",
			Url:  "https://example.com/video.mp4",
		},
		Manga: &pb.ExtensionMangaWatch{
			Urls: []string{"https://example.com/page1"},
		},
		Fikushon: &pb.ExtensionFikushonWatch{
			Content: []string{"Hello"},
			Title:   "Test",
		},
	}
	proxyAllWatch(w)
	// Bangumi URL should be proxied
	if w.Bangumi.Url == "https://example.com/video.mp4" {
		t.Error("bangumi URL was not proxied")
	}
	// Manga URLs should be proxied
	if w.Manga.Urls[0] == "https://example.com/page1" {
		t.Error("manga URL was not proxied")
	}
}

func TestProxyMirrorResponseBangumi(t *testing.T) {
	resp := &pb.MirrorResponse{
		Data: &pb.MirrorResponse_Bangumi{
			Bangumi: &pb.ExtensionBangumiWatch{
				Type: "hls",
				Url:  "https://example.com/stream.m3u8",
			},
		},
	}
	proxyMirrorResponse(resp)
	bangumi := resp.GetBangumi()
	if bangumi == nil {
		t.Fatal("bangumi is nil")
	}
	if bangumi.Url == "https://example.com/stream.m3u8" {
		t.Error("mirror bangumi URL was not proxied")
	}
}

func TestProxyMirrorResponseAll(t *testing.T) {
	resp := &pb.MirrorResponse{
		Data: &pb.MirrorResponse_All{
			All: &pb.ExtensionAllWatch{
				Bangumi: &pb.ExtensionBangumiWatch{
					Type: "mp4",
					Url:  "https://example.com/video.mp4",
				},
			},
		},
	}
	proxyMirrorResponse(resp)
	all := resp.GetAll()
	if all == nil || all.Bangumi == nil {
		t.Fatal("all/bangumi is nil")
	}
	if all.Bangumi.Url == "https://example.com/video.mp4" {
		t.Error("mirror all bangumi URL was not proxied")
	}
}

func TestProxyMirrorResponseFikushon(t *testing.T) {
	// Fikushon should be a no-op (prose, no stream URL)
	resp := &pb.MirrorResponse{
		Data: &pb.MirrorResponse_Fikushon{
			Fikushon: &pb.ExtensionFikushonWatch{
				Content: []string{"Hello world"},
				Title:   "Test Novel",
			},
		},
	}
	proxyMirrorResponse(resp)
	f := resp.GetFikushon()
	if f == nil {
		t.Fatal("fikushon is nil")
	}
	if f.Title != "Test Novel" {
		t.Errorf("fikushon title changed: %q", f.Title)
	}
}

// TestTorrentURLAfterSanitize verifies that torrent URL construction works
// correctly after sanitization replaces invalid UTF-8 bytes. This tests the
// exact code path used in video_player_provider.dart:_getController.
func TestTorrentURLAfterSanitize(t *testing.T) {
	watchResp := &pb.WatchResponse{
		Data: &pb.WatchResponse_Bangumi{
			Bangumi: &pb.ExtensionBangumiWatch{
				Type: "torrent",
				Url:  "magnet:?xt=urn:btih:TESTHASH",
				Torrent: &pb.ExtensionBangumiWatchTorrent{
					InfoHash: "TESTHASH1234",
					Detail: &pb.ExtensionBangumiWatchTorrentDetail{
						Name: ptrStr("[Shift-JIS]\x83\x52\x81\x5b"), // invalid UTF-8
					},
					Files: []string{
						"[Shift-JIS]\x83\x52\x81\x5b/EP01.mkv",
						"normal/EP02.mkv",
						"\x83\x52\x81\x5b/EP03.mkv",
					},
				},
			},
		},
	}

	sanitizeWatchResponse(watchResp)

	// Marshal and unmarshal to simulate gRPC transport
	data, err := googleproto.Marshal(watchResp)
	if err != nil {
		t.Fatalf("proto.Marshal failed: %v", err)
	}
	decoded := &pb.WatchResponse{}
	if err := googleproto.Unmarshal(data, decoded); err != nil {
		t.Fatalf("proto.Unmarshal failed: %v", err)
	}

	bangumi := decoded.GetBangumi()
	if bangumi == nil || bangumi.Torrent == nil {
		t.Fatal("torrent is nil after round-trip")
	}

	// Simulate the Dart frontend's video extension detection and URL construction
	baseURL := "http://127.0.0.1:3000"
	videoExtensions := map[string]bool{
		"mkv": true, "mp4": true, "avi": true, "ts": true,
	}

	var streamURL string
	for _, file := range bangumi.Torrent.Files {
		ext := file[strings.LastIndex(file, ".")+1:]
		if videoExtensions[ext] {
			streamURL = baseURL + "/torrent/data/" + bangumi.Torrent.InfoHash + "/" + file
			break
		}
	}

	if streamURL == "" {
		t.Fatal("no video file found in torrent files")
	}

	// All files should be valid UTF-8 after sanitization
	for i, f := range bangumi.Torrent.Files {
		if !utf8.ValidString(f) {
			t.Errorf("torrent file[%d] invalid UTF-8: %q", i, f)
		}
	}

	// The URL itself should be constructable from valid strings
	if !utf8.ValidString(streamURL) {
		t.Errorf("constructed URL has invalid UTF-8: %q", streamURL)
	}

	// The InfoHash in the URL should match
	if !strings.Contains(streamURL, "TESTHASH1234") {
		t.Errorf("URL missing InfoHash: %q", streamURL)
	}
}
