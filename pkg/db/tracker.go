package db

import (
	"context"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/ent/detail"
	"github.com/miru-project/miru-core/ent/tracker"
	"github.com/miru-project/miru-core/ext"
)

// UpsertTracker creates or updates a tracker.
func UpsertTracker(detailUrl string, pkg string, t *ent.Tracker) (*ent.Tracker, error) {
	client := ext.EntClient()

	// Get Detail
	d, err := client.Detail.Query().Where(detail.Package(pkg), detail.DetailUrl(detailUrl)).Only(context.Background())
	if err != nil {
		return nil, err
	}

	// Try to find existing tracker by tracker_id and provider
	existing, _ := client.Tracker.Query().
		Where(
			tracker.TrackerID(t.TrackerID),
			tracker.ProviderEQ(t.Provider),
		).Only(context.Background())

	if existing != nil {
		// Update tracker info
		updated, err := client.Tracker.UpdateOne(existing).
			SetStatus(t.Status).
			SetProgress(t.Progress).
			SetNillableScore(t.Score).
			SetNillableStartDate(t.StartDate).
			SetNillableFinishDate(t.FinishDate).
			SetNillableTotalProgress(t.TotalProgress).
			Save(context.Background())
		if err != nil {
			return nil, err
		}

		// Update TrackIds map and ensure it's linked
		trackIds := d.TrackIds
		if trackIds == nil {
			trackIds = make(map[string]string)
		}
		trackIds[string(updated.Provider)] = updated.TrackerID

		err = client.Detail.UpdateOne(d).
			AddTrackers(updated).
			SetTrackIds(trackIds).
			Exec(context.Background())
		if err != nil {
			return nil, err
		}
		return updated, nil
	}

	// Create and link
	res, err := client.Tracker.Create().
		AddDetails(d).
		SetTrackerID(t.TrackerID).
		SetProvider(t.Provider).
		SetStatus(t.Status).
		SetProgress(t.Progress).
		SetNillableScore(t.Score).
		SetNillableStartDate(t.StartDate).
		SetNillableFinishDate(t.FinishDate).
		SetNillableTotalProgress(t.TotalProgress).
		Save(context.Background())

	if err != nil {
		return nil, err
	}

	// Update TrackIds map on detail
	trackIds := d.TrackIds
	if trackIds == nil {
		trackIds = make(map[string]string)
	}
	trackIds[string(res.Provider)] = res.TrackerID
	client.Detail.UpdateOne(d).
		SetTrackIds(trackIds).
		Exec(context.Background())

	return res, nil
}

// DeleteTracker (Unlink) removes a tracker relationship.
// If no more details are connected to the tracker, it deletes the tracker record.
func DeleteTracker(detailUrl string, pkg string, provider string) error {
	client := ext.EntClient()

	d, err := client.Detail.Query().Where(detail.Package(pkg), detail.DetailUrl(detailUrl)).Only(context.Background())
	if err != nil {
		return err
	}

	// Find the tracker for this detail and provider
	t, err := client.Tracker.Query().
		Where(
			tracker.ProviderEQ(tracker.Provider(provider)),
			tracker.HasDetailsWith(detail.ID(d.ID)),
		).Only(context.Background())
	if err != nil {
		if ent.IsNotFound(err) {
			return nil
		}
		return err
	}

	// Remove relationship
	err = client.Tracker.UpdateOne(t).
		RemoveDetails(d).
		Exec(context.Background())
	if (err != nil) {
		return err
	}

	// Remove from TrackIds map on detail
	trackIds := d.TrackIds
	if trackIds != nil {
		delete(trackIds, provider)
		client.Detail.UpdateOne(d).
			SetTrackIds(trackIds).
			Exec(context.Background())
	}

	// Check if any details are still connected
	count, err := client.Detail.Query().
		Where(detail.HasTrackersWith(tracker.ID(t.ID))).
		Count(context.Background())
	if err != nil {
		return err
	}

	if count == 0 {
		// Last one, delete tracker
		return client.Tracker.DeleteOne(t).Exec(context.Background())
	}

	return nil
}

// DeleteTrackerByTrackerId deletes a tracker and its associated cache (Track).
// It also removes the tracker ID from the TrackIds map of all associated details.
func DeleteTrackerByTrackerId(trackerId string, provider string) error {
	client := ext.EntClient()
	ctx := context.Background()

	// Find all details associated with this tracker
	details, err := client.Detail.Query().
		Where(detail.HasTrackersWith(
			tracker.TrackerID(trackerId),
			tracker.ProviderEQ(tracker.Provider(provider)),
		)).
		All(ctx)
	
	if err == nil {
		for _, d := range details {
			if d.TrackIds != nil {
				delete(d.TrackIds, provider)
				client.Detail.UpdateOne(d).
					SetTrackIds(d.TrackIds).
					Exec(ctx)
			}
		}
	}

	// Delete from Tracker
	_, err = client.Tracker.Delete().
		Where(
			tracker.TrackerID(trackerId),
			tracker.ProviderEQ(tracker.Provider(provider)),
		).Exec(ctx)
	if err != nil {
		return err
	}

	// Delete from Track cache if it exists
	return DeleteTrack(trackerId, provider)
}
