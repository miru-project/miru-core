package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/pkg/torrent"
)

func GetTorrentData(c *fiber.Ctx) error {
	return torrent.GetTorrentData(c)
}
