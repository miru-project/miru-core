package download

import (
	"os"
	"path/filepath"
	"testing"
)

// TestHLSProgressRecovery_AllSegmentsExist simulates:
//   - Backend saves progress=2, total=2 for an HLS task with 2 segments.
//   - Backend shuts down.
//   - Both segment files still exist on disk.
//   - Backend restarts, Init restores progress.
//
// Expected: progress stays at 2.
func TestHLSProgressRecovery_AllSegmentsExist(t *testing.T) {
	dir := t.TempDir()

	// Create 2 segment files that match the HLS naming convention
	// "{index}{ext}" (e.g. 0.ts, 1.ts).
	for _, name := range []string{"0.ts", "1.ts"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	p := &Progress{
		Progrss: 2,
		Total:   2,
		Status:  Paused,
		MediaType: Hls,
		TaskID:  42,
		SavePath: dir,
	}

	verifyAndAdjustProgress(p)

	if p.Progrss != 2 {
		t.Errorf("expected progress=2, got %d", p.Progrss)
	}
	if p.Total != 2 {
		t.Errorf("expected total=2, got %d", p.Total)
	}
}

// TestHLSProgressRecovery_OneSegmentDeleted simulates:
//   - Backend saved progress=2, total=2.
//   - While offline, segment "1.ts" was deleted.
//   - Backend restarts.
//
// Expected: progress clamped to 1 (only 1 file on disk).
func TestHLSProgressRecovery_OneSegmentDeleted(t *testing.T) {
	dir := t.TempDir()

	// Only segment 0 exists; segment 1 was deleted.
	if err := os.WriteFile(filepath.Join(dir, "0.ts"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	p := &Progress{
		Progrss:  2,
		Total:    2,
		Status:   Paused,
		MediaType: Hls,
		TaskID:   43,
		SavePath: dir,
	}

	verifyAndAdjustProgress(p)

	if p.Progrss != 1 {
		t.Errorf("expected progress clamped to 1, got %d", p.Progrss)
	}
	if p.Total < p.Progrss {
		t.Errorf("total (%d) should be >= progress (%d)", p.Total, p.Progrss)
	}
}

// TestHLSProgressRecovery_AllSegmentsDeleted simulates:
//   - Backend saved progress=2, total=2.
//   - Entire segment directory was deleted while offline.
//   - Backend restarts.
//
// Expected: progress=0, total=0.
func TestHLSProgressRecovery_AllSegmentsDeleted(t *testing.T) {
	dir := t.TempDir()
	// Don't create any files — directory is empty.

	p := &Progress{
		Progrss:  2,
		Total:    2,
		Status:   Paused,
		MediaType: Hls,
		TaskID:   44,
		SavePath: dir,
	}

	verifyAndAdjustProgress(p)

	if p.Progrss != 0 {
		t.Errorf("expected progress=0 (dir empty), got %d", p.Progrss)
	}
	// Total is playlist metadata — preserved even when files are gone,
	// so resume knows how many segments to re-download.
	if p.Total != 2 {
		t.Errorf("expected total=2 (playlist size preserved), got %d", p.Total)
	}
}

// TestHLSProgressRecovery_DirMissing simulates:
//   - Backend saved progress=1, total=2.
//   - The segment directory itself was deleted.
//
// Expected: progress=0, total=0.
func TestHLSProgressRecovery_DirMissing(t *testing.T) {
	p := &Progress{
		Progrss:  1,
		Total:    2,
		Status:   Paused,
		MediaType: Hls,
		TaskID:   45,
		SavePath: filepath.Join(t.TempDir(), "nonexistent"),
	}

	verifyAndAdjustProgress(p)

	if p.Progrss != 0 {
		t.Errorf("expected progress=0 (dir missing), got %d", p.Progrss)
	}
	if p.Total != 0 {
		t.Errorf("expected total=0, got %d", p.Total)
	}
}

// TestHLSProgressRecovery_OldDB_NoTotal simulates an old DB record that only
// stored progress (1 element). Total comes back as 0.
//
// Expected: verifyAndAdjustProgress clamps total to at least progress.
func TestHLSProgressRecovery_OldDB_NoTotal(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"0.ts", "1.ts"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	p := &Progress{
		Progrss:  2,
		Total:    0, // Old DB record — total was never saved.
		Status:   Paused,
		MediaType: Hls,
		TaskID:   46,
		SavePath: dir,
	}

	verifyAndAdjustProgress(p)

	if p.Progrss != 2 {
		t.Errorf("expected progress=2, got %d", p.Progrss)
	}
	if p.Total < p.Progrss {
		t.Errorf("total (%d) should be bumped to at least progress (%d)", p.Total, p.Progrss)
	}
}

// TestMP4ProgressRecovery_FileExists simulates:
//   - Backend saved progress for an MP4 download.
//   - File still exists on disk.
//
// Expected: progress unchanged.
func TestMP4ProgressRecovery_FileExists(t *testing.T) {
	f := filepath.Join(t.TempDir(), "video.mp4")
	if err := os.WriteFile(f, []byte("video-data"), 0644); err != nil {
		t.Fatal(err)
	}

	p := &Progress{
		Progrss:  5000000,
		Total:    10000000,
		Status:   Paused,
		MediaType: Mp4,
		TaskID:   47,
		SavePath: f,
	}

	verifyAndAdjustProgress(p)

	if p.Progrss != 5000000 {
		t.Errorf("expected progress=5000000, got %d", p.Progrss)
	}
}

// TestMP4ProgressRecovery_FileMissing simulates:
//   - Backend saved progress for an MP4 download.
//   - The file was deleted while offline.
//
// Expected: progress=0.
func TestMP4ProgressRecovery_FileMissing(t *testing.T) {
	p := &Progress{
		Progrss:  5000000,
		Total:    10000000,
		Status:   Paused,
		MediaType: Mp4,
		TaskID:   48,
		SavePath: filepath.Join(t.TempDir(), "deleted.mp4"),
	}

	verifyAndAdjustProgress(p)

	if p.Progrss != 0 {
		t.Errorf("expected progress=0 (file missing), got %d", p.Progrss)
	}
}

// TestHLSProgressRecovery_MixedExtensions verifies that files with different
// extensions but numeric bases are all counted as segments (e.g. 0.ts, 1.m4s).
func TestHLSProgressRecovery_MixedExtensions(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"0.ts", "1.m4s"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	p := &Progress{
		Progrss:  2,
		Total:    2,
		Status:   Paused,
		MediaType: Hls,
		TaskID:   49,
		SavePath: dir,
	}

	verifyAndAdjustProgress(p)

	if p.Progrss != 2 {
		t.Errorf("expected progress=2 with mixed extensions, got %d", p.Progrss)
	}
}

// TestHLSProgressRecovery_NonSegmentFilesIgnored verifies that non-segment
// files (e.g. .log, .tmp, .json) in the directory are NOT counted.
func TestHLSProgressRecovery_NonSegmentFilesIgnored(t *testing.T) {
	dir := t.TempDir()

	// 1 real segment + junk files
	if err := os.WriteFile(filepath.Join(dir, "0.ts"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, junk := range []string{"meta.json", "state.log", "temp.tmp"} {
		if err := os.WriteFile(filepath.Join(dir, junk), []byte("junk"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	p := &Progress{
		Progrss:  3, // DB claims 3 segments done
		Total:    3,
		Status:   Paused,
		MediaType: Hls,
		TaskID:   50,
		SavePath: dir,
	}

	verifyAndAdjustProgress(p)

	if p.Progrss != 1 {
		t.Errorf("expected progress clamped to 1 (only 1 segment), got %d", p.Progrss)
	}
}

// TestHLSProgressRecovery_ConvertingRebuildsNames verifies that after a
// backend restart, a Converting task with nil Names has them rebuilt from
// the segment files on disk.  The Names list is not persisted in the DB,
// so it must be reconstructed from the filesystem.
func TestHLSProgressRecovery_ConvertingRebuildsNames(t *testing.T) {
	dir := t.TempDir()

	// Create 3 segment files.
	for _, name := range []string{"0.ts", "1.ts", "2.ts"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	p := &Progress{
		Progrss:   3,
		Total:     3,
		Status:    Converting,
		MediaType: Hls,
		TaskID:    60,
		SavePath:  dir,
		Names:     nil, // Simulates restart — Names not persisted.
	}

	verifyAndAdjustProgress(p)

	if p.Names == nil {
		t.Fatal("expected Names to be rebuilt, got nil")
	}
	names := *p.Names
	if len(names) != 3 {
		t.Fatalf("expected 3 Names, got %d", len(names))
	}
	// Verify names are sorted by index and contain the full path.
	for i, expectedName := range []string{"0.ts", "1.ts", "2.ts"} {
		expected := filepath.Join(dir, expectedName)
		if names[i] != expected {
			t.Errorf("Names[%d] = %q, want %q", i, names[i], expected)
		}
	}
}

// TestHLSProgressRecovery_ConvertingEmptyNamesRebuilt verifies that Names
// rebuilt from disk even when the Names field is an empty slice (not nil).
func TestHLSProgressRecovery_ConvertingEmptyNamesRebuilt(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"0.ts", "1.ts"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	empty := []string{}
	p := &Progress{
		Progrss:   2,
		Total:     2,
		Status:    Converting,
		MediaType: Hls,
		TaskID:    61,
		SavePath:  dir,
		Names:     &empty, // Empty slice — should be rebuilt.
	}

	verifyAndAdjustProgress(p)

	names := *p.Names
	if len(names) != 2 {
		t.Fatalf("expected 2 Names, got %d", len(names))
	}
}

// TestHLSProgressRecovery_ConvertingNoSegmentsOnDisk verifies that when a
// Converting task has no segment files on disk, Names is set to empty.
func TestHLSProgressRecovery_ConvertingNoSegmentsOnDisk(t *testing.T) {
	dir := t.TempDir()

	p := &Progress{
		Progrss:   3,
		Total:     3,
		Status:    Converting,
		MediaType: Hls,
		TaskID:    62,
		SavePath:  dir,
		Names:     nil,
	}

	verifyAndAdjustProgress(p)

	// Progress clamped to 0 (no files), Names should be empty.
	if p.Progrss != 0 {
		t.Errorf("expected progress=0 (no files), got %d", p.Progrss)
	}
	if p.Names == nil {
		t.Fatal("expected Names to be non-nil (empty slice), got nil")
	}
	if len(*p.Names) != 0 {
		t.Errorf("expected Names to be empty, got %d", len(*p.Names))
	}
}

// TestHLSProgressRecovery_SortedByIndex verifies that Names are sorted by
// numeric index even if the filesystem returns them in a different order.
func TestHLSProgressRecovery_SortedByIndex(t *testing.T) {
	dir := t.TempDir()

	// Create files in reverse order (filesystem may return them this way).
	for _, name := range []string{"2.ts", "0.ts", "1.ts"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	p := &Progress{
		Progrss:   3,
		Total:     3,
		Status:    Converting,
		MediaType: Hls,
		TaskID:    63,
		SavePath:  dir,
		Names:     nil,
	}

	verifyAndAdjustProgress(p)

	names := *p.Names
	if len(names) != 3 {
		t.Fatalf("expected 3 Names, got %d", len(names))
	}

	// Verify the names are sorted by their numeric index.
	expectedOrder := []string{"0.ts", "1.ts", "2.ts"}
	for i, exp := range expectedOrder {
		expected := filepath.Join(dir, exp)
		if names[i] != expected {
			t.Errorf("Names[%d] = %q, want %q", i, names[i], expected)
		}
	}
}

// TestHLSProgressRecovery_NamesPreservedWhenAlreadyPopulated verifies that
// if Names are already populated (e.g. task was not restarted), they are
// NOT overwritten by the disk scan.
func TestHLSProgressRecovery_NamesPreservedWhenAlreadyPopulated(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"0.ts", "1.ts"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	existingNames := []string{"/custom/path/0.ts", "/custom/path/1.ts"}
	p := &Progress{
		Progrss:   2,
		Total:     2,
		Status:    Converting,
		MediaType: Hls,
		TaskID:    64,
		SavePath:  dir,
		Names:     &existingNames,
	}

	verifyAndAdjustProgress(p)

	// Names should be preserved — not overwritten from disk.
	names := *p.Names
	if len(names) != 2 {
		t.Fatalf("expected 2 Names, got %d", len(names))
	}
	if names[0] != "/custom/path/0.ts" {
		t.Errorf("expected Names[0] preserved as /custom/path/0.ts, got %q", names[0])
	}
}
