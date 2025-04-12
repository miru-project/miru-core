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

func InitRouter(app *fiber.App) {

	app.Get("/", func(c *fiber.Ctx) error {
		result, err := handler.HelloMiru()
		if err != nil {
			return err
		}
		return c.JSON(result)
	})

	app.Get("/ext/latest/:pkg/:page", func(c *fiber.Ctx) error {

		result, err := handler.Latest(c.Params("page"), c.Params("pkg"))
		if err != nil {
			return err
		}
		return c.JSON(result)
	})

	app.Get("/ext/search/:pkg/:page/:kw", func(c *fiber.Ctx) error {
		result, err := handler.Search(c.Params("page"), c.Params("pkg"), c.Params("kw"), string(c.Body()))
		if err != nil {
			return err
		}
		return c.JSON(result)

	})
	// Param url  in flutter need to encoded and decoded here
	app.Get("/ext/watch/:pkg", func(c *fiber.Ctx) error {

		result, err := handler.Watch(c.Params("pkg"), string(c.Body()))
		if err != nil {
			return err
		}
		return c.JSON(result)
	})

	app.Get("/ext/detail/:pkg", func(c *fiber.Ctx) error {

		result, err := handler.Detail(c.Params("pkg"), string(c.Body()))
		if err != nil {
			return err
		}
		return c.JSON(result)
	})

	// WebDav login
	app.Post("/drive/login", func(c *fiber.Ctx) error {

		var jsonReq *WebDavLoginJson

		if e := json.Unmarshal(c.Body(), &jsonReq); e != nil {
			return c.JSON(result.NewErrorResult("Invalid JSON in request body sent to miru_core", 400))
		}

		host, user, passwd := jsonReq.Host, jsonReq.User, jsonReq.Passwd

		if host == "" || user == "" || passwd == "" {
			return c.JSON(result.NewErrorResult("Invalid URL in resuest body sent to miru_core", 400))
		}

		result, err := handler.Login(host, user, passwd)
		if err != nil {
			return err
		}
		return c.JSON(result)

	})

	// Backup the database to WebDav
	app.Get("/drive/backup", func(c *fiber.Ctx) error {
		result, err := handler.Backup()
		if err != nil {
			return err
		}
		return c.JSON(result)
	})

	// Restore the database from WebDav
	app.Get("/drive/restore", func(c *fiber.Ctx) error {
		result, err := handler.Restore()
		if err != nil {
			return err
		}
		return c.JSON(result)
	})

	app.Get("/appSetting", func(c *fiber.Ctx) error {
		result, err := handler.GetAppSetting()
		if err != nil {
			return err
		}
		return c.JSON(result)

	})

	app.Put("/appSetting", func(c *fiber.Ctx) error {
		var jsonUrl *[]ext.AppSettingJson

		if e := json.Unmarshal(c.Body(), &jsonUrl); e != nil {
			return c.JSON(result.NewErrorResult("Invalid JSON in request body sent to miru_core", 400))
		}

		if jsonUrl == nil {
			return c.JSON(result.NewErrorResult("Invalid JSON in request body sent to miru_core", 400))
		}

		if err := handler.SetAppSetting(jsonUrl); err != nil {

			// Generate error string
			errStr := "Failed to set app settings"
			for _, e := range err {
				errStr += e.Error() + ","
			}

			return c.JSON(result.NewErrorResult(errStr, 500))
		}

		return c.SendStatus(200)

	})

	// Setup redirect link in Anilist > Settings > Developer > <client> > Redirect URL
	// User will be directed to /anilist then will be redirected to /anilist/token
	app.Get("/anilist", func(c *fiber.Ctx) error {

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

	app.Get("/anilist/token", func(c *fiber.Ctx) error {

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

	app.Get("/anilist/user", func(c *fiber.Ctx) error {

		result, err := handler.GetAnilistUserData()

		if err != nil {
			return err
		}

		return c.JSON(result)
	})

	app.Get("/anilist/collection/:userId/:mediaType", func(c *fiber.Ctx) error {

		result, err := handler.GetAnilistCollection(c.Params("userId"), c.Params("mediaType"))

		if err != nil {
			return err
		}

		return c.JSON(result)
	})

	app.Get("/anilist/media/:page/:mediaType", func(c *fiber.Ctx) error {

		// Use requestbody as search string
		result, err := handler.GetAnilistMediaQuery(c.Params("page"), string(c.Body()), c.Params("mediaType"))

		if err != nil {
			return err
		}

		return c.JSON(result)
	})

	app.Post("/anilist/edit", func(c *fiber.Ctx) error {
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

type WebDavLoginJson struct {
	Host   string `json:"host"`
	Passwd string `json:"passwd"`
	User   string `json:"user"`
}
