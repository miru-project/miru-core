package router

import (
	"encoding/json"
	"strings"

	fasthttp_router "github.com/fasthttp/router"
	"github.com/miru-project/miru-core/pkg/anilist"
	"github.com/miru-project/miru-core/pkg/result"
	"github.com/miru-project/miru-core/router/handler"
	"github.com/valyala/fasthttp"
)

// initAnilistRouter handles all Anilist related routes
//
//	@Summary		Anilist integration API
//	@Description	API endpoints for Anilist integration with Miru
//	@Tags			anilist
func initAnilistRouter(app *fasthttp_router.Router) {
	AnilistOAuth(app)
	ProcessAnilistToken(app)
	GetAnilistUser(app)
	GetAnilistCollection(app)
	SearchAnilistMedia(app)
	EditAnilistList(app)
}

func sendJSON(c *fasthttp.RequestCtx, data interface{}) {
	res, _ := json.Marshal(data)
	c.SetContentType("application/json")
	c.SetBody(res)
}

func sendError(c *fasthttp.RequestCtx, err error) {
	c.SetStatusCode(500)
	c.SetContentType("application/json")
	res, _ := json.Marshal(result.NewErrorResultAny(err.Error(), 500))
	c.SetBody(res)
}

// @Summary		Redirect for Anilist OAuth
// @Description	Handles Anilist OAuth redirect flow
// @Tags			anilist
// @Produce		html
// @Success		200	{string}	html	"Redirecting HTML"
// @Router			/anilist [get]
func AnilistOAuth(app *fasthttp_router.Router) {
	app.GET("/anilist", func(c *fasthttp.RequestCtx) {
		//Save url fragement to cookie
		const html = `
			<!DOCTYPE html>
			<html>
			<head>
				<p>Redirecting...</p>
			</head>
			<body>
			<script>
				const fragment = window.location.hash
				document.cookie = "anilist=" + fragment + "; path=/"
				window.location.href = "http://127.127.127.127:12777/anilist/token"
			</script>
			</body>
			</html>
		`
		c.Response.Header.Set("Content-Type", "text/html")
		c.SetBodyString(html)
	})
}

// @Summary		Process Anilist token
// @Description	Processes and stores the Anilist authentication token
// @Tags			anilist
// @Produce		plain
// @Success		200	{object}	string
// @Failure		500	{object}	result.Result[string]
// @Router			/anilist/token [get]
func ProcessAnilistToken(app *fasthttp_router.Router) {
	app.GET("/anilist/token", func(c *fasthttp.RequestCtx) {
		// Get the cookie from request headers
		cookie := string(c.Request.Header.Cookie("anilist"))
		if cookie == "" {
			c.SetStatusCode(500)
			sendJSON(c, result.NewErrorResultAny("Failed to get cookie", 500))
			return
		}

		// Parse the cookie to get the token
		metaData := strings.Split(cookie, "&")
		token := strings.Split(metaData[0], "=")[1]

		// Save the token to the database
		if e := handler.SetAppSetting("anilist_token", token); e != nil {
			c.SetStatusCode(500)
			sendJSON(c, result.NewErrorResultAny("Failed to set app settings"+e.Error(), 500))
			return
		}

		anilist.InitToken()
		c.SetBodyString("Authorized successfully, you can close this page now.")
	})
}

// @Summary		Get Anilist user data
// @Description	Retrieves authenticated user data from Anilist
// @Tags			anilist
// @Produce		json
// @Success		200	{object}	result.Result[map[string]string]
// @Failure		500	{object}	result.Result[string]	"Failed to retrieve user data"
// @Router			/anilist/user [get]
func GetAnilistUser(app *fasthttp_router.Router) {
	app.GET("/anilist/user", func(c *fasthttp.RequestCtx) {
		res, err := handler.GetAnilistUserData()

		if err != nil {
			sendError(c, err)
			return
		}

		sendJSON(c, res)
	})
}

// @Summary		Get Anilist collection
// @Description	Retrieves a user's media collection from Anilist
// @Tags			anilist
// @Produce		json
// @Param			userId		path		string	true	"User ID"
// @Param			mediaType	path		string	true	"Media type (ANIME or MANGA)"
// @Success		200			{object}	result.Result[map[string]string]
// @Failure		500			{object}	result.Result[string]	"Failed to retrieve collection"
// @Router			/anilist/collection/{userId}/{mediaType} [get]
func GetAnilistCollection(app *fasthttp_router.Router) {
	app.GET("/anilist/collection/{userId}/{mediaType}", func(c *fasthttp.RequestCtx) {
		userId := c.UserValue("userId").(string)
		mediaType := c.UserValue("mediaType").(string)
		res, err := handler.GetAnilistCollection(userId, mediaType)

		if err != nil {
			sendError(c, err)
			return
		}

		sendJSON(c, res)
	})
}

// @Summary		Search Anilist media
// @Description	Search for anime or manga on Anilist
// @Tags			anilist
// @Accept			plain
// @Produce		json
// @Param			page		path		string	true	"Page number"
// @Param			mediaType	path		string	true	"Media type (ANIME or MANGA)"
// @Param			query		body		string	true	"Search query"
// @Success		200			{object}	result.Result[map[string]string]
// @Failure		500			{object}	result.Result[string]	"Search failed"
// @Router			/anilist/media/{page}/{mediaType} [get]
func SearchAnilistMedia(app *fasthttp_router.Router) {
	app.GET("/anilist/media/{page}/{mediaType}", func(c *fasthttp.RequestCtx) {
		// Use requestbody as search string
		page := c.UserValue("page").(string)
		mediaType := c.UserValue("mediaType").(string)
		res, err := handler.GetAnilistMediaQuery(page, string(c.PostBody()), mediaType)

		if err != nil {
			sendError(c, err)
			return
		}

		sendJSON(c, res)
	})
}

// @Summary		Edit Anilist list entry
// @Description	Update or create an entry in the user's Anilist
// @Tags			anilist
// @Accept			json
// @Produce		json
// @Param			data	body		anilist.AnilistEditListJson	true	"List entry data"
// @Success		200		{object}	result.Result[map[string]string]
// @Failure		400		{object}	result.Result[string]	"Invalid JSON"
// @Failure		500		{object}	result.Result[string]	"Edit failed"
// @Router			/anilist/edit [post]
func EditAnilistList(app *fasthttp_router.Router) {
	app.POST("/anilist/edit", func(c *fasthttp.RequestCtx) {
		var jsonReq *anilist.AnilistEditListJson

		if e := json.Unmarshal(c.PostBody(), &jsonReq); e != nil {
			c.SetStatusCode(400)
			sendJSON(c, result.NewErrorResultAny("Invalid JSON in request body sent to miru_core", 400))
			return
		}

		res, err := handler.EditAnilistList(jsonReq.Status, jsonReq.MediaId, jsonReq.Id, jsonReq.Progress, jsonReq.Score, jsonReq.StartDate, jsonReq.EndDate, jsonReq.IsPrivate)

		if err != nil {
			sendError(c, err)
			return
		}

		sendJSON(c, res)
	})
}
