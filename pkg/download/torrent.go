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
func downloadTorrent(filePath string, url string, header map[string]string, mediaType string, title string, pkg string, key string, detailUrl string, watchUrl string) (MultipleLinkJson, error) {
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
	p := &Progress{
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
		DetailUrl: detailUrl,
		WatchUrl:  watchUrl,
	}
	statusMap.Store(taskId, p)
	p.SyncDB()

	torrentParam := &TorrentTaskParam{
		TaskParam:  TaskParam{taskID: taskId},
		url:        url,
		title:      title,
		pkg:        pkg,
		filePath:   fullPath,
		targetFile: targetFile,
		key:        key,
	}
	taskParams.Store(taskId, torrentParam)

	// Honour the concurrency cap / priority queue when first starting.
	enqueueOrStart(taskId)
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
		v, _ := statusMap.Load(taskId)
		p := v.(*Progress)
		p.Status = Failed
		p.SyncDB()
		return
	}

	file, err := os.OpenFile(param.filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		logger.Println("Error opening file:", err)
		v, _ := statusMap.Load(taskId)
		p := v.(*Progress)
		p.Status = Failed
		p.SyncDB()
		return
	}
	defer file.Close()

	// Resume support
	stat, _ := file.Stat()
	currentSize := stat.Size()
	if currentSize > 0 {
		if _, err := reader.Seek(currentSize, io.SeekStart); err != nil {
			logger.Println("Error seeking torrent:", err)
			v, _ := statusMap.Load(taskId)
			p := v.(*Progress)
			p.Status = Failed
			p.SyncDB()
			return
		}
		v, _ := statusMap.Load(taskId)
		p := v.(*Progress)
		p.Progrss = int(currentSize)
		p.SyncDB()
	}

	v, _ := statusMap.Load(taskId)
	p := v.(*Progress)
	p.CurrentDownloading = param.filePath

	buf := make([]byte, 1024*1024) // 1MB buffer
	for {
		select {
		case <-ctx.Done():
			p.Status = Canceled
			p.SyncDB()
			return
		default:
			n, err := reader.Read(buf)
			if n > 0 {
				if _, wErr := file.Write(buf[:n]); wErr != nil {
					logger.Println("Write error:", wErr)
					p.Status = Failed
					p.SyncDB()
					return
				}
				p.Progrss += n
				p.SyncDB()
			}
			if err == io.EOF {
				p.Status = Completed
				p.SyncDB()
				miruTorrent.DeleteTorrent(param.key, true)
				return
			}
			if err != nil {
				logger.Println("Read torrent error:", err)
				p.Status = Failed
				p.SyncDB()
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

func resumeTorrentTask(taskId int) error {
	tp, _ := taskParams.Load(taskId)
	if tp == nil {
		return fmt.Errorf("task %d not found", taskId)
	}

	torrentTaskParam, ok := tp.(*TorrentTaskParam)
	if !ok {
		return fmt.Errorf("task %d is not a torrent task", taskId)
	}

	if strings.HasPrefix(torrentTaskParam.url, "magnet:") {
		_, err := miruTorrent.AddMagnet(torrentTaskParam.url, torrentTaskParam.title, torrentTaskParam.pkg)
		return err
	}
	_, err := miruTorrent.AddTorrent(torrentTaskParam.url, torrentTaskParam.title, torrentTaskParam.pkg)
	return err
}
