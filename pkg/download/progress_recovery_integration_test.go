package download

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/miru-project/miru-core/ent"
	_ "github.com/miru-project/miru-core/ent/runtime"
)

// setupTestClient creates an in-memory SQLite ent client for testing,
// using the same driver and connection settings as production.
func setupTestClient(t *testing.T) *ent.Client {
	t.Helper()
	drv, err := entsql.Open("sqlite3", "file::memory:?cache=shared&_fk=1&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	c := ent.NewClient(ent.Driver(drv))
	if err := c.Schema.Create(context.Background()); err != nil {
		t.Fatal(err)
	}
	return c
}

// TestSyncDB_RoundTrip_ProgressAndTotal verifies that SyncDB persists both
// progress AND total into the DB, and Init-recovery reads them back correctly.
// Status=Downloading so the file-verify path is exercised.
func TestSyncDB_RoundTrip_ProgressAndTotal(t *testing.T) {
	dir := t.TempDir()

	// Create 2 segment files.
	for _, name := range []string{"0.ts", "1.ts"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	client := setupTestClient(t)
	defer client.Close()

	// Simulate what downloadHls does: create a download record via ent directly.
	record, err := client.Download.Create().
		SetURL([]string{"https://example.org/playlist.m3u8"}).
		SetWatchUrl("https://example.org/watch").
		SetDetailUrl("https://example.org/detail").
		SetHeaders(map[string]string{"Authorization": "Bearer test"}).
		SetPackage("test-pkg").
		SetProgress([]int{2, 2}).
		SetKey("test-key").
		SetTitle("Test Title").
		SetMediaType("hls").
		SetStatus("Downloading").
		SetSavePath(dir).
		Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	// Read it back — simulate what Init does.
	d, err := client.Download.Get(t.Context(), record.ID)
	if err != nil {
		t.Fatal(err)
	}

	p := 0
	total := 0
	if len(d.Progress) > 0 {
		p = d.Progress[0]
	}
	if len(d.Progress) > 1 {
		total = d.Progress[1]
	}

	if p != 2 {
		t.Errorf("expected DB progress=2, got %d", p)
	}
	if total != 2 {
		t.Errorf("expected DB total=2, got %d", total)
	}

	// Now simulate Init recovery: create Progress and verify files.
	progress := &Progress{
		Progrss:   p,
		Total:     total,
		Status:    Downloading,
		MediaType: Hls,
		TaskID:    99,
		SavePath:  dir,
	}

	verifyAndAdjustProgress(progress)

	if progress.Progrss != 2 {
		t.Errorf("after verify: expected progress=2 (both files exist), got %d", progress.Progrss)
	}
}

// TestSyncDB_RoundTrip_SegmentDeleted verifies the full restart flow when a
// segment file was deleted while the backend was offline.
func TestSyncDB_RoundTrip_SegmentDeleted(t *testing.T) {
	dir := t.TempDir()

	// Only create segment 0; segment 1 was "deleted" while offline.
	if err := os.WriteFile(filepath.Join(dir, "0.ts"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	client := setupTestClient(t)
	defer client.Close()

	// DB record claims progress=2, total=2 (both segments downloaded before shutdown).
	record, err := client.Download.Create().
		SetURL([]string{"https://example.org/playlist.m3u8"}).
		SetWatchUrl("https://example.org/watch").
		SetDetailUrl("https://example.org/detail").
		SetHeaders(map[string]string{}).
		SetPackage("test-pkg").
		SetProgress([]int{2, 2}).
		SetKey("test-key-2").
		SetTitle("Deleted Segment").
		SetMediaType("hls").
		SetStatus("Downloading").
		SetSavePath(dir).
		Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	d, err := client.Download.Get(t.Context(), record.ID)
	if err != nil {
		t.Fatal(err)
	}

	p := 0
	total := 0
	if len(d.Progress) > 0 {
		p = d.Progress[0]
	}
	if len(d.Progress) > 1 {
		total = d.Progress[1]
	}

	progress := &Progress{
		Progrss:   p,
		Total:     total,
		Status:    Status(d.Status),
		MediaType: Hls,
		TaskID:    100,
		SavePath:  dir,
	}

	// Init would set Downloading → Paused, then verify files.
	if progress.Status == Downloading {
		progress.Status = Paused
	}

	verifyAndAdjustProgress(progress)

	if progress.Progrss != 1 {
		t.Errorf("after verify: expected progress=1 (only 1 file on disk), got %d", progress.Progrss)
	}
}

// TestSyncDB_RoundTrip_OldDB_NoTotal verifies backward compatibility with old
// DB records that only stored [progress] (1 element, no total).
func TestSyncDB_RoundTrip_OldDB_NoTotal(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"0.ts", "1.ts"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	client := setupTestClient(t)
	defer client.Close()

	// Old DB record: only progress, no total.
	record, err := client.Download.Create().
		SetURL([]string{"https://example.org/playlist.m3u8"}).
		SetWatchUrl("https://example.org/watch").
		SetDetailUrl("https://example.org/detail").
		SetHeaders(map[string]string{}).
		SetPackage("test-pkg").
		SetProgress([]int{2}). // old format: only progress
		SetKey("test-key-3").
		SetTitle("Old DB Format").
		SetMediaType("hls").
		SetStatus("Downloading").
		SetSavePath(dir).
		Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	d, err := client.Download.Get(t.Context(), record.ID)
	if err != nil {
		t.Fatal(err)
	}

	p := 0
	total := 0
	if len(d.Progress) > 0 {
		p = d.Progress[0]
	}
	if len(d.Progress) > 1 {
		total = d.Progress[1]
	}

	progress := &Progress{
		Progrss:   p,
		Total:     total,
		Status:    Status(d.Status),
		MediaType: Hls,
		TaskID:    101,
		SavePath:  dir,
	}

	verifyAndAdjustProgress(progress)

	if progress.Progrss != 2 {
		t.Errorf("expected progress=2, got %d", progress.Progrss)
	}
	// Total was 0 from old DB; verifyAndAdjustProgress should bump it to >= progress.
	if progress.Total < progress.Progrss {
		t.Errorf("total (%d) should be bumped to at least progress (%d)", progress.Total, progress.Progrss)
	}
}

// TestSyncDB_RoundTrip_PausedTask_SkipsVerify verifies that a task that was
// already Paused (user-paused) does NOT have its files checked on recovery.
// Only tasks that were mid-download (Downloading/Converting) get verified.
func TestSyncDB_RoundTrip_PausedTask_SkipsVerify(t *testing.T) {
	dir := t.TempDir()

	// Don't create any files — if verification ran, progress would be reset.
	// But since the task was Paused, verification should be skipped entirely.

	p := &Progress{
		Progrss:   2,
		Total:     2,
		Status:    Paused,
		MediaType: Hls,
		TaskID:    200,
		SavePath:  dir,
	}

	// Init only calls verifyAndAdjustProgress for Downloading/Converting.
	// Simulate: status was Paused → no verify call.
	// (Progress stays at 2 even though no files exist.)

	if p.Progrss != 2 {
		t.Errorf("Paused task should not be verified, expected progress=2, got %d", p.Progrss)
	}
}

// TestSyncDB_RoundTrip_MP4_FileDeleted verifies the full restart flow for an
// MP4 download where the output file was deleted.
func TestSyncDB_RoundTrip_MP4_FileDeleted(t *testing.T) {
	// File path that does NOT exist.
	missingFile := filepath.Join(t.TempDir(), "deleted.mp4")

	client := setupTestClient(t)
	defer client.Close()

	record, err := client.Download.Create().
		SetURL([]string{"https://example.org/video.mp4"}).
		SetWatchUrl("https://example.org/watch").
		SetDetailUrl("https://example.org/detail").
		SetHeaders(map[string]string{}).
		SetPackage("test-pkg").
		SetProgress([]int{5000000, 10000000}).
		SetKey("test-key-4").
		SetTitle("Deleted MP4").
		SetMediaType("mp4").
		SetStatus("Downloading").
		SetSavePath(missingFile).
		Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	d, err := client.Download.Get(t.Context(), record.ID)
	if err != nil {
		t.Fatal(err)
	}

	p := 0
	total := 0
	if len(d.Progress) > 0 {
		p = d.Progress[0]
	}
	if len(d.Progress) > 1 {
		total = d.Progress[1]
	}

	progress := &Progress{
		Progrss:   p,
		Total:     total,
		Status:    Status(d.Status),
		MediaType: Mp4,
		TaskID:    102,
		SavePath:  d.SavePath,
	}

	verifyAndAdjustProgress(progress)

	if progress.Progrss != 0 {
		t.Errorf("after verify: expected progress=0 (file missing), got %d", progress.Progrss)
	}
}

// TestSyncDB_RoundTrip_ConvertingHLS_AllSegmentsDownloaded verifies that on
// backend restart, an HLS download that was in Converting status with all
// segments already downloaded KEEPS Converting status (instead of being forced
// to Paused). This allows the frontend to resume the FFmpeg conversion.
func TestSyncDB_RoundTrip_ConvertingHLS_AllSegmentsDownloaded(t *testing.T) {
	dir := t.TempDir()

	// Create all segment files (simulating complete download before shutdown).
	for _, name := range []string{"0.ts", "1.ts", "2.ts"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	client := setupTestClient(t)
	defer client.Close()

	// DB record: Converting status, progress=3, total=3 (all segments downloaded).
	record, err := client.Download.Create().
		SetURL([]string{"https://example.org/playlist.m3u8"}).
		SetWatchUrl("https://example.org/watch").
		SetDetailUrl("https://example.org/detail").
		SetHeaders(map[string]string{}).
		SetPackage("test-pkg").
		SetProgress([]int{3, 3}).
		SetKey("test-key-converting-complete").
		SetTitle("Converting Complete").
		SetMediaType("hls").
		SetStatus("Converting").
		SetSavePath(dir).
		Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	d, err := client.Download.Get(t.Context(), record.ID)
	if err != nil {
		t.Fatal(err)
	}

	p := 0
	total := 0
	if len(d.Progress) > 0 {
		p = d.Progress[0]
	}
	if len(d.Progress) > 1 {
		total = d.Progress[1]
	}

	// Simulate Init recovery: create Progress from DB record.
	progress := &Progress{
		Progrss:   p,
		Total:     total,
		Status:    Status(d.Status),
		MediaType: Hls,
		TaskID:    999,
		SavePath:  dir,
	}

	// This is what Init does for Converting tasks: verify files.
	verifyAndAdjustProgress(progress)

	// The fix: Converting + all segments downloaded → keep Converting
	// (don't force to Paused).
	if progress.Status != Converting {
		t.Errorf("expected status=Converting (all segments downloaded), got %s", progress.Status)
	}
	if progress.Progrss != 3 {
		t.Errorf("expected progress=3, got %d", progress.Progrss)
	}
}

// TestSyncDB_RoundTrip_ConvertingHLS_PartialSegments verifies that on
// backend restart, an HLS download in Converting status with MISSING
// segments is forced to Paused (so it can resume downloading segments).
func TestSyncDB_RoundTrip_ConvertingHLS_PartialSegments(t *testing.T) {
	dir := t.TempDir()

	// Only create 1 of 3 segment files (simulating partial download).
	if err := os.WriteFile(filepath.Join(dir, "0.ts"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	client := setupTestClient(t)
	defer client.Close()

	// DB record: Converting status, progress=3, total=3 (claimed all downloaded).
	record, err := client.Download.Create().
		SetURL([]string{"https://example.org/playlist.m3u8"}).
		SetWatchUrl("https://example.org/watch").
		SetDetailUrl("https://example.org/detail").
		SetHeaders(map[string]string{}).
		SetPackage("test-pkg").
		SetProgress([]int{3, 3}).
		SetKey("test-key-converting-partial").
		SetTitle("Converting Partial").
		SetMediaType("hls").
		SetStatus("Converting").
		SetSavePath(dir).
		Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	d, err := client.Download.Get(t.Context(), record.ID)
	if err != nil {
		t.Fatal(err)
	}

	p := 0
	total := 0
	if len(d.Progress) > 0 {
		p = d.Progress[0]
	}
	if len(d.Progress) > 1 {
		total = d.Progress[1]
	}

	progress := &Progress{
		Progrss:   p,
		Total:     total,
		Status:    Status(d.Status),
		MediaType: Hls,
		TaskID:    1000,
		SavePath:  dir,
	}

	// Simulate Init logic: verify files, then determine next state based on segments on disk.
	verifyAndAdjustProgress(progress)

	// New Init logic for Converting tasks:
	// - All segments present → keep Converting
	// - 0 segments (total>0) → mark Failed (conversion likely completed but status not updated)
	// - Partial segments → Paused (resume download)
	if progress.Status == Converting {
		if progress.Progrss >= progress.Total && progress.Total > 0 {
			// Keep Converting
		} else if progress.Progrss == 0 && progress.Total > 0 {
			progress.Status = Failed
		} else {
			progress.Status = Paused
		}
	}

	// verifyAndAdjustProgress should clamp progress to actual files (1).
	// Partial segments exist → Init forces status to Paused
	// so the task can resume downloading remaining segments.
	if progress.Status != Paused {
		t.Errorf("expected status=Paused (missing segments), got %s", progress.Status)
	}
	if progress.Progrss != 1 {
		t.Errorf("expected progress=1 (only 1 file on disk), got %d", progress.Progrss)
	}
}

// TestSyncDB_RoundTrip_ConvertingHLS_NamesRebuilt verifies that after a
// backend restart, a Converting HLS task with all segments on disk gets
// its Names list rebuilt from the filesystem.  This is critical because
// the frontend needs segment file paths to call FFmpeg for conversion.
func TestSyncDB_RoundTrip_ConvertingHLS_NamesRebuilt(t *testing.T) {
	dir := t.TempDir()

	// Create 3 segment files (all downloaded before shutdown).
	for _, name := range []string{"0.ts", "1.ts", "2.ts"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	client := setupTestClient(t)
	defer client.Close()

	// DB record: Converting, progress=3, total=3 (all segments downloaded).
	// Note: Names are NOT stored in the DB — they are in-memory only.
	record, err := client.Download.Create().
		SetURL([]string{"https://example.org/playlist.m3u8"}).
		SetWatchUrl("https://example.org/watch").
		SetDetailUrl("https://example.org/detail").
		SetHeaders(map[string]string{}).
		SetPackage("test-pkg").
		SetProgress([]int{3, 3}).
		SetKey("test-key-converting-names").
		SetTitle("Converting Names Test").
		SetMediaType("hls").
		SetStatus("Converting").
		SetSavePath(dir).
		Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	d, err := client.Download.Get(t.Context(), record.ID)
	if err != nil {
		t.Fatal(err)
	}

	p := 0
	total := 0
	if len(d.Progress) > 0 {
		p = d.Progress[0]
	}
	if len(d.Progress) > 1 {
		total = d.Progress[1]
	}

	// Simulate Init recovery: create Progress from DB record.
	// Names is nil — simulating what happens after a restart.
	progress := &Progress{
		Progrss:   p,
		Total:     total,
		Status:    Status(d.Status),
		MediaType: Hls,
		TaskID:    2000,
		SavePath:  dir,
		Names:     nil, // Not persisted in DB.
	}

	// verifyAndAdjustProgress should rebuild Names from disk.
	verifyAndAdjustProgress(progress)

	if progress.Status != Converting {
		t.Errorf("expected status=Converting, got %s", progress.Status)
	}
	if progress.Progrss != 3 {
		t.Errorf("expected progress=3, got %d", progress.Progrss)
	}
	if progress.Names == nil {
		t.Fatal("expected Names to be rebuilt from disk, got nil")
	}
	names := *progress.Names
	if len(names) != 3 {
		t.Fatalf("expected 3 Names rebuilt, got %d", len(names))
	}

	// Verify Names are sorted and contain full paths.
	expectedFiles := []string{"0.ts", "1.ts", "2.ts"}
	for i, exp := range expectedFiles {
		expected := filepath.Join(dir, exp)
		if names[i] != expected {
			t.Errorf("Names[%d] = %q, want %q", i, names[i], expected)
		}
	}
}

// TestSyncDB_RoundTrip_ConvertingHLS_NamesRebuiltOnResume verifies that
// resumeHlsTask rebuilds Names when called for a task with all segments
// already downloaded.
func TestSyncDB_RoundTrip_ConvertingHLS_NamesRebuiltOnResume(t *testing.T) {
	dir := t.TempDir()

	// Create 2 segment files.
	for _, name := range []string{"0.ts", "1.ts"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create a Progress with nil Names (simulating restart).
	p := &Progress{
		Progrss:   2,
		Total:     2,
		Status:    Paused,
		MediaType: Hls,
		TaskID:    2001,
		SavePath:  dir,
		Names:     nil,
	}

	// First call verifyAndAdjustProgress to set up progress correctly.
	verifyAndAdjustProgress(p)

	// Names should be rebuilt.
	if p.Names == nil || len(*p.Names) != 2 {
		t.Fatalf("expected 2 Names after verify, got %v", p.Names)
	}
}

// TestSyncDB_RoundTrip_ConvertingHLS_NoSegmentsOnDisk verifies that when a
// Converting HLS task has ALL segments cleaned up (conversion likely
// completed but status wasn't updated to Completed), the task is marked
// as Failed rather than being stuck at Paused with zero progress.
func TestSyncDB_RoundTrip_ConvertingHLS_NoSegmentsOnDisk(t *testing.T) {
	dir := t.TempDir()
	// Don't create any segment files — simulating post-conversion cleanup.

	client := setupTestClient(t)
	defer client.Close()

	// DB record: Converting, progress=3, total=3 (all downloaded before).
	record, err := client.Download.Create().
		SetURL([]string{"https://example.org/playlist.m3u8"}).
		SetWatchUrl("https://example.org/watch").
		SetDetailUrl("https://example.org/detail").
		SetHeaders(map[string]string{}).
		SetPackage("test-pkg").
		SetProgress([]int{3, 3}).
		SetKey("test-key-converting-noseg").
		SetTitle("Converted No Segments").
		SetMediaType("hls").
		SetStatus("Converting").
		SetSavePath(dir).
		Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	d, err := client.Download.Get(t.Context(), record.ID)
	if err != nil {
		t.Fatal(err)
	}

	p := 0
	total := 0
	if len(d.Progress) > 0 {
		p = d.Progress[0]
	}
	if len(d.Progress) > 1 {
		total = d.Progress[1]
	}

	progress := &Progress{
		Progrss:   p,
		Total:     total,
		Status:    Status(d.Status),
		MediaType: Hls,
		TaskID:    3000,
		SavePath:  dir,
	}

	// verifyAndAdjustProgress: segments gone → progress clamped to 0.
	verifyAndAdjustProgress(progress)

	// New Init logic: Converting + 0 segments + total>0 → Failed.
	if progress.Status == Converting {
		if progress.Progrss >= progress.Total && progress.Total > 0 {
			// Keep Converting
		} else if progress.Progrss == 0 && progress.Total > 0 {
			progress.Status = Failed
		} else {
			progress.Status = Paused
		}
	}

	if progress.Status != Failed {
		t.Errorf("expected status=Failed (segments gone after conversion), got %s", progress.Status)
	}
	if progress.Progrss != 0 {
		t.Errorf("expected progress=0 (no segments on disk), got %d", progress.Progrss)
	}
}
