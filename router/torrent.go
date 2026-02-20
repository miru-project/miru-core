package router

import (
	fasthttp_router "github.com/fasthttp/router"
	"github.com/miru-project/miru-core/router/handler"
	"github.com/valyala/fasthttp"
)

func initTorrentRouter(app *fasthttp_router.Router) {
	GetTorrentData(app)
}

// @Summary		Get torrent data
// @Description	Get file data from a torrent
// @Tags			torrent
// @Param			infoHash	path	string	true	"Torrent InfoHash"
// @Param			path		path	string	false	"File path within torrent"
// @Router			/torrent/data/{infoHash}/{path} [get]
func GetTorrentData(app *fasthttp_router.Router) {
	app.GET("/torrent/data/{infoHash}/{*path}", func(c *fasthttp.RequestCtx) {
		handler.GetTorrentData(c)
	})
}
