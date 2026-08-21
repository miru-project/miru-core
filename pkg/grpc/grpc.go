package grpc

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/miru-project/miru-core/config"
	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/pkg/download"
	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
	"github.com/miru-project/miru-core/pkg/event"
	"github.com/miru-project/miru-core/pkg/extension/js"
	"github.com/miru-project/miru-core/pkg/logger"
	"github.com/miru-project/miru-core/pkg/torrent"
	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/miru-project/miru-core/router/handler"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	pb "google.golang.org/protobuf/proto"
)

type MiruCoreServer struct {
	proto.UnimplementedMiruCoreServiceServer
	proto.UnimplementedAppSettingServiceServer
	proto.UnimplementedExtensionServiceServer
	proto.UnimplementedRepoServiceServer
	proto.UnimplementedDownloadServiceServer
	proto.UnimplementedDbServiceServer
	proto.UnimplementedNetworkServiceServer
	proto.UnimplementedEventServiceServer
}

var (
	helloCache      *proto.HelloMiruResponse
	helloCacheMutex sync.RWMutex
	helloCacheTime  time.Time
	helloCacheTTL   = 5 * time.Second
)

func invalidateHelloCache() {
	helloCacheMutex.Lock()
	helloCache = nil
	helloCacheMutex.Unlock()
}

func (s *MiruCoreServer) HelloMiru(ctx context.Context, req *proto.HelloMiruRequest) (*proto.HelloMiruResponse, error) {
	helloCacheMutex.RLock()
	cached := helloCache
	if cached != nil && time.Since(helloCacheTime) < helloCacheTTL {
		helloCacheMutex.RUnlock()
		b, _ := pb.Marshal(cached)
		out := &proto.HelloMiruResponse{}
		pb.Unmarshal(b, out)
		return out, nil
	}
	helloCacheMutex.RUnlock()

	res, err := handler.HelloMiru()
	if err != nil {
		return nil, err
	}

	data := res.Data.(map[string]any)
	extMeta := data["extensionMeta"].([]*js.Ext)
	downloadStatus := data["downloadStatus"].(map[int]*download.Progress)

	protoExtMeta := make([]*proto.ExtensionMeta, len(extMeta))
	for i, e := range extMeta {
		protoExtMeta[i] = &proto.ExtensionMeta{
			Name:        sanitizeUTF8(e.Name),
			Version:     sanitizeUTF8(e.Version),
			Author:      sanitizeUTF8(e.Author),
			License:     sanitizeUTF8(e.License),
			Lang:        sanitizeUTF8(e.Lang),
			Icon:        sanitizeUTF8(e.Icon),
			Package:     sanitizeUTF8(e.Pkg),
			WebSite:     sanitizeUTF8(e.Website),
			Description: sanitizeUTF8(e.Description),
			Tags:        sanitizeTags(e.Tags),
			Api:         sanitizeUTF8(e.ApiVersion),
			Error:       sanitizeUTF8(e.Error),
			Type:        sanitizeUTF8(string(e.WatchType)),
		}
	}

	protoDownloadStatus := make(map[int32]*proto.DownloadProgress)
	for id, p := range downloadStatus {
		protoDownloadStatus[int32(id)] = toProtoDownloadProgress(p)
	}

	history := data["history"].([]*ent.History)
	protoHistory := make([]*proto.History, len(history))
	for i, h := range history {
		protoHistory[i] = toProtoHistory(h)
	}

	torrentStats := torrent.TorrentStatus()
	resp := &proto.HelloMiruResponse{
		ExtensionMeta:  protoExtMeta,
		DownloadStatus: protoDownloadStatus,
		History:        protoHistory,
		Torrent: &proto.TorrentStats{
			TotalDown: torrentStats.ConnStats.BytesReadData.Int64(),
			TotalUp:   torrentStats.ConnStats.BytesWrittenData.Int64(),
		},
	}

	if resp.Torrent == nil {
		resp.Torrent = &proto.TorrentStats{}
	}

	helloCacheMutex.Lock()
	helloCache = resp
	helloCacheTime = time.Now()
	helloCacheMutex.Unlock()

	return resp, nil
}

func StartServer() {
	// Resolve the gRPC port defensively. An empty/invalid "gRPCPort" value
	// makes strconv.Atoi return 0, and net.Listen on port 0 asks the OS for a
	// random ephemeral port (e.g. 44458) that the Flutter client can't predict.
	// Fall back to the documented default (HTTP port + 1) so both sides agree.
	grpcPort, err := strconv.Atoi(config.Global.GRPCPort)
	if err != nil || grpcPort <= 0 {
		previous := config.Global.GRPCPort
		if httpPort, e := strconv.Atoi(config.Global.Port); e == nil && httpPort > 0 {
			grpcPort = httpPort + 1
		} else {
			grpcPort = 3001
		}
		logger.Printf("invalid gRPCPort %q, defaulting to %d", previous, grpcPort)
		config.Global.GRPCPort = strconv.Itoa(grpcPort)
	}
	lis, err := net.Listen("tcp", config.Global.Address+":"+strconv.Itoa(grpcPort))
	if err != nil {
		logger.Printf("failed to listen for gRPC: %v", err)
		return
	}

	event.SetHelloCacheInvalidator(invalidateHelloCache)

	s := grpc.NewServer()
	srv := &MiruCoreServer{}
	proto.RegisterMiruCoreServiceServer(s, srv)
	proto.RegisterAppSettingServiceServer(s, srv)
	proto.RegisterExtensionServiceServer(s, srv)
	proto.RegisterRepoServiceServer(s, srv)
	proto.RegisterDownloadServiceServer(s, srv)
	proto.RegisterDbServiceServer(s, srv)
	proto.RegisterNetworkServiceServer(s, srv)
	proto.RegisterEventServiceServer(s, srv)
	reflection.Register(s)

	// Initialize callbacks for real-time events
	download.OnStatusUpdate = func(status map[int]*download.Progress) {
		event.SendDownloadUpdate(status)
	}
	js.OnExtensionUpdate = func(exts []*js.ExtApi) {
		event.SendExtensionUpdate(exts)
	}

	logger.Printf("gRPC server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		errorhandle.PanicF("failed to serve gRPC: %v", err)
	}
}

// Helpers
func safeSprint(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(*string); ok {
		if s == nil {
			return ""
		}
		return *s
	}
	return fmt.Sprint(v)
}

func toProtoDownloadProgress(p *download.Progress) *proto.DownloadProgress {
	names := []string{}
	if p.Names != nil {
		names = sanitizeNames(*p.Names)
	}
	url := ""
	if len(p.URL) > 0 {
		url = p.URL[0]
	}
	return &proto.DownloadProgress{
		Progress:           int32(p.Progrss),
		Names:              names,
		Total:              int32(p.Total),
		Status:             download.StatusToProto(p.Status),
		MediaType:          mediaTypeToProto(p.MediaType),
		Category:           categoryToProto(p.Category),
		CurrentDownloading: sanitizeUTF8(p.CurrentDownloading),
		TaskId:             int32(p.TaskID),
		Title:              sanitizeUTF8(p.Title),
		Package:            sanitizeUTF8(p.Package),
		Key:                sanitizeUTF8(p.Key),
		Priority:           int32(p.Priority),
		Url:                sanitizeUTF8(url),
		Error:              sanitizeUTF8(p.Error),
	}
}

// categoryToProto converts the persisted content-category string (the ent enum
// value: unspecified / video / manga / novel) into the proto DownloadCategory
// enum. The enum value NAMES are identical between the ent column and the proto
// definition, so this is a direct lookup with no semantic remapping; unknown
// values fall back to unspecified.
func categoryToProto(category download.Category) proto.DownloadCategory {
	if v, ok := proto.DownloadCategory_value[string(category)]; ok {
		return proto.DownloadCategory(v)
	}
	return proto.DownloadCategory_unspecified
}

// mediaTypeToProto converts the internal download media type (in-memory value /
// the ent enum column string) into the proto DownloadMediaType enum. Similar to
// [categoryToProto], the value names are aligned (hls / mp4 / torrent / magnet),
// so this is a direct mapping; any unknown value falls back to unspecified.
func mediaTypeToProto(mt download.MediaType) proto.DownloadMediaType {
	switch mt {
	case download.Hls:
		return proto.DownloadMediaType_hls
	case download.Mp4:
		return proto.DownloadMediaType_mp4
	case download.Torrent:
		return proto.DownloadMediaType_torrent
	case download.Magnet:
		return proto.DownloadMediaType_magnet
	default:
		return proto.DownloadMediaType_media_type_unspecified
	}
}

// mediaTypeFromProto converts a proto DownloadMediaType back into the internal
// download media type. Unspecified (or unknown) values return the empty string so
// the backend falls back to inferring the type from the URL.
func mediaTypeFromProto(mt proto.DownloadMediaType) download.MediaType {
	switch mt {
	case proto.DownloadMediaType_hls:
		return download.Hls
	case proto.DownloadMediaType_mp4:
		return download.Mp4
	case proto.DownloadMediaType_torrent:
		return download.Torrent
	case proto.DownloadMediaType_magnet:
		return download.Magnet
	default:
		return ""
	}
}

func toProtoFavorite(f *ent.Favorite) *proto.Favorite {
	if f == nil {
		return &proto.Favorite{}
	}
	return &proto.Favorite{
		Id:      int32(f.ID),
		Package: f.Package,
		Url:     f.URL,
		Type:    f.Type,
		Title:   f.Title,
		Cover: func() string {
			if f.Cover != nil {
				return *f.Cover
			}
			return ""
		}(),
		Date: f.Date.Format(time.RFC3339),
	}
}

func toProtoFavoriteGroup(g *ent.FavoriteGroup) *proto.FavoriteGroup {
	if g == nil {
		return &proto.FavoriteGroup{}
	}
	favs := []*proto.Favorite{}
	for _, f := range g.Edges.Favorites {
		if f != nil {
			favs = append(favs, toProtoFavorite(f))
		}
	}

	return &proto.FavoriteGroup{
		Id:        int32(g.ID),
		Name:      g.Name,
		Date:      g.Date.Format(time.RFC3339),
		Favorites: favs,
	}
}

func toProtoHistory(h *ent.History) *proto.History {
	if h == nil {
		return &proto.History{}
	}
	return &proto.History{
		Id:        int32(h.ID),
		Package:   h.Package,
		Url:       h.URL,
		DetailUrl: h.DetailUrl,
		Cover: func() string {
			if h.Cover != nil {
				return *h.Cover
			}
			return ""
		}(),
		Type:           h.Type,
		EpisodeGroupId: int32(h.EpisodeGroupID),
		EpisodeId:      int32(h.EpisodeID),
		Title:          h.Title,
		EpisodeTitle:   h.EpisodeTitle,
		Progress:       int32(h.Progress),
		TotalProgress:  int32(h.TotalProgress),
		Date:           h.Date.Format(time.RFC3339),
	}
}

func toProtoTracker(t *ent.Tracker) *proto.Tracker {
	if t == nil {
		return &proto.Tracker{}
	}
	tracker := &proto.Tracker{
		Id:        int32(t.ID),
		TrackerId: t.TrackerID,
		Provider:  string(t.Provider),
		Status:    t.Status,
		Progress:  int32(t.Progress),
	}
	if t.TotalProgress != nil {
		totalProgress := int32(*t.TotalProgress)
		tracker.TotalProgress = &totalProgress
	}
	if t.Score != nil {
		score := int32(*t.Score)
		tracker.Score = &score
	}
	if t.StartDate != nil {
		tracker.StartDate = t.StartDate
	}
	if t.FinishDate != nil {
		tracker.FinishDate = t.FinishDate
	}
	return tracker
}

func toProtoDetail(d *ent.Detail) *proto.Detail {
	if d == nil {
		return &proto.Detail{}
	}
	var trackers []*proto.Tracker
	if d.Edges.Trackers != nil {
		for _, t := range d.Edges.Trackers {
			trackers = append(trackers, toProtoTracker(t))
		}
	}
	return &proto.Detail{
		Id:         int32(d.ID),
		Title:      d.Title,
		Cover:      d.Cover,
		Desc:       d.Desc,
		DetailUrl:  d.DetailUrl,
		Package:    d.Package,
		Downloaded: d.Downloaded,
		Episodes:   d.Episodes,
		Headers:    d.Headers,
		TrackIds:   d.TrackIds,
		Trackers:   trackers,
	}
}

func toProtoTrack(t *ent.Track) *proto.Track {
	if t == nil {
		return &proto.Track{}
	}
	return &proto.Track{
		Id:         int32(t.ID),
		TrackingId: t.TrackingID,
		Data:       t.Data,
		MediaType:  t.MediaType,
		Provider:   string(t.Provider),
	}
}

func fromProtoHistory(h *proto.History) *ent.History {
	if h == nil {
		return nil
	}
	date, _ := time.Parse(time.RFC3339, h.Date)
	if h.Date == "" {
		date = time.Now()
	}
	return &ent.History{
		ID:             int(h.Id),
		Package:        h.Package,
		URL:            h.Url,
		DetailUrl:      h.DetailUrl,
		Cover:          &h.Cover,
		Type:           h.Type,
		EpisodeGroupID: int(h.EpisodeGroupId),
		EpisodeID:      int(h.EpisodeId),
		Title:          h.Title,
		EpisodeTitle:   h.EpisodeTitle,
		Progress:       int(h.Progress),
		TotalProgress:  int(h.TotalProgress),
		Date:           date,
	}
}
