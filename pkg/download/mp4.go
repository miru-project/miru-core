package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"

	log "github.com/miru-project/miru-core/pkg/logger"

	"github.com/miru-project/miru-core/pkg/network"
)

func downloadMp4(filePath string, url string, header map[string]string, title string, pkg string, key string) (MultipleLinkJson, error) {

	// Create the file path
	fileName := filepath.Join(filePath, path.Base(url))

	taskId := genTaskID()

	taskParamMap[taskId] = &Mp4TaskParam{
		TaskParam:     TaskParam{taskID: &taskId},
		filePath:      fileName,
		header:        header,
		url:           url,
		startingPoint: 0,
		title:         title,
		pkg:           pkg,
		key:           key,
	}
	startDownloadTask(taskParamMap[taskId].(*Mp4TaskParam), downloadMp4Task)

	return MultipleLinkJson{IsDownloading: true, TaskID: taskId}, nil
}

func downloadMp4Task(param *Mp4TaskParam, ctx context.Context) {

	param.ctx = ctx
	if _, e := network.Request[[]byte](param.url, &network.RequestOptions{Headers: param.header, Method: "GET"}, param.readAndSavePartial); e != nil {
		log.Println("Error downloading mp4 file:", e)
		status[*param.taskID].Status = Failed

		return
	}

}
func (t *Mp4TaskParam) readAndSavePartial(res *http.Response) ([]byte, error) {

	var downloadedBytes int64 = t.startingPoint
	const bufferSize = 1024 * 1024 // 1MB
	buf := make([]byte, bufferSize)
	taskId := *t.taskID
	ctx := t.ctx

	totalBytes := res.ContentLength
	if rangeHeader := res.Header.Get("Content-Range"); rangeHeader != "" {
		var totalSize int64
		if _, err := fmt.Sscanf(rangeHeader, "bytes %d-%d/%d",
			new(int64), new(int64), &totalSize); err == nil {
			totalBytes = totalSize
		}
	}

	// Initialize the status
	status[taskId] = &Progress{
		Progrss:   int(t.startingPoint),
		Names:     &[]string{},
		Total:     int(totalBytes),
		Status:    Downloading,
		MediaType: Mp4,
		TaskID:    taskId,
		Title:     t.title,
		Package:   t.pkg,
		Key:       t.key,
		URL:       []string{t.url},
		SavePath:  t.filePath,
	}
	status[taskId].syncDB()

	status[taskId].CurrentDownloading = t.filePath

	var file *os.File
	var err error

	// Create new file at the first time
	if t.isResuming {
		file, err = os.OpenFile(t.filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	} else {
		file, err = os.Create(t.filePath)
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	for {
		select {
		case <-ctx.Done():
			status[taskId].Status = Canceled
			log.Printf("Mp4 download task %d canceled", taskId)
			return nil, nil
		default:
			n, err := res.Body.Read(buf)

			if n > 0 {
				_, writeErr := file.Write(buf[:n])
				if writeErr != nil {
					return nil, writeErr
				}
				downloadedBytes += int64(n)
				status[taskId].Progrss = int(downloadedBytes)
				status[taskId].syncDB()
				// log.Printf("\rDownloading... %d%% complete", 100*downloadedBytes/totalBytes)
			}

			if err == io.EOF {
				status[taskId].Status = Completed
				status[taskId].syncDB()
				return nil, nil
			}

			if err != nil {
				status[taskId].Status = Failed
				status[taskId].syncDB()
				return nil, err
			}

		}

	}

}

func resumeMp4Task(taskId int) error {

	taskParam := taskParamMap[taskId]
	if taskParam == nil {
		return fmt.Errorf("task %d not found", taskId)
	}

	mp4TaskParam, ok := taskParam.(*Mp4TaskParam)
	if !ok {
		return fmt.Errorf("task %d is not a mp4 task", taskId)
	}

	completed := status[taskId].Progrss

	if mp4TaskParam.header == nil {
		mp4TaskParam.header = make(map[string]string)
	}

	mp4TaskParam.header["Range"] = fmt.Sprintf("bytes=%d-", completed)
	mp4TaskParam.startingPoint = int64(completed)
	mp4TaskParam.isResuming = true

	startDownloadTask(mp4TaskParam, downloadMp4Task)

	return nil
}

type Mp4TaskParam struct {
	TaskParam
	filePath      string
	header        map[string]string
	url           string
	startingPoint int64
	ctx           context.Context
	isResuming    bool
	title         string
	pkg           string
	key           string
}
