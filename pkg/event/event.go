package event

import (
	"sync"
)

type EventType string

const (
	DownloadStatusUpdate EventType = "download_status_update"
	ExtensionUpdate      EventType = "extension_update"
)

type Event struct {
	Type EventType
	Data any
}

type Bus struct {
	subscribers map[chan Event]struct{}
	lock        sync.RWMutex
}

var GlobalBus = &Bus{
	subscribers: make(map[chan Event]struct{}),
}

func (b *Bus) Subscribe() chan Event {
	b.lock.Lock()
	defer b.lock.Unlock()
	ch := make(chan Event, 100)
	b.subscribers[ch] = struct{}{}
	return ch
}

func (b *Bus) Unsubscribe(ch chan Event) {
	b.lock.Lock()
	defer b.lock.Unlock()
	delete(b.subscribers, ch)
	close(ch)
}

func (b *Bus) Publish(e Event) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- e:
		default:
			// Buffer full, skip or handle accordingly
		}
	}
}

func SendDownloadUpdate(data any) {
	GlobalBus.Publish(Event{
		Type: DownloadStatusUpdate,
		Data: data,
	})
}

func SendExtensionUpdate(data any) {
	GlobalBus.Publish(Event{
		Type: ExtensionUpdate,
		Data: data,
	})
}
