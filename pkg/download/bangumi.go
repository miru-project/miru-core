package download

import (
	"errors"
	"path"
	"strings"

	"github.com/miru-project/miru-core/pkg/logger"
)

func Download(filePath string, url string, header map[string]string, mediaType string, title string, pkg string, key string, detailUrl string, watchUrl string) (MultipleLinkJson, error) {

	mediaType = strings.ToLower(mediaType)
	// Check if the URL is a valid HLS URL
	if mediaType == "hls" || isHlsUrl(url) {
		logger.Println("Downloading HLS : " + url)
		return downloadHls(filePath, url, header, title, pkg, key, detailUrl, watchUrl)
	}

	if mediaType == "torrent" || isTorrent(url) {
		logger.Println("Downloading Torrent : " + url)
		return downloadTorrent(filePath, url, header, mediaType, title, pkg, key, detailUrl, watchUrl)
	}

	if mediaType == "mp4" || isMp4Url(url) {
		logger.Println("Downloading MP4 : " + url)
		return downloadMp4(filePath, url, header, title, pkg, key, detailUrl, watchUrl)
	}

	return MultipleLinkJson{}, errors.New("Unsupported media type: " + mediaType)
}

func isHlsUrl(url string) bool {
	fileExt := path.Ext(url)
	return fileExt == ".m3u8"
}

func isTorrent(url string) bool {
	return path.Ext(url) == ".torrent" || strings.HasPrefix(url, "magnet:")
}

func isMp4Url(url string) bool {
	return path.Ext(url) == ".mp4"
}

// Request schema
type DownloadOptions struct {
	Header       map[string]string `json:"header"`
	Url          string            `json:"url"`
	DownloadPath string            `json:"download_path"`
	MediaType    string            `json:"media_type"`
}
