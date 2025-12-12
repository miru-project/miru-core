package router

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/pkg/result"
	"github.com/miru-project/miru-core/router/handler"
)

// initTorrentRouter handles all torrent related routes
//
//	@Summary		Torrent management API
//	@Description	API endpoints for managing torrents in Miru
//	@Tags			torrent
func initTorrentRouter(app *fiber.App) {
	ListTorrent(app)
	AddTorrent(app)
	DeleteTorrent(app)
	GetTorrentData(app)
	AddMagnet(app)
}

// @Summary		List all torrents
// @Description	Get a list of all active torrents
// @Tags			torrent
// @Produce		json
// @Success		200	{object}	result.Result[[]result.TorrentResult]
// @Router			/torrent [get]
func ListTorrent(app *fiber.App) fiber.Router {
	return app.Get("/torrent", func(c *fiber.Ctx) error {
		return handler.ListTorrent(c)
	})
}

type AddTorrentJson struct {
	Url string `json:"url"`
}

// @Summary		Add a torrent
// @Description	Add a new torrent by URL or magnet link
// @Tags			torrent
// @Accept			json
// @Produce		json
// @Param			url	body		AddTorrentJson	true	"Torrent URL"
// @Success		200	{object}	result.Result[result.TorrentDetailResult]
// @Failure		400	{object}	result.Result[string]	"Invalid JSON"
// @Router			/torrent [post]
func AddTorrent(app *fiber.App) fiber.Router {
	return app.Post("/torrent", func(c *fiber.Ctx) error {
		var jsonReq *AddTorrentJson
		if e := json.Unmarshal(c.Body(), &jsonReq); e != nil {
			return c.JSON(result.NewErrorResult("Invalid JSON in request body sent to miru_core", 400, nil))
		}
		res, err := handler.AddTorrent(jsonReq.Url)
		if err != nil {
			return err
		}
		return c.JSON(res)
	})
}

// @Summary		Delete a torrent
// @Description	Delete a torrent task by InfoHash
// @Tags			torrent
// @Param			infoHash	path	string	true	"Torrent InfoHash"
// @Success		200
// @Router			/torrent/{infoHash} [delete]
func DeleteTorrent(app *fiber.App) fiber.Router {
	return app.Delete("/torrent/:infoHash", func(c *fiber.Ctx) error {
		return handler.DeleteTorrent(c)
	})
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

// @Summary		Add a magnet link
// @Description	Add a new torrent by magnet link
// @Tags			torrent
// @Accept			json
// @Produce		json
// @Param			url	body	AddTorrentJson	true	"Torrent URL"
// @Success		200	{object}result.Result[result.TorrentDetailResult]
// @Failure		400	{object}result.Result[string]"Invalid JSON"
// @Router			/torrent/magnet [post]
func AddMagnet(app *fiber.App) fiber.Router {
	return app.Post("/torrent/magnet", func(c *fiber.Ctx) error {
		var jsonReq *AddTorrentJson
		if e := json.Unmarshal(c.Body(), &jsonReq); e != nil {
			return c.JSON(result.NewErrorResult("Invalid JSON in request body sent to miru_core", 400, nil))
		}
		res, err := handler.AddMagnet(jsonReq.Url)
		if err != nil {
			return err
		}
		return c.JSON(res)
	})
}
