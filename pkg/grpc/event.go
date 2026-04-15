package grpc

import (
	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/pkg/download"
	"github.com/miru-project/miru-core/pkg/event"
	"github.com/miru-project/miru-core/pkg/jsExtension"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

func (s *MiruCoreServer) WatchEvents(req *proto.WatchEventsRequest, stream proto.EventService_WatchEventsServer) error {
	ch := event.GlobalBus.Subscribe()
	defer event.GlobalBus.Unsubscribe(ch)

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case e := <-ch:
			var resp *proto.WatchEventsResponse
			switch e.Type {
			case event.DownloadStatusUpdate:
				status := e.Data.(map[int]*download.Progress)
				protoStatus := make(map[int32]*proto.DownloadProgress)
				for id, p := range status {
					protoStatus[int32(id)] = toProtoDownloadProgress(p)
				}
				resp = &proto.WatchEventsResponse{
					Event: &proto.WatchEventsResponse_DownloadEvent{
						DownloadEvent: &proto.DownloadEvent{
							DownloadStatus: protoStatus,
						},
					},
				}
			case event.ExtensionUpdate:
				exts := e.Data.([]*jsExtension.ExtApi)
				protoExtMeta := make([]*proto.ExtensionMeta, len(exts))
				for i, ea := range exts {
					e := ea.Ext
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
				resp = &proto.WatchEventsResponse{
					Event: &proto.WatchEventsResponse_ExtensionEvent{
						ExtensionEvent: &proto.ExtensionEvent{
							ExtensionMeta: protoExtMeta,
						},
					},
				}
			case event.HistoryUpdate:
				history := e.Data.([]*ent.History)
				protoHistory := make([]*proto.History, len(history))
				for i, h := range history {
					protoHistory[i] = toProtoHistory(h)
				}
				resp = &proto.WatchEventsResponse{
					Event: &proto.WatchEventsResponse_HistoryEvent{
						HistoryEvent: &proto.HistoryEvent{
							History: protoHistory,
						},
					},
				}
			}

			if resp != nil {
				if err := stream.Send(resp); err != nil {
					return err
				}
			}
		}
	}
}
