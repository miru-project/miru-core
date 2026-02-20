package handler

import (
	"github.com/miru-project/miru-core/pkg/torrent"
	"github.com/valyala/fasthttp"
)

func GetTorrentData(c *fasthttp.RequestCtx) {
	torrent.GetTorrentData(c)
}
