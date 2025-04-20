package download

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"sync"
)

var tasks = sync.Map{}
var status = make(map[int]*Progrss)
var taskParamMap = make(map[int]TaskParamInterface)

type Progrss struct {
	Progrss            int       `json:"progress"`
	Names              *[]string `json:"names"`
	Total              int       `json:"total"`
	Status             Status    `json:"status"`
	MediaType          MediaType `json:"media_type"`
	CurrentDownloading string    `json:"current_downloading"`
	TaskID             int       `json:"task_id"`
}

type TaskParam struct {
	taskID *int
}

type TaskParamInterface interface {
	GetTaskID() *int
}

type MediaType string

const (
	Hls MediaType = "hls"
	Mp4 MediaType = "mp4"
)

type Status string

const (
	Downloading Status = "downloading"
	Paused      Status = "paused"
	Completed   Status = "completed"
	Failed      Status = "failed"
	Canceled    Status = "canceled"
)

func (t *TaskParam) GetTaskID() *int {
	return t.taskID
}

func DownloadStatus() map[int]*Progrss {
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

	// Cancel the task if it exists
	if cancelFunc, ok := tasks.Load(taskId); ok {
		cancelFunc.(context.CancelFunc)()
		tasks.Delete(taskId)
	}

	if _, ok := taskParamMap[taskId]; ok {
		status[taskId].Status = Canceled

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
			return nil
		case Mp4:
			// Remove single mp4 file
			if err := os.Remove(status[taskId].CurrentDownloading); err != nil {
				return fmt.Errorf("failed to remove file %s: %v", status[taskId].CurrentDownloading, err)
			}
		}

		return nil
	}

	return fmt.Errorf("task %d not found", taskId)
}

func PauseTask(taskId int) error {

	if cancelFunc, ok := tasks.Load(taskId); ok {
		cancelFunc.(context.CancelFunc)()
		tasks.Delete(taskId)
		status[taskId].Status = Paused

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

	}

	return fmt.Errorf("task %d not found", taskId)
}

// Start donwnload task and store it in the task map
func startDownloadTask[T TaskParamInterface](param T, taskFunc func(param T, ctx context.Context)) {

	ctx, cancel := context.WithCancel(context.Background())
	taskId := *param.GetTaskID()
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
