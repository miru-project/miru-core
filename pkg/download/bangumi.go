package download

import (
	"errors"
	"path"
	"strings"

	"github.com/miru-project/miru-core/pkg/logger"
	"github.com/miru-project/miru-core/pkg/network"
)

func Download(fileLoc string, url string, header map[string]string, mediaType string, title string, pkg string, key string, detailUrl string, watchUrl string) (MultipleLinkJson, error) {
	fileLoc = network.SanitizeFolderPath(fileLoc)
	mediaType = strings.ToLower(mediaType)

	// When the media type is not explicitly supplied, infer it from the REAL
	// upstream target. A proxy URL (http://.../proxy/name?__u=<b64>) carries no
	// usable extension in its path, so resolve it to the original target first.
	if mediaType == "" {
		mediaType = inferMediaTypeFromURL(url)
	}

	// Check if the URL is a valid HLS URL
	if mediaType == "hls" || isHlsUrl(url) {
		logger.Println("Downloading HLS : " + url)
		return downloadHls(fileLoc, url, header, title, pkg, key, detailUrl, watchUrl)
	}

	if mediaType == "torrent" || isTorrent(url) {
		logger.Println("Downloading Torrent : " + url)
		return downloadTorrent(fileLoc, url, header, mediaType, title, pkg, key, detailUrl, watchUrl)
	}

	if mediaType == "mp4" || isMp4Url(url) {
		logger.Println("Downloading MP4 : " + url)
		return downloadMp4(fileLoc, url, header, title, pkg, key, detailUrl, watchUrl)
	}

	return MultipleLinkJson{}, errors.New("Unsupported media type: " + mediaType)
}

// inferMediaTypeFromURL resolves a (possibly proxied) URL to its REAL target and
// returns the media type implied by its extension. Returns "" when it cannot be
// determined, so the caller can still fall back to the explicit mediaType or the
// "Unsupported media type" error.
func inferMediaTypeFromURL(rawURL string) string {
	target, _ := network.ResolveProxyTarget(rawURL)
	switch strings.ToLower(path.Ext(target)) {
	case ".m3u8":
		return "hls"
	case ".mp4", ".m4v", ".mov", ".webm":
		return "mp4"
	case ".torrent":
		return "torrent"
	}
	if strings.HasPrefix(target, "magnet:") {
		return "torrent"
	}
	return ""
}

func isHlsUrl(url string) bool {
	target, _ := network.ResolveProxyTarget(url)
	fileExt := path.Ext(target)
	return fileExt == ".m3u8"
}

func isTorrent(url string) bool {
	target, _ := network.ResolveProxyTarget(url)
	return path.Ext(target) == ".torrent" || strings.HasPrefix(target, "magnet:")
}

func isMp4Url(url string) bool {
	target, _ := network.ResolveProxyTarget(url)
	return path.Ext(target) == ".mp4"
}

// Request schema
type DownloadOptions struct {
	Header       map[string]string `json:"header"`
	Url          string            `json:"url"`
	DownloadPath string            `json:"download_path"`
	MediaType    string            `json:"media_type"`
}
