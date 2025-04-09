package router

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/handler"
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

		var jsonUrl *JsonUrl
		json.Unmarshal(c.Body(), &jsonUrl)
		if jsonUrl.Url == "" {
			return c.JSON(result.NewErrorResult("Invalid URL in resuest  body", 400))
		}

		result, err := handler.Watch(c.Params("pkg"), jsonUrl.Url)
		if err != nil {
			return err
		}
		return c.JSON(result)
	})

	app.Get("/ext/detail/:pkg", func(c *fiber.Ctx) error {

		var jsonUrl *JsonUrl
		json.Unmarshal(c.Body(), &jsonUrl)
		if jsonUrl.Url == "" {
			return c.JSON(result.NewErrorResult("Invalid URL in resuest body sent to miru_core", 400))
		}

		result, err := handler.Detail(c.Params("pkg"), jsonUrl.Url)
		if err != nil {
			return err
		}
		return c.JSON(result)
	})

	// WebDav login
	app.Post("/drive/login", func(c *fiber.Ctx) error {

		var jsonReq *WebDavLoginJson
		json.Unmarshal(c.Body(), &jsonReq)

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
}

type JsonUrl struct {
	Url string `json:"url"`
}

type WebDavLoginJson struct {
	Host   string `json:"host"`
	Passwd string `json:"passwd"`
	User   string `json:"user"`
}
