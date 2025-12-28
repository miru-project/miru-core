package torrent

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/config"
	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/logger"
	"github.com/miru-project/miru-core/pkg/network"
	"github.com/miru-project/miru-core/pkg/result"
)

var (
	BTClient *torrent.Client
	Torrents = make(map[string]*torrent.Torrent)
	DataDir  string
)

func Init() {
	DataDir = config.Global.BTDataDir

	cc := torrent.NewDefaultClientConfig()
	cc.DataDir = DataDir
	cc.NoUpload = false
	client, e := torrent.NewClient(cc)
	BTClient = client
	if e != nil {
		logger.Fatal(e)
	}

	go syncProgress()
}

func syncProgress() {
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		for hex, t := range Torrents {
			if t.Info() == nil {
				continue
			}
			kbReceived := t.BytesCompleted() / 1024
			totalKb := t.Length() / 1024
			progress := int(kbReceived * 100 / totalKb)

			db.UpsertDownload(&ent.Download{
				Key:      hex,
				Progress: []int{progress},
				Status:   "Downloading",
			})
		}
	}
}

func TorrentStatus() torrent.ClientStats {
	return BTClient.Stats()
}

func AddMagnet(magnet string, title string, pkg string) (result.TorrentDetailResult, error) {
	t, err := BTClient.AddMagnet(magnet)
	if err != nil {
		return result.TorrentDetailResult{}, err
	}

	<-t.GotInfo()
	return addStream(t, title, pkg, magnet)
}

func AddTorrentBytes(body []byte, title string, pkg string) (result.TorrentDetailResult, error) {
	mediaInfo, err := metainfo.Load(bytes.NewReader(body))
	if err != nil {
		return result.TorrentDetailResult{}, err
	}

	t, err := BTClient.AddTorrent(mediaInfo)
	if err != nil {
		return result.TorrentDetailResult{}, err
	}

	return addStream(t, title, pkg, "")
}

func AddTorrent(link string, title string, pkg string) (result.TorrentDetailResult, error) {
	body, err := network.Request[[]byte](link, &network.RequestOptions{}, network.ReadAll)
	if err != nil {
		return result.TorrentDetailResult{}, err
	}

	mediaInfo, err := metainfo.Load(bytes.NewReader(body))
	if err != nil {
		return result.TorrentDetailResult{}, err
	}

	t, err := BTClient.AddTorrent(mediaInfo)
	if err != nil {
		return result.TorrentDetailResult{}, err
	}

	return addStream(t, title, pkg, link)
}

func addStream(t *torrent.Torrent, title string, pkg string, url string) (result.TorrentDetailResult, error) {
	hex := t.InfoHash().HexString()

	Torrents[hex] = t

	files := []string{}
	if len(t.Info().Files) == 0 {
		files = append(files, t.Name())
	} else {
		for _, file := range t.Info().Files {
			files = append(files, file.DisplayPath(t.Info()))
		}
	}

	db.UpsertDownload(&ent.Download{
		URL:       []string{url},
		Package:   pkg,
		Key:       hex,
		Title:     title,
		MediaType: "torrent",
		Status:    "Downloading",
	})

	return result.TorrentDetailResult{
		InfoHash: hex,
		Detail:   t.Info(),
		Files:    files,
	}, nil
}

func DeleteTorrent(infoHash string, forceDeleteFiles bool) error {
	t, ok := Torrents[infoHash]
	if !ok {
		return fmt.Errorf("torrent not found")
	}
	t.Drop()

	deleteFiles := forceDeleteFiles
	if !deleteFiles {
		// Check if it's in the download library
		_, err := db.GetDownloadByKey(infoHash)
		if err != nil {
			// Not in library, safe to delete cache
			deleteFiles = true
		}
	}

	if deleteFiles {
		// Auto delete cache file
		if err := os.RemoveAll(path.Join(DataDir, t.Name()+".part")); err != nil {
			return err
		}
	}

	delete(Torrents, infoHash)
	logger.Println("Torrent deleted: " + infoHash)
	return nil
}

func GetTorrentData(c *fiber.Ctx) error {
	params := c.AllParams()
	infoHash := params["infoHash"]
	filePath := params["*1"]
	t, ok := Torrents[infoHash]
	if !ok {
		return c.Status(http.StatusNotFound).SendString("torrent not found")
	}
	if filePath == "" {
		files := []string{}
		if len(t.Info().Files) == 0 {
			files = append(files, t.Name())
		} else {
			for _, file := range t.Info().Files {
				files = append(files, file.DisplayPath(t.Info()))
			}
		}
		return c.JSON(result.TorrentDetailResult{
			InfoHash: infoHash,
			Detail:   t.Info(),
			Files:    files,
		})
	}
	files := t.Files()
	unescape, err := url.PathUnescape(filePath)
	logger.Println(unescape)
	if err != nil {
		logger.Println(err.Error())
	}
	// 获取文件后缀
	fileExtension := path.Ext(unescape)
	if len(files) == 0 && unescape == t.Name() {
		return serverTorrentData(c, fileExtension, t.NewReader(), t.Length())
	}
	for _, file := range files {
		if file.DisplayPath() == unescape {
			return serverTorrentData(c, fileExtension, file.NewReader(), file.Length())
		}
	}
	return c.Status(http.StatusNotFound).SendString("file not found")
}

func serverTorrentData(c *fiber.Ctx, fileExtension string, reader torrent.Reader, fileSize int64) error {
	// 获取文件后缀
	mime, ok := isMedia(fileExtension)
	logger.Println(mime, fileExtension)
	if !ok {
		c.Set("Content-Type", "application/octet-stream")
		return c.SendStream(reader)
	}

	c.Set("Content-Type", mime)
	reader.SetResponsive()
	rangeHeader := c.Get("Range")
	if rangeHeader != "" {
		ranges := strings.Split(rangeHeader, "=")[1]
		rangeParts := strings.Split(ranges, "-")
		start, _ := strconv.ParseInt(rangeParts[0], 10, 64)
		end := fileSize - 1
		if rangeParts[1] != "" {
			end, _ = strconv.ParseInt(rangeParts[1], 10, 64)
		}

		c.Status(http.StatusPartialContent)
		c.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
		c.Set("Accept-Ranges", "bytes")
		c.Set("Content-Length", strconv.FormatInt(end-start+1, 10))

		logger.Printf("bytes %d-%d/%d", start, end, fileSize)

		_, err := reader.Seek(start, 0)
		if err != nil {
			return c.Status(http.StatusInternalServerError).SendString("Internal server error")
		}
		return c.SendStream(reader)
	}

	return c.SendStream(reader)
}
