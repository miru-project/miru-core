package grpc

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/miru-project/miru-core/config"
	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/pkg/download"
	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
	"github.com/miru-project/miru-core/pkg/event"
	"github.com/miru-project/miru-core/pkg/jsExtension"
	"github.com/miru-project/miru-core/pkg/logger"
	"github.com/miru-project/miru-core/pkg/torrent"
	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/miru-project/miru-core/router/handler"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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

func (s *MiruCoreServer) HelloMiru(ctx context.Context, req *proto.HelloMiruRequest) (*proto.HelloMiruResponse, error) {
	res, err := handler.HelloMiru()
	if err != nil {
		return nil, err
	}

	data := res.Data.(map[string]any)
	extMeta := data["extensionMeta"].([]*jsExtension.Ext)
	downloadStatus := data["downloadStatus"].(map[int]*download.Progress)

	protoExtMeta := make([]*proto.ExtensionMeta, len(extMeta))
	for i, e := range extMeta {
		protoExtMeta[i] = &proto.ExtensionMeta{
			Name:        e.Name,
			Version:     e.Version,
			Author:      e.Author,
			License:     e.License,
			Lang:        e.Lang,
			Icon:        e.Icon,
			Package:     e.Pkg,
			WebSite:     e.Website,
			Description: e.Description,
			Tags:        e.Tags,
			Api:         e.ApiVersion,
			Error:       e.Error,
			Type:        e.WatchType,
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

	return resp, nil
}

func StartServer() {
	grpcPort, _ := strconv.Atoi(config.Global.Port)
	grpcPort++ // Use next port for gRPC
	lis, err := net.Listen("tcp", config.Global.Address+":"+strconv.Itoa(grpcPort))
	if err != nil {
		logger.Printf("failed to listen for gRPC: %v", err)
		return
	}

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
	jsExtension.OnExtensionUpdate = func(exts []*jsExtension.ExtApi) {
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
		names = *p.Names
	}
	return &proto.DownloadProgress{
		Progress:           int32(p.Progrss),
		Names:              names,
		Total:              int32(p.Total),
		Status:             string(p.Status),
		MediaType:          string(p.MediaType),
		CurrentDownloading: p.CurrentDownloading,
		TaskId:             int32(p.TaskID),
		Title:              p.Title,
		Package:            p.Package,
		Key:                p.Key,
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
