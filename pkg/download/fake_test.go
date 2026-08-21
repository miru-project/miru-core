package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	entDownload "github.com/miru-project/miru-core/ent/download"
	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTaskParam carries the parameters of a simulated download. It implements
// TaskParamInterface so it can be parked/started through the same scheduler as
// real downloads.
type fakeTaskParam struct {
	TaskParam
	partialPath string // temp partial file while downloading
	finalPath   string // final file in the download saved folder when complete
	totalBytes  int
	chunkBytes  int
	interval    time.Duration
}

// fakeDownloadForTest starts a simulated download that writes totalBytes to a
// partial file in tempDir, updating progress each chunk, then moves the file
// into downloadDir and marks the task Completed. This exercises the scheduler,
// progress tracking, and the temp → download-saved-folder storage transition
// without any network activity. Test-only: the real service has no fake
// download endpoint.
func fakeDownloadForTest(tempDir, downloadDir, title string, category Category, totalBytes, chunkBytes int, interval time.Duration) (MultipleLinkJson, error) {
	if totalBytes < 1 {
		return MultipleLinkJson{}, fmt.Errorf("fake download byte size must be positive")
	}
	if chunkBytes < 1 {
		chunkBytes = totalBytes
	}
	if interval <= 0 {
		interval = 10 * time.Millisecond
	}

	taskId := genTaskID()
	unique := fmt.Sprintf("fake-%d", taskId)
	partialPath := filepath.Join(tempDir, fmt.Sprintf("%d.part", taskId))
	finalName := network.SanitizeFilename(title)
	if finalName == "" {
		finalName = "fake-download"
	}
	finalPath := filepath.Join(downloadDir, finalName+".mp4")

	p := &Progress{
		Progrss:            0,
		Names:              &[]string{partialPath},
		Total:              totalBytes,
		Status:             Downloading,
		MediaType:          Mp4,
		Category:           category,
		TaskID:             taskId,
		Title:              title,
		Package:            "fake",
		Key:                unique,
		URL:                []string{"fake://download"},
		CurrentDownloading: partialPath,
		SavePath:           finalPath,
		DetailUrl:          "fake://detail/" + unique,
		WatchUrl:           "fake://watch/" + unique,
	}
	statusMap.Store(taskId, p)
	p.SyncDB()

	fakeParam := &fakeTaskParam{
		TaskParam:   TaskParam{taskID: taskId},
		partialPath: partialPath,
		finalPath:   finalPath,
		chunkBytes:  chunkBytes,
		totalBytes:  totalBytes,
		interval:    interval,
	}
	taskParams.Store(taskId, fakeParam)
	startDownloadTask(fakeParam, downloadFakeTask)

	return MultipleLinkJson{IsDownloading: true, TaskID: taskId}, nil
}

// downloadFakeTask writes the simulated file chunk by chunk, syncing progress
// after every chunk, then moves the finished file into the download saved
// folder. It honours the task context so pause/cancel stop the writes.
func downloadFakeTask(param *fakeTaskParam, ctx context.Context) {
	taskId := param.taskID
	v, _ := statusMap.Load(taskId)
	if v == nil {
		return
	}
	p := v.(*Progress)

	completed := p.Progrss
	if completed >= param.totalBytes {
		p.Status = Completed
		p.SyncDB()
		return
	}

	if err := os.MkdirAll(filepath.Dir(param.partialPath), 0755); err != nil {
		p.Status = Failed
		p.Error = err.Error()
		p.SyncDB()
		return
	}
	var file *os.File
	var err error
	if completed > 0 {
		file, err = os.OpenFile(param.partialPath, os.O_WRONLY|os.O_APPEND, 0644)
	} else {
		file, err = os.Create(param.partialPath)
	}
	if err != nil {
		p.Status = Failed
		p.Error = err.Error()
		p.SyncDB()
		return
	}
	defer file.Close()

	buf := make([]byte, param.chunkBytes)
	for written := completed; written < param.totalBytes; {
		select {
		case <-ctx.Done():
			if p.Status != Paused {
				p.Status = Canceled
			}
			p.SyncDB()
			return
		default:
		}
		n := param.chunkBytes
		if written+n > param.totalBytes {
			n = param.totalBytes - written
		}
		if _, err := file.Write(buf[:n]); err != nil {
			p.Status = Failed
			p.Error = err.Error()
			p.SyncDB()
			return
		}
		written += n
		p.Progrss = written
		p.CurrentDownloading = param.partialPath
		p.SyncDB()
		time.Sleep(param.interval)
	}
	file.Close()

	// Move the finished file into the download saved folder.
	if err := os.MkdirAll(filepath.Dir(param.finalPath), 0755); err != nil {
		p.Status = Failed
		p.Error = err.Error()
		p.SyncDB()
		return
	}
	if err := os.Rename(param.partialPath, param.finalPath); err != nil {
		// Fall back to copy+delete when rename crosses filesystems.
		data, rerr := os.ReadFile(param.partialPath)
		if rerr == nil {
			rerr = os.WriteFile(param.finalPath, data, 0644)
		}
		if rerr != nil {
			p.Status = Failed
			p.Error = rerr.Error()
			p.SyncDB()
			return
		}
		_ = os.Remove(param.partialPath)
	}

	p.Progrss = param.totalBytes
	p.CurrentDownloading = ""
	p.SavePath = param.finalPath
	p.Status = Completed
	p.SyncDB()
}

// TestFakeDownloadWritesBytesAndProgress verifies the simulated download writes
// bytes to a temp partial file while reporting progress, then on completion
// moves the file into the download saved folder and persists the Completed row.
func TestFakeDownloadWritesBytesAndProgress(t *testing.T) {
	resetSchedulerState()
	withEntClient(t, setupTestDB(t))
	t.Cleanup(ClearActiveProgressForTest)

	tempDir := t.TempDir()
	downloadDir := t.TempDir()

	const totalBytes = 1 << 20 // 1 MiB
	res, err := fakeDownloadForTest(tempDir, downloadDir, "Fake Video", CategoryVideo, totalBytes, 64*1024, 5*time.Millisecond)
	require.NoError(t, err)
	require.True(t, res.IsDownloading)
	taskID := res.TaskID

	// Mid-download: progress advances and the partial file grows in the temp dir.
	deadline := time.Now().Add(5 * time.Second)
	var sawProgress bool
	var sawPartial bool
	var completed *Progress
	for time.Now().Before(deadline) {
		for _, p := range DownloadStatus() {
			if p.TaskID != taskID {
				continue
			}
			if p.Progrss > 0 && p.Progrss < totalBytes {
				sawProgress = true
			}
			if p.Status == Completed {
				completed = p
			}
		}
		if info, err := os.Stat(filepath.Join(tempDir, fmt.Sprintf("%d.part", taskID))); err == nil && info.Size() > 0 && info.Size() < totalBytes {
			sawPartial = true
		}
		if completed != nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	require.NotNil(t, completed, "task %d should reach Completed", taskID)
	assert.True(t, sawProgress, "progress should advance mid-download")
	assert.True(t, sawPartial, "partial file in the temp dir should grow mid-download")

	// Completed: partial gone, final file in the download folder with full size.
	_, err = os.Stat(filepath.Join(tempDir, fmt.Sprintf("%d.part", taskID)))
	assert.True(t, os.IsNotExist(err), "partial file should be removed on completion")
	files, err := os.ReadDir(downloadDir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	info, err := files[0].Info()
	require.NoError(t, err)
	assert.Equal(t, int64(totalBytes), info.Size(), "saved file size")
	assert.Equal(t, "Fake Video.mp4", files[0].Name(), "saved file name")
	assert.Equal(t, "", completed.CurrentDownloading, "no longer downloading")
	assert.Equal(t, filepath.Join(downloadDir, "Fake Video.mp4"), completed.SavePath)

	// DB row reflects the completed state with the category and final path.
	d, err := db.GetDownloadByKey(fmt.Sprintf("fake-%d", taskID))
	require.NoError(t, err)
	assert.Equal(t, string(Completed), d.Status)
	assert.Equal(t, entDownload.CategoryVideo, d.Category)
	assert.Equal(t, filepath.Join(downloadDir, "Fake Video.mp4"), d.SavePath)
}