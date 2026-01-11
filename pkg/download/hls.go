package download

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"path"
	"path/filepath"

	log "github.com/miru-project/miru-core/pkg/logger"

	"github.com/grafov/m3u8"
	"github.com/miru-project/miru-core/pkg/network"
)

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
func downloadHls(filePath string, url string, headers map[string]string, title string, pkg string, key string) (MultipleLinkJson, error) {

	// Get hls content from url
	res, e := network.Request[string](url, &network.RequestOptions{Headers: headers, Method: "GET"}, network.ReadAll)
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
	// Filter out nil segments
	playList.Segments = filterSegments(playList.Segments)
	// Generate random task id
	taskId := genTaskID()
	// Initialize the status
	status[taskId] = &Progress{
		Progrss:   0,
		Names:     &[]string{},
		Total:     len(playList.Segments),
		Status:    Downloading,
		MediaType: Hls,
		TaskID:    taskId,
		Title:     title,
		Package:   pkg,
		Key:       key,
		URL:       []string{url},
		SavePath:  filePath,
	}
	status[taskId].SyncDB()

	fetchedKey := downloadKey(playList.Key, url, headers)
	iv := getIV(playList.Key, playList.SeqNo)

	taskParamMap[taskId] = &HlsTaskParam{
		TaskParam:   TaskParam{taskID: taskId},
		playList:    playList,
		filePath:    filePath,
		headers:     headers,
		playListUrl: url,
		Key:         &fetchedKey,
		IV:          &iv,
	}
	startDownloadTask((taskParamMap[taskId]).(*HlsTaskParam), downloadSegment)

	return MultipleLinkJson{IsDownloading: true, TaskID: taskId}, nil

}
func filterSegments(segments []*m3u8.MediaSegment) []*m3u8.MediaSegment {
	lis := make([]*m3u8.MediaSegment, 0)

	for _, s := range segments {
		if s == nil {
			continue
		}
		lis = append(lis, s)
	}

	return lis
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

func avaliableVarient(variants []*m3u8.Variant, prevUrl string) []*AvailableHlsVariant {

	lis := make([]*AvailableHlsVariant, 0)

	for _, v := range variants {

		if v == nil {
			continue
		}

		lis = append(lis, &AvailableHlsVariant{
			Resolution: v.Resolution,
			Url:        parsePath(prevUrl, v.URI),
			Codecs:     v.Codecs,
		})
	}

	return lis
}

func downloadKey(key *m3u8.Key, playListUrl string, headers map[string]string) []byte {
	if key == nil {
		return nil
	}
	// Download the key
	url := parsePath(playListUrl, key.URI)
	res, e := network.Request[[]byte](url, &network.RequestOptions{Headers: headers, Method: "GET"}, network.ReadAll)
	if e != nil {
		log.Println("Error downloading key:", e)
		return nil
	}

	return res
}

// Download hls segment inside go routine
func downloadSegment(param *HlsTaskParam, ctx context.Context) {

	key := param.playList.Key
	seg := param.playList.Segments
	taskId := param.taskID
	completed := status[taskId].Progrss

	for i, s := range seg {

		select {
		case <-ctx.Done():
			log.Printf("HLS download task %d canceled", taskId)
			return
		default:
			// Define the file name
			name := fmt.Sprintf("%d%s", i+completed, path.Ext(s.URI))
			fileName := filepath.Join(param.filePath, name)
			status[taskId].CurrentDownloading = fileName

			// Download the segment
			url := parsePath(param.playListUrl, s.URI)
			res, e := network.Request[[]byte](url, &network.RequestOptions{Headers: param.headers, Method: "GET"}, network.ReadAll)
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
			err := network.SaveFile(fileName, &res)
			if err != nil {
				log.Println("Error saving segment:", err)
				continue
			}

			// Update status

			status[taskId].Progrss++
			*status[taskId].Names = append(*status[taskId].Names, fileName)
			status[taskId].SyncDB()
			log.Println("Downloaded segment:", url)
		}

	}

	status[taskId].Status = Completed
	status[taskId].SyncDB()
}

func resumeHlsTask(taskId int) error {

	taskParam := taskParamMap[taskId]
	if taskParam == nil {
		return fmt.Errorf("task %d not found", taskId)
	}

	// Check if the task is a hls task
	hlsTaskParam, ok := taskParam.(*HlsTaskParam)
	if !ok {
		return fmt.Errorf("task %d is not a hls task", taskId)
	}

	completed := status[taskId].Progrss
	seg := hlsTaskParam.playList.Segments

	hlsTaskParam.playList.Segments = seg[completed:]
	startDownloadTask(hlsTaskParam, downloadSegment)
	return nil
}

// Summary of available variant
type AvailableHlsVariant struct {
	Resolution string `json:"resolution"`
	Url        string `json:"url"`
	Codecs     string `json:"codec"`
}

type HlsTaskParam struct {
	TaskParam
	playList    *m3u8.MediaPlaylist
	filePath    string
	headers     map[string]string
	playListUrl string
	Key         *[]byte
	IV          *[]byte
}

// A Multiple response Json for hls that can be used on master playlist and media playlist
type MultipleLinkJson struct {
	Header         map[string]string      `json:"header"`
	IsDownloading  bool                   `json:"is_downloading"`
	Key            m3u8.Key               `json:"key"`
	TaskID         int                    `json:"task_id"`
	VariantSummary []*AvailableHlsVariant `json:"variant_summary"`
	Variant        []*m3u8.Variant        `json:"variant"`
}
