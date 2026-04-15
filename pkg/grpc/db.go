package grpc

import (
	"context"
	"time"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

// DB - Detail
func (s *MiruCoreServer) GetDetail(ctx context.Context, req *proto.GetDetailRequest) (*proto.GetDetailResponse, error) {
	d, err := db.GetDetailByPackageAndUrl(req.Package, req.DetailUrl)
	if err != nil {
		if ent.IsNotFound(err) {
			return &proto.GetDetailResponse{}, nil
		}
		return nil, err
	}
	return &proto.GetDetailResponse{Detail: toProtoDetail(d)}, nil
}

func (s *MiruCoreServer) UpsertDetail(ctx context.Context, req *proto.UpsertDetailRequest) (*proto.UpsertDetailResponse, error) {
	d := &ent.Detail{
		Title:      req.Title,
		Cover:      req.Cover,
		Desc:       req.Desc,
		DetailUrl:  req.DetailUrl,
		Package:    req.Package,
		Downloaded: req.Downloaded,
		Episodes:   req.Episodes,
		Headers:    req.Headers,
		TrackIds:   req.TrackIds,
	}

	saved, err := db.UpsertDetail(d)
	if err != nil {
		return nil, err
	}
	return &proto.UpsertDetailResponse{Detail: toProtoDetail(saved)}, nil
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
	histories, err := db.GetHistoriesByType(&req.Type, int(req.Page), int(req.PageSize))
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

func (s *MiruCoreServer) GetHistoryByPackageAndDetailUrl(ctx context.Context, req *proto.GetHistoryByPackageAndDetailUrlRequest) (*proto.GetHistoryByPackageAndDetailUrlResponse, error) {
	histories, err := db.GetHistoryByPackageAndDetailUrl(req.Package, req.DetailUrl)
	if err != nil {
		return nil, err
	}
	protoHistories := []*proto.History{}
	for _, h := range histories {
		if h != nil {
			protoHistories = append(protoHistories, toProtoHistory(h))
		}
	}
	return &proto.GetHistoryByPackageAndDetailUrlResponse{History: protoHistories}, nil
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
	histories, err := db.GetHistorysFiltered(&req.Type, beforeDate, int(req.Page), int(req.PageSize))
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

// Track
func (s *MiruCoreServer) GetTrack(ctx context.Context, req *proto.GetTrackRequest) (*proto.GetTrackResponse, error) {
	t, err := db.GetTrack(req.TrackingId, req.Provider)
	if err != nil {
		if ent.IsNotFound(err) {
			return &proto.GetTrackResponse{}, nil
		}
		return nil, err
	}
	return &proto.GetTrackResponse{Track: toProtoTrack(t)}, nil
}

func (s *MiruCoreServer) PutTrack(ctx context.Context, req *proto.PutTrackRequest) (*proto.PutTrackResponse, error) {
	t, err := db.PutTrack(req.TrackingId, req.Data, req.MediaType, req.Provider)
	if err != nil {
		return nil, err
	}
	return &proto.PutTrackResponse{Track: toProtoTrack(t)}, nil
}
