package db

import (
	"context"
	"time"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/ent/favorite"
	"github.com/miru-project/miru-core/ent/history"
	"github.com/miru-project/miru-core/ext"
)

// GetFavoritesByType returns favorites optionally filtered by type and limited.
func GetFavoritesByType(t *string, limit *int) ([]*ent.Favorite, error) {
	client := ext.EntClient()
	ctx := context.Background()

	q := client.Favorite.Query()
	if t != nil {
		q = q.Where(favorite.TypeEQ(*t))
	}
	q = q.Order(ent.Desc(favorite.FieldDate))
	if limit != nil && *limit > 0 {
		q = q.Limit(*limit)
	}
	return q.All(ctx)
}

// GetHistorysByType returns histories optionally filtered by type ordered by date desc.
func GetHistorysByType(t *string) ([]*ent.History, error) {
	client := ext.EntClient()
	ctx := context.Background()
	q := client.History.Query()
	if t != nil {
		q = q.Where(history.TypeEQ(*t))
	}
	q = q.Order(ent.Desc(history.FieldDate))
	return q.All(ctx)
}

// GetHistoryByPackageAndUrl returns the first history matching package and url or (nil, nil) if not found.
func GetHistoryByPackageAndUrl(pkg string, url string) (*ent.History, error) {
	client := ext.EntClient()
	ctx := context.Background()

	h, err := client.History.Query().Where(history.PackageEQ(pkg), history.URLEQ(url)).First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return h, nil
}

// PutHistory creates or updates a history record. Returns the ID of the saved record.
func PutHistory(h *ent.History) (int, error) {
	client := ext.EntClient()
	ctx := context.Background()

	// Try to find existing by package and url
	existing, err := client.History.Query().Where(history.PackageEQ(h.Package), history.URLEQ(h.URL)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return 0, err
	}
	if existing != nil {
		// update fields
		u := existing.Update()
		if h.Cover != nil {
			u = u.SetCover(*h.Cover)
		} else {
			u = u.ClearCover()
		}
		u = u.SetType(h.Type)
		u = u.SetEpisodeGroupID(h.EpisodeGroupID)
		u = u.SetEpisodeID(h.EpisodeID)
		u = u.SetTitle(h.Title)
		u = u.SetEpisodeTitle(h.EpisodeTitle)
		u = u.SetProgress(h.Progress)
		u = u.SetTotalProgress(h.TotalProgress)
		u = u.SetDate(time.Now())

		nh, err := u.Save(ctx)
		if err != nil {
			return 0, err
		}
		return nh.ID, nil
	}

	// create
	c := client.History.Create().SetPackage(h.Package).SetURL(h.URL)
	if h.Cover != nil {
		c = c.SetCover(*h.Cover)
	}
	c = c.SetType(h.Type).
		SetEpisodeGroupID(h.EpisodeGroupID).
		SetEpisodeID(h.EpisodeID).
		SetTitle(h.Title).
		SetEpisodeTitle(h.EpisodeTitle).
		SetProgress(h.Progress).
		SetTotalProgress(h.TotalProgress).
		SetDate(time.Now())

	nh, err := c.Save(ctx)
	if err != nil {
		return 0, err
	}
	return nh.ID, nil
}

// DeleteHistoryByPackageAndUrl deletes history records matching package and url.
func DeleteHistoryByPackageAndUrl(pkg string, url string) error {
	client := ext.EntClient()
	ctx := context.Background()

	_, err := client.History.Delete().Where(history.PackageEQ(pkg), history.URLEQ(url)).Exec(ctx)
	return err
}

// DeleteAllHistory deletes all history records and returns the number deleted.
func DeleteAllHistory() (int, error) {
	client := ext.EntClient()
	ctx := context.Background()

	n, err := client.History.Delete().Exec(ctx)
	return n, err
}
