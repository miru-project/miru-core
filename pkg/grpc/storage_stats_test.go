package grpc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/ent/download"
	"github.com/miru-project/miru-core/ent/enttest"
	"github.com/miru-project/miru-core/ext"
	dl "github.com/miru-project/miru-core/pkg/download"
	pb "github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// TestGetStorageStatsEndpoint exercises the GetStorageStats gRPC handler as a
// real endpoint: it wires an in-memory ent DB (via the global ent client used by
// db.GetAllDownloads) and injects in-progress tasks into the download status
// map, then asserts the per-category, temp, and total byte counts.
func TestGetStorageStatsEndpoint(t *testing.T) {
	ctx := context.Background()

	// In-memory SQLite, injected as the global ent client.
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1&_pragma=foreign_keys(1)")
	defer client.Close()
	oldClient := ext.GetEntClientForTest()
	ext.SetEntClientForTest(client)
	t.Cleanup(func() { ext.SetEntClientForTest(oldClient) })

	// Clean any injected tasks afterwards.
	t.Cleanup(dl.ClearActiveProgressForTest)

	dir := t.TempDir()

	// --- Completed downloads (persisted rows) -------------------------------
	// Video: a single 100-byte file.
	videoFile := filepath.Join(dir, "video.mp4")
	require.NoError(t, os.WriteFile(videoFile, make([]byte, 100), 0644))
	// Manga: a single 200-byte file.
	mangaFile := filepath.Join(dir, "manga.cbz")
	require.NoError(t, os.WriteFile(mangaFile, make([]byte, 200), 0644))
	// Novel: a directory with two segment files (50 + 70 = 120 bytes).
	novelDir := filepath.Join(dir, "novel")
	require.NoError(t, os.MkdirAll(novelDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(novelDir, "ch1.txt"), make([]byte, 50), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(novelDir, "ch2.txt"), make([]byte, 70), 0644))
	// A completed row with no category should contribute nothing.
	uncatFile := filepath.Join(dir, "unknown.bin")
	require.NoError(t, os.WriteFile(uncatFile, make([]byte, 999), 0644))

	seedDownload(t, client, "video", "Completed", videoFile)
	seedDownload(t, client, "manga", "Completed", mangaFile)
	seedDownload(t, client, "novel", "Completed", novelDir)
	seedDownload(t, client, "unspecified", "Completed", uncatFile)

	// --- In-progress (temp) downloads --------------------------------------
	// Active task writing a 30-byte partial file (not yet a completed save_path).
	tempFile := filepath.Join(dir, "temp.partial")
	require.NoError(t, os.WriteFile(tempFile, make([]byte, 30), 0644))
	dl.StoreActiveProgressForTest(&dl.Progress{
		TaskID:             1,
		Status:             dl.Downloading,
		CurrentDownloading: tempFile,
		SavePath:           filepath.Join(dir, "will-not-match"),
	})

	// Active task whose SavePath EQUALS a completed save_path must NOT be
	// double-counted as temp (it is already in the completed totals).
	dl.StoreActiveProgressForTest(&dl.Progress{
		TaskID:   2,
		Status:    dl.Downloading,
		SavePath:  videoFile, // same path as the completed video download
		Category:  "video",
	})

	// Paused/Queued/Converting tasks should also contribute their partial bytes.
	pausedFile := filepath.Join(dir, "paused.partial")
	require.NoError(t, os.WriteFile(pausedFile, make([]byte, 11), 0644))
	dl.StoreActiveProgressForTest(&dl.Progress{
		TaskID:             3,
		Status:             dl.Paused,
		CurrentDownloading: pausedFile,
	})
	queuedFile := filepath.Join(dir, "queued.partial")
	require.NoError(t, os.WriteFile(queuedFile, make([]byte, 9), 0644))
	dl.StoreActiveProgressForTest(&dl.Progress{
		TaskID:             4,
		Status:             dl.Queued,
		CurrentDownloading: queuedFile,
	})
	convertingFile := filepath.Join(dir, "converting.partial")
	require.NoError(t, os.WriteFile(convertingFile, make([]byte, 7), 0644))
	dl.StoreActiveProgressForTest(&dl.Progress{
		TaskID:             5,
		Status:             dl.Converting,
		CurrentDownloading: convertingFile,
	})

	// --- Call the endpoint -------------------------------------------------
	srv := &MiruCoreServer{}
	resp, err := srv.GetStorageStats(ctx, &pb.GetStorageStatsRequest{DownloadPath: dir})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Stats)

	// video=100, manga=200, novel=120, uncategorized=999(ignored).
	// temp = 30 (task1) + 11 (paused) + 9 (queued) + 7 (converting) = 57.
	// task2's SavePath matches the completed video file -> excluded from temp.
	assert.Equal(t, int64(100), resp.Stats.VideoBytes, "video bytes")
	assert.Equal(t, int64(200), resp.Stats.MangaBytes, "manga bytes")
	assert.Equal(t, int64(120), resp.Stats.NovelBytes, "novel bytes")
	assert.Equal(t, int64(57), resp.Stats.TempBytes, "temp bytes")
	assert.Equal(t, int64(100+200+120+57), resp.Stats.TotalBytes, "total bytes")
}


// TestGetStorageStatsEmpty verifies the handler returns zeroed stats (and no
// error) when there are no downloads and no active tasks.
func TestGetStorageStatsEmpty(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1&_pragma=foreign_keys(1)")
	defer client.Close()
	oldClient := ext.GetEntClientForTest()
	ext.SetEntClientForTest(client)
	t.Cleanup(func() { ext.SetEntClientForTest(oldClient) })
	t.Cleanup(dl.ClearActiveProgressForTest)

	srv := &MiruCoreServer{}
	resp, err := srv.GetStorageStats(ctx, &pb.GetStorageStatsRequest{DownloadPath: t.TempDir()})
	require.NoError(t, err)
	require.NotNil(t, resp.Stats)
	assert.Equal(t, int64(0), resp.Stats.VideoBytes)
	assert.Equal(t, int64(0), resp.Stats.MangaBytes)
	assert.Equal(t, int64(0), resp.Stats.NovelBytes)
	assert.Equal(t, int64(0), resp.Stats.TempBytes)
	assert.Equal(t, int64(0), resp.Stats.TotalBytes)
}

// seedDownload inserts a Download row with the given category/status/savePath.
func seedDownload(t *testing.T, client *ent.Client, category, status, savePath string) {
	t.Helper()
	cat := download.Category(category)
	// watch_url + detail_url must be unique per row; vary them by category+path.
	watchURL := "https://example.org/watch-" + category + "-" + filepath.Base(savePath)
	detailURL := "https://example.org/detail-" + category + "-" + filepath.Base(savePath)
	_, err := client.Download.Create().
		SetURL([]string{"https://example.org/x.m3u8"}).
		SetWatchUrl(watchURL).
		SetDetailUrl(detailURL).
		SetHeaders(map[string]string{}).
		SetPackage("test-pkg").
		SetProgress([]int{0, 100}).
		SetKey("key-" + category + "-" + savePath).
		SetTitle("title-" + category).
		SetMediaType("mp4").
		SetStatus(status).
		SetSavePath(savePath).
		SetCategory(cat).
		Save(context.Background())
	require.NoError(t, err)
}
