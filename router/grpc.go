package router

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/miru-project/miru-core/config"
	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/download"
	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
	"github.com/miru-project/miru-core/pkg/jsExtension"
	"github.com/miru-project/miru-core/pkg/logger"
	"github.com/miru-project/miru-core/pkg/network"
	"github.com/miru-project/miru-core/pkg/torrent"
	"github.com/miru-project/miru-core/proto"
	"github.com/miru-project/miru-core/router/handler"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type MiruCoreServer struct {
	proto.UnimplementedMiruCoreServiceServer
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
			Api:         e.Api,
			Error:       e.Error,
			Type:        e.WatchType,
		}
	}

	protoDownloadStatus := make(map[int32]*proto.DownloadProgress)
	for id, p := range downloadStatus {
		protoDownloadStatus[int32(id)] = toProtoDownloadProgress(p)
	}

	torrentStats := torrent.TorrentStatus()
	resp := &proto.HelloMiruResponse{
		ExtensionMeta:  protoExtMeta,
		DownloadStatus: protoDownloadStatus,
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

func (s *MiruCoreServer) GetAppSetting(ctx context.Context, req *proto.GetAppSettingRequest) (*proto.GetAppSettingResponse, error) {
	res, err := handler.GetAppSetting()
	if err != nil {
		return nil, err
	}

	settings := res.Data.([]*ent.AppSetting)
	protoSettings := make([]*proto.AppSetting, len(settings))
	for i, s := range settings {
		protoSettings[i] = &proto.AppSetting{
			Key:   s.Key,
			Value: s.Value,
		}
	}

	return &proto.GetAppSettingResponse{Settings: protoSettings}, nil
}

func (s *MiruCoreServer) SetAppSetting(ctx context.Context, req *proto.SetAppSettingRequest) (*proto.SetAppSettingResponse, error) {
	settings := make([]db.AppSettingJson, len(req.Settings))
	for i, s := range req.Settings {
		settings[i] = db.AppSettingJson{
			Key:   s.Key,
			Value: s.Value,
		}
	}

	errs := handler.SetAppSetting(&settings)
	if len(errs) > 0 {
		return nil, errs[0]
	}

	return &proto.SetAppSettingResponse{Message: "Settings updated successfully"}, nil
}

func (s *MiruCoreServer) Search(ctx context.Context, req *proto.SearchRequest) (*proto.SearchResponse, error) {
	res := handler.Search(strconv.Itoa(int(req.Page)), req.Pkg, req.Kw, req.Filter)
	if res.Code != 200 {
		return nil, fmt.Errorf("search failed with code %d: %s", res.Code, res.Message)
	}

	items := res.Data.([]any)
	protoItems := make([]*proto.ExtensionListItem, len(items))
	for i, item := range items {
		m := item.(map[string]any)
		protoItems[i] = &proto.ExtensionListItem{
			Title:  safeSprint(m["title"]),
			Url:    safeSprint(m["url"]),
			Cover:  safeSprint(m["cover"]),
			Update: safeSprint(m["update"]),
		}
	}

	return &proto.SearchResponse{Items: protoItems}, nil
}

func (s *MiruCoreServer) Latest(ctx context.Context, req *proto.LatestRequest) (*proto.LatestResponse, error) {
	res := handler.Latest(strconv.Itoa(int(req.Page)), req.Pkg)
	if res.Code != 200 {
		return nil, fmt.Errorf("latest failed with code %d: %s", res.Code, res.Message)
	}

	items := res.Data.([]any)
	protoItems := make([]*proto.ExtensionListItem, len(items))
	for i, item := range items {
		m := item.(map[string]any)
		protoItems[i] = &proto.ExtensionListItem{
			Title:  safeSprint(m["title"]),
			Url:    safeSprint(m["url"]),
			Cover:  safeSprint(m["cover"]),
			Update: safeSprint(m["update"]),
		}
	}

	return &proto.LatestResponse{Items: protoItems}, nil
}

func (s *MiruCoreServer) Detail(ctx context.Context, req *proto.DetailRequest) (*proto.DetailResponse, error) {
	res := handler.Detail(req.Pkg, req.Url)
	if res.Code != 200 {
		return nil, fmt.Errorf("detail failed with code %d: %s", res.Code, res.Message)
	}

	jsonData, err := json.Marshal(res.Data)
	if err != nil {
		return nil, err
	}

	return &proto.DetailResponse{Data: string(jsonData)}, nil
}

func (s *MiruCoreServer) Watch(ctx context.Context, req *proto.WatchRequest) (*proto.WatchResponse, error) {
	res := handler.Watch(req.Pkg, req.Url)
	if res.Code != 200 {
		return nil, fmt.Errorf("watch failed with code %d: %s", res.Code, res.Message)
	}

	jsonData, err := json.Marshal(res.Data)
	if err != nil {
		return nil, err
	}

	return &proto.WatchResponse{Data: string(jsonData)}, nil
}

// DB - Favorite
func (s *MiruCoreServer) GetAllFavorite(ctx context.Context, req *proto.GetAllFavoriteRequest) (*proto.GetAllFavoriteResponse, error) {
	favs, err := db.GetAllFavorite()
	if err != nil {
		return nil, err
	}
	protoFavs := []*proto.Favorite{}
	for _, f := range favs {
		if f != nil {
			protoFavs = append(protoFavs, toProtoFavorite(f))
		}
	}
	return &proto.GetAllFavoriteResponse{Favorites: protoFavs}, nil
}

func (s *MiruCoreServer) GetFavoriteByPackageAndUrl(ctx context.Context, req *proto.GetFavoriteByPackageAndUrlRequest) (*proto.GetFavoriteByPackageAndUrlResponse, error) {
	f, err := db.GetFavoriteByPackageAndUrl(req.Package, req.Url)
	if err != nil {
		return nil, err
	}
	return &proto.GetFavoriteByPackageAndUrlResponse{Favorite: toProtoFavorite(f)}, nil
}

func (s *MiruCoreServer) PutFavoriteByIndex(ctx context.Context, req *proto.PutFavoriteByIndexRequest) (*proto.PutFavoriteByIndexResponse, error) {
	groups := make([]*ent.FavoriteGroup, len(req.Groups))
	for i, g := range req.Groups {
		date, _ := time.Parse(time.RFC3339, g.Date)
		favorites := make([]*ent.Favorite, len(g.Favorites))
		for j, f := range g.Favorites {
			favorites[j] = &ent.Favorite{ID: int(f.Id)}
		}
		groups[i] = &ent.FavoriteGroup{
			Name: g.Name,
			Date: date,
			Edges: ent.FavoriteGroupEdges{
				Favorites: favorites,
			},
		}
	}
	err := db.PutFavoriteByIndex(groups)
	if err != nil {
		return nil, err
	}
	return &proto.PutFavoriteByIndexResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) PutFavorite(ctx context.Context, req *proto.PutFavoriteRequest) (*proto.PutFavoriteResponse, error) {
	f, err := db.PutFavorite(req.Url, &req.Cover, req.Package, req.Type, req.Title)
	if err != nil {
		return nil, err
	}
	return &proto.PutFavoriteResponse{Favorite: toProtoFavorite(f)}, nil
}

func (s *MiruCoreServer) DeleteFavorite(ctx context.Context, req *proto.DeleteFavoriteRequest) (*proto.DeleteFavoriteResponse, error) {
	err := db.DeleteFavorite(req.Url, req.Package)
	if err != nil {
		return nil, err
	}
	return &proto.DeleteFavoriteResponse{Message: "Success"}, nil
}

// DB - Favorite Group
func (s *MiruCoreServer) GetFavoriteGroupsById(ctx context.Context, req *proto.GetFavoriteGroupsByIdRequest) (*proto.GetFavoriteGroupsByIdResponse, error) {
	groups, err := db.GetFavoriteGroupsById(int(req.Id))
	if err != nil {
		return nil, err
	}
	protoGroups := []*proto.FavoriteGroup{}
	for _, g := range groups {
		if g != nil {
			protoGroups = append(protoGroups, toProtoFavoriteGroup(g))
		}
	}
	return &proto.GetFavoriteGroupsByIdResponse{Groups: protoGroups}, nil
}

func (s *MiruCoreServer) GetAllFavoriteGroup(ctx context.Context, req *proto.GetAllFavoriteGroupRequest) (*proto.GetAllFavoriteGroupResponse, error) {
	groups, err := db.GetAllFavoriteGroup()
	if err != nil {
		return nil, err
	}
	protoGroups := []*proto.FavoriteGroup{}
	for _, g := range groups {
		if g != nil {
			protoGroups = append(protoGroups, toProtoFavoriteGroup(g))
		}
	}
	return &proto.GetAllFavoriteGroupResponse{Groups: protoGroups}, nil
}

func (s *MiruCoreServer) PutFavoriteGroup(ctx context.Context, req *proto.PutFavoriteGroupRequest) (*proto.PutFavoriteGroupResponse, error) {
	items := make([]int, len(req.Items))
	for i, it := range req.Items {
		items[i] = int(it)
	}
	g, err := db.PutFavoriteGroup(req.Name, items)
	if err != nil {
		return nil, err
	}
	return &proto.PutFavoriteGroupResponse{Group: toProtoFavoriteGroup(g)}, nil
}

func (s *MiruCoreServer) RenameFavoriteGroup(ctx context.Context, req *proto.RenameFavoriteGroupRequest) (*proto.RenameFavoriteGroupResponse, error) {
	err := db.RenameFavoriteGroup(req.OldName, req.NewName)
	if err != nil {
		return nil, err
	}
	return &proto.RenameFavoriteGroupResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) DeleteFavoriteGroup(ctx context.Context, req *proto.DeleteFavoriteGroupRequest) (*proto.DeleteFavoriteGroupResponse, error) {
	err := db.DeleteFavoriteGroup(req.Names)
	if err != nil {
		return nil, err
	}
	return &proto.DeleteFavoriteGroupResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) GetFavoriteGroupsByFavorite(ctx context.Context, req *proto.GetFavoriteGroupsByFavoriteRequest) (*proto.GetFavoriteGroupsByFavoriteResponse, error) {
	groups, err := db.GetFavoriteGroupsByFavorite(req.Package, req.Url)
	if err != nil {
		return nil, err
	}
	protoGroups := []*proto.FavoriteGroup{}
	for _, g := range groups {
		if g != nil {
			protoGroups = append(protoGroups, toProtoFavoriteGroup(g))
		}
	}
	return &proto.GetFavoriteGroupsByFavoriteResponse{Groups: protoGroups}, nil
}

// DB - History
func (s *MiruCoreServer) GetHistoriesByType(ctx context.Context, req *proto.GetHistoriesByTypeRequest) (*proto.GetHistoriesByTypeResponse, error) {
	histories, err := db.GetHistoriesByType(&req.Type)
	if err != nil {
		return nil, err
	}
	protoHistories := []*proto.History{}
	for _, h := range histories {
		if h != nil {
			protoHistories = append(protoHistories, toProtoHistory(h))
		}
	}
	return &proto.GetHistoriesByTypeResponse{Histories: protoHistories}, nil
}

func (s *MiruCoreServer) GetHistoryByPackageAndUrl(ctx context.Context, req *proto.GetHistoryByPackageAndUrlRequest) (*proto.GetHistoryByPackageAndUrlResponse, error) {
	h, err := db.GetHistoryByPackageAndUrl(req.Package, req.Url)
	if err != nil {
		return nil, err
	}
	return &proto.GetHistoryByPackageAndUrlResponse{History: toProtoHistory(h)}, nil
}

func (s *MiruCoreServer) PutHistory(ctx context.Context, req *proto.PutHistoryRequest) (*proto.PutHistoryResponse, error) {
	h := fromProtoHistory(req.History)
	id, err := db.PutHistory(h)
	if err != nil {
		return nil, err
	}
	h.ID = id
	return &proto.PutHistoryResponse{History: toProtoHistory(h)}, nil
}

func (s *MiruCoreServer) DeleteHistoryByPackageAndUrl(ctx context.Context, req *proto.DeleteHistoryByPackageAndUrlRequest) (*proto.DeleteHistoryByPackageAndUrlResponse, error) {
	err := db.DeleteHistoryByPackageAndUrl(req.Package, req.Url)
	if err != nil {
		return nil, err
	}
	return &proto.DeleteHistoryByPackageAndUrlResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) DeleteAllHistory(ctx context.Context, req *proto.DeleteAllHistoryRequest) (*proto.DeleteAllHistoryResponse, error) {
	_, err := db.DeleteAllHistory()
	if err != nil {
		return nil, err
	}
	return &proto.DeleteAllHistoryResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) GetHistorysFiltered(ctx context.Context, req *proto.GetHistorysFilteredRequest) (*proto.GetHistorysFilteredResponse, error) {
	var beforeDate *time.Time
	if req.BeforeDate != "" {
		t, err := time.Parse(time.RFC3339, req.BeforeDate)
		if err == nil {
			beforeDate = &t
		}
	}
	histories, err := db.GetHistorysFiltered(&req.Type, beforeDate)
	if err != nil {
		return nil, err
	}
	protoHistories := []*proto.History{}
	for _, h := range histories {
		if h != nil {
			protoHistories = append(protoHistories, toProtoHistory(h))
		}
	}
	return &proto.GetHistorysFilteredResponse{Histories: protoHistories}, nil
}

// Download
func (s *MiruCoreServer) GetDownloadStatus(ctx context.Context, req *proto.GetDownloadStatusRequest) (*proto.GetDownloadStatusResponse, error) {
	status := download.DownloadStatus()
	protoStatus := make(map[int32]*proto.DownloadProgress)
	for id, p := range status {
		protoStatus[int32(id)] = toProtoDownloadProgress(p)
	}
	return &proto.GetDownloadStatusResponse{DownloadStatus: protoStatus}, nil
}

func (s *MiruCoreServer) CancelDownload(ctx context.Context, req *proto.CancelDownloadRequest) (*proto.CancelDownloadResponse, error) {
	err := download.CancelTask(int(req.TaskId))
	if err != nil {
		return nil, err
	}
	return &proto.CancelDownloadResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) ResumeDownload(ctx context.Context, req *proto.ResumeDownloadRequest) (*proto.ResumeDownloadResponse, error) {
	err := download.ResumeTask(int(req.TaskId))
	if err != nil {
		return nil, err
	}
	return &proto.ResumeDownloadResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) PauseDownload(ctx context.Context, req *proto.PauseDownloadRequest) (*proto.PauseDownloadResponse, error) {
	err := download.PauseTask(int(req.TaskId))
	if err != nil {
		return nil, err
	}
	return &proto.PauseDownloadResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) DownloadBangumi(ctx context.Context, req *proto.DownloadBangumiRequest) (*proto.DownloadBangumiResponse, error) {
	res, err := download.DownloadBangumi(req.DownloadPath, req.Url, req.Header, req.IsHls)
	if err != nil {
		return nil, err
	}

	protoVariants := make([]*proto.AvailableHlsVariant, len(res.VariantSummary))
	for i, v := range res.VariantSummary {
		protoVariants[i] = &proto.AvailableHlsVariant{
			Resolution: v.Resolution,
			Url:        v.Url,
			Codec:      v.Codecs,
		}
	}

	return &proto.DownloadBangumiResponse{
		TaskId:         int32(res.TaskID),
		VariantSummary: protoVariants,
		IsDownloading:  res.IsDownloading,
	}, nil
}

// Torrent
func (s *MiruCoreServer) ListTorrent(ctx context.Context, req *proto.ListTorrentRequest) (*proto.ListTorrentResponse, error) {
	torResult := make([]*proto.TorrentResult, 0)
	for hash, t := range torrent.Torrents {
		files := []string{}
		if len(t.Info().Files) == 0 {
			files = append(files, t.Name())
		} else {
			for _, file := range t.Info().Files {
				files = append(files, file.DisplayPath(t.Info()))
			}
		}
		torResult = append(torResult, &proto.TorrentResult{
			InfoHash: hash,
			Name:     t.Name(),
			Files:    files,
		})
	}
	return &proto.ListTorrentResponse{Torrents: torResult}, nil
}

func (s *MiruCoreServer) AddTorrent(ctx context.Context, req *proto.AddTorrentRequest) (*proto.AddTorrentResponse, error) {
	res, err := torrent.AddTorrentBytes(req.Torrent)
	if err != nil {
		return nil, err
	}

	detailJson, _ := json.Marshal(res.Detail)

	return &proto.AddTorrentResponse{
		InfoHash:   res.InfoHash,
		DetailJson: string(detailJson),
		Files:      res.Files,
	}, nil
}

func (s *MiruCoreServer) AddMagnet(ctx context.Context, req *proto.AddMagnetRequest) (*proto.AddMagnetResponse, error) {
	res, err := torrent.AddMagnet(req.Url)
	if err != nil {
		return nil, err
	}

	detailJson, _ := json.Marshal(res.Detail)

	return &proto.AddMagnetResponse{
		InfoHash:   res.InfoHash,
		DetailJson: string(detailJson),
		Files:      res.Files,
	}, nil
}

func (s *MiruCoreServer) DeleteTorrent(ctx context.Context, req *proto.DeleteTorrentRequest) (*proto.DeleteTorrentResponse, error) {
	err := torrent.DeleteTorrent(req.InfoHash)
	if err != nil {
		return nil, err
	}
	return &proto.DeleteTorrentResponse{Message: "Success"}, nil
}

// Repo
func (s *MiruCoreServer) GetRepos(ctx context.Context, req *proto.GetReposRequest) (*proto.GetReposResponse, error) {
	repos, err := jsExtension.LoadExtensionRepo()
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(repos)
	return &proto.GetReposResponse{Data: string(data)}, nil
}

func (s *MiruCoreServer) SetRepo(ctx context.Context, req *proto.SetRepoRequest) (*proto.SetRepoResponse, error) {
	err := jsExtension.SaveExtensionRepo(req.RepoUrl, req.Name)
	if err != nil {
		return nil, err
	}
	return &proto.SetRepoResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) DeleteRepo(ctx context.Context, req *proto.DeleteRepoRequest) (*proto.DeleteRepoResponse, error) {
	err := jsExtension.RemoveExtensionRepo(req.RepoUrl)
	if err != nil {
		return nil, err
	}
	return &proto.DeleteRepoResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) FetchRepoList(ctx context.Context, req *proto.FetchRepoListRequest) (*proto.FetchRepoListResponse, error) {
	repoList, _, err := jsExtension.FetchExtensionRepo()
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(repoList)
	return &proto.FetchRepoListResponse{Data: string(data)}, nil
}

// Extension Management
func (s *MiruCoreServer) DownloadExtension(ctx context.Context, req *proto.DownloadExtensionRequest) (*proto.DownloadExtensionResponse, error) {
	err := jsExtension.DownloadExtension(req.RepoUrl, req.Pkg)
	if err != nil {
		return nil, err
	}
	return &proto.DownloadExtensionResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) RemoveExtension(ctx context.Context, req *proto.RemoveExtensionRequest) (*proto.RemoveExtensionResponse, error) {
	err := jsExtension.RemoveExtension(req.Pkg)
	if err != nil {
		return nil, err
	}
	return &proto.RemoveExtensionResponse{Message: "Success"}, nil
}

// Network
func (s *MiruCoreServer) SetCookie(ctx context.Context, req *proto.SetCookieRequest) (*proto.SetCookieResponse, error) {
	err := network.SetCookies(req.Url, strings.Split(req.Cookie, ";"))
	if err != nil {
		return nil, err
	}
	return &proto.SetCookieResponse{Message: "Success"}, nil
}

// Helpers
func safeSprint(v any) string {
	if v == nil {
		return ""
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
	// Check if edges are loaded
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

func StartGRPCServer() {
	grpcPort, _ := strconv.Atoi(config.Global.Port)
	grpcPort++ // Use next port for gRPC
	lis, err := net.Listen("tcp", config.Global.Address+":"+strconv.Itoa(grpcPort))
	if err != nil {
		logger.Printf("failed to listen for gRPC: %v", err)
		return
	}

	s := grpc.NewServer()
	proto.RegisterMiruCoreServiceServer(s, &MiruCoreServer{})
	reflection.Register(s)

	logger.Printf("gRPC server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		errorhandle.PanicF("failed to serve gRPC: %v", err)
	}
}
