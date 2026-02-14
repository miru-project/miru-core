package db

import (
	"context"
	"time"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/ent/download"
	"github.com/miru-project/miru-core/ext"
)

func UpsertDownload(d *ent.Download) (*ent.Download, error) {
	client := ext.EntClient()
	ctx := context.Background()
	old, err := client.Download.Query().
		Where(
			download.Package(d.Package),
			download.WatchUrl(d.WatchUrl),
			download.DetailUrl(d.DetailUrl),
		).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return client.Download.Create().
				SetURL(d.URL).
				SetWatchUrl(d.WatchUrl).
				SetDetailUrl(d.DetailUrl).
				SetHeaders(d.Headers).
				SetPackage(d.Package).
				SetProgress(d.Progress).
				SetKey(d.Key).
				SetTitle(d.Title).
				SetMediaType(d.MediaType).
				SetStatus(d.Status).
				SetSavePath(d.SavePath).
				Save(ctx)
		}
		return nil, err
	}
	return client.Download.UpdateOne(old).
		SetURL(d.URL).
		SetWatchUrl(d.WatchUrl).
		SetDetailUrl(d.DetailUrl).
		SetHeaders(d.Headers).
		SetPackage(d.Package).
		SetProgress(d.Progress).
		SetTitle(d.Title).
		SetMediaType(d.MediaType).
		SetStatus(d.Status).
		SetSavePath(d.SavePath).
		SetDate(time.Now()).
		SetKey(d.Key).
		Save(ctx)
}

func GetAllDownloads() ([]*ent.Download, error) {
	client := ext.EntClient()
	return client.Download.Query().All(context.Background())
}

func GetPendingDownloads() ([]*ent.Download, error) {
	client := ext.EntClient()
	return client.Download.Query().Where(download.Status("Downloading"), download.Status("Paused")).All(context.Background())
}

func DeleteDownloadByID(id int) error {
	client := ext.EntClient()
	return client.Download.DeleteOneID(id).Exec(context.Background())
}

func GetDownloadByID(id int) (*ent.Download, error) {
	client := ext.EntClient()
	return client.Download.Get(context.Background(), id)
}

func GetDownloadByKey(key string) (*ent.Download, error) {
	client := ext.EntClient()
	return client.Download.Query().Where(download.Key(key)).Only(context.Background())
}

func GetDownloadByPackageWatchUrlDetailUrl(pkg string, watchUrl string, detailUrl string) (*ent.Download, error) {
	client := ext.EntClient()
	return client.Download.Query().
		Where(
			download.Package(pkg),
			download.WatchUrl(watchUrl),
			download.DetailUrl(detailUrl),
		).Only(context.Background())
}

func GetDownloadsByPackageAndDetailUrl(pkg, detailUrl string) ([]*ent.Download, error) {
	client := ext.EntClient()
	return client.Download.Query().
		Where(
			download.Package(pkg),
			download.DetailUrl(detailUrl),
		).All(context.Background())
}
