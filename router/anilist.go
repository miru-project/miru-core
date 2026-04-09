package router

import (
	"encoding/json"
	"strings"

	fasthttp_router "github.com/fasthttp/router"
	"github.com/miru-project/miru-core/pkg/result"
	"github.com/miru-project/miru-core/router/handler"
	"github.com/valyala/fasthttp"
)

// initAnilistRouter handles all Anilist related routes
func initAnilistRouter(app *fasthttp_router.Router) {
	AnilistOAuth(app)
	ProcessAnilistToken(app)
	AnilistLogout(app)
}

func sendJSON(c *fasthttp.RequestCtx, data interface{}) {
	res, _ := json.Marshal(data)
	c.SetContentType("application/json")
	c.SetBody(res)
}

func sendError(c *fasthttp.RequestCtx, err error) {
	c.SetStatusCode(500)
	sendJSON(c, result.NewErrorResultAny(err.Error(), 500))
}

// @Summary		Redirect for Anilist OAuth
// @Description	Handles Anilist OAuth redirect flow
// @Tags			anilist
// @Produce		html
// @Success		200	{string}	html	"Redirecting HTML"
// @Router			/anilist [get]
func AnilistOAuth(app *fasthttp_router.Router) {
	app.GET("/anilist", func(c *fasthttp.RequestCtx) {
		// Save url fragment to cookie via JS because fragment is not sent to server
		const html = `
			<!DOCTYPE html>
			<html>
			<head>
				<title>Redirecting...</title>
			</head>
			<body>
			<script>
				const fragment = window.location.hash
				if (fragment) {
					document.cookie = "anilist=" + fragment + "; path=/"
					window.location.href = "/anilist/token"
				} else {
					window.location.href = "https://anilist.co/api/v2/oauth/authorize?client_id=25956&response_type=token"
				}
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
			sendJSON(c, result.NewErrorResultAny("Failed to get auth cookie", 500))
			return
		}

		// Parse the fragment style cookie
		fragment := strings.TrimPrefix(cookie, "#")
		parts := strings.Split(fragment, "&")
		var token string
		for _, part := range parts {
			kv := strings.Split(part, "=")
			if len(kv) == 2 && kv[0] == "access_token" {
				token = kv[1]
				break
			}
		}

		if token == "" {
			c.SetStatusCode(500)
			sendJSON(c, result.NewErrorResultAny("Failed to extract token", 500))
			return
		}

		// Save the token to the database
		if e := handler.SetAppSetting("anilist_token", token); e != nil {
			c.SetStatusCode(500)
			sendJSON(c, result.NewErrorResultAny("Failed to set app settings: "+e.Error(), 500))
			return
		}

		c.SetBodyString("Authorized successfully, you can close this page now.")
	})
}
// @Summary		Anilist logout
// @Description	Clears the Anilist authentication token
// @Tags			anilist
// @Produce		plain
// @Success		200	{string}	string	"Logged out"
// @Router			/anilist/logout [get]
func AnilistLogout(app *fasthttp_router.Router) {
	app.GET("/anilist/logout", func(c *fasthttp.RequestCtx) {
		if e := handler.SetAppSetting("anilist_token", ""); e != nil {
			sendError(c, e)
			return
		}
		c.SetBodyString("Logout success")
	})
}
