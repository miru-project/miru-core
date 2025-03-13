package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/handler"
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

}
