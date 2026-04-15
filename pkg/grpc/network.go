package grpc

import (
	"context"
	"strings"

	"github.com/miru-project/miru-core/pkg/network"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

// Network
func (s *MiruCoreServer) SetCookie(ctx context.Context, req *proto.SetCookieRequest) (*proto.SetCookieResponse, error) {
	err := network.SetCookiesString(req.Url, strings.Split(req.Cookie, ";"))
	if err != nil {
		return nil, err
	}
	return &proto.SetCookieResponse{Message: "Success"}, nil
}
