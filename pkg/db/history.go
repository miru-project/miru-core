package db

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
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
	if t != nil && *t != "" {
		q = q.Where(favorite.TypeEQ(*t))
	}
	q = q.Order(ent.Desc(favorite.FieldDate))
	if limit != nil && *limit > 0 {
		q = q.Limit(*limit)
	}
	return q.All(ctx)
}

// GetHistoriesByType returns histories optionally filtered by type ordered by date desc.
func GetHistoriesByType(t *string) ([]*ent.History, error) {
	client := ext.EntClient()
	ctx := context.Background()
	q := client.History.Query()
	if t != nil && *t != "" {
		q = q.Where(history.TypeEQ(*t))
	}

	q = q.Order(ent.Desc(history.FieldDate))
	return q.All(ctx)
}

// GetHistoryByPackageAndDetailUrl returns all history matching package and detailUrl.
func GetHistoryByPackageAndDetailUrl(pkg string, detailUrl string) ([]*ent.History, error) {
	client := ext.EntClient()
	ctx := context.Background()

	return client.History.Query().
		Where(history.PackageEQ(pkg), history.DetailUrlEQ(detailUrl)).
		Order(ent.Desc(history.FieldDate)).
		All(ctx)
}

// PutHistory creates or updates a history record. Returns the ID of the saved record.
func PutHistory(h *ent.History) (int, error) {
	client := ext.EntClient()
	ctx := context.Background()

	// Try to find existing by package, url and detailUrl
	existing, err := client.History.Query().
		Where(history.PackageEQ(h.Package), history.URLEQ(h.URL), history.DetailUrlEQ(h.DetailUrl)).
		Only(ctx)
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
		u = u.SetDetailUrl(h.DetailUrl)
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
	c.OnConflict(sql.ConflictColumns(history.FieldPackage, history.FieldURL, history.FieldDetailUrl), sql.ResolveWithNewValues())
	if h.Cover != nil {
		c = c.SetCover(*h.Cover)
	}
	c = c.SetType(h.Type).
		SetDetailUrl(h.DetailUrl).
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

// GetHistorysFiltered returns histories optionally filtered by type and date.
func GetHistorysFiltered(t *string, beforeDate *time.Time) ([]*ent.History, error) {
	client := ext.EntClient()
	ctx := context.Background()
	q := client.History.Query()
	if t != nil && *t != "" {
		q = q.Where(history.TypeEQ(*t))
	}
	if beforeDate != nil {
		q = q.Where(history.DateLT(*beforeDate))
	}

	return q.Order(ent.Desc(history.FieldDate)).All(ctx)
}
