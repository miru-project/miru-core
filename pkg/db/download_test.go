package db

import (
	"context"
	"testing"
	"time"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/ent/download"
	"github.com/miru-project/miru-core/ent/enttest"
	"github.com/stretchr/testify/assert"
)

func TestUpsertDownload(t *testing.T) {
	// We can't easily test the actual UpsertDownload function because it uses ext.EntClient()
	// which relies on a global configuration. Instead, we test the logic it performs.

	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	// Helper function that mimics UpsertDownload logic but takes the client
	upsert := func(d *ent.Download) (*ent.Download, error) {
		old, err := client.Download.Query().Where(download.Key(d.Key)).Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return client.Download.Create().
					SetURL(d.URL).
					SetHeaders(d.Headers).
					SetPackage(d.Package).
					SetProgress(d.Progress).
					SetKey(d.Key).
					SetTitle(d.Title).
					SetMediaType(d.MediaType).
					SetStatus(d.Status).
					SetSavePath(d.SavePath).
					SetDate(time.Now()).
					Save(ctx)
			}
			return nil, err
		}
		return client.Download.UpdateOne(old).
			SetURL(d.URL).
			SetHeaders(d.Headers).
			SetPackage(d.Package).
			SetProgress(d.Progress).
			SetTitle(d.Title).
			SetMediaType(d.MediaType).
			SetStatus(d.Status).
			SetSavePath(d.SavePath).
			SetDate(time.Now()).
			Save(ctx)
	}

	d := &ent.Download{
		URL:       []string{"http://example.com/1"},
		Key:       "key1",
		Title:     "Title 1",
		Package:   "pkg1",
		MediaType: "mp4",
		Status:    "Downloading",
	}

	// Test Insert
	res, err := upsert(d)
	assert.NoError(t, err)
	assert.Equal(t, "Title 1", res.Title)
	assert.Equal(t, "key1", res.Key)

	// Test Update
	d.Title = "Title 1 Updated"
	d.Status = "Completed"
	res, err = upsert(d)
	assert.NoError(t, err)
	assert.Equal(t, "Title 1 Updated", res.Title)
	assert.Equal(t, "Completed", res.Status)

	// Ensure still only 1 record
	count, err := client.Download.Query().Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestDeleteDownloadByID(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	d, err := client.Download.Create().
		SetURL([]string{"url"}).
		SetKey("key").
		SetTitle("title").
		SetPackage("pkg").
		SetMediaType("mp4").
		SetStatus("status").
		Save(ctx)
	assert.NoError(t, err)

	err = client.Download.DeleteOneID(d.ID).Exec(ctx)
	assert.NoError(t, err)

	count, _ := client.Download.Query().Count(ctx)
	assert.Equal(t, 0, count)
}
