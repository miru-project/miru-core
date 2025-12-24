package db

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/ent/detail"
	"github.com/miru-project/miru-core/ent/predicate"
	"github.com/miru-project/miru-core/ext"
)

// GetDetailByPackageAndUrl returns a detail by package and detailUrl.
func GetDetailByPackageAndUrl(pkg, url string) (*ent.Detail, error) {
	client := ext.EntClient()
	return client.Detail.Query().
		Where(detail.Package(pkg), detail.DetailUrl(url)).
		First(context.Background())
}

// UpsertDetail creates or updates a detail.
func UpsertDetail(d *ent.Detail) (*ent.Detail, error) {
	client := ext.EntClient()
	// Create an upsert on conflict
	// It's safer to query first to get ID for OnConflict update or utilize Ent's OnConflict features if available for all fields
	// Here we use OnConflict columns.

	// Since we haven't enabled the specific upsert feature flags or complex mixins, we can use the OnConflict builder.
	// Note: 'downloaded' is JSON, appending might be tricky in one go without reading, but requirements say "replace old one if conflict".
	// The requirement "1.field called "Downloaded" which contains list of episode url that downloaded"
	// implies we probably want to PRESERVE downloaded if we are just updating the detail info,
	// OR if the frontend sends the full latest state including downloaded, we overwrite.
	// The prompt says: "front end first request the db (detail) and show it in detail, later request js extension and overwrite the old detail. If fetch extension detail sucessfully, replace the old db entry."
	// AND "field called "Downloaded" which contains list of episode url that downloaded"
	// This implies the "Downloaded" field is stateful and should probably NOT be overwritten by the extension data which lacks it.
	// However, the prompt also says "replace the old db entry".
	// Let's assume the frontend sends the correct 'Downloaded' list (merging state there) OR we handle merge here.
	// For now, naive Upsert replacing everything provided.

	// IMPORTANT: Use proper bulk upsert or OnConflict.
	// Use the style from history.go for consistency
	id, err := client.Detail.Create().
		SetNillableTitle(d.Title).
		SetNillableCover(d.Cover).
		SetNillableDesc(d.Desc).
		SetDetailUrl(d.DetailUrl).
		SetPackage(d.Package).
		SetNillableEpisodes(d.Episodes).
		SetNillableHeaders(d.Headers).
		SetDownloaded(d.Downloaded).
		OnConflict(
			sql.ConflictColumns(detail.FieldPackage, detail.FieldDetailUrl),
			sql.ResolveWithNewValues(),
		).
		ID(context.Background())

	if err != nil {
		return nil, err
	}
	d.ID = id
	return d, nil
}

// UpdateDetailDownloaded updates only the downloaded field for a specific detail.
func UpdateDetailDownloaded(pkg, url string, downloaded []string) error {
	client := ext.EntClient()
	_, err := client.Detail.Update().
		Where(detail.Package(pkg), detail.DetailUrl(url)).
		SetDownloaded(downloaded).
		Save(context.Background())
	return err
}

// Helper query building
func GetDetails(opts ...func(*ent.DetailQuery)) ([]*ent.Detail, error) {
	client := ext.EntClient()
	q := client.Detail.Query()
	for _, opt := range opts {
		opt(q)
	}
	return q.All(context.Background())
}

func WithDetailPackage(pkg string) func(*ent.DetailQuery) {
	return func(q *ent.DetailQuery) {
		q.Where(detail.Package(pkg))
	}
}

// Filter helpers
func DetailFilter(ps ...predicate.Detail) func(*ent.DetailQuery) {
	return func(q *ent.DetailQuery) {
		q.Where(ps...)
	}
}
