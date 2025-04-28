package router

import (
	"encoding/json"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/ext"
	"github.com/miru-project/miru-core/handler"
	"github.com/miru-project/miru-core/pkg/anilist"
	"github.com/miru-project/miru-core/pkg/result"
)

// initAnilistRouter handles all Anilist related routes
//
//	@Summary		Anilist integration API
//	@Description	API endpoints for Anilist integration with Miru
//	@Tags			anilist
func initAnilistRouter(app *fiber.App) {
	AnilistOAuth(app)
	ProcessAnilistToken(app)
	GetAnilistUser(app)
	GetAnilistCollection(app)
	SearchAnilistMedia(app)
	EditAnilistList(app)
}

// @Summary		Redirect for Anilist OAuth
// @Description	Handles Anilist OAuth redirect flow
// @Tags			anilist
// @Produce		html
// @Success		200	{string}	html	"Redirecting HTML"
// @Router			/anilist [get]
func AnilistOAuth(app *fiber.App) fiber.Router {
	return app.Get("/anilist", func(c *fiber.Ctx) error {
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
		c.Set("Content-Type", "text/html")
		return c.SendString(html)
	})
}

// @Summary		Process Anilist token
// @Description	Processes and stores the Anilist authentication token
// @Tags			anilist
// @Produce		plain
// @Success		200	{object}	string
// @Failure		500	{object}	result.Result[string]
// @Router			/anilist/token [get]
func ProcessAnilistToken(app *fiber.App) fiber.Router {
	return app.Get("/anilist/token", func(c *fiber.Ctx) error {
		// Get the cookie from request headers
		cookie := c.Cookies("anilist")
		if cookie == "" {
			return c.JSON(result.NewErrorResult("Failed to get cookie", 500))
		}

		// Parse the cookie to get the token
		setting := make([]ext.AppSettingJson, 0)
		metaData := strings.Split(cookie, "&")
		token := strings.Split(metaData[0], "=")[1]

		setting = append(setting, ext.AppSettingJson{
			Key:   "anilist_token",
			Value: token,
		})

		// Save the token to the database
		if e := handler.SetAppSetting(&setting); e != nil {
			return c.JSON(result.NewErrorResult("Failed to set app settings"+e[0].Error(), 500))
		}

		anilist.InitToken()
		return c.SendString("Authorized successfully, you can close this page now.")
	})
}

// @Summary		Get Anilist user data
// @Description	Retrieves authenticated user data from Anilist
// @Tags			anilist
// @Produce		json
// @Success		200	{object}	result.Result[map[string]string]
// @Failure		500	{object}	result.Result[string]	"Failed to retrieve user data"
// @Router			/anilist/user [get]
func GetAnilistUser(app *fiber.App) fiber.Router {
	return app.Get("/anilist/user", func(c *fiber.Ctx) error {
		result, err := handler.GetAnilistUserData()

		if err != nil {
			return err
		}

		return c.JSON(result)
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
func GetAnilistCollection(app *fiber.App) fiber.Router {
	return app.Get("/anilist/collection/:userId/:mediaType", func(c *fiber.Ctx) error {
		result, err := handler.GetAnilistCollection(c.Params("userId"), c.Params("mediaType"))

		if err != nil {
			return err
		}

		return c.JSON(result)
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
func SearchAnilistMedia(app *fiber.App) fiber.Router {
	return app.Get("/anilist/media/:page/:mediaType", func(c *fiber.Ctx) error {
		// Use requestbody as search string
		result, err := handler.GetAnilistMediaQuery(c.Params("page"), string(c.Body()), c.Params("mediaType"))

		if err != nil {
			return err
		}

		return c.JSON(result)
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
func EditAnilistList(app *fiber.App) fiber.Router {
	return app.Post("/anilist/edit", func(c *fiber.Ctx) error {
		var jsonReq *anilist.AnilistEditListJson

		if e := json.Unmarshal(c.Body(), &jsonReq); e != nil {
			return c.JSON(result.NewErrorResult("Invalid JSON in request body sent to miru_core", 400))
		}

		res, err := handler.EditAnilistList(jsonReq.Status, jsonReq.MediaId, jsonReq.Id, jsonReq.Progress, jsonReq.Score, jsonReq.StartDate, jsonReq.EndDate, jsonReq.IsPrivate)

		if err != nil {
			return err
		}

		return c.JSON(res)
	})
}
