package download

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"path"
	"path/filepath"

	"github.com/grafov/m3u8"
	"github.com/miru-project/miru-core/pkg/network"
)

func downloadHls(filePath string, url string, header map[string]string) (MultipleLinkJson, error) {

	// Get hls content from url
	res, e := network.Request[string](url, &network.RequestOptions{Headers: header, Method: "GET"})
	if e != nil {
		return MultipleLinkJson{}, e
	}
	o := bytes.NewBufferString(res)

	// Decode the m3u8 file
	pl, li, e := m3u8.Decode(*o, true)
	log.Println("Decode m3u8 file:", url)

	if e != nil {
		return MultipleLinkJson{}, e
	}

	// Handle master playlist
	if li == m3u8.MASTER {

		playList := pl.(*m3u8.MasterPlaylist)

		return MultipleLinkJson{
			VariantSummary: avaliableVarient(playList.Variants, url),
			Variant:        playList.Variants,
		}, nil

	}

	// Handle media playlist
	playList := pl.(*m3u8.MediaPlaylist)

	taskId := genTaskID()
	// Initialize the status
	Status[taskId] = &Progrss{
		Progrss: 0,
		Names:   &[]string{},
		Total:   len(playList.Segments),
		Status:  "Downloading",
	}

	fetchedKey := downloadKey(playList.Key, url)
	iv := getIV(playList.Key, playList.SeqNo)

	startDownloadTask(&TaskParam{
		taskID:      taskId,
		playList:    playList,
		filePath:    filePath,
		header:      header,
		playListUrl: url,
		Key:         &fetchedKey,
		IV:          &iv,
	}, downloadSegment)

	return MultipleLinkJson{IsDownloading: true, TaskID: taskId}, nil

}

func getIV(keyMeta *m3u8.Key, seqNo uint64) []byte {
	if keyMeta != nil && len(keyMeta.IV) == 16 {
		return []byte(keyMeta.IV)
	}
	iv := make([]byte, 16)
	iv[8] = byte(seqNo >> 56)
	iv[9] = byte(seqNo >> 48)
	iv[10] = byte(seqNo >> 40)
	iv[11] = byte(seqNo >> 32)
	iv[12] = byte(seqNo >> 24)
	iv[13] = byte(seqNo >> 16)
	iv[14] = byte(seqNo >> 8)
	iv[15] = byte(seqNo)
	return iv
}

func avaliableVarient(variants []*m3u8.Variant, prevUrl string) []*AvailableVariant {

	lis := make([]*AvailableVariant, 0)

	for _, v := range variants {

		if v == nil {
			continue
		}

		lis = append(lis, &AvailableVariant{
			Resolution: v.Resolution,
			Url:        parsePath(prevUrl, v.URI),
			Codecs:     v.Codecs,
		})
	}

	return lis
}

// Generate a unique task ID
func genTaskID() int {

	for {
		id := rand.Intn(1000000)
		if Status[id] == nil {
			return id
		}
	}
}

// Start donwnload task and store it in the task map
func startDownloadTask(param *TaskParam, taskFunc func(param *TaskParam)) {

	_, cancel := context.WithCancel(context.Background())
	Tasks.Store(param.taskID, cancel)

	// Start the task in a goroutine
	go func() {
		defer Tasks.Delete(param.taskID)
		taskFunc(param)
	}()
}
func downloadKey(key *m3u8.Key, playListUrl string) []byte {
	if key == nil {
		return nil
	}
	// Download the key
	url := parsePath(playListUrl, key.URI)
	res, e := network.Request[[]byte](url, &network.RequestOptions{Headers: nil, Method: "GET"})
	if e != nil {
		log.Println("Error downloading key:", e)
		return nil
	}

	return res
}

// Download hls segment inside go routine
func downloadSegment(param *TaskParam) {

	key := param.playList.Key
	seg := param.playList.Segments

	for i, s := range seg {
		// Download the segment
		url := parsePath(param.playListUrl, s.URI)
		res, e := network.Request[[]byte](url, &network.RequestOptions{Headers: param.header, Method: "GET"})
		if e != nil {
			log.Println("Error downloading segment:", e)
			continue
		}

		// Decypt segment if needed
		if key != nil {

			res, e = hlsDecrypt(res, *param.Key, *param.IV)
			if e != nil {
				log.Println("Error decrypting segment:", e)
				continue
			}

		}

		// Save the segment to file
		name := fmt.Sprintf("%d%s", i, path.Ext(s.URI))
		fileName := filepath.Join(param.filePath, name)
		err := network.SaveFile(fileName, &res)
		if err != nil {
			log.Println("Error saving segment:", err)
			continue
		}

		// Update status
		Status[param.taskID].Progrss++
		*Status[param.taskID].Names = append(*Status[param.taskID].Names, fileName)
		log.Println("Downloaded segment:", url)
	}

	Status[param.taskID].Status = "Completed"
}

func DownloadBangumi(filePath string, url string, header map[string]string, isHLS bool) (MultipleLinkJson, error) {

	// Check if the URL is a valid HLS URL
	if isHlsUrl(url) || isHLS {
		return downloadHls(filePath, url, header)
	}

	downloadMp4()
	return MultipleLinkJson{}, nil
}

func isHlsUrl(url string) bool {

	fileExt := path.Ext(url)

	return fileExt == ".m3u8"
}

func hlsDecrypt(enc []byte, key []byte, iv []byte) ([]byte, error) {

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(enc)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of the block size")
	}
	mode := cipher.NewCBCDecrypter(block, []byte(iv))
	decrypted := make([]byte, len(enc))
	mode.CryptBlocks(decrypted, enc)
	return decrypted, nil
}

// Check if the path is absolute or relative. For path in hls playlist, it can be either
// absolute or relative. If it is relative, join it with the previous path
func parsePath(prevPath string, fileName string) string {

	// Get the current working directory and join it with the file name
	link, _ := url.Parse(prevPath)
	// Check if the path is absolute
	if link.IsAbs() {
		return fileName
	}

	// relativePath := path.Join(url.Path, fileName)
	dir := filepath.Dir(link.Path)
	link.Path = filepath.Join(dir, fileName)
	return link.String()

}

func downloadMp4() {

}

// Response schema
type MultipleLinkJson struct {
	Header         map[string]string   `json:"header"`
	IsDownloading  bool                `json:"is_downloading"`
	Key            m3u8.Key            `json:"key"`
	TaskID         int                 `json:"task_id"`
	VariantSummary []*AvailableVariant `json:"variant_summary"`
	Variant        []*m3u8.Variant     `json:"variant"`
}

// Request schema
type DownloadOptions struct {
	Header       map[string]string `json:"header"`
	Url          string            `json:"url"`
	DownloadPath string            `json:"download_path"`
	IsHls        bool              `json:"is_hls"`
}

type AvailableVariant struct {
	Resolution string `json:"resolution"`
	Url        string `json:"url"`
	Codecs     string `json:"codec"`
}

type TaskParam struct {
	taskID      int
	playList    *m3u8.MediaPlaylist
	filePath    string
	header      map[string]string
	playListUrl string
	Key         *[]byte
	IV          *[]byte
}
