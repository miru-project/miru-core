package download

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/anacrolix/torrent"
	"github.com/miru-project/miru-core/pkg/logger"
	miruTorrent "github.com/miru-project/miru-core/pkg/torrent"
)

// add torrent to torrent client -> start torrent download -> download torrent like mp4
func downloadTorrent(filePath string, url string, header map[string]string, mediaType string, title string, pkg string, key string) (MultipleLinkJson, error) {
	var t *torrent.Torrent
	var err error
	if strings.HasPrefix(url, "magnet:") {
		t, err = miruTorrent.FetchMagnet(url)
	} else {
		t, err = miruTorrent.FetchTorrent(url)
	}

	if err != nil {
		return MultipleLinkJson{}, err
	}

	// Find the largest file
	var targetFile *torrent.File
	var maxSize int64
	for _, f := range t.Files() {
		if f.Length() > maxSize {
			maxSize = f.Length()
			targetFile = f
		}
	}

	if targetFile == nil {
		return MultipleLinkJson{}, fmt.Errorf("no files in torrent")
	}

	// Prepare file path (filePath arg is treated as directory)
	fullPath := filepath.Join(filePath, targetFile.DisplayPath())

	taskId := genTaskID()
	status[taskId] = &Progress{
		Progrss:   0,
		Names:     &[]string{targetFile.DisplayPath()},
		Total:     int(maxSize),
		Status:    Downloading,
		MediaType: Torrent,
		TaskID:    taskId,
		Title:     title,
		Package:   pkg,
		Key:       t.InfoHash().HexString(),
		URL:       []string{url},
		SavePath:  fullPath,
	}
	status[taskId].syncDB()

	taskParamMap[taskId] = &TorrentTaskParam{
		TaskParam:  TaskParam{taskID: taskId},
		url:        url,
		title:      title,
		pkg:        pkg,
		filePath:   fullPath,
		targetFile: targetFile,
	}

	startDownloadTask(taskParamMap[taskId].(*TorrentTaskParam), downloadTorrentTask)
	return MultipleLinkJson{IsDownloading: true, TaskID: taskId}, nil
}

func downloadTorrentTask(param *TorrentTaskParam, ctx context.Context) {
	param.readAndSavePartial(ctx)
}

func (param *TorrentTaskParam) readAndSavePartial(ctx context.Context) {
	taskId := param.taskID
	reader := param.targetFile.NewReader()
	reader.SetResponsive()

	// Ensure directory exists and open file
	if err := os.MkdirAll(filepath.Dir(param.filePath), 0755); err != nil {
		logger.Println("Error creating directory:", err)
		status[taskId].Status = Failed
		status[taskId].syncDB()
		return
	}

	file, err := os.OpenFile(param.filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		logger.Println("Error opening file:", err)
		status[taskId].Status = Failed
		status[taskId].syncDB()
		return
	}
	defer file.Close()

	// Resume support
	stat, _ := file.Stat()
	currentSize := stat.Size()
	if currentSize > 0 {
		if _, err := reader.Seek(currentSize, io.SeekStart); err != nil {
			logger.Println("Error seeking torrent:", err)
			status[taskId].Status = Failed
			status[taskId].syncDB()
			return
		}
		status[taskId].Progrss = int(currentSize)
		status[taskId].syncDB()
	}

	status[taskId].CurrentDownloading = param.filePath

	buf := make([]byte, 1024*1024) // 1MB buffer
	for {
		select {
		case <-ctx.Done():
			status[taskId].Status = Canceled
			status[taskId].syncDB()
			return
		default:
			n, err := reader.Read(buf)
			if n > 0 {
				if _, wErr := file.Write(buf[:n]); wErr != nil {
					logger.Println("Write error:", wErr)
					status[taskId].Status = Failed
					status[taskId].syncDB()
					return
				}
				status[taskId].Progrss += n
				status[taskId].syncDB()
			}
			if err == io.EOF {
				status[taskId].Status = Completed
				status[taskId].syncDB()
				// Clean up network temporary file if needed, but here we saved to user path
				// We might want to remove it from miruTorrent.Torrents to stop seeding?
				// Keeping it allows seeding.
				miruTorrent.DeleteTorrent(param.key, true)
				return
			}
			if err != nil {
				logger.Println("Read torrent error:", err)
				status[taskId].Status = Failed
				status[taskId].syncDB()
				return
			}
		}
	}
}

type TorrentTaskParam struct {
	TaskParam
	url        string
	title      string
	pkg        string
	infoHash   string
	key        string
	filePath   string
	targetFile *torrent.File
}
