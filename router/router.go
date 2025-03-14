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

}

type JsonUrl struct {
	Url string `json:"url"`
}
