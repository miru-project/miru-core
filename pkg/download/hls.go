package download

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

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

// fetchPlaylistText downloads an m3u8 playlist through the validated fetcher
// (status/empty checks + retries), so an HTML error page or expired signed
// URL surfaces as a clear fetch error instead of reaching the decoder.
func fetchPlaylistText(url string, headers map[string]string) (string, error) {
	body, e := fetchHlsResource(url, headers)
	if e != nil {
		return "", fmt.Errorf("playlist fetch failed: %w", e)
	}
	return string(body), nil
}

// decodePlaylist parses m3u8 text, wrapping decoder errors with the source
// URL and a leading body snippet so "#EXTM3U absent" style failures are
// diagnosable from the toast alone.
func decodePlaylist(text string, sourceUrl string) (m3u8.Playlist, m3u8.ListType, error) {
	o := bytes.NewBufferString(text)
	pl, li, e := m3u8.Decode(*o, true)
	if e != nil {
		snippet := strings.TrimSpace(text)
		if len(snippet) > 80 {
			snippet = snippet[:80]
		}
		return nil, 0, fmt.Errorf(
			"%s is not a valid m3u8 playlist: %w (body starts with %q)",
			sourceUrl, e, snippet)
	}
	return pl, li, nil
}

func downloadHls(filePath string, url string, headers map[string]string, title string, pkg string, key string, detailUrl string, watchUrl string, category Category) (MultipleLinkJson, error) {

	// Get hls content from url
	res, e := fetchPlaylistText(url, headers)
	if e != nil {
		return MultipleLinkJson{}, e
	}

	// Decode the m3u8 file
	log.Println("Decode m3u8 file:", url)
	pl, li, e := decodePlaylist(res, url)
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
	p := &Progress{
		Progrss:   0,
		Names:     &[]string{},
		Total:     len(playList.Segments),
		Status:    Downloading,
		MediaType: Hls,
		Category:  category,
		TaskID:    taskId,
		Title:     title,
		Package:   pkg,
		Key:       key,
		URL:       []string{url},
		SavePath:  filePath,
		Headers:   headers,
		DetailUrl: detailUrl,
		WatchUrl:  watchUrl,
	}
	statusMap.Store(taskId, p)
	p.SyncDB()

	hlsParam := &HlsTaskParam{
		TaskParam:   TaskParam{taskID: taskId},
		playList:    playList,
		filePath:    filePath,
		headers:     headers,
		playListUrl: url,
		keyResolver: newHlsKeyResolver(url, headers),
	}
	taskParams.Store(taskId, hlsParam)
	startDownloadTask(hlsParam, downloadSegment)

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
	// EXT-X-KEY IV is a hex literal ("0x...") per the HLS spec; decode it
	// properly instead of taking the ASCII bytes of the string.
	if keyMeta != nil && keyMeta.IV != "" {
		raw := strings.TrimPrefix(strings.TrimPrefix(keyMeta.IV, "0x"), "0X")
		if len(raw) < 32 {
			raw = strings.Repeat("0", 32-len(raw)) + raw
		}
		if decoded, err := hex.DecodeString(raw); err == nil && len(decoded) == 16 {
			return decoded
		}
	}
	// Default IV is the media sequence number as a big-endian 128-bit value.
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

// fetchHlsResource downloads one HLS resource (media segment or key) and
// validates the response. network.Request only reports transport errors, so
// without the status/size checks a 403/404 error page would be saved as a
// "good" segment and corrupt the eventual FFmpeg merge. Transient failures
// (network errors, 5xx, 408/429, empty bodies) are retried with exponential
// backoff; permanent 4xx fail immediately.
func fetchHlsResource(url string, headers map[string]string) ([]byte, error) {
	const attempts = 3
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			time.Sleep(time.Duration(1<<(attempt-2)) * 500 * time.Millisecond)
		}
		res, e := network.Request[[]byte](url,
			&network.RequestOptions{Headers: headers, Method: "GET"}, network.ReadAll)
		if e != nil {
			lastErr = e
			continue
		}
		if res.StatusCode >= 400 {
			lastErr = fmt.Errorf("HTTP status %d", res.StatusCode)
			if res.StatusCode < 500 && res.StatusCode != 408 && res.StatusCode != 429 {
				return nil, fmt.Errorf("fetch %s: %w", url, lastErr)
			}
			continue
		}
		if len(res.Body) == 0 {
			lastErr = errors.New("empty response body")
			continue
		}
		return res.Body, nil
	}
	return nil, fmt.Errorf("fetch %s failed after %d attempts: %w", url, attempts, lastErr)
}

// hlsKeyResolver fetches and caches AES-128 keys by URI. Playlists may
// rotate keys mid-stream, so resolution happens per segment instead of once
// per task.
type hlsKeyResolver struct {
	playListUrl string
	headers     map[string]string
	cache       map[string][]byte
}

func newHlsKeyResolver(playListUrl string, headers map[string]string) *hlsKeyResolver {
	return &hlsKeyResolver{
		playListUrl: playListUrl,
		headers:     headers,
		cache:       make(map[string][]byte),
	}
}

// resolve returns the raw key bytes for the given EXT-X-KEY metadata, or
// nil when the segment is unencrypted.
func (r *hlsKeyResolver) resolve(meta *m3u8.Key) ([]byte, error) {
	if meta == nil || meta.Method == "" || meta.Method == "NONE" {
		return nil, nil
	}
	if meta.Method != "AES-128" {
		return nil, fmt.Errorf("unsupported HLS encryption method %q", meta.Method)
	}
	if k, ok := r.cache[meta.URI]; ok {
		return k, nil
	}
	k, e := fetchHlsResource(parsePath(r.playListUrl, meta.URI), r.headers)
	if e != nil {
		return nil, fmt.Errorf("key download failed: %w", e)
	}
	if len(k) != 16 {
		return nil, fmt.Errorf("invalid AES-128 key size %d for %s", len(k), meta.URI)
	}
	r.cache[meta.URI] = k
	return k, nil
}

// Download hls segment inside go routine
func downloadSegment(param *HlsTaskParam, ctx context.Context) {

	seg := param.playList.Segments
	taskId := param.taskID

	v, _ := statusMap.Load(taskId)
	if v == nil {
		log.Printf("HLS download task %d: status missing, aborting", taskId)
		return
	}
	p := v.(*Progress)
	// Ensure Names slice exists — DB may not have persisted it.
	if p.Names == nil {
		p.Names = &[]string{}
	}
	completed := p.Progrss

	if param.keyResolver == nil {
		param.keyResolver = newHlsKeyResolver(param.playListUrl, param.headers)
	}
	resolver := param.keyResolver

	// EXT-X-KEY applies to every following segment until the next tag, and
	// the parser only attaches it to the segment right after the tag, so
	// carry the last seen key forward (seeded on resume via inheritedKey).
	currentKey := param.inheritedKey

	failTask := func(msg string) {
		log.Printf("HLS task %d failed: %s", taskId, msg)
		p.Status = Failed
		p.Error = msg
		p.SyncDB()
	}

	for i, s := range seg {

		select {
		case <-ctx.Done():
			log.Printf("HLS download task %d canceled", taskId)
			return
		default:
		}

		if s.Key != nil {
			currentKey = s.Key
		}

		// Define the file name. Resolve the REAL segment target (a proxy URL
		// carries the original .ts/.m4s extension in its __u param) so the
		// written chunk keeps the correct container extension instead of a
		// garbled proxy placeholder like .ts?__u=....
		name := fmt.Sprintf("%d%s", i+completed, path.Ext(network.ProxyURLTargetName(s.URI)))
		fileName := filepath.Join(param.filePath, network.SanitizeFilename(name))
		p.CurrentDownloading = fileName

		// Download the segment
		url := parsePath(param.playListUrl, s.URI)
		body, e := fetchHlsResource(url, param.headers)
		if e != nil {
			failTask(fmt.Sprintf("segment %d: %v", i+completed, e))
			return
		}

		// Decrypt segment if needed
		keyBytes, e := resolver.resolve(currentKey)
		if e != nil {
			failTask(fmt.Sprintf("segment %d: %v", i+completed, e))
			return
		}
		if keyBytes != nil {
			body, e = hlsDecrypt(body, keyBytes, getIV(currentKey, s.SeqId))
			if e != nil {
				failTask(fmt.Sprintf("segment %d: decrypt failed: %v", i+completed, e))
				return
			}
		}

		// Save the segment to file
		if err := network.SaveFile(fileName, &body); err != nil {
			failTask(fmt.Sprintf("segment %d: save failed: %v", i+completed, err))
			return
		}

		// Update status
		p.Progrss++
		*p.Names = append(*p.Names, fileName)
		p.SyncDB()
		log.Println("Downloaded segment:", url, "to", fileName)
	}

	p.Status = Converting
	p.Error = ""
	p.SyncDB()
}

func resumeHlsTask(taskId int) error {

	tp, _ := taskParams.Load(taskId)
	if tp == nil {
		return fmt.Errorf("task %d not found", taskId)
	}

	// Check if the task is a hls task
	hlsTaskParam, ok := tp.(*HlsTaskParam)
	if !ok {
		return fmt.Errorf("task %d is not a hls task", taskId)
	}

	sv, _ := statusMap.Load(taskId)
	completed := sv.(*Progress).Progrss

	// After a backend restart the parsed playList is nil (not persisted in DB).
	// Re-fetch the m3u8 playlist from the stored URL so we can resume.
	if hlsTaskParam.playList == nil {
		playList, e := fetchPlaylistFunc(hlsTaskParam.playListUrl, hlsTaskParam.headers)
		if e != nil {
			return fmt.Errorf("task %d: %w", taskId, e)
		}
		hlsTaskParam.playList = playList
		// Keys/IVs are resolved lazily per segment by the resolver, so no
		// upfront key download is needed after a restart.
		hlsTaskParam.keyResolver = newHlsKeyResolver(hlsTaskParam.playListUrl, hlsTaskParam.headers)
	}

	seg := hlsTaskParam.playList.Segments
	if completed >= len(seg) {
		// All segments were already downloaded — skip to converting.
		sp, _ := statusMap.Load(taskId)
		p := sp.(*Progress)
		// Ensure Names are populated for the frontend to run FFmpeg.
		// After a restart, Names are rebuilt by verifyHlsProgress, but
		// this path is also hit when the user resumes a Converting task
		// that already has all segments on disk.
		if p.Names == nil || len(*p.Names) == 0 {
			verifyHlsProgress(p)
		}
		p.Status = Converting
		p.SyncDB()
		return nil
	}

	hlsTaskParam.playList.Segments = seg[completed:]
	// Carry the last EXT-X-KEY seen before the resume point: segments that
	// inherit a key have a nil .Key, so without this the resumed slice
	// would be stored un-decrypted.
	for _, s := range seg[:completed] {
		if s.Key != nil {
			hlsTaskParam.inheritedKey = s.Key
		}
	}
	startDownloadTask(hlsTaskParam, downloadSegment)
	return nil
}

// fetchPlaylistFunc fetches and parses an m3u8 media playlist from [playlistURL].
// It defaults to fetchPlaylist but can be overridden in tests.
var fetchPlaylistFunc = fetchPlaylist

func fetchPlaylist(playlistURL string, headers map[string]string) (*m3u8.MediaPlaylist, error) {
	text, e := fetchPlaylistText(playlistURL, headers)
	if e != nil {
		return nil, e
	}
	pl, li, e := decodePlaylist(text, playlistURL)
	if e != nil {
		return nil, e
	}
	if li != m3u8.MEDIA {
		return nil, fmt.Errorf("URL is not a media playlist")
	}
	playList := pl.(*m3u8.MediaPlaylist)
	playList.Segments = filterSegments(playList.Segments)
	return playList, nil
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
	keyResolver *hlsKeyResolver
	// inheritedKey is the last EXT-X-KEY active at the resume point.
	inheritedKey *m3u8.Key
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
