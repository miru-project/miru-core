package grpc

import (
	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/pkg/download"
	"github.com/miru-project/miru-core/pkg/event"
	"github.com/miru-project/miru-core/pkg/extension/js"
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
				// The payload is the merged Go + JS snapshot published by
				// handler.BuildExtensionMeta and converted by the same helper
				// HelloMiru uses. It used to read the JS runtime's own cache, so any
				// JS-side change (install, hot reload, lazy load on first use, or an
				// extension merely reporting an error) pushed a JS-only list, and the
				// frontend replaces its whole list on receipt — which wiped every Go
				// extension from the UI until the app restarted.
				exts := e.Data.([]*js.Ext)
				protoExtMeta := toProtoExtensionMeta(exts)
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
			case event.DevLog:
				raw := e.Data.(*proto.DevLogEvent)
				resp = &proto.WatchEventsResponse{
					Event: &proto.WatchEventsResponse_DevLogEvent{
						DevLogEvent: &proto.DevLogEvent{
							Package:   sanitizeUTF8(raw.Package),
							Message:   sanitizeUTF8(raw.Message),
							Level:     sanitizeUTF8(raw.Level),
							Timestamp: raw.Timestamp,
						},
					},
				}
			case event.DevNetwork:
				raw := e.Data.(*proto.DevNetworkEvent)
				resp = &proto.WatchEventsResponse{
					Event: &proto.WatchEventsResponse_DevNetworkEvent{
						DevNetworkEvent: &proto.DevNetworkEvent{
							Package:         sanitizeUTF8(raw.Package),
							Url:             sanitizeUTF8(raw.Url),
							Method:          sanitizeUTF8(raw.Method),
							Status:          raw.Status,
							Duration:        raw.Duration,
							Timestamp:       raw.Timestamp,
							RequestHeaders:  sanitizeUTF8(raw.RequestHeaders),
							RequestBody:     sanitizeUTF8(raw.RequestBody),
							ResponseHeaders: sanitizeUTF8(raw.ResponseHeaders),
							ResponseBody:    sanitizeUTF8(raw.ResponseBody),
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
