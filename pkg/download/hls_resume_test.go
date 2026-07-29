package download

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/grafov/m3u8"
	"github.com/miru-project/miru-core/pkg/network"
)

// resetTestState clears the package-level scheduler state so each test is
// isolated. It also installs a no-op resumeFunc.
func resetTestState() {
	// Clear statusMap
	statusMap.Range(func(k, _ any) bool {
		statusMap.Delete(k)
		return true
	})
	// Clear taskParams
	taskParams.Range(func(k, _ any) bool {
		taskParams.Delete(k)
		return true
	})
	tasks = sync.Map{}
	maxConcurrent = DefaultMaxConcurrentDownload
	network.Init()
	resumeFunc = func(taskId int) error {
		tasks.Store(taskId, func() {})
		return nil
	}
	// Reset the playlist fetcher to the real implementation by default.
	fetchPlaylistFunc = fetchPlaylist
}

// parseTestPlaylist is a helper that parses an m3u8 string into a
// *m3u8.MediaPlaylist, filtering nil segments.
func parseTestPlaylist(t *testing.T, body string) *m3u8.MediaPlaylist {
	t.Helper()
	pl, li, err := m3u8.Decode(*bytes.NewBufferString(body), false)
	if err != nil {
		t.Fatalf("failed to decode test playlist: %v", err)
	}
	if li != m3u8.MEDIA {
		t.Fatalf("expected MEDIA playlist, got %d", li)
	}
	playList := pl.(*m3u8.MediaPlaylist)
	playList.Segments = filterSegments(playList.Segments)
	return playList
}

// TestResumeHlsTaskNilPlaylist verifies that resumeHlsTask does not panic when
// the HlsTaskParam.playList is nil (which happens after a backend restart — the
// parsed m3u8 playlist is not persisted in the DB, only the URL).
func TestResumeHlsTaskNilPlaylist(t *testing.T) {
	resetTestState()

	const segmentsAlreadyDownloaded = 2
	totalSegments := 5

	// Mock the fetcher to return a 5-segment playlist.
	fetchPlaylistFunc = func(url string, headers map[string]string) (*m3u8.MediaPlaylist, error) {
		body := "#EXTM3U\n" +
			"#EXT-X-TARGETDURATION:10\n" +
			"#EXT-X-MEDIA-SEQUENCE:0\n" +
			"#EXTINF:10.0,\nsegment0.ts\n" +
			"#EXTINF:10.0,\nsegment1.ts\n" +
			"#EXTINF:10.0,\nsegment2.ts\n" +
			"#EXTINF:10.0,\nsegment3.ts\n" +
			"#EXTINF:10.0,\nsegment4.ts\n" +
			"#EXT-X-ENDLIST\n"
		return parseTestPlaylist(t, body), nil
	}

	const taskID = 42
	statusMap.Store(taskID, &Progress{
		TaskID:    taskID,
		Status:    Paused,
		Progrss:   segmentsAlreadyDownloaded,
		Total:     totalSegments,
		Title:     "test-hls",
		MediaType: Hls,
		URL:       []string{"http://fake/playlist.m3u8"},
	})

	// Task param with nil playList — exactly what Init() reconstructs.
	taskParams.Store(taskID, &HlsTaskParam{
		TaskParam:   TaskParam{taskID: taskID},
		playListUrl: "http://fake/playlist.m3u8",
		filePath:    t.TempDir(),
		headers:     make(map[string]string),
		// playList is deliberately nil — simulates post-restart state.
	})

	resumeFunc = func(id int) error {
		tasks.Store(id, func() {})
		return nil
	}

	err := resumeHlsTask(taskID)
	if err != nil {
		t.Fatalf("resumeHlsTask returned unexpected error: %v", err)
	}

	hlsV, _ := taskParams.Load(taskID)
	hls := hlsV.(*HlsTaskParam)
	if hls.playList == nil {
		t.Fatal("expected playList to be populated after re-fetch")
	}

	remaining := len(hls.playList.Segments)
	if remaining != totalSegments-segmentsAlreadyDownloaded {
		t.Fatalf("expected %d remaining segments, got %d",
			totalSegments-segmentsAlreadyDownloaded, remaining)
	}

	// Cancel the background goroutine so it doesn't hit network.Request.
	if cancel, ok := tasks.Load(taskID); ok {
		cancel.(context.CancelFunc)()
	}
}

// TestResumeHlsTaskAllSegmentsDownloaded covers the edge case where every
// segment was already downloaded before the backend crashed.
func TestResumeHlsTaskAllSegmentsDownloaded(t *testing.T) {
	resetTestState()

	fetchPlaylistFunc = func(url string, headers map[string]string) (*m3u8.MediaPlaylist, error) {
		body := "#EXTM3U\n" +
			"#EXT-X-TARGETDURATION:10\n" +
			"#EXT-X-MEDIA-SEQUENCE:0\n" +
			"#EXTINF:10.0,\nsegment0.ts\n" +
			"#EXTINF:10.0,\nsegment1.ts\n" +
			"#EXT-X-ENDLIST\n"
		return parseTestPlaylist(t, body), nil
	}

	const taskID = 99
	statusMap.Store(taskID, &Progress{
		TaskID:    taskID,
		Status:    Paused,
		Progrss:   2, // all segments done
		Total:     2,
		Title:     "test-hls-done",
		MediaType: Hls,
		URL:       []string{"http://fake/playlist.m3u8"},
	})
	taskParams.Store(taskID, &HlsTaskParam{
		TaskParam:   TaskParam{taskID: taskID},
		playListUrl: "http://fake/playlist.m3u8",
		filePath:    t.TempDir(),
		headers:     make(map[string]string),
	})

	resumeFunc = func(id int) error {
		t.Fatal("resumeFunc should not be called when all segments are done")
		return nil
	}

	err := resumeHlsTask(taskID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sv, _ := statusMap.Load(taskID)
	if sv.(*Progress).Status != Converting {
		t.Fatalf("expected status Converting, got %s", sv.(*Progress).Status)
	}
}

// TestResumeHlsTaskFetchError verifies that a network error during
// playlist re-fetch returns a descriptive error instead of panicking.
func TestResumeHlsTaskFetchError(t *testing.T) {
	resetTestState()

	fetchPlaylistFunc = func(url string, headers map[string]string) (*m3u8.MediaPlaylist, error) {
		return nil, fmt.Errorf("connection refused")
	}

	const taskID = 77
	statusMap.Store(taskID, &Progress{
		TaskID:    taskID,
		Status:    Paused,
		Progrss:   1,
		Total:     5,
		MediaType: Hls,
	})
	taskParams.Store(taskID, &HlsTaskParam{
		TaskParam:   TaskParam{taskID: taskID},
		playListUrl: "http://127.0.0.1:1",
		filePath:    t.TempDir(),
		headers:     make(map[string]string),
	})

	err := resumeHlsTask(taskID)
	if err == nil {
		t.Fatal("expected error for fetch failure, got nil")
	}
}

// TestResumeHlsTaskPlaylistAlreadyPopulated verifies that resumeHlsTask
// skips the re-fetch when playList is already set (normal resume, no restart).
func TestResumeHlsTaskPlaylistAlreadyPopulated(t *testing.T) {
	resetTestState()

	fetchPlaylistFunc = func(url string, headers map[string]string) (*m3u8.MediaPlaylist, error) {
		t.Fatal("fetchPlaylistFunc should not be called when playList is already set")
		return nil, nil
	}

	body := "#EXTM3U\n" +
		"#EXT-X-TARGETDURATION:10\n" +
		"#EXT-X-MEDIA-SEQUENCE:0\n" +
		"#EXTINF:10.0,\nsegment0.ts\n" +
		"#EXTINF:10.0,\nsegment1.ts\n" +
		"#EXTINF:10.0,\nsegment2.ts\n" +
		"#EXTINF:10.0,\nsegment3.ts\n" +
		"#EXTINF:10.0,\nsegment4.ts\n" +
		"#EXT-X-ENDLIST\n"
	playList := parseTestPlaylist(t, body)

	const taskID = 55
	statusMap.Store(taskID, &Progress{
		TaskID:    taskID,
		Status:    Paused,
		Progrss:   0,
		Total:     5,
		MediaType: Hls,
	})
	taskParams.Store(taskID, &HlsTaskParam{
		TaskParam:   TaskParam{taskID: taskID},
		playListUrl: "http://fake/playlist.m3u8",
		filePath:    t.TempDir(),
		headers:     make(map[string]string),
		playList:    playList, // already populated
	})

	err := resumeHlsTask(taskID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tp, _ := taskParams.Load(taskID)
	if len(tp.(*HlsTaskParam).playList.Segments) != 5 {
		t.Fatalf("expected 5 segments (none skipped), got %d",
			len(tp.(*HlsTaskParam).playList.Segments))
	}

	// Cancel the background goroutine so it doesn't hit network.Request.
	if cancel, ok := tasks.Load(taskID); ok {
		cancel.(context.CancelFunc)()
	}
}
