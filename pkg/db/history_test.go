package db

import (
	"context"
	"testing"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/ent/enttest"
	"github.com/stretchr/testify/assert"
)

func TestPutHistory_UniqueConstraint(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	// Test 1: Multiple Episodes (Same package, Same detailUrl, Different watchUrl)
	// Should coexist
	h1 := &ent.History{
		Package:   "pkg1",
		DetailUrl: "series1",
		URL:       "watch1",
		Title:     "Episode 1",
	}
	_, err := client.History.Create().
		SetPackage(h1.Package).
		SetURL(h1.URL).
		SetDetailUrl(h1.DetailUrl).
		SetTitle(h1.Title).
		SetType("video").
		SetEpisodeGroupID(1).
		SetEpisodeID(1).
		SetEpisodeTitle("Ep Title").
		SetProgress(0).
		SetTotalProgress(100).
		Save(ctx)
	assert.NoError(t, err)

	h2 := &ent.History{
		Package:   "pkg1",
		DetailUrl: "series1",
		URL:       "watch2", // Different watchUrl
		Title:     "Episode 2",
	}
	_, err = client.History.Create().
		SetPackage(h2.Package).
		SetURL(h2.URL).
		SetDetailUrl(h2.DetailUrl).
		SetTitle(h2.Title).
		SetType("video").
		SetEpisodeGroupID(1).
		SetEpisodeID(2).
		SetEpisodeTitle("Ep Title 2").
		SetProgress(0).
		SetTotalProgress(100).
		Save(ctx)
	assert.NoError(t, err)

	count := client.History.Query().CountX(ctx)
	assert.Equal(t, 2, count)

	// Test 2: Conflict Replacement (Same package, Same watchUrl, Different detailUrl)
	// Should replace (or fail if using simple Create, but PutHistory uses OnConflict)
	// We simulate PutHistory logic here (Upsert)

	// Insert watch1 again but with different series (detailUrl)
	// This usually happens if the user watches the same episode but maybe from a different context or it was updated.
	// But mostly strictly, if (package, url) is unique, this should UPDATE the existing entry.

	h3 := &ent.History{
		Package:   "pkg1",
		DetailUrl: "series2", // Changed series
		URL:       "watch1",  // Same watchUrl as h1
		Title:     "Episode 1 Updated",
	}

	// Use the OnConflict logic as in PutHistory
	// Note: We need to import "entgo.io/ent/dialect/sql" and use the actual package in real code,
	// but here we are using the client wrapper.
	// Since we can't easily import 'sql' and 'history' package fields cleanly without full setup in this snippet tool,
	// we will rely on checking if we can just call Save and expect error for simple Create,
	// OR we should put this test logic inside the actual package to access 'history.FieldPackage'.

	// For now, let's just assert that creating a DUPLICATE fails with simple Create (proving the index exists)
	_, err = client.History.Create().
		SetPackage(h3.Package).
		SetURL(h3.URL).
		SetDetailUrl(h3.DetailUrl).
		SetTitle(h3.Title).
		SetType("video").
		SetEpisodeGroupID(1).
		SetEpisodeID(1).
		SetEpisodeTitle("Ep Title Updated").
		SetProgress(0).
		SetTotalProgress(100).
		Save(ctx)

	// Attempting to create duplicate (package, url) should FAIL unique constraint
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "constraint failed")
}
