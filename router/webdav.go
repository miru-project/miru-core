package router

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/handler"
	"github.com/miru-project/miru-core/pkg/result"
)

func initWebDavRouter(app *fiber.App) {
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
}
