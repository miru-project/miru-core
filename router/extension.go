package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/handler"
)

func initExtensionRouter(app *fiber.App) {
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
}
