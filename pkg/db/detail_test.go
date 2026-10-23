package db

import (
	"context"
	"testing"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/ent/enttest"
	"github.com/miru-project/miru-core/ext"
	"github.com/stretchr/testify/assert"
	_ "modernc.org/sqlite"
)

func TestDetail(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1&_pragma=foreign_keys(1)")
	defer client.Close()
	ext.SetEntClientForTest(client)
	defer ext.SetEntClientForTest(nil)

	ctx := context.Background()

	// Clean up before test
	client.Detail.Delete().ExecX(ctx)

	t.Run("UpsertDetail", func(t *testing.T) {
		pkg := "com.example.test"
		url := "http://example.com/detail/1"
		title := "Test Detail"
		downloaded := []string{"ep1", "ep2"}

		d := client.Detail.Create().
			SetPackage(pkg).
			SetDetailUrl(url).
			SetTitle(title).
			SetDownloaded(downloaded).
			SaveX(ctx)

		upserted, err := UpsertDetail(d)
		assert.NoError(t, err)
		assert.NotNil(t, upserted)
		assert.Equal(t, title, *upserted.Title)
		assert.Equal(t, downloaded, upserted.Downloaded)

		// Test Update via Upsert
		newTitle := "Updated Title"
		d.Title = &newTitle
		updated, err := UpsertDetail(d)
		assert.NoError(t, err)
		assert.Equal(t, newTitle, *updated.Title)

		// Verify in DB
		fetched, err := GetDetailByPackageAndUrl(pkg, url)
		assert.NoError(t, err)
		assert.Equal(t, newTitle, *fetched.Title)
	})

	t.Run("UpdateDetailDownloaded", func(t *testing.T) {
		pkg := "com.example.test2"
		url := "http://example.com/detail/2"
		title := "Test Detail 2"

		// Create initial
		client.Detail.Create().
			SetPackage(pkg).
			SetDetailUrl(url).
			SetTitle(title).
			ExecX(ctx)

		newDownloaded := []string{"epA", "epB"}
		err := UpdateDetailDownloaded(pkg, url, newDownloaded)
		assert.NoError(t, err)

		fetched, err := GetDetailByPackageAndUrl(pkg, url)
		assert.NoError(t, err)
		assert.Equal(t, newDownloaded, fetched.Downloaded)
	})
}

// TestUpsertDetailTagsPreservation proves the tag column survives an upsert
// from a client that does not carry tags, and is replaced when it does.
func TestUpsertDetailTagsPreservation(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:enttags?mode=memory&cache=shared&_fk=1&_pragma=foreign_keys(1)")
	defer client.Close()
	ext.SetEntClientForTest(client)
	defer ext.SetEntClientForTest(nil)

	ctx := context.Background()
	pkg := "com.example.tags"
	url := "http://example.com/detail/tags"

	_, err := client.Detail.Create().
		SetPackage(pkg).
		SetDetailUrl(url).
		SetTitle("Tagged").
		SetTags([]string{"action", "comedy"}).
		Save(ctx)
	assert.NoError(t, err)

	// Upsert WITHOUT tags (older frontend / partial refresh) keeps them.
	d := &ent.Detail{Package: pkg, DetailUrl: url}
	_, err = UpsertDetail(d)
	assert.NoError(t, err)
	fetched, err := GetDetailByPackageAndUrl(pkg, url)
	assert.NoError(t, err)
	assert.Equal(t, []string{"action", "comedy"}, fetched.Tags,
		"an upsert that omits tags must not wipe the stored list")

	// Upsert WITH tags replaces them.
	newTitle := "Tagged v2"
	d2 := &ent.Detail{Package: pkg, DetailUrl: url, Title: &newTitle, Tags: []string{"drama"}}
	_, err = UpsertDetail(d2)
	assert.NoError(t, err)
	fetched, err = GetDetailByPackageAndUrl(pkg, url)
	assert.NoError(t, err)
	assert.Equal(t, []string{"drama"}, fetched.Tags)
}
