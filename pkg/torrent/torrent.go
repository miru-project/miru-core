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

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/config"
	"github.com/miru-project/miru-core/pkg/logger"
	"github.com/miru-project/miru-core/pkg/network"
	"github.com/miru-project/miru-core/pkg/result"
)

var (
	BTClient *torrent.Client
	Torrents = make(map[string]*torrent.Torrent)
	DataDir  string
)

//	type TorrentDB struct {
//		storageDir string
//	}
//
//	func (db TorrentDB) OpenTorrent(ctx context.Context, info *metainfo.Info, infoHash metainfo.Hash) (storage.TorrentImpl, error) {
//		return nil, nil
//	}
func Init() {
	DataDir = config.Global.BTDataDir

	cc := torrent.NewDefaultClientConfig()
	cc.DataDir = DataDir
	cc.NoUpload = false
	// cc.DefaultStorage = TorrentDB{storageDir: DataDir}
	client, e := torrent.NewClient(cc)
	BTClient = client
	if e != nil {
		logger.Fatal(e)
	}
}

func TorrentStatus() torrent.ClientStats {
	return BTClient.Stats()
}

func AddMagnet(magnet string) (result.TorrentDetailResult, error) {
	t, err := BTClient.AddMagnet(magnet)
	if err != nil {
		return result.TorrentDetailResult{}, err
	}

	<-t.GotInfo()
	return addStream(t)
}

func AddTorrentBytes(body []byte) (result.TorrentDetailResult, error) {
	mediaInfo, err := metainfo.Load(bytes.NewReader(body))
	if err != nil {
		return result.TorrentDetailResult{}, err
	}

	t, err := BTClient.AddTorrent(mediaInfo)
	if err != nil {
		return result.TorrentDetailResult{}, err
	}

	return addStream(t)
}

func AddTorrent(link string) (result.TorrentDetailResult, error) {
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

	return addStream(t)
}

func addStream(t *torrent.Torrent) (result.TorrentDetailResult, error) {
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

	return result.TorrentDetailResult{
		InfoHash: hex,
		Detail:   t.Info(),
		Files:    files,
	}, nil
}

func DeleteTorrent(infoHash string) error {
	t, ok := Torrents[infoHash]
	if !ok {
		return fmt.Errorf("torrent not found")
	}
	t.Drop()

	// Auto delete cache file
	if err := os.RemoveAll(path.Join(DataDir, t.Name())); err != nil {
		return err
	}

	delete(Torrents, infoHash)
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
