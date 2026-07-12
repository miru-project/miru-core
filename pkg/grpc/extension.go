package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/extension/endpoint"
	"github.com/miru-project/miru-core/pkg/extension/js"
	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/miru-project/miru-core/router/handler"
)

func (s *MiruCoreServer) Search(ctx context.Context, req *proto.SearchRequest) (*proto.SearchResponse, error) {
	res := handler.Search(strconv.Itoa(int(req.Page)), req.Pkg, req.Kw, req.Filter)
	if res.Code != 200 {
		return nil, fmt.Errorf("search failed with code %d: %s", res.Code, res.Message)
	}

	return &proto.SearchResponse{Items: res.Data}, nil
}

func (s *MiruCoreServer) CreateFilter(ctx context.Context, req *proto.CreateFilterRequest) (*proto.CreateFilterResponse, error) {
	res := handler.CreateFilter(req.Pkg, req.Filter)
	if res.Code != 200 {
		return nil, fmt.Errorf("create filter failed with code %d: %s", res.Code, res.Message)
	}

	return &proto.CreateFilterResponse{Filters: res.Data}, nil
}

func (s *MiruCoreServer) Latest(ctx context.Context, req *proto.LatestRequest) (*proto.LatestResponse, error) {
	res := handler.Latest(strconv.Itoa(int(req.Page)), req.Pkg)
	if res.Code != 200 {
		return nil, fmt.Errorf("latest failed with code %d: %s", res.Code, res.Message)
	}
	if res.Data == nil {
		return &proto.LatestResponse{}, nil
	}

	return &proto.LatestResponse{Items: res.Data}, nil
}

func (s *MiruCoreServer) Detail(ctx context.Context, req *proto.DetailRequest) (*proto.DetailResponse, error) {
	res := handler.Detail(req.Pkg, req.Url)
	if res.Code != 200 {
		return nil, fmt.Errorf("detail failed with code %d: %s", res.Code, res.Message)
	}

	return &proto.DetailResponse{Data: res.Data}, nil
}

func (s *MiruCoreServer) Watch(ctx context.Context, req *proto.WatchRequest) (*proto.WatchResponse, error) {
	res, api := handler.Watch(req.Pkg, req.Url)
	if res.Code != 200 {
		return nil, fmt.Errorf("watch failed with code %d: %s", res.Code, res.Message)
	}
	if api == nil {
		return nil, fmt.Errorf("extension metadata not found for package %q", req.Pkg)
	}

	// Golang (Scriggo) extensions are v2-only: they always emit the generic
	// proto.ExtensionWatch shape, never the v1 per-type watch objects. Force the
	// V2 branch so a missing @apiVersion in the extension metadata cannot route
	// them into the v1 watch-type switch.
	if endpoint.IsGolang(req.Pkg) {
		api.ApiVersion = "2"
	}

	watchResp := &proto.WatchResponse{}

	// If it's V1, we return the specialized watch objects
	switch api.ApiVersion {
	case "2":
		// V2 returns the generic ExtensionWatch which contains mirrors
		data, err := endpoint.Unmarshal[proto.ExtensionWatch](res.Data)
		if err != nil {
			if s, ok := res.Data.(string); ok {
				watchResp.Data = &proto.WatchResponse_Raw{Raw: s}
			} else {
				jsonData, _ := json.Marshal(res.Data)
				watchResp.Data = &proto.WatchResponse_Raw{Raw: string(jsonData)}
			}
		} else {
			watchResp.Data = &proto.WatchResponse_Watch{Watch: data}
		}
	default:
		switch api.WatchType {
		case "bangumi":
			data, err := endpoint.Unmarshal[proto.ExtensionBangumiWatch](res.Data)
			if err != nil {
				return nil, err
			}
			watchResp.Data = &proto.WatchResponse_Bangumi{Bangumi: data}
		case "manga":
			data, err := endpoint.Unmarshal[proto.ExtensionMangaWatch](res.Data)
			if err != nil {
				return nil, err
			}
			watchResp.Data = &proto.WatchResponse_Manga{Manga: data}
		case "fikushon":
			data, err := endpoint.Unmarshal[proto.ExtensionFikushonWatch](res.Data)
			if err != nil {
				return nil, err
			}
			watchResp.Data = &proto.WatchResponse_Fikushon{Fikushon: data}
		}
	}

	return watchResp, nil
}

func (s *MiruCoreServer) Mirror(ctx context.Context, req *proto.MirrorRequest) (*proto.MirrorResponse, error) {
	rt, err := endpoint.GetRuntime(req.Pkg)
	if err != nil {
		return nil, err
	}

	res, err := rt.Mirror(req.Pkg, req.Url)
	if err != nil {
		return nil, err
	}

	api, err := rt.GetExtensionMeta(req.Pkg)
	if err != nil {
		return nil, err
	}

	// Golang (Scriggo) extensions are v2-only and emit a flat
	// []*proto.ExtensionMirror list (the Mirror step that follows Watch).
	// Ship it as raw JSON. The watch-type switch below is the legacy v1 JS path.
	if endpoint.IsGolang(req.Pkg) {
		b, err := json.Marshal(res)
		if err != nil {
			return nil, err
		}
		return &proto.MirrorResponse{
			Data: &proto.MirrorResponse_Raw{Raw: string(b)},
		}, nil
	}

	mirrorResp := &proto.MirrorResponse{}
	switch api.WatchType {
	case "bangumi":
		data, err := endpoint.Unmarshal[proto.ExtensionBangumiWatch](res)
		if err != nil {
			return nil, err
		}
		mirrorResp.Data = &proto.MirrorResponse_Bangumi{Bangumi: data}
	case "manga":
		data, err := endpoint.Unmarshal[proto.ExtensionMangaWatch](res)
		if err != nil {
			return nil, err
		}
		mirrorResp.Data = &proto.MirrorResponse_Manga{Manga: data}
	case "fikushon":
		data, err := endpoint.Unmarshal[proto.ExtensionFikushonWatch](res)
		if err != nil {
			return nil, err
		}
		mirrorResp.Data = &proto.MirrorResponse_Fikushon{Fikushon: data}
	default:
		if val, ok := res.(string); ok {
			mirrorResp.Data = &proto.MirrorResponse_Raw{Raw: val}
		} else if b, ok := res.([]byte); ok {
			mirrorResp.Data = &proto.MirrorResponse_Raw{Raw: string(b)}
		} else {
			jsonData, _ := json.Marshal(res)
			mirrorResp.Data = &proto.MirrorResponse_Raw{Raw: string(jsonData)}
		}
	}

	return mirrorResp, nil
}

func (s *MiruCoreServer) DownloadExtension(ctx context.Context, req *proto.DownloadExtensionRequest) (*proto.DownloadExtensionResponse, error) {
	err := js.DownloadExtension(req.RepoUrl, req.Pkg)
	if err != nil {
		return nil, err
	}
	return &proto.DownloadExtensionResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) RemoveExtension(ctx context.Context, req *proto.RemoveExtensionRequest) (*proto.RemoveExtensionResponse, error) {
	err := js.RemoveExtension(req.Pkg)
	if err != nil {
		return nil, err
	}
	return &proto.RemoveExtensionResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) GetExtensionSettings(ctx context.Context, req *proto.GetExtensionSettingsRequest) (*proto.GetExtensionSettingsResponse, error) {
	settings, err := db.GetSettingsByPackage(req.Pkg)
	if err != nil {
		return nil, err
	}

	protoSettings := make([]*proto.ExtensionSetting, len(settings))
	for i, s := range settings {
		protoSettings[i] = &proto.ExtensionSetting{
			Id:           int32(s.ID),
			Package:      s.Package,
			Title:        s.Title,
			Key:          s.Key,
			Value:        s.Value,
			DefaultValue: safeSprint(s.DefaultValue),
			Type:         proto.ExtensionSettingType(proto.ExtensionSettingType_value[string(s.DbType)]),
			Description:  s.Description,
			Options:      s.Options,
		}
	}

	return &proto.GetExtensionSettingsResponse{Settings: protoSettings}, nil
}

func (s *MiruCoreServer) SaveExtensionSettings(ctx context.Context, req *proto.SaveExtensionSettingsRequest) (*proto.SaveExtensionSettingsResponse, error) {
	for _, s := range req.Settings {
		val := ""
		if s.Value != nil {
			val = *s.Value
		}
		err := db.SetSetting(req.Pkg, s.Key, val)
		if err != nil {
			return nil, err
		}
	}
	return &proto.SaveExtensionSettingsResponse{Message: "Success"}, nil
}
