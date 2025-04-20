package download

import (
	"path"
)

func DownloadBangumi(filePath string, url string, header map[string]string, isHLS bool) (MultipleLinkJson, error) {

	// Check if the URL is a valid HLS URL
	if isHlsUrl(url) || isHLS {
		return downloadHls(filePath, url, header)
	}

	return downloadMp4(filePath, url, header)
}

func isHlsUrl(url string) bool {

	fileExt := path.Ext(url)

	return fileExt == ".m3u8"
}

// Request schema
type DownloadOptions struct {
	Header       map[string]string `json:"header"`
	Url          string            `json:"url"`
	DownloadPath string            `json:"download_path"`
	IsHls        bool              `json:"is_hls"`
}
