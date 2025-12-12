package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/pkg/result"
	"github.com/miru-project/miru-core/pkg/torrent"
)

func AddTorrent(link string) (*result.Result[any], error) {
	res, err := torrent.AddTorrent(link)
	if err != nil {
		return result.NewErrorResult("Failed to add torrent", 500, nil), err
	}
	return result.NewSuccessResult(res), nil
}

func ListTorrent(c *fiber.Ctx) error {
	return torrent.ListTorrent(c)
}

func DeleteTorrent(c *fiber.Ctx) error {
	return torrent.DeleteTorrent(c)
}

func GetTorrentData(c *fiber.Ctx) error {
	return torrent.GetTorrentData(c)
}

func AddMagnet(magnet string) (*result.Result[any], error) {
	res, err := torrent.AddMagnet(magnet)
	if err != nil {
		return result.NewErrorResult("Failed to add magnet", 500, nil), err
	}
	return result.NewSuccessResult(res), nil
}
