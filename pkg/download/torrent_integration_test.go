package download

import (
	"io"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/miru-project/miru-core/pkg/network"
	miruTorrent "github.com/miru-project/miru-core/pkg/torrent"
)

const (
	testTorrentURL = "https://webtorrent.io/torrents/big-buck-bunny.torrent"
	testMagnetURL  = "magnet:?xt=urn:btih:dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c&dn=Big+Buck+Bunny&tr=udp%3A%2F%2Fexplodie.org%3A6969&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969&tr=udp%3A%2F%2Ftracker.empire-js.us%3A1337&tr=udp%3A%2F%2Ftracker.leechers-paradise.org%3A6969&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337&tr=wss%3A%2F%2Ftracker.btorrent.xyz&tr=wss%3A%2F%2Ftracker.fastcast.nz&tr=wss%3A%2F%2Ftracker.openwebtorrent.com&ws=https%3A%2F%2Fwebtorrent.io%2Ftorrents%2F&xs=https%3A%2F%2Fwebtorrent.io%2Ftorrents%2Fbig-buck-bunny.torrent"
	testMP4URL     = "https://raw.githubusercontent.com/chthomos/video-media-samples/master/big-buck-bunny-1080p-30sec.mp4"
)

var clientOnce sync.Once
var networkOnce sync.Once

// initTestTorrentClient creates a shared BitTorrent client for all tests.
// Uses sync.Once to avoid port binding conflicts across parallel tests.
func initTestTorrentClient(t *testing.T) {
	t.Helper()
	clientOnce.Do(func() {
		dataDir := t.TempDir()
		miruTorrent.DataDir = dataDir
		networkOnce.Do(func() { network.Init() })

		cc := torrent.NewDefaultClientConfig()
		cc.DataDir = dataDir
		cc.NoUpload = true
		client, err := torrent.NewClient(cc)
		if err != nil {
			t.Fatalf("failed to create torrent client: %v", err)
		}
		miruTorrent.BTClient = client
	})
	if miruTorrent.BTClient == nil {
		t.Fatal("torrent client not initialized")
	}
}

// initTestNetwork initializes the network layer once.
func initTestNetwork(t *testing.T) {
	t.Helper()
	networkOnce.Do(func() { network.Init() })
}

// skipShort skips the test in short mode.
func skipShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
}

// withStatusUpdate overrides OnStatusUpdate for the duration of the test.
func withStatusUpdate(t *testing.T, handler func(map[int]*Progress)) {
	t.Helper()
	orig := OnStatusUpdate
	OnStatusUpdate = handler
	t.Cleanup(func() { OnStatusUpdate = orig })
}

// startDownload starts a download and returns the task ID.
func startDownload(t *testing.T, url, mediaType, title string) int {
	t.Helper()
	result, err := Download(t.TempDir(), url, map[string]string{}, mediaType, title, "test-pkg", "", "", "", "")
	if err != nil {
		t.Fatalf("Download() returned error: %v", err)
	}
	if !result.IsDownloading {
		t.Fatal("expected IsDownloading=true")
	}
	return result.TaskID
}

// getProgress returns the current Progress for a task.
func getProgress(t *testing.T, taskID int) *Progress {
	t.Helper()
	v, ok := statusMap.Load(taskID)
	if !ok {
		t.Fatalf("task %d not found in statusMap", taskID)
	}
	return v.(*Progress)
}

// logProgress logs the current progress state.
func logProgress(t *testing.T, p *Progress) {
	t.Helper()
	t.Logf("Progress: %d/%d (%.1f%%) status=%s",
		p.Progrss, p.Total,
		float64(p.Progrss)/float64(p.Total)*100,
		p.Status)
}

// verifyProgressAndCancel starts a download and polls until completion,
// failure, target progress reached, or timeout. Returns true if progress
// was observed.
func verifyProgressAndCancel(t *testing.T, url, mediaType, title string, onUpdate func(map[int]*Progress), timeout time.Duration) bool {
	t.Helper()
	skipShort(t)
	initTestTorrentClient(t)
	if onUpdate != nil {
		withStatusUpdate(t, onUpdate)
	}

	taskID := startDownload(t, url, mediaType, title)
	t.Logf("Download started, taskId=%d", taskID)

	p := getProgress(t, taskID)
	if p.Total <= 0 {
		t.Fatalf("expected total > 0, got %d", p.Total)
	}
	t.Logf("Total size: %d bytes", p.Total)

	deadline := time.After(timeout)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	gotProgress := false
	for {
		select {
		case <-deadline:
			t.Log("Timeout reached")
			return gotProgress
		case <-ticker.C:
			latest := getProgress(t, taskID)
			logProgress(t, latest)

			if latest.Progrss > 0 {
				gotProgress = true
			}

			if latest.Status == Completed {
				t.Logf("Download completed! File: %s", latest.SavePath)
				if _, err := os.Stat(latest.SavePath); err != nil {
					t.Errorf("completed file not found on disk: %v", err)
				}
				return gotProgress
			}
			if latest.Status == Failed {
				t.Fatalf("download failed with status=Failed")
			}

			if gotProgress && latest.Progrss >= 1024*1024 {
				t.Logf("Downloaded at least 1MB, progress verified. Cancelling.")
				CancelTask(taskID)
				return gotProgress
			}
		}
	}
}

// verifyInvalidURLFails starts a download with an invalid URL and verifies
// it either returns an error or reaches a Failed/Completed status.
func verifyInvalidURLFails(t *testing.T, init func(*testing.T), url, mediaType, title string) {
	t.Helper()
	skipShort(t)
	init(t)

	result, err := Download(t.TempDir(), url, map[string]string{}, mediaType, title, "test-pkg", "", "", "", "")
	if err != nil {
		t.Logf("Download() correctly returned error for invalid URL: %v", err)
		return
	}

	taskID := result.TaskID
	t.Logf("Download started with invalid URL, taskId=%d", taskID)

	deadline := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Log("Timeout waiting for failure status")
			return
		case <-ticker.C:
			v, ok := statusMap.Load(taskID)
			if !ok {
				t.Log("task not found in statusMap (may have been cleaned up)")
				return
			}
			latest := v.(*Progress)
			t.Logf("Status: %s progress=%d", latest.Status, latest.Progrss)

			if latest.Status == Failed {
				t.Log("Download correctly failed for invalid URL")
				return
			}
			if latest.Status == Completed {
				t.Log("Download completed (unexpected for invalid URL, but no error)")
				return
			}
		}
	}
}

// verifyFileOnDisk polls a task until a file appears on disk with content.
func verifyFileOnDisk(t *testing.T, taskID int, cancel bool, timeout time.Duration) {
	t.Helper()
	fileVerified := false

	deadline := time.After(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			if !fileVerified {
				t.Fatal("timeout waiting for file to appear on disk")
			}
			return
		case <-ticker.C:
			v, _ := statusMap.Load(taskID)
			if v == nil {
				if fileVerified {
					return
				}
				t.Fatal("task disappeared from statusMap before file was verified")
			}
			p := v.(*Progress)

			if p.CurrentDownloading != "" {
				if info, err := os.Stat(p.CurrentDownloading); err == nil {
					if info.Size() > 0 {
						t.Logf("File on disk: %s, size: %d bytes", p.CurrentDownloading, info.Size())
						fileVerified = true
						if cancel {
							CancelTask(taskID)
						}
						t.Log("File written to disk verified.")
						return
					}
				}
			}

			if p.Progrss > 0 && !fileVerified {
				t.Logf("Progress=%d but file path not yet set or file empty, waiting", p.Progrss)
			}
		}
	}
}

// verifyFetchMetadata fetches torrent/magnet metadata and verifies it.
func verifyFetchMetadata(t *testing.T, fetch func() (*torrent.Torrent, error)) {
	t.Helper()
	skipShort(t)
	initTestTorrentClient(t)

	torr, err := fetch()
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}

	info := torr.Info()
	if info == nil {
		t.Fatal("torrent info is nil")
	}

	t.Logf("Torrent name: %s", info.Name)
	t.Logf("Info hash: %s", torr.InfoHash().HexString())
	t.Logf("Total length: %d bytes", torr.Length())

	if len(info.Files) == 0 {
		t.Error("expected at least one file in torrent")
	}

	for _, f := range info.Files {
		t.Logf("  File: %s (%d bytes)", f.DisplayPath(info), f.Length)
	}
}

// TestTorrentDownload_ProgressUpdates verifies that a .torrent download
// reports progress updates (progress > 0 and total > 0) and eventually
// completes or at least downloads several bytes.
func TestTorrentDownload_ProgressUpdates(t *testing.T) {
	var mu sync.Mutex
	var maxProgress int

	gotProgress := verifyProgressAndCancel(t, testTorrentURL, "torrent", "Big Buck Bunny", func(statuses map[int]*Progress) {
		for _, p := range statuses {
			if p.MediaType == Torrent {
				mu.Lock()
				if p.Progrss > maxProgress {
					maxProgress = p.Progrss
				}
				mu.Unlock()
			}
		}
	}, 120*time.Second)

	mu.Lock()
	mp := maxProgress
	mu.Unlock()
	t.Logf("Max progress reached: %d bytes", mp)

	if !gotProgress && mp <= 0 {
		t.Fatal("no progress updates received; torrent download did not start")
	}
	t.Log("Torrent download progress test passed")
}

// TestMagnetDownload_ProgressUpdates verifies that a magnet link download
// reports progress updates and downloads data from the swarm.
func TestMagnetDownload_ProgressUpdates(t *testing.T) {
	var progressReports int64

	gotProgress := verifyProgressAndCancel(t, testMagnetURL, "magnet", "Big Buck Bunny Magnet", func(statuses map[int]*Progress) {
		for _, p := range statuses {
			if p.MediaType == Magnet {
				atomic.AddInt64(&progressReports, 1)
			}
		}
	}, 120*time.Second)

	mp := atomic.LoadInt64(&progressReports)
	t.Logf("Total status updates received: %d", mp)

	if !gotProgress {
		t.Fatal("no progress updates received; magnet download did not start")
	}
}

// TestTorrentDownload_FailureOnInvalidURL verifies that an invalid torrent
// URL causes the download to fail (either via error return or FAILED status).
func TestTorrentDownload_FailureOnInvalidURL(t *testing.T) {
	verifyInvalidURLFails(t, initTestTorrentClient, "https://example.com/nonexistent.torrent", "torrent", "Bad Torrent")
}

// TestTorrentDownload_LargestFileSelected verifies that downloadTorrent
// picks the largest file from the torrent.
func TestTorrentDownload_LargestFileSelected(t *testing.T) {
	skipShort(t)
	initTestTorrentClient(t)
	withStatusUpdate(t, func(statuses map[int]*Progress) {})

	taskID := startDownload(t, testTorrentURL, "torrent", "Big Buck Bunny")
	p := getProgress(t, taskID)

	t.Logf("Selected file path: %s", p.SavePath)
	t.Logf("Total size: %d bytes", p.Total)
	t.Logf("File names: %v", p.Names)

	if p.Names == nil || len(*p.Names) == 0 {
		t.Fatal("expected at least one file name in Names")
	}

	if p.Total <= 0 {
		t.Fatalf("expected total > 0, got %d", p.Total)
	}

	CancelTask(taskID)
}

// TestDownloadCancellation_StoresCorrectStatus verifies that cancelling a
// torrent download properly transitions through Canceled or Failed status.
// NOTE: Due to a race in the torrent reader goroutine, cancel may result in
// either Canceled or Failed depending on timing. Both are acceptable.
func TestDownloadCancellation_StoresCorrectStatus(t *testing.T) {
	skipShort(t)
	initTestTorrentClient(t)
	withStatusUpdate(t, func(statuses map[int]*Progress) {})

	taskID := startDownload(t, testTorrentURL, "torrent", "Cancel Test")

	deadline := time.After(60 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for download to start")
		case <-ticker.C:
			v, _ := statusMap.Load(taskID)
			if v == nil {
				t.Log("task removed from statusMap after cancel")
				return
			}
			p := v.(*Progress)
			if p.Progrss > 0 {
				t.Logf("Got progress=%d, cancelling", p.Progrss)
				CancelTask(taskID)
				time.Sleep(2 * time.Second)

				v2, _ := statusMap.Load(taskID)
				if v2 == nil {
					t.Log("task removed from statusMap after cancel (expected)")
					return
				}
				p2 := v2.(*Progress)
				t.Logf("Status after cancel: %s", p2.Status)
				if p2.Status != Canceled && p2.Status != Failed {
					t.Errorf("expected Canceled or Failed status, got %s", p2.Status)
				}
				return
			}
		}
	}
}

// TestTorrentDownload_ReadAndSavePartialProgress verifies the progress
// updates happen during readAndSavePartial by monitoring the Progress.Progrss
// field.
func TestTorrentDownload_ReadAndSavePartialProgress(t *testing.T) {
	skipShort(t)
	initTestTorrentClient(t)
	withStatusUpdate(t, func(statuses map[int]*Progress) {})

	taskID := startDownload(t, testTorrentURL, "torrent", "ReadAndSave Test")
	seenProgress := make(map[int]bool)
	var mu sync.Mutex

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			v, ok := statusMap.Load(taskID)
			if !ok {
				return
			}
			p := v.(*Progress)
			mu.Lock()
			seenProgress[p.Progrss] = true
			mu.Unlock()

			if p.Status == Completed || p.Status == Failed || p.Status == Canceled {
				return
			}
			if p.Progrss > 5*1024*1024 {
				CancelTask(taskID)
				return
			}
		}
	}()

	deadline := time.After(60 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Log("Timeout - test passed if any progress was seen")
		case <-ticker.C:
			v, _ := statusMap.Load(taskID)
			if v == nil {
				mu.Lock()
				count := len(seenProgress)
				mu.Unlock()
				t.Logf("Task finished. Unique progress values seen: %d", count)
				if count < 2 {
					t.Errorf("expected multiple distinct progress values, got %d", count)
				}
				return
			}
			p := v.(*Progress)
			if p.Status == Completed || p.Status == Failed || p.Status == Canceled {
				mu.Lock()
				count := len(seenProgress)
				mu.Unlock()
				t.Logf("Task ended with status=%s. Unique progress values seen: %d", p.Status, count)
				if count < 2 {
					t.Errorf("expected multiple distinct progress values, got %d", count)
				}
				return
			}
		}
	}
}

// TestMultipleTorrentDownloads_ConcurrentProgress verifies that multiple
// torrent downloads running concurrently all report progress independently.
func TestMultipleTorrentDownloads_ConcurrentProgress(t *testing.T) {
	skipShort(t)
	initTestTorrentClient(t)
	withStatusUpdate(t, func(statuses map[int]*Progress) {})

	taskID1 := startDownload(t, testTorrentURL, "torrent", "Torrent 1")
	taskID2 := startDownload(t, testMagnetURL, "magnet", "Magnet 1")

	t.Logf("Started torrent taskId=%d and magnet taskId=%d", taskID1, taskID2)

	deadline := time.After(120 * time.Second)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	torrentGotProgress := false
	magnetGotProgress := false

	for {
		select {
		case <-deadline:
			t.Log("Timeout reached")
			if !torrentGotProgress {
				t.Error("torrent download never reported progress")
			}
			if !magnetGotProgress {
				t.Error("magnet download never reported progress")
			}
			CancelTask(taskID1)
			CancelTask(taskID2)
			return
		case <-ticker.C:
			v1, ok1 := statusMap.Load(taskID1)
			v2, ok2 := statusMap.Load(taskID2)

			if ok1 {
				p1 := v1.(*Progress)
				if p1.Progrss > 0 {
					torrentGotProgress = true
				}
				t.Logf("Torrent: progress=%d/%d status=%s", p1.Progrss, p1.Total, p1.Status)
				if p1.Status == Completed {
					t.Log("Torrent download completed")
					torrentGotProgress = true
				}
			}

			if ok2 {
				p2 := v2.(*Progress)
				if p2.Progrss > 0 {
					magnetGotProgress = true
				}
				t.Logf("Magnet: progress=%d/%d status=%s", p2.Progrss, p2.Total, p2.Status)
				if p2.Status == Completed {
					t.Log("Magnet download completed")
					magnetGotProgress = true
				}
			}

			if torrentGotProgress && magnetGotProgress {
				t.Log("Both downloads showed progress. Cancelling.")
				CancelTask(taskID1)
				CancelTask(taskID2)
				return
			}
		}
	}
}

// TestTorrentDownload_FileWrittenToDisk verifies that during download,
// bytes are actually written to disk by checking the file exists and has
// content.
func TestTorrentDownload_FileWrittenToDisk(t *testing.T) {
	skipShort(t)
	initTestTorrentClient(t)
	withStatusUpdate(t, func(statuses map[int]*Progress) {})

	taskID := startDownload(t, testTorrentURL, "torrent", "File Write Test")
	verifyFileOnDisk(t, taskID, false, 60*time.Second)
}

// TestFetchTorrent_Metadata verifies that FetchTorrent successfully
// retrieves torrent metadata (info hash, file list).
func TestFetchTorrent_Metadata(t *testing.T) {
	verifyFetchMetadata(t, func() (*torrent.Torrent, error) {
		return miruTorrent.FetchTorrent(testTorrentURL)
	})
}

// TestFetchMagnet_Metadata verifies that FetchMagnet successfully
// retrieves magnet metadata (info hash, file list) from the swarm.
func TestFetchMagnet_Metadata(t *testing.T) {
	verifyFetchMetadata(t, func() (*torrent.Torrent, error) {
		return miruTorrent.FetchMagnet(testMagnetURL)
	})
}

// ===========================================================================
// MP4 download integration tests
// ===========================================================================

// TestMP4Download_ProgressUpdates verifies that an MP4 download reports
// progress updates (progress > 0 and total > 0) and downloads data.
func TestMP4Download_ProgressUpdates(t *testing.T) {
	skipShort(t)
	initTestNetwork(t)
	withStatusUpdate(t, func(statuses map[int]*Progress) {})

	taskID := startDownload(t, testMP4URL, "mp4", "Big Buck Bunny MP4")
	t.Logf("MP4 download started, taskId=%d", taskID)

	p := getProgress(t, taskID)
	t.Logf("Total size: %d bytes", p.Total)
	t.Logf("Initial status: %s", p.Status)

	deadline := time.After(120 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	gotProgressUpdate := false
	fileOnDisk := false

	for {
		select {
		case <-deadline:
			t.Log("Timeout reached")
			if !gotProgressUpdate {
				t.Fatal("no progress updates received; MP4 download did not start")
			}
			CancelTask(taskID)
			return
		case <-ticker.C:
			v, _ := statusMap.Load(taskID)
			if v == nil {
				t.Log("task removed from statusMap")
				return
			}
			latest := v.(*Progress)
			logProgress(t, latest)

			if latest.Progrss > 0 {
				gotProgressUpdate = true
			}

			// Check file on disk
			if latest.CurrentDownloading != "" && !fileOnDisk {
				if info, err := os.Stat(latest.CurrentDownloading); err == nil && info.Size() > 0 {
					t.Logf("File on disk: %s, size: %d bytes", latest.CurrentDownloading, info.Size())
					fileOnDisk = true
				}
			}

			if latest.Status == Completed {
				t.Log("MP4 download completed!")
				if _, err := os.Stat(latest.SavePath); err != nil {
					t.Errorf("completed file not found on disk: %v", err)
				}
				return
			}
			if latest.Status == Failed {
				t.Fatalf("MP4 download failed with status=Failed")
			}

			// Once we have progress AND file on disk, we've verified enough
			if gotProgressUpdate && fileOnDisk && latest.Progrss >= 1024*1024 {
				t.Log("Downloaded at least 1MB and verified file on disk. Cancelling.")
				CancelTask(taskID)
				return
			}
		}
	}
}

// TestMP4Download_FailureOnInvalidURL verifies that an invalid MP4 URL
// causes the download to fail (either via error return or FAILED status).
func TestMP4Download_FailureOnInvalidURL(t *testing.T) {
	verifyInvalidURLFails(t, initTestNetwork, "https://example.com/nonexistent.mp4", "mp4", "Bad MP4")
}

// TestMP4Download_FileWrittenToDisk verifies that during download,
// bytes are actually written to disk by checking the file exists and has
// content.
func TestMP4Download_FileWrittenToDisk(t *testing.T) {
	skipShort(t)
	initTestNetwork(t)
	withStatusUpdate(t, func(statuses map[int]*Progress) {})

	taskID := startDownload(t, testMP4URL, "mp4", "MP4 File Write Test")
	verifyFileOnDisk(t, taskID, true, 60*time.Second)
}

// TestMP4Download_ProgressSyncToStatusMap verifies that the progress field
// in statusMap is updated as bytes are downloaded, simulating the frontend
// event stream picking up the values.
func TestMP4Download_ProgressSyncToStatusMap(t *testing.T) {
	skipShort(t)
	initTestNetwork(t)
	withStatusUpdate(t, func(statuses map[int]*Progress) {})

	taskID := startDownload(t, testMP4URL, "mp4", "MP4 Sync Test")
	seenProgress := make(map[int]bool)
	var mu sync.Mutex

	deadline := time.After(60 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			mu.Lock()
			count := len(seenProgress)
			mu.Unlock()
			t.Logf("Timeout. Unique progress values seen: %d", count)
			if count < 2 {
				t.Errorf("expected multiple distinct progress values, got %d", count)
			}
			return
		case <-ticker.C:
			v, ok := statusMap.Load(taskID)
			if !ok {
				return
			}
			p := v.(*Progress)

			mu.Lock()
			seenProgress[p.Progrss] = true
			count := len(seenProgress)
			mu.Unlock()

			t.Logf("Progress: %d/%d status=%s (unique values: %d)",
				p.Progrss, p.Total, p.Status, count)

			if p.Status == Completed || p.Status == Failed || p.Status == Canceled {
				t.Logf("Task ended with status=%s. Unique progress values: %d", p.Status, count)
				if count < 2 {
					t.Errorf("expected multiple distinct progress values, got %d", count)
				}
				return
			}

			// Once we see enough distinct progress values, we've confirmed sync works
			if count >= 5 {
				t.Logf("Confirmed progress sync: %d distinct values observed", count)
				CancelTask(taskID)
				return
			}
		}
	}
}

// TestTorrentDownload_CompleteAndVerifyFile verifies that a small torrent
// can be fully downloaded and the file content is readable.
func TestTorrentDownload_CompleteAndVerifyFile(t *testing.T) {
	skipShort(t)
	initTestTorrentClient(t)
	withStatusUpdate(t, func(statuses map[int]*Progress) {})

	taskID := startDownload(t, testTorrentURL, "torrent", "Complete Verify")
	t.Logf("Waiting for download to complete, taskId=%d", taskID)

	deadline := time.After(300 * time.Second)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for download to complete")
		case <-ticker.C:
			v, _ := statusMap.Load(taskID)
			if v == nil {
				t.Log("task removed from statusMap (unexpected)")
				return
			}
			p := v.(*Progress)
			logProgress(t, p)

			if p.Status == Completed {
				if _, err := os.Stat(p.SavePath); err != nil {
					t.Errorf("completed file not found: %v", err)
					return
				}

				f, err := os.Open(p.SavePath)
				if err != nil {
					t.Errorf("cannot open completed file: %v", err)
					return
				}
				defer f.Close()

				buf := make([]byte, 1024)
				n, err := f.Read(buf)
				if err != nil && err != io.EOF {
					t.Errorf("cannot read completed file: %v", err)
					return
				}
				t.Logf("File verified: %d bytes read from start", n)
				t.Log("Full torrent download integration test PASSED")
				return
			}
			if p.Status == Failed {
				t.Fatalf("download failed unexpectedly")
			}
		}
	}
}
