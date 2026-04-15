package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/download"
	"github.com/miru-project/miru-core/pkg/torrent"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

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

func (s *MiruCoreServer) UpdateDownloadStatus(ctx context.Context, req *proto.UpdateDownloadStatusRequest) (*proto.UpdateDownloadStatusResponse, error) {
	status := download.DownloadStatus()
	p, ok := status[int(req.TaskId)]
	if !ok {
		return nil, fmt.Errorf("task %d not found", req.TaskId)
	}
	p.Status = download.Status(req.Status)
	if req.SavePath != nil {
		p.SavePath = *req.SavePath
	}
	p.SyncDB()
	return &proto.UpdateDownloadStatusResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) Download(ctx context.Context, req *proto.DownloadRequest) (*proto.DownloadResponse, error) {
	res, err := download.Download(req.DownloadPath, req.Url, req.Headers, req.MediaType, req.Title, req.Package, req.Key, req.DetailUrl, req.WatchUrl)
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

	return &proto.DownloadResponse{
		TaskId:         int32(res.TaskID),
		VariantSummary: protoVariants,
		IsDownloading:  res.IsDownloading,
	}, nil
}

func (s *MiruCoreServer) GetAllDownloads(ctx context.Context, req *proto.GetAllDownloadsRequest) (*proto.GetAllDownloadsResponse, error) {
	downloads, err := db.GetAllDownloads()
	if err != nil {
		return nil, err
	}
	protoDownloads := make([]*proto.Download, len(downloads))
	for i, d := range downloads {
		protoDownloads[i] = &proto.Download{
			Id:      int32(d.ID),
			Url:     d.URL,
			Headers: d.Headers,
			Package: d.Package,
			Progress: func() []int32 {
				res := make([]int32, len(d.Progress))
				for i, v := range d.Progress {
					res[i] = int32(v)
				}
				return res
			}(),
			Key:       d.Key,
			Title:     d.Title,
			MediaType: d.MediaType,
			Status:    d.Status,
			SavePath:  d.SavePath,
			Date:      d.Date.Format(time.RFC3339),
		}
	}
	return &proto.GetAllDownloadsResponse{Downloads: protoDownloads}, nil
}

func (s *MiruCoreServer) GetDownloadsByPackageAndDetailUrl(ctx context.Context, req *proto.GetDownloadsByPackageAndDetailUrlRequest) (*proto.GetDownloadsByPackageAndDetailUrlResponse, error) {
	downloads, err := db.GetDownloadsByPackageAndDetailUrl(req.Package, req.DetailUrl)
	if err != nil {
		return nil, err
	}
	protoDownloads := make([]*proto.Download, len(downloads))
	for i, d := range downloads {
		protoDownloads[i] = &proto.Download{
			Id:      int32(d.ID),
			Url:     d.URL,
			Headers: d.Headers,
			Package: d.Package,
			Progress: func() []int32 {
				res := make([]int32, len(d.Progress))
				for i, v := range d.Progress {
					res[i] = int32(v)
				}
				return res
			}(),
			Key:       d.Key,
			Title:     d.Title,
			MediaType: d.MediaType,
			Status:    d.Status,
			SavePath:  d.SavePath,
			Date:      d.Date.Format(time.RFC3339),
		}
	}
	return &proto.GetDownloadsByPackageAndDetailUrlResponse{Downloads: protoDownloads}, nil
}

func (s *MiruCoreServer) GetDownloadByPackageWatchUrlDetailUrl(ctx context.Context, req *proto.GetDownloadByPackageWatchUrlDetailUrlRequest) (*proto.GetDownloadByPackageWatchUrlDetailUrlResponse, error) {
	d, err := db.GetDownloadByPackageWatchUrlDetailUrl(req.Package, req.WatchUrl, req.DetailUrl)
	if d == nil {
		return nil, err
	}
	return &proto.GetDownloadByPackageWatchUrlDetailUrlResponse{Download: &proto.Download{
		Id:      int32(d.ID),
		Url:     d.URL,
		Headers: d.Headers,
		Package: d.Package,
		Progress: func() []int32 {
			res := make([]int32, len(d.Progress))
			for i, v := range d.Progress {
				res[i] = int32(v)
			}
			return res
		}(),
		Key:       d.Key,
		Title:     d.Title,
		MediaType: d.MediaType,
		Status:    d.Status,
		SavePath:  d.SavePath,
		Date:      d.Date.Format(time.RFC3339),
	}}, nil
}

func (s *MiruCoreServer) DeleteDownload(ctx context.Context, req *proto.DeleteDownloadRequest) (*proto.DeleteDownloadResponse, error) {
	d, err := db.GetDownloadByID(int(req.Id))
	if err == nil && d.MediaType == "torrent" {
		torrent.DeleteTorrent(d.Key, true)
	}
	err = db.DeleteDownloadByID(int(req.Id))
	if err != nil {
		return nil, err
	}
	return &proto.DeleteDownloadResponse{Message: "Success"}, nil
}
