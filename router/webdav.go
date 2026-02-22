package router

import (
	"encoding/json"

	fasthttp_router "github.com/fasthttp/router"
	"github.com/miru-project/miru-core/pkg/result"
	"github.com/miru-project/miru-core/router/handler"
	"github.com/valyala/fasthttp"
)

// initWebDavRouter handles all WebDav related routes
//
//	@Summary		WebDav integration API
//	@Description	API endpoints for WebDav integration with Miru
//	@Tags			webdav
func initWebDavRouter(app *fasthttp_router.Router) {
	WebDavLogin(app)
	WebDavBackup(app)
	WebDavRestore(app)
}

// @Summary		Login to WebDav server
// @Description	Authenticate with a WebDav server
// @Tags			webdav
// @Accept			json
// @Produce		json
// @Param			credentials	body		WebDavLoginJson	true	"WebDav login credentials"
// @Success		200			{object}	result.Result[string]
// @Failure		400			{object}	result.Result[string]	"Invalid JSON or missing required fields"
// @Router			/drive/login [post]
func WebDavLogin(app *fasthttp_router.Router) {
	app.POST("/drive/login", func(c *fasthttp.RequestCtx) {
		var jsonReq *WebDavLoginJson

		if e := json.Unmarshal(c.PostBody(), &jsonReq); e != nil {
			c.SetStatusCode(400)
			sendJSON(c, result.NewErrorResultAny("Invalid JSON in request body sent to miru_core", 400))
			return
		}

		host, user, passwd := jsonReq.Host, jsonReq.User, jsonReq.Passwd

		if host == "" || user == "" || passwd == "" {
			c.SetStatusCode(400)
			sendJSON(c, result.NewErrorResultAny("Invalid URL in resuest body sent to miru_core", 400))
			return
		}

		res, err := handler.Login(host, user, passwd)
		if err != nil {
			sendError(c, err)
			return
		}
		sendJSON(c, res)
	})
}

// @Summary		Backup database to WebDav
// @Description	Backup the Miru database to a connected WebDav server
// @Tags			webdav
// @Produce		json
// @Success		200	{object}	result.Result[string]
// @Failure		500	{object}	result.Result[string]	"Backup failed"
// @Router			/drive/backup [get]
func WebDavBackup(app *fasthttp_router.Router) {
	app.GET("/drive/backup", func(c *fasthttp.RequestCtx) {
		res, err := handler.Backup()
		if err != nil {
			sendError(c, err)
			return
		}
		sendJSON(c, res)
	})
}

// @Summary		Restore database from WebDav
// @Description	Restore the Miru database from a backup on a connected WebDav server
// @Tags			webdav
// @Produce		json
// @Success		200	{object}	result.Result[string]
// @Failure		500	{object}	result.Result[string]	"Restore failed"
// @Router			/drive/restore [get]
func WebDavRestore(app *fasthttp_router.Router) {
	app.GET("/drive/restore", func(c *fasthttp.RequestCtx) {
		res, err := handler.Restore()
		if err != nil {
			sendError(c, err)
			return
		}
		sendJSON(c, res)
	})
}
