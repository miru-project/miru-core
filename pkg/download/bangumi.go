package download

import (
	"path"
)

func DownloadBangumi(filePath string, url string, header map[string]string, isHLS bool, title string, pkg string, key string) (MultipleLinkJson, error) {

	// Check if the URL is a valid HLS URL
	if isHlsUrl(url) || isHLS {
		return downloadHls(filePath, url, header, title, pkg, key)
	}

	return downloadMp4(filePath, url, header, title, pkg, key)
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
