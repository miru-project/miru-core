package download

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCancelTask_RemovesHLSFilesAndDBEntry verifies that cancelling an HLS
// task removes the segment directory, the in-memory maps, and the DB entry.
func TestCancelTask_RemovesHLSFilesAndDBEntry(t *testing.T) {
	resetSchedulerState()
	dir := t.TempDir()

	for _, name := range []string{"0.ts", "1.ts"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	names := []string{filepath.Join(dir, "0.ts"), filepath.Join(dir, "1.ts")}
	taskId := 9001
	p := &Progress{
		Progrss:   2,
		Total:     2,
		Status:    Downloading,
		MediaType: Hls,
		TaskID:    taskId,
		Title:     "Cancel HLS",
		Package:   "test.pkg",
		Key:       "cancel-hls-key",
		URL:       []string{"https://example.org/playlist.m3u8"},
		SavePath:  dir,
		Names:     &names,
		DetailUrl: "https://example.org/detail",
		WatchUrl:  "https://example.org/watch",
	}
	statusMap.Store(taskId, p)
	taskParams.Store(taskId, &HlsTaskParam{
		TaskParam: TaskParam{taskID: taskId},
		filePath:  dir,
	})

	client := setupTestDB(t)
	record, err := client.Download.Create().
		SetURL([]string{"https://example.org/playlist.m3u8"}).
		SetWatchUrl("https://example.org/watch").
		SetDetailUrl("https://example.org/detail").
		SetHeaders(map[string]string{}).
		SetPackage("test.pkg").
		SetProgress([]int{2, 2}).
		SetKey("cancel-hls-key").
		SetTitle("Cancel HLS").
		SetMediaType("hls").
		SetStatus("Downloading").
		SetSavePath(dir).
		Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	withEntClient(t, client)

	if err := CancelTask(taskId); err != nil {
		t.Fatalf("CancelTask returned error: %v", err)
	}

	// Verify dir removed
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("expected segment dir to be removed after cancel")
	}

	// Verify DB entry deleted
	_, err = client.Download.Get(t.Context(), record.ID)
	if err == nil {
		t.Errorf("expected DB entry to be deleted")
	}

	// Verify in-memory cleanup
	if _, ok := statusMap.Load(taskId); ok {
		t.Errorf("expected statusMap entry to be deleted")
	}
	if _, ok := taskParams.Load(taskId); ok {
		t.Errorf("expected taskParams entry to be deleted")
	}
}

// TestCancelTask_RemovesMp4FileAndDBEntry verifies MP4 cancel removes file + DB.
func TestCancelTask_RemovesMp4FileAndDBEntry(t *testing.T) {
	resetSchedulerState()
	dir := t.TempDir()

	filePath := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(filePath, []byte("fake video"), 0644); err != nil {
		t.Fatal(err)
	}

	taskId := 9002
	p := &Progress{
		Progrss:            100,
		Total:              100,
		Status:             Downloading,
		MediaType:          Mp4,
		TaskID:             taskId,
		Title:              "Cancel MP4",
		Package:            "test.pkg",
		Key:                "cancel-mp4-key",
		URL:                []string{"https://example.org/video.mp4"},
		SavePath:           filePath,
		CurrentDownloading: filePath,
		DetailUrl:          "https://example.org/detail",
		WatchUrl:           "https://example.org/watch",
	}
	statusMap.Store(taskId, p)

	client := setupTestDB(t)
	_, err := client.Download.Create().
		SetURL([]string{"https://example.org/video.mp4"}).
		SetWatchUrl("https://example.org/watch").SetDetailUrl("https://example.org/detail").
		SetHeaders(map[string]string{}).
		SetPackage("test.pkg").SetProgress([]int{100, 100}).
		SetKey("cancel-mp4-key").SetTitle("Cancel MP4").
		SetMediaType("mp4").SetStatus("Downloading").
		SetSavePath(filePath).Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	withEntClient(t, client)

	if err := CancelTask(taskId); err != nil {
		t.Fatalf("CancelTask error: %v", err)
	}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("expected MP4 file removed")
	}
	count, _ := client.Download.Query().Count(t.Context())
	if count != 0 {
		t.Errorf("expected 0 DB entries, got %d", count)
	}
}

// TestCancelTask_TorrentFileAndDB verifies torrent cancel removes file + DB.
func TestCancelTask_TorrentFileAndDB(t *testing.T) {
	resetSchedulerState()
	dir := t.TempDir()

	filePath := filepath.Join(dir, "movie.mkv")
	if err := os.WriteFile(filePath, []byte("torrent data"), 0644); err != nil {
		t.Fatal(err)
	}

	taskId := 9003
	p := &Progress{
		Progrss:            5000000,
		Total:              10000000,
		Status:             Downloading,
		MediaType:          Torrent,
		TaskID:             taskId,
		Title:              "Cancel Torrent",
		Package:            "test.pkg",
		Key:                "cancel-torrent-key",
		URL:                []string{"https://example.org/torrent.torrent"},
		SavePath:           filePath,
		CurrentDownloading: filePath,
		DetailUrl:          "https://example.org/detail",
		WatchUrl:           "https://example.org/watch",
	}
	statusMap.Store(taskId, p)

	client := setupTestDB(t)
	_, err := client.Download.Create().
		SetURL([]string{"https://example.org/torrent.torrent"}).
		SetWatchUrl("https://example.org/watch").SetDetailUrl("https://example.org/detail").
		SetHeaders(map[string]string{}).
		SetPackage("test.pkg").SetProgress([]int{5000000, 10000000}).
		SetKey("cancel-torrent-key").SetTitle("Cancel Torrent").
		SetMediaType("torrent").SetStatus("Downloading").
		SetSavePath(filePath).Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	withEntClient(t, client)

	if err := CancelTask(taskId); err != nil {
		t.Fatalf("CancelTask error: %v", err)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("expected torrent file removed")
	}
	count, _ := client.Download.Query().Count(t.Context())
	if count != 0 {
		t.Errorf("expected 0 DB entries, got %d", count)
	}
}
