package db

import (
	"context"
	"testing"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/ent/enttest"
	"github.com/stretchr/testify/assert"
)

func TestPutHistory_UniqueConstraint(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1&_pragma=foreign_keys(1)")
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

	// Test 2: Conflict on unique index (package, url, detailUrl)
	// Inserting a record with the same (package, url, detailUrl) as h1 should
	// FAIL the unique constraint.

	h3 := &ent.History{
		Package:   "pkg1",
		DetailUrl: "series1", // Same as h1
		URL:       "watch1",  // Same as h1
		Title:     "Episode 1 Updated",
	}

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

	// Attempting to create duplicate (package, url, detailUrl) should FAIL unique constraint
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "constraint failed")
}
