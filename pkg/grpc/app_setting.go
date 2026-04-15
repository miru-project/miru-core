package grpc

import (
	"context"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/miru-project/miru-core/router/handler"
)

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
	settings := make(map[string]string, len(req.Settings))
	for _, s := range req.Settings {
		settings[s.Key] = s.Value
	}

	errs := handler.SetAppSettings(settings)
	if len(errs) > 0 {
		return nil, errs[0]
	}

	return &proto.SetAppSettingResponse{Message: "Settings updated successfully"}, nil
}
