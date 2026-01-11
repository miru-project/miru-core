package download

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/torrent"
	miruTorrent "github.com/miru-project/miru-core/pkg/torrent"
)

var tasks = sync.Map{}
var status = make(map[int]*Progress)
var taskParamMap = make(map[int]TaskParamInterface)

var OnStatusUpdate func(map[int]*Progress)

type Progress struct {
	Progrss            int       `json:"progress"`
	Names              *[]string `json:"names"`
	Total              int       `json:"total"`
	Status             Status    `json:"status"`
	MediaType          MediaType `json:"media_type"`
	CurrentDownloading string    `json:"current_downloading"`
	TaskID             int       `json:"task_id"`
	Title              string    `json:"title"`
	Package            string    `json:"package"`
	Key                string    `json:"key"`
	URL                []string  `json:"url"`
	Headers            string    `json:"headers"`
	SavePath           string    `json:"save_path"`
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
)

type Status string

const (
	Downloading Status = "Downloading"
	Paused      Status = "Paused"
	Completed   Status = "Completed"
	Failed      Status = "Failed"
	Canceled    Status = "Canceled"
	Converted   Status = "Converted"
)

func (t *TaskParam) GetTaskID() int {
	return t.taskID
}

func DownloadStatus() map[int]*Progress {
	// Get the status of all tasks
	return status
}

// Generate a unique task ID
func genTaskID() int {

	for {
		id := rand.Intn(1000000)
		if status[id] == nil {
			return id
		}
	}
}

func CancelTask(taskId int) error {

	if cancelFunc, ok := tasks.Load(taskId); ok {
		cancelFunc.(context.CancelFunc)()
		tasks.Delete(taskId)
	}
	if _, ok := taskParamMap[taskId]; !ok {
		return fmt.Errorf("task %d not found", taskId)
	}

	status[taskId].Status = Canceled

	if status[taskId].Names == nil {
		status[taskId].SyncDB()
		return nil
	}
	names := *status[taskId].Names

	// Remove files if the task is canceled
	switch status[taskId].MediaType {
	case Hls:
		for _, file := range names {
			// Remove the file
			if err := os.Remove(file); err != nil {
				return fmt.Errorf("failed to remove file %s: %v", file, err)
			}
		}
		status[taskId].SyncDB()
		return nil
	case Mp4:
		// Remove single mp4 file
		if err := os.Remove(status[taskId].CurrentDownloading); err != nil {
			return fmt.Errorf("failed to remove file %s: %v", status[taskId].CurrentDownloading, err)
		}
		status[taskId].SyncDB()
	case Torrent:
		miruTorrent.DeleteTorrent(status[taskId].Key, true)
		if err := os.Remove(status[taskId].CurrentDownloading); err != nil {
			return fmt.Errorf("failed to remove file %s: %v", status[taskId].CurrentDownloading, err)
		}
		status[taskId].SyncDB()
	}

	return nil

}

func PauseTask(taskId int) error {

	if cancelFunc, ok := tasks.Load(taskId); ok {
		cancelFunc.(context.CancelFunc)()
		tasks.Delete(taskId)
		status[taskId].Status = Paused
		status[taskId].SyncDB()

		return nil
	}

	return fmt.Errorf("task %d not found", taskId)
}

func ResumeTask(taskId int) error {
	// Resume the task if it exists
	if _, ok := tasks.Load(taskId); ok {
		return fmt.Errorf("task %d already running", taskId)
	}

	switch status[taskId].MediaType {

	case Hls:
		return resumeHlsTask(taskId)
	case Mp4:
		return resumeMp4Task(taskId)
	case Torrent:
		return resumeTorrentTask(taskId)
	}

	return fmt.Errorf("task %d not found", taskId)
}

// Start donwnload task and store it in the task map
func startDownloadTask[T TaskParamInterface](param T, taskFunc func(param T, ctx context.Context)) {

	ctx, cancel := context.WithCancel(context.Background())
	taskId := param.GetTaskID()
	tasks.Store(taskId, cancel)

	// Start the task in a goroutine
	go func() {
		defer tasks.Delete(taskId)
		defer cancel()
		taskFunc(param, ctx)
	}()
}

// Check if the path is absolute or relative. For path in hls playlist, it can be either
// absolute or relative. If it is relative, join it with the previous path
func parsePath(basePath string, fileName string) string {

	// Get the current working directory and join it with the file name
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
	db.UpsertDownload(&ent.Download{
		URL:       p.URL,
		Headers:   p.Headers,
		Package:   p.Package,
		Progress:  []int{p.Progrss}, // Use list of ints as requested
		Key:       p.Key,
		Title:     p.Title,
		MediaType: string(p.MediaType),
		Status:    string(p.Status),
		SavePath:  p.SavePath,
	})
	if OnStatusUpdate != nil {
		OnStatusUpdate(status)
	}
}

func GetTaskParam(taskId int) TaskParamInterface {
	return taskParamMap[taskId]
}

func Init() {
	downloads, err := db.GetAllDownloads()
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

		headers := make(map[string]string)
		if d.Headers != "" {
			_ = json.Unmarshal([]byte(d.Headers), &headers)
		}

		status[id] = &Progress{
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
		}
		// status
		if status[id].Status == Downloading {
			status[id].Status = Paused
		}

		// Reconstruct TaskParam
		switch MediaType(d.MediaType) {
		case Hls:
			taskParamMap[id] = &HlsTaskParam{
				TaskParam:   TaskParam{taskID: id},
				playListUrl: d.URL[0],
				filePath:    d.SavePath,
				headers:     headers,
			}
		case Mp4:
			taskParamMap[id] = &Mp4TaskParam{
				TaskParam: TaskParam{taskID: id},
				url:       d.URL[0],
				filePath:  d.SavePath,
				header:    headers,
				title:     d.Title,
				pkg:       d.Package,
				key:       d.Key,
			}
		case Torrent:
			taskParamMap[id] = &TorrentTaskParam{
				TaskParam: TaskParam{taskID: id},
				url:       d.URL[0],
				title:     d.Title,
				pkg:       d.Package,
			}
		}
	}
}

func resumeTorrentTask(taskId int) error {
	taskParam := taskParamMap[taskId]
	if taskParam == nil {
		return fmt.Errorf("task %d not found", taskId)
	}

	torrentTaskParam, ok := taskParam.(*TorrentTaskParam)
	if !ok {
		return fmt.Errorf("task %d is not a torrent task", taskId)
	}

	if strings.HasPrefix(torrentTaskParam.url, "magnet:") {
		_, err := torrent.AddMagnet(torrentTaskParam.url, torrentTaskParam.title, torrentTaskParam.pkg)
		return err
	}
	_, err := torrent.AddTorrent(torrentTaskParam.url, torrentTaskParam.title, torrentTaskParam.pkg)
	return err
}
