package grpc

import (
	"context"
	"fmt"
	"strconv"

	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/extension"
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

	watchResp := &proto.WatchResponse{}

	// Golang (Scriggo) extensions are V2-only: Watch() emits either the generic
	// proto.ExtensionWatch (the source/group list) or the combined
	// proto.ExtensionAllWatch. Route the typed result to the matching oneof
	// variant. A standalone per-type watch (bangumi/manga/fikushon) is never
	// returned by the golang runtime -- those shapes only appear as members of
	// proto.ExtensionAllWatch -- so any other concrete type is a server-side
	// bug and surfaces as an error.
	if endpoint.IsGolang(req.Pkg) {
		switch d := res.Data.(type) {
		case *proto.ExtensionWatch:
			watchResp.Data = &proto.WatchResponse_Watch{Watch: d}
		case *proto.ExtensionAllWatch:
			watchResp.Data = &proto.WatchResponse_All{All: d}
		default:
			return nil, fmt.Errorf("watch result for golang extension %q does not conform to the V2 watch contract (got %T); golang extensions must return proto.ExtensionWatch or proto.ExtensionAllWatch", req.Pkg, res.Data)
		}
		sanitizeWatchResponse(watchResp)
		return watchResp, nil
	}

	// JavaScript (v1/v2) path: decide the response shape from the API version
	// and the declared watch type. Every case below must produce a well-formed
	// watch; non-conforming results are an error rather than a raw fallback.
	if api.ApiVersion == "2" {
		// V2 returns the generic ExtensionWatch which contains mirrors
		data, err := endpoint.Unmarshal[proto.ExtensionWatch](res.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal v2 watch for %q: %w", req.Pkg, err)
		}
		watchResp.Data = &proto.WatchResponse_Watch{Watch: data}
	} else {
		switch api.WatchType {
		case extension.WatchTypeBangumi:
			data, err := endpoint.Unmarshal[proto.ExtensionBangumiWatch](res.Data)
			if err != nil {
				return nil, err
			}
			watchResp.Data = &proto.WatchResponse_Bangumi{Bangumi: data}
		case extension.WatchTypeManga:
			data, err := endpoint.Unmarshal[proto.ExtensionMangaWatch](res.Data)
			if err != nil {
				return nil, err
			}
			watchResp.Data = &proto.WatchResponse_Manga{Manga: data}
		case extension.WatchTypeFikushon:
			data, err := endpoint.Unmarshal[proto.ExtensionFikushonWatch](res.Data)
			if err != nil {
				return nil, err
			}
			watchResp.Data = &proto.WatchResponse_Fikushon{Fikushon: data}
		case extension.WatchTypeAll:
			data, err := endpoint.Unmarshal[proto.ExtensionAllWatch](res.Data)
			if err != nil {
				return nil, err
			}
			watchResp.Data = &proto.WatchResponse_All{All: data}
		default:
			return nil, fmt.Errorf("watch result for extension %q does not conform to any known watch type (got %q)", req.Pkg, api.WatchType)
		}
	}

	sanitizeWatchResponse(watchResp)
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

	mirrorResp := &proto.MirrorResponse{}

	// Golang (Scriggo) extensions are v2-only: Watch() yields the mirror list
	// and Mirror() resolves the chosen mirror to the final per-type watch (exactly
	// like the JavaScript V2 runtime). Route it into the matching MirrorResponse
	// oneof variant by the declared @type, the same switch the JS path uses.
	if endpoint.IsGolang(req.Pkg) {
		switch api.WatchType {
		case extension.WatchTypeBangumi:
			data, err := endpoint.Unmarshal[proto.ExtensionBangumiWatch](res)
			if err != nil {
				return nil, err
			}
			mirrorResp.Data = &proto.MirrorResponse_Bangumi{Bangumi: data}
		case extension.WatchTypeManga:
			data, err := endpoint.Unmarshal[proto.ExtensionMangaWatch](res)
			if err != nil {
				return nil, err
			}
			mirrorResp.Data = &proto.MirrorResponse_Manga{Manga: data}
		case extension.WatchTypeFikushon:
			data, err := endpoint.Unmarshal[proto.ExtensionFikushonWatch](res)
			if err != nil {
				return nil, err
			}
			mirrorResp.Data = &proto.MirrorResponse_Fikushon{Fikushon: data}
		case extension.WatchTypeAll:
			data, err := endpoint.Unmarshal[proto.ExtensionAllWatch](res)
			if err != nil {
				return nil, err
			}
			mirrorResp.Data = &proto.MirrorResponse_All{All: data}
		default:
			return nil, fmt.Errorf("mirror result for extension %q (golang runtime) does not conform to any known watch type (got %q)", req.Pkg, api.WatchType)
		}
		// Default every resolved stream/mirror through miru-core (like torrents),
		// so the player/downloader fetches it server-side with the mirror headers
		// and optional tls fingerprint applied. Already-proxied URLs pass through.
		sanitizeMirrorResponse(mirrorResp)
		proxyMirrorResponse(mirrorResp)
		return mirrorResp, nil
	}

	switch api.WatchType {
	case extension.WatchTypeBangumi:
		data, err := endpoint.Unmarshal[proto.ExtensionBangumiWatch](res)
		if err != nil {
			return nil, err
		}
		mirrorResp.Data = &proto.MirrorResponse_Bangumi{Bangumi: data}
	case extension.WatchTypeManga:
		data, err := endpoint.Unmarshal[proto.ExtensionMangaWatch](res)
		if err != nil {
			return nil, err
		}
		mirrorResp.Data = &proto.MirrorResponse_Manga{Manga: data}
	case extension.WatchTypeFikushon:
		data, err := endpoint.Unmarshal[proto.ExtensionFikushonWatch](res)
		if err != nil {
			return nil, err
		}
		mirrorResp.Data = &proto.MirrorResponse_Fikushon{Fikushon: data}
	case extension.WatchTypeAll:
		data, err := endpoint.Unmarshal[proto.ExtensionAllWatch](res)
		if err != nil {
			return nil, err
		}
		mirrorResp.Data = &proto.MirrorResponse_All{All: data}
	default:
		return nil, fmt.Errorf("mirror result for extension %q does not conform to any known watch type (got %q)", req.Pkg, api.WatchType)
	}

	// Sanitize before proxying so proxy URL building operates on valid UTF-8.
	sanitizeMirrorResponse(mirrorResp)
	// Default every resolved stream/mirror through miru-core (like torrents).
	proxyMirrorResponse(mirrorResp)
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
