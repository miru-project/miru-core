package download

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	log "github.com/miru-project/miru-core/pkg/logger"
	"github.com/valyala/fasthttp"

	"github.com/miru-project/miru-core/pkg/network"
)

func downloadMp4(filePath string, url string, header map[string]string, title string, pkg string, key string, detailUrl string, watchUrl string, category Category) (MultipleLinkJson, error) {

	// Derive the saved file name from the REAL upstream target, not from the
	// proxy path placeholder / query string. When url is a miru-core proxy URL
	// (e.g. http://127.0.0.1:3000/proxy/file-16-f2-v1-a1.xls?__u=<b64>...), the
	// original file name lives in the base64url __u param, so we resolve it and
	// use path.Base of that. This fixes the long/garbled file name bug where
	// path.Base(proxyURL) returned the whole query string glued onto the
	// placeholder name.
	fileName := filepath.Join(filePath, network.ProxyURLTargetName(url))

	taskId := genTaskID()
	p := &Progress{
		Progrss:   0,
		Names:     &[]string{network.ProxyURLTargetName(url)},
		Total:     0,
		Status:    Downloading,
		MediaType: Mp4,
		Category: category,
		TaskID:    taskId,
		Title:     title,
		Package:   pkg,
		Key:       key,
		URL:       []string{url},
		SavePath:  fileName,
		DetailUrl: detailUrl,
		WatchUrl:  watchUrl,
	}
	statusMap.Store(taskId, p)
	p.SyncDB()

	mp4Param := &Mp4TaskParam{
		TaskParam:     TaskParam{taskID: taskId},
		filePath:      fileName,
		header:        header,
		url:           url,
		startingPoint: 0,
		title:         title,
		pkg:           pkg,
		key:           key,
	}
	taskParams.Store(taskId, mp4Param)
	startDownloadTask(mp4Param, downloadMp4Task)

	return MultipleLinkJson{IsDownloading: true, TaskID: taskId}, nil
}

func downloadMp4Task(param *Mp4TaskParam, ctx context.Context) {

	param.ctx = ctx
	if _, e := network.Request[[]byte](param.url, &network.RequestOptions{Headers: param.header, Method: "GET"}, param.readAndSavePartial); e != nil {
		log.Println("Error downloading mp4 file:", e)
		statusMap.Store(param.taskID, &Progress{
			TaskID: param.taskID,
			Status: Failed,
		})
		return
	}

}
func (t *Mp4TaskParam) readAndSavePartial(res *fasthttp.Response) ([]byte, error) {

	var downloadedBytes int64 = t.startingPoint
	const bufferSize = 1024 * 1024 // 1MB
	buf := make([]byte, bufferSize)
	taskId := t.taskID
	ctx := t.ctx

	totalBytes := int64(res.Header.ContentLength())
	if rangeHeader := string(res.Header.Peek("Content-Range")); rangeHeader != "" {
		var totalSize int64
		if _, err := fmt.Sscanf(rangeHeader, "bytes %d-%d/%d",
			new(int64), new(int64), &totalSize); err == nil {
			totalBytes = totalSize
		}
	}

	// Update the status
	v, _ := statusMap.Load(taskId)
	p := v.(*Progress)
	p.Progrss = int(t.startingPoint)
	p.Total = int(totalBytes)
	p.Status = Downloading
	p.SyncDB()

	p.CurrentDownloading = t.filePath

	var file *os.File
	var err error

	// Create new file at the first time
	if t.isResuming {
		file, err = os.OpenFile(t.filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	} else {
		file, err = network.TouchFile(network.SanitizeFilename(t.filePath))
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Get decompressed reader to handle gzip, deflate, brotli, zstd
	bodyReader, err := network.GetDecompressedReader(res)
	if err != nil {
		return nil, err
	}
	defer res.CloseBodyStream()

	for {
		select {
		case <-ctx.Done():
			p.Status = Canceled
			p.SyncDB()
			log.Printf("Mp4 download task %d canceled", taskId)
			return nil, nil
		default:
			n, err := bodyReader.Read(buf)
			// logger.Println(t.title, "Downloading", n, "bytes", downloadedBytes, "of", totalBytes)
			if n > 0 {
				_, writeErr := file.Write(buf[:n])
				if writeErr != nil {
					return nil, writeErr
				}
				downloadedBytes += int64(n)
				p.Progrss = int(downloadedBytes)
				p.SyncDB()
				// log.Printf("\rDownloading... %d%% complete", 100*downloadedBytes/totalBytes)
			}

			if err == io.EOF {
				p.Status = Converting
				p.SyncDB()
				return nil, nil
			}

			if err != nil {
				p.Status = Failed
				p.SyncDB()
				return nil, err
			}

		}

	}

}

func resumeMp4Task(taskId int) error {

	tp, _ := taskParams.Load(taskId)
	if tp == nil {
		return fmt.Errorf("task %d not found", taskId)
	}

	mp4TaskParam, ok := tp.(*Mp4TaskParam)
	if !ok {
		return fmt.Errorf("task %d is not a mp4 task", taskId)
	}

	sv, _ := statusMap.Load(taskId)
	completed := sv.(*Progress).Progrss

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
