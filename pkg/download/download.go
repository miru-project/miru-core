package download

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/miru-project/miru-core/ent"
	entDownload "github.com/miru-project/miru-core/ent/download"
	"github.com/miru-project/miru-core/ext"
	"github.com/miru-project/miru-core/pkg/db"
	miruTorrent "github.com/miru-project/miru-core/pkg/torrent"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

var tasks = sync.Map{}  // taskId → context.CancelFunc (running goroutines)
var statusMap sync.Map  // taskId → *Progress (all known tasks)
var taskParams sync.Map // taskId → TaskParamInterface

var OnStatusUpdate func(map[int]*Progress)

// schedulerMu serialises the scheduler loop so only one goroutine at a time
// picks the next task and promotes it. Without this, two concurrent callers
// could both select the same highest-priority task.
var schedulerMu sync.Mutex

type Progress struct {
	Progrss            int               `json:"progress"`
	Names              *[]string         `json:"names"`
	Total              int               `json:"total"`
	Status             Status            `json:"status"`
	MediaType          MediaType         `json:"media_type"`
	CurrentDownloading string            `json:"current_downloading"`
	TaskID             int               `json:"task_id"`
	Title              string            `json:"title"`
	Package            string            `json:"package"`
	Key                string            `json:"key"`
	URL                []string          `json:"url"`
	Headers            map[string]string `json:"headers"`
	SavePath           string            `json:"save_path"`
	Category           Category          `json:"category"`
	DetailUrl          string            `json:"detail_url"`
	WatchUrl           string            `json:"watch_url"`
	// Priority used by the concurrency-limited scheduler. Higher runs first.
	Priority int `json:"priority"`
	// Error message when status is Failed.
	Error string `json:"error"`
}

type TaskParam struct {
	taskID int
}

type TaskParamInterface interface {
	GetTaskID() int
}

type MediaType string

const (
	Hls     MediaType = "hls"
	Mp4     MediaType = "mp4"
	Torrent MediaType = "torrent"
	Magnet  MediaType = "magnet"
)

// Category is the content category of a download, used for storage grouping.
// Its string values match the ent enum column and the proto DownloadCategory
// value names (video / manga / novel); an empty value means unspecified.
type Category string

const (
	CategoryVideo Category = "video"
	CategoryManga Category = "manga"
	CategoryNovel Category = "novel"
)

// CategoryFromProto converts a proto DownloadCategory into the internal content
// category. Unspecified/unknown values map to "" so callers can treat it as "no
// category" (stored back as unspecified).
func CategoryFromProto(c proto.DownloadCategory) Category {
	switch c {
	case proto.DownloadCategory_video:
		return CategoryVideo
	case proto.DownloadCategory_manga:
		return CategoryManga
	case proto.DownloadCategory_novel:
		return CategoryNovel
	default:
		return ""
	}
}

// CategoryToProto converts the internal content category into the proto
// DownloadCategory enum. Empty / unknown values map to unspecified.
func CategoryToProto(c Category) proto.DownloadCategory {
	switch c {
	case CategoryVideo:
		return proto.DownloadCategory_video
	case CategoryManga:
		return proto.DownloadCategory_manga
	case CategoryNovel:
		return proto.DownloadCategory_novel
	default:
		return proto.DownloadCategory_unspecified
	}
}

type Status string

const (
	Downloading Status = "Downloading"
	Paused      Status = "Paused"
	Completed   Status = "Completed"
	Failed      Status = "Failed"
	Canceled    Status = "Canceled"
	Queued      Status = "Queued"
	Converting  Status = "Converting"
)

func StatusToProto(s Status) proto.DownloadStatus {
	switch s {
	case Downloading:
		return proto.DownloadStatus_DOWNLOADING
	case Paused:
		return proto.DownloadStatus_PAUSED
	case Completed:
		return proto.DownloadStatus_COMPLETED
	case Failed:
		return proto.DownloadStatus_FAILED
	case Canceled:
		return proto.DownloadStatus_CANCELLED
	case Converting:
		return proto.DownloadStatus_CONVERTING
	default:
		return proto.DownloadStatus_QUEUED
	}
}

func StatusFromProto(s proto.DownloadStatus) Status {
	switch s {
	case proto.DownloadStatus_DOWNLOADING:
		return Downloading
	case proto.DownloadStatus_PAUSED:
		return Paused
	case proto.DownloadStatus_COMPLETED:
		return Completed
	case proto.DownloadStatus_FAILED:
		return Failed
	case proto.DownloadStatus_CANCELLED:
		return Canceled
	case proto.DownloadStatus_CONVERTING:
		return Converting
	default:
		return Queued
	}
}

func (t *TaskParam) GetTaskID() int {
	return t.taskID
}

func DownloadStatus() map[int]*Progress {
	snapshot := make(map[int]*Progress)
	statusMap.Range(func(k, v any) bool {
		snapshot[k.(int)] = v.(*Progress)
		return true
	})
	return snapshot
}

// genTaskID generates a unique task ID not currently in use.
func genTaskID() int {
	for {
		id := rand.Intn(1000000)
		if _, exists := statusMap.Load(id); !exists {
			return id
		}
	}
}

// ---------------------------------------------------------------------------
// Concurrency-limited scheduler
//
// `maxConcurrent` caps how many tasks may actually be running at once. Tasks
// that cannot start immediately are parked in the `Queued` state and promoted
// by priority order (higher Priority first) whenever a running slot frees up.
// ---------------------------------------------------------------------------

// maxConcurrent is the configured cap on simultaneously running downloads.
// It is read from the app setting "downloadConcurrent" (default 3) in Init.
var maxConcurrent = 3

// DefaultMaxConcurrentDownload is used when the setting is missing/invalid.
const DefaultMaxConcurrentDownload = 3

// settingKeyMaxConcurrent mirrors the key used by the frontend settings UI.
const settingKeyMaxConcurrent = "downloadConcurrent"

// GetMaxConcurrent returns the current concurrency cap.
func GetMaxConcurrent() int {
	if maxConcurrent < 1 {
		return 1
	}
	return maxConcurrent
}

// SetMaxConcurrent updates the in-memory cap and persists it as an app setting.
func SetMaxConcurrent(n int) {
	if n < 1 {
		n = 1
	}
	maxConcurrent = n
	if dbErr := db.SetAppSetting(settingKeyMaxConcurrent, fmt.Sprintf("%d", n)); dbErr != nil {
		log.Printf("failed to persist maxConcurrent setting: %v", dbErr)
	}
	// A larger cap may allow previously queued tasks to start now.
	scheduleDownloads()
}

// loadMaxConcurrent reads the persisted cap from app settings (best effort).
func loadMaxConcurrent() {
	v, err := db.GetAPPSetting(settingKeyMaxConcurrent)
	if err != nil || v == "" {
		maxConcurrent = DefaultMaxConcurrentDownload
		return
	}
	if n, e := strconv.Atoi(v); e == nil && n >= 1 {
		maxConcurrent = n
	} else {
		maxConcurrent = DefaultMaxConcurrentDownload
	}
}

// activeRunningCount returns how many tasks are currently executing (live
// goroutines in the Downloading/Converting state).
func activeRunningCount() int {
	count := 0
	tasks.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

// resumeFunc is the function used to actually (re)start a task. It defaults to
// resumeByID but can be overridden in tests to avoid real network activity.
var resumeFunc func(int) error

func init() {
	resumeFunc = resumeByID
}

// resumeByID starts (resumes) the task with the given id using its media type.
// Returns an error if the task is unknown or already running.
func resumeByID(taskId int) error {
	if _, ok := tasks.Load(taskId); ok {
		return fmt.Errorf("task %d already running", taskId)
	}
	v, ok := statusMap.Load(taskId)
	if !ok {
		return fmt.Errorf("task %d not found", taskId)
	}
	p := v.(*Progress)
	switch p.MediaType {
	case Hls:
		return resumeHlsTask(taskId)
	case Mp4:
		return resumeMp4Task(taskId)
	case Torrent, Magnet:
		return resumeTorrentTask(taskId)
	}
	return fmt.Errorf("task %d has unknown media type", taskId)
}

// enqueueOrStart either launches the task immediately (when a slot is free) or
// parks it in the Queued state for the scheduler to promote later.
func enqueueOrStart(taskId int) {
	v, ok := statusMap.Load(taskId)
	if !ok {
		return
	}
	p := v.(*Progress)
	if activeRunningCount() < GetMaxConcurrent() {
		p.Status = Downloading
		_ = resumeFunc(taskId)
		return
	}
	p.Status = Queued
	p.SyncDB()
}

// scheduleDownloads promotes the highest-priority Queued tasks until the
// running count reaches the concurrency cap. It is safe to call from any
// lifecycle transition (pause/cancel/complete/setting change).
func scheduleDownloads() {
	schedulerMu.Lock()
	defer schedulerMu.Unlock()
	for activeRunningCount() < GetMaxConcurrent() {
		next := highestPriorityQueued()
		if next == 0 {
			return
		}
		v, _ := statusMap.Load(next)
		p := v.(*Progress)
		p.Status = Downloading
		_ = resumeFunc(next)
	}
}

// highestPriorityQueued returns the task id of the Queued task with the highest
// Priority (ties broken by lower task id for determinism). Returns 0 if none.
func highestPriorityQueued() int {
	bestID := 0
	bestPriority := -1
	statusMap.Range(func(k, v any) bool {
		id := k.(int)
		p := v.(*Progress)
		if p.Status != Queued {
			return true
		}
		if p.Priority > bestPriority || (p.Priority == bestPriority && (bestID == 0 || id < bestID)) {
			bestPriority = p.Priority
			bestID = id
		}
		return true
	})
	return bestID
}

// SetPriority updates a task's scheduler priority and re-runs the scheduler so
// a higher-priority task can preempt a free slot immediately.
func SetPriority(taskId int, priority int) error {
	v, ok := statusMap.Load(taskId)
	if !ok {
		return fmt.Errorf("task %d not found", taskId)
	}
	p := v.(*Progress)
	p.Priority = priority
	p.SyncDB()
	scheduleDownloads()
	return nil
}

// ReorderTasks receives the full desired order of task ids (front = highest
// priority). It reassigns priorities so the list order is respected and then
// runs the scheduler. Unknown ids are ignored.
func ReorderTasks(orderedIDs []int) error {
	for i, id := range orderedIDs {
		if v, ok := statusMap.Load(id); ok {
			p := v.(*Progress)
			// Front of the list gets the highest priority.
			p.Priority = len(orderedIDs) - i
			p.SyncDB()
		}
	}
	scheduleDownloads()
	return nil
}

func CancelTask(taskId int) error {
	if cancelFunc, ok := tasks.Load(taskId); ok {
		cancelFunc.(context.CancelFunc)()
		tasks.Delete(taskId)
	}
	v, ok := statusMap.Load(taskId)
	if !ok {
		return fmt.Errorf("task %d not found", taskId)
	}
	p := v.(*Progress)

	p.Status = Canceled

	// Remove files based on media type
	switch p.MediaType {
	case Hls:
		// Remove the entire segment directory
		if p.SavePath != "" {
			if err := os.RemoveAll(p.SavePath); err != nil {
				log.Printf("cancel task %d: failed to remove HLS dir %s: %v", taskId, p.SavePath, err)
			}
		}
		// Also remove any individual segment files from Names
		if p.Names != nil {
			for _, file := range *p.Names {
				if err := os.Remove(file); err != nil {
					log.Printf("cancel task %d: failed to remove segment %s: %v", taskId, file, err)
				}
			}
		}
	case Mp4:
		if p.CurrentDownloading != "" {
			if err := os.Remove(p.CurrentDownloading); err != nil && !os.IsNotExist(err) {
				log.Printf("cancel task %d: failed to remove file %s: %v", taskId, p.CurrentDownloading, err)
			}
		}
	case Torrent, Magnet:
		miruTorrent.DeleteTorrent(p.Key, true)
		// Remove the downloaded file
		if p.CurrentDownloading != "" {
			if err := os.Remove(p.CurrentDownloading); err != nil && !os.IsNotExist(err) {
				log.Printf("cancel task %d: failed to remove file %s: %v", taskId, p.CurrentDownloading, err)
			}
		}
		// Remove parent dir if empty
		if p.CurrentDownloading != "" {
			dir := filepath.Dir(p.CurrentDownloading)
			if dir != "" {
				_ = os.Remove(dir) // best-effort: only succeeds if empty
			}
		}
	}

	// Remove DB entry and in-memory state
	removeTaskFromDB(taskId)
	scheduleDownloads()
	return nil
}

// removeTaskFromDB deletes the download entry from the DB and cleans up
// in-memory maps so the task does not reappear after a restart.
func removeTaskFromDB(taskId int) {
	if ext.IsDBReady() {
		v, ok := statusMap.Load(taskId)
		if ok {
			p := v.(*Progress)
			// Delete by the Key field which is unique per download
			// and always populated when the download is created.
			d, err := db.GetDownloadByKey(p.Key)
			if err == nil && d != nil {
				if delErr := db.DeleteDownloadByID(d.ID); delErr != nil {
					log.Printf("cancel task %d: failed to delete DB entry: %v", taskId, delErr)
				}
			} else if err != nil {
				log.Printf("cancel task %d: failed to find DB entry by key %q: %v", taskId, p.Key, err)
			}
		}
	}
	statusMap.Delete(taskId)
	taskParams.Delete(taskId)
}

func PauseTask(taskId int) error {
	if cancelFunc, ok := tasks.Load(taskId); ok {
		cancelFunc.(context.CancelFunc)()
		tasks.Delete(taskId)
		v, _ := statusMap.Load(taskId)
		p := v.(*Progress)
		p.Status = Paused
		p.SyncDB()

		scheduleDownloads()
		return nil
	}

	return fmt.Errorf("task %d not found", taskId)
}

func ResumeTask(taskId int) error {
	// Resume the task if it exists. Routing through the scheduler keeps the
	// concurrency cap honoured: a resumed task may be re-queued if at capacity.
	if _, ok := tasks.Load(taskId); ok {
		return fmt.Errorf("task %d already running", taskId)
	}
	if _, ok := statusMap.Load(taskId); !ok {
		return fmt.Errorf("task %d not found", taskId)
	}
	enqueueOrStart(taskId)
	return nil
}

// startDownloadTask stores the cancel func and runs taskFunc in a goroutine.
// When the goroutine finishes it cleans up and promotes queued tasks.
func startDownloadTask[T TaskParamInterface](param T, taskFunc func(param T, ctx context.Context)) {
	ctx, cancel := context.WithCancel(context.Background())
	taskId := param.GetTaskID()
	tasks.Store(taskId, cancel)

	go func() {
		defer cancel()
		taskFunc(param, ctx)
		tasks.Delete(taskId)
		scheduleDownloads()
	}()
}

// Check if the path is absolute or relative. For path in hls playlist, it can be either
// absolute or relative. If it is relative, join it with the previous path
func parsePath(basePath string, fileName string) string {

	// Get the current working directory or relative path
	link, _ := url.Parse(basePath)
	name, _ := url.Parse(fileName)
	// Return fileName if it is absolute
	if name.IsAbs() {
		return fileName
	}

	dir := filepath.Dir(link.Path)
	link.Path = filepath.Join(dir, fileName)
	return link.String()

}

func (p *Progress) SyncDB() {
	// Persist to the DB only when one is configured. The in-memory status map
	// (used by the frontend for live updates) is always kept up to date, so
	// callers/tests without a DB still work.
	if ext.IsDBReady() {
		cat := entDownload.Category(string(p.Category))
		if cat == "" {
			cat = entDownload.CategoryUnspecified
		}
		db.UpsertDownload(&ent.Download{
			URL:       p.URL,
			Headers:   p.Headers,
			Package:   p.Package,
			Progress:  []int{p.Progrss, p.Total}, // [0]=progress, [1]=total
			Key:       p.Key,
			Title:     p.Title,
			MediaType: entDownload.MediaType(string(p.MediaType)),
			Status:    string(p.Status),
			Category:  cat,
			SavePath:  p.SavePath,
			DetailUrl: p.DetailUrl,
			WatchUrl:  p.WatchUrl,
			Priority:  p.Priority,
		})
	}
	if OnStatusUpdate != nil {
		OnStatusUpdate(DownloadStatus())
	}
}

func GetTaskParam(taskId int) TaskParamInterface {
	v, ok := taskParams.Load(taskId)
	if !ok {
		return nil
	}
	return v.(TaskParamInterface)
}

// verifyAndAdjustProgress checks whether the on-disk segment files still exist
// for a recovered download and clamps Progress downward when files are missing.
// This prevents the resumed download from claiming N segments are done when
// some files were deleted while the backend was offline.
func verifyAndAdjustProgress(p *Progress) {
	switch p.MediaType {
	case Hls:
		verifyHlsProgress(p)
	case Mp4:
		verifyMp4Progress(p)
	case Torrent, Magnet:
		verifyTorrentProgress(p)
	}
}

// verifyHlsProgress counts the segment files in SavePath (the segment
// directory).  Segment names follow the pattern "{index}{ext}" (e.g.
// 0.ts, 1.ts).  We count files whose base name is purely numeric,
// clamp Progrss to that count if it is lower, and rebuild the Names
// list from disk when it is nil/empty (which happens after a backend
// restart because Names are not persisted in the DB).
func verifyHlsProgress(p *Progress) {
	if p.SavePath == "" {
		return
	}
	dir := p.SavePath
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Directory gone — nothing downloaded.
		log.Printf("HLS task %d: segment dir %s missing, resetting progress", p.TaskID, dir)
		p.Progrss = 0
		p.Total = 0
		p.Names = &[]string{}
		return
	}

	// Collect segment files sorted by their numeric index so Names is
	// rebuilt in the correct order for FFmpeg concatenation.
	type segmentFile struct {
		index int
		path  string
	}
	var segmentFiles []segmentFile

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Strip extension and check if the base is a number (segment file).
		base := strings.TrimSuffix(name, filepath.Ext(name))
		if idx, err := strconv.Atoi(base); err == nil {
			segmentFiles = append(segmentFiles, segmentFile{
				index: idx,
				path:  filepath.Join(dir, name),
			})
		}
	}

	// Sort by numeric index to ensure correct segment order.
	for i := 0; i < len(segmentFiles); i++ {
		for j := i + 1; j < len(segmentFiles); j++ {
			if segmentFiles[j].index < segmentFiles[i].index {
				segmentFiles[i], segmentFiles[j] = segmentFiles[j], segmentFiles[i]
			}
		}
	}

	segmentCount := len(segmentFiles)
	if segmentCount < p.Progrss {
		log.Printf("HLS task %d: had progress=%d but only %d segment files on disk, adjusting",
			p.TaskID, p.Progrss, segmentCount)
		p.Progrss = segmentCount
	}

	// Rebuild Names from disk when nil or empty.  This happens after a
	// backend restart because the Names field is not persisted in the DB.
	if p.Names == nil || len(*p.Names) == 0 {
		if segmentCount > 0 {
			names := make([]string, segmentCount)
			for i, sf := range segmentFiles {
				names[i] = sf.path
			}
			p.Names = &names
			log.Printf("HLS task %d: rebuilt %d segment names from disk", p.TaskID, segmentCount)
		} else {
			p.Names = &[]string{}
		}
	}

	// Total may have been 0 from old DB records; try to preserve it but
	// never set it below the current progress.
	if p.Total < p.Progrss {
		p.Total = p.Progrss
	}
}

// verifyMp4Progress checks whether the single output file exists.
func verifyMp4Progress(p *Progress) {
	if p.SavePath == "" {
		return
	}
	if _, err := os.Stat(p.SavePath); os.IsNotExist(err) {
		log.Printf("MP4 task %d: file %s missing, resetting progress", p.TaskID, p.SavePath)
		p.Progrss = 0
	}
}

// verifyTorrentProgress checks whether the downloaded torrent/magnet file
// still exists on disk. For Converting tasks, the file must exist so the
// frontend can copy it to the download folder. If missing, the task is
// marked Failed.
func verifyTorrentProgress(p *Progress) {
	filePath := p.CurrentDownloading
	if filePath == "" {
		filePath = p.SavePath
	}
	if filePath == "" {
		return
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("Torrent task %d: file %s missing, resetting progress", p.TaskID, filePath)
		if p.Status == Converting {
			p.Status = Failed
			p.Error = "downloaded file missing after restart"
			p.SyncDB()
		} else {
			p.Progrss = 0
		}
	}
}

func Init() {
	// Restore the configured concurrency cap before (re)scheduling.
	loadMaxConcurrent()

	downloads, err := db.GetPendingDownloads()
	if err != nil {
		return
	}
	for _, d := range downloads {
		id := genTaskID()
		p := 0
		total := 0
		if len(d.Progress) > 0 {
			p = d.Progress[0]
		}
		if len(d.Progress) > 1 {
			total = d.Progress[1]
		}

		prog := &Progress{
			Progrss:   p,
			Total:     total,
			Status:    Status(d.Status),
			MediaType: MediaType(d.MediaType),
			TaskID:    id,
			Title:     d.Title,
			Package:   d.Package,
			Key:       d.Key,
			URL:       d.URL,
			Headers:   d.Headers,
			SavePath:  d.SavePath,
			Priority:  d.Priority,
			// Category was not restored, so every task that survived a backend
			// restart came back as `unspecified` and vanished from the mobile
			// Video/Manga/Novel tabs (only "All Downloads" still listed them).
			Category: Category(d.Category),
		}
		statusMap.Store(id, prog)

		// Verify that segment/output files still exist on disk ONLY for
		// tasks that were actively downloading when the backend stopped.
		// Already-paused or queued tasks were not mid-stream so their
		// files should be intact.
		if prog.Status == Converting || prog.Status == Downloading {
			verifyAndAdjustProgress(prog)

			if prog.Status == Converting {
				switch prog.MediaType {
				case Hls:
					// HLS Converting tasks: decide next state based on what's on disk.
					if prog.Progrss >= prog.Total && prog.Total > 0 {
						// All segments present — keep Converting so the
						// frontend can re-run FFmpeg.
					} else if prog.Progrss == 0 && prog.Total > 0 {
						// All segments were cleaned up (conversion likely
						// completed and segments deleted) but the DB was not
						// updated to Completed (e.g. gRPC call failed).
						// There are no segments to convert, so mark Failed
						// rather than misleading Paused-with-zero-progress.
						prog.Status = Failed
						log.Printf("HLS task %d: Converting but 0 segments on disk, marking Failed (conversion may have completed)", prog.TaskID)
						prog.SyncDB()
					} else {
						// Partial segments missing — fall back to Paused so
						// the user can resume the download.
						prog.Status = Paused
					}
				case Torrent, Magnet, Mp4:
					// Non-HLS Converting: verifyTorrentProgress/verifyMp4Progress
					// already ran above. If the file is still on disk, keep
					// Converting so the frontend can copy it to the download
					// folder. If it was missing, verifyTorrentProgress already
					// marked the task as Failed.
				}
			} else {
				// Downloading tasks always fall back to Paused on restart.
				prog.Status = Paused
			}
		}

		// Reconstruct TaskParam
		switch MediaType(d.MediaType) {
		case Hls:
			taskParams.Store(id, &HlsTaskParam{
				TaskParam:   TaskParam{taskID: id},
				playListUrl: d.URL[0],
				filePath:    d.SavePath,
				headers:     d.Headers,
			})
		case Mp4:
			taskParams.Store(id, &Mp4TaskParam{
				TaskParam: TaskParam{taskID: id},
				url:       d.URL[0],
				filePath:  d.SavePath,
				header:    d.Headers,
				title:     d.Title,
				pkg:       d.Package,
				key:       d.Key,
			})
		case Torrent, Magnet:
			taskParams.Store(id, &TorrentTaskParam{
				TaskParam: TaskParam{taskID: id},
				url:       d.URL[0],
				title:     d.Title,
				pkg:       d.Package,
			})
		}
	}

	// (Re)start downloads respecting the concurrency cap and priority order.
	scheduleDownloads()
}
