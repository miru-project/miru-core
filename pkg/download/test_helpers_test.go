package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/ext"
	"github.com/miru-project/miru-core/pkg/network"
)

// resetSchedulerState clears the package-level scheduler state so each test is
// isolated. It resets statusMap/taskParams/tasks, restores maxConcurrent,
// re-initializes the network layer, resets the playlist fetcher to the real
// implementation, and installs a no-op resumeFunc.
func resetSchedulerState() {
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
	// Reset the playlist fetcher to the real implementation by default.
	fetchPlaylistFunc = fetchPlaylist
	// Override the real "resume" with a fake that simply registers the task as
	// running, so the scheduler's running-slot accounting is exercised without
	// performing any real network downloads.
	resumeFunc = func(taskId int) error {
		tasks.Store(taskId, noopCancel())
		return nil
	}
}

func noopCancel() context.CancelFunc { return func() {} }

// setupTestDB creates an in-memory SQLite ent client for testing.
func setupTestDB(t *testing.T) *ent.Client {
	t.Helper()
	drv, err := entsql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&_fk=1&_pragma=foreign_keys(1)", t.TempDir()+"/test.db"))
	if err != nil {
		t.Fatal(err)
	}
	c := ent.NewClient(ent.Driver(drv))
	if err := c.Schema.Create(context.Background()); err != nil {
		t.Fatal(err)
	}
	return c
}

// withEntClient swaps the package-level ent client for the test duration.
func withEntClient(t *testing.T, client *ent.Client) {
	t.Helper()
	oldClient := ext.GetEntClientForTest()
	ext.SetEntClientForTest(client)
	t.Cleanup(func() { ext.SetEntClientForTest(oldClient) })
}

// writeTestFile writes a small test file, failing the test on error.
func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
}

// writeSegmentFiles writes n segment files named "0.ts"..."n-1.ts" into dir.
func writeSegmentFiles(t *testing.T, dir string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		writeTestFile(t, filepath.Join(dir, fmt.Sprintf("%d.ts", i)))
	}
}

// createDownloadRecord creates a Download DB record, reads it back, and
// returns the record plus the progress/total values extracted from
// d.Progress (the same way Init reads them).
func createDownloadRecord(t *testing.T, client *ent.Client, progress []int, key, title, mediaType, status, savePath string) (*ent.Download, int, int) {
	t.Helper()
	record, err := client.Download.Create().
		SetURL([]string{"https://example.org/playlist.m3u8"}).
		SetWatchUrl("https://example.org/watch").
		SetDetailUrl("https://example.org/detail").
		SetHeaders(map[string]string{}).
		SetPackage("test-pkg").
		SetProgress(progress).
		SetKey(key).
		SetTitle(title).
		SetMediaType(mediaType).
		SetStatus(status).
		SetSavePath(savePath).
		Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	d, err := client.Download.Get(t.Context(), record.ID)
	if err != nil {
		t.Fatal(err)
	}

	var p, total int
	if len(d.Progress) > 0 {
		p = d.Progress[0]
	}
	if len(d.Progress) > 1 {
		total = d.Progress[1]
	}
	return d, p, total
}
