package grpc

import (
	"context"
	"encoding/json"

	"github.com/miru-project/miru-core/pkg/torrent"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

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
	res, err := torrent.AddTorrent(req.Url, req.Title, req.Package)
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
	res, err := torrent.AddMagnet(req.Url, req.Title, req.Package)
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
	err := torrent.DeleteTorrent(req.InfoHash, false)
	if err != nil {
		return nil, err
	}
	return &proto.DeleteTorrentResponse{Message: "Success"}, nil
}
