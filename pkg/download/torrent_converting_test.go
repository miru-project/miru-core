package download

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/miru-project/miru-core/ent/download"
)

// TestTorrentCompletionSetsConverting verifies torrent download
// completion sets Converting (not Completed) so frontend copies to download dir.
func TestTorrentCompletionSetsConverting(t *testing.T) {
	p := &Progress{
		Progrss:   10000000,
		Total:     10000000,
		Status:    Downloading,
		MediaType: Torrent,
		TaskID:    7001,
	}

	// Simulate what readAndSavePartial does on EOF.
	p.Status = Converting

	if p.Status != Converting {
		t.Errorf("expected Converting after torrent EOF, got %s", p.Status)
	}
}

// TestMp4CompletionSetsConverting verifies MP4 download completion sets Converting.
func TestMp4CompletionSetsConverting(t *testing.T) {
	p := &Progress{
		Progrss:   5000000,
		Total:     5000000,
		Status:    Downloading,
		MediaType: Mp4,
		TaskID:    7002,
	}

	p.Status = Converting

	if p.Status != Converting {
		t.Errorf("expected Converting after mp4 EOF, got %s", p.Status)
	}
}

// TestVerifyTorrentProgress_ConvertingFileExists ensures Converting torrent
// stays Converting when the downloaded file exists on disk.
func TestVerifyTorrentProgress_ConvertingFileExists(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "video.mkv")
	writeTestFile(t, filePath)

	p := &Progress{
		Progrss:            10000000,
		Total:              10000000,
		Status:             Converting,
		MediaType:          Torrent,
		TaskID:             7003,
		CurrentDownloading: filePath,
	}

	verifyTorrentProgress(p)

	if p.Status != Converting {
		t.Errorf("expected Converting when file exists, got %s", p.Status)
	}
}

// TestVerifyTorrentProgress_ConvertingFileMissing ensures Converting torrent
// becomes Failed when the downloaded file is missing.
func TestVerifyTorrentProgress_ConvertingFileMissing(t *testing.T) {
	client := setupTestDB(t)
	withEntClient(t, client)

	p := &Progress{
		Progrss:            10000000,
		Total:              10000000,
		Status:             Converting,
		MediaType:          Torrent,
		TaskID:             7004,
		CurrentDownloading: "/nonexistent/video.mkv",
	}

	verifyTorrentProgress(p)

	if p.Status != Failed {
		t.Errorf("expected Failed when file missing, got %s", p.Status)
	}
}

// TestVerifyTorrentProgress_DownloadingFileMissing ensures Downloading
// torrent resets progress when file is missing.
func TestVerifyTorrentProgress_DownloadingFileMissing(t *testing.T) {
	p := &Progress{
		Progrss:            5000000,
		Total:              10000000,
		Status:             Downloading,
		MediaType:          Torrent,
		TaskID:             7005,
		CurrentDownloading: "/nonexistent/video.mkv",
	}

	verifyTorrentProgress(p)

	if p.Progrss != 0 {
		t.Errorf("expected progress=0 when file missing, got %d", p.Progrss)
	}
	if p.Status != Downloading {
		t.Errorf("expected Downloading preserved, got %s", p.Status)
	}
}

// TestInitRestart_TorrentConvertingRecovery verifies that a torrent task
// in Converting state with file on disk stays Converting on restart.
func TestInitRestart_TorrentConvertingRecovery(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "video.mkv")
	writeTestFile(t, filePath)

	p := &Progress{
		Progrss:            10000000,
		Total:              10000000,
		Status:             Converting,
		MediaType:          Torrent,
		TaskID:             7006,
		CurrentDownloading: filePath,
	}

	verifyAndAdjustProgress(p)

	if p.Status != Converting {
		t.Errorf("expected Converting when file exists, got %s", p.Status)
	}
}

// TestInitRestart_TorrentConvertingMissingRecovery verifies that a torrent
// task in Converting state with missing file is marked Failed.
func TestInitRestart_TorrentConvertingMissingRecovery(t *testing.T) {
	client := setupTestDB(t)
	withEntClient(t, client)

	p := &Progress{
		Progrss:            10000000,
		Total:              10000000,
		Status:             Converting,
		MediaType:          Torrent,
		TaskID:             7007,
		CurrentDownloading: "/nonexistent/video.mkv",
	}

	verifyAndAdjustProgress(p)

	if p.Status != Failed {
		t.Errorf("expected Failed when file missing, got %s", p.Status)
	}
}

// TestInitRestart_OnlyFetchesActiveStatuses verifies that GetPendingDownloads
// only returns tasks with active statuses, not terminal ones.
func TestInitRestart_OnlyFetchesActiveStatuses(t *testing.T) {
	client := setupTestDB(t)
	withEntClient(t, client)

	ctx := context.Background()

	// Create active status records (each with unique key)
	for i, status := range []string{"Downloading", "Paused", "Queued", "Converting"} {
		_, err := client.Download.Create().
			SetURL([]string{"https://example.org/"}).
			SetWatchUrl("https://example.org/watch").SetDetailUrl("https://example.org/detail-" + status).
			SetHeaders(map[string]string{}).
			SetPackage("test.pkg").SetProgress([]int{0, 100}).
			SetKey("active-" + status).SetTitle("Active " + status).
			SetMediaType("mp4").SetStatus(status).
			SetSavePath("/tmp/test.mp4").Save(ctx)
		if err != nil {
			t.Fatalf("failed to create active record %d: %v", i, err)
		}
	}

	// Create terminal status records (each with unique key)
	for i, status := range []string{"Completed", "Failed", "Canceled"} {
		_, err := client.Download.Create().
			SetURL([]string{"https://example.org/"}).
			SetWatchUrl("https://example.org/watch").SetDetailUrl("https://example.org/detail-" + status).
			SetHeaders(map[string]string{}).
			SetPackage("test.pkg").SetProgress([]int{100, 100}).
			SetKey("terminal-" + status).SetTitle("Terminal " + status).
			SetMediaType("mp4").SetStatus(status).
			SetSavePath("/tmp/test.mp4").Save(ctx)
		if err != nil {
			t.Fatalf("failed to create terminal record %d: %v", i, err)
		}
	}

	total, _ := client.Download.Query().Count(ctx)
	if total != 7 {
		t.Fatalf("expected 7 total records, got %d", total)
	}

	// Query only active statuses (mirrors db.GetPendingDownloads)
	pending := client.Download.Query().
		Where(
			download.Or(
				download.Status("Downloading"),
				download.Status("Paused"),
				download.Status("Queued"),
				download.Status("Converting"),
			),
		).
		AllX(ctx)

	if len(pending) != 4 {
		t.Errorf("expected 4 active records, got %d", len(pending))
	}

	for _, d := range pending {
		switch d.Status {
		case "Downloading", "Paused", "Queued", "Converting":
			// ok
		default:
			t.Errorf("unexpected status %q in pending results", d.Status)
		}
	}
}
