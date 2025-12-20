package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/router/handler"
)

func initTorrentRouter(app *fiber.App) {
	GetTorrentData(app)
}

// @Summary		Get torrent data
// @Description	Get file data from a torrent
// @Tags			torrent
// @Param			infoHash	path	string	true	"Torrent InfoHash"
// @Param			path		path	string	false	"File path within torrent"
// @Router			/torrent/data/{infoHash}/{path} [get]
func GetTorrentData(app *fiber.App) fiber.Router {
	return app.Get("/torrent/data/:infoHash/*", func(c *fiber.Ctx) error {
		return handler.GetTorrentData(c)
	})
}
