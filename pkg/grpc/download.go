package grpc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/download"
	"github.com/miru-project/miru-core/pkg/torrent"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

// GetStorageStats returns per-category storage usage for the given download
// path, including bytes occupied by in-progress (temp) downloads. It groups the
// persisted download rows by content category and sums the on-disk size of each
// save_path, then adds the partial bytes of currently active tasks.
func (s *MiruCoreServer) GetStorageStats(ctx context.Context, req *proto.GetStorageStatsRequest) (*proto.GetStorageStatsResponse, error) {
	var video, manga, novel int64

	// 1) Completed (persisted) downloads grouped by category.
	downloads, err := db.GetAllDownloads()
	if err != nil {
		return nil, err
	}
	completedPaths := make(map[string]struct{})
	for _, d := range downloads {
		sz := fileSizeOnDisk(d.SavePath)
		if sz > 0 {
			completedPaths[d.SavePath] = struct{}{}
		}
		switch d.Category {
		case "video":
			video += sz
		case "manga":
			manga += sz
		case "novel":
			novel += sz
		}
	}

	// 2) In-progress (temp) downloads: partial bytes not already counted as a
	//    completed save_path. Active tasks expose CurrentDownloading (the live
	//    partial file) and, for HLS, SavePath (the segment directory).
	var temp int64
	for _, p := range download.DownloadStatus() {
		if p.Status != download.Downloading && p.Status != download.Paused &&
			p.Status != download.Queued && p.Status != download.Converting {
			continue
		}
		for _, path := range []string{p.CurrentDownloading, p.SavePath} {
			if path == "" {
				continue
			}
			if _, done := completedPaths[path]; done {
				continue
			}
			temp += fileSizeOnDisk(path)
		}
	}

	total := video + manga + novel + temp
	return &proto.GetStorageStatsResponse{
		Stats: &proto.StorageStats{
			VideoBytes: video,
			MangaBytes: manga,
			NovelBytes: novel,
			TempBytes:  temp,
			TotalBytes: total,
		},
	}, nil
}

// fileSizeOnDisk returns the on-disk size (bytes) of a file or directory.
// Returns 0 for empty/missing paths; all filesystem access is guarded.
func fileSizeOnDisk(path string) int64 {
	if path == "" {
		return 0
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	if info.IsDir() {
		var total int64
		_ = filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
			if err == nil && fi != nil && !fi.IsDir() {
				total += fi.Size()
			}
			return nil
		})
		return total
	}
	return info.Size()
}

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
	p.Status = download.StatusFromProto(req.Status)
	if req.SavePath != nil {
		p.SavePath = *req.SavePath
	}
	p.SyncDB()
	return &proto.UpdateDownloadStatusResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) SetDownloadPriority(ctx context.Context, req *proto.SetDownloadPriorityRequest) (*proto.SetDownloadPriorityResponse, error) {
	if err := download.SetPriority(int(req.TaskId), int(req.Priority)); err != nil {
		return nil, err
	}
	return &proto.SetDownloadPriorityResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) SetDownloadConcurrent(ctx context.Context, req *proto.SetDownloadConcurrentRequest) (*proto.SetDownloadConcurrentResponse, error) {
	download.SetMaxConcurrent(int(req.MaxConcurrent))
	return &proto.SetDownloadConcurrentResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) ReorderDownloads(ctx context.Context, req *proto.ReorderDownloadsRequest) (*proto.ReorderDownloadsResponse, error) {
	ids := make([]int, len(req.OrderedTaskIds))
	for i, v := range req.OrderedTaskIds {
		ids[i] = int(v)
	}
	if err := download.ReorderTasks(ids); err != nil {
		return nil, err
	}
	return &proto.ReorderDownloadsResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) Download(ctx context.Context, req *proto.DownloadRequest) (*proto.DownloadResponse, error) {
	res, err := download.Download(req.DownloadPath, req.Url, req.Headers, string(mediaTypeFromProto(req.GetMediaType())), req.Title, req.Package, req.Key, req.DetailUrl, req.WatchUrl, download.CategoryFromProto(req.GetCategory()))
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
			MediaType: mediaTypeToProto(download.MediaType(d.MediaType)),
			Status:    download.StatusToProto(download.Status(d.Status)),
			SavePath:  d.SavePath,
			Date:      d.Date.Format(time.RFC3339),
			Priority:  int32(d.Priority),
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
			MediaType: mediaTypeToProto(download.MediaType(d.MediaType)),
			Status:    download.StatusToProto(download.Status(d.Status)),
			SavePath:  d.SavePath,
			Date:      d.Date.Format(time.RFC3339),
			Priority:  int32(d.Priority),
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
		MediaType: mediaTypeToProto(download.MediaType(d.MediaType)),
		Status:    download.StatusToProto(download.Status(d.Status)),
		SavePath:  d.SavePath,
		Date:      d.Date.Format(time.RFC3339),
		Priority:  int32(d.Priority),
	}}, nil
}

func (s *MiruCoreServer) DeleteDownload(ctx context.Context, req *proto.DeleteDownloadRequest) (*proto.DeleteDownloadResponse, error) {
	d, err := db.GetDownloadByID(int(req.Id))
	if err == nil && download.MediaType(d.MediaType) == download.Torrent {
		torrent.DeleteTorrent(d.Key, true)
	}
	err = db.DeleteDownloadByID(int(req.Id))
	if err != nil {
		return nil, err
	}
	return &proto.DeleteDownloadResponse{Message: "Success"}, nil
}
