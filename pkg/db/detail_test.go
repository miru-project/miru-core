package db

import (
	"context"
	"testing"

	"github.com/miru-project/miru-core/config"
	"github.com/miru-project/miru-core/ext"
	"github.com/stretchr/testify/assert"
)

func TestDetail(t *testing.T) {
	config.Global.Database.Driver = "sqlite3"
	config.Global.Database.DBName = ":memory:"
	Initialize()
	client := ext.EntClient()
	defer client.Close()

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
