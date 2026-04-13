package db

import (
	"context"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/ent/track"
	"github.com/miru-project/miru-core/ext"
)

// GetTrack returns a Track record by its Tracking ID and Provider.
func GetTrack(trackingID string, provider string) (*ent.Track, error) {
	client := ext.EntClient()
	ctx := context.Background()

	return client.Track.Query().
		Where(track.TrackingID(trackingID), track.ProviderEQ(track.Provider(provider))).
		Only(ctx)
}

// PutTrack creates or updates a Track record.
func PutTrack(trackingID string, data string, mediaType string, provider string) (*ent.Track, error) {
	client := ext.EntClient()
	ctx := context.Background()

	existing, err := client.Track.Query().
		Where(track.TrackingID(trackingID), track.ProviderEQ(track.Provider(provider))).
		Only(ctx)

	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	if existing != nil {
		return existing.Update().
			SetData(data).
			SetMediaType(mediaType).
			Save(ctx)
	}

	return client.Track.Create().
		SetTrackingID(trackingID).
		SetData(data).
		SetMediaType(mediaType).
		SetProvider(track.Provider(provider)).
		Save(ctx)
}
