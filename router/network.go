package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/handler"
)

func initNetworkRouter(app *fiber.App) {
	listCookies(app)
	setCookies(app)
}

func listCookies(app *fiber.App) fiber.Router {
	return app.Get("/network/cookies", func(c *fiber.Ctx) error {
		res := handler.GetCookies(c.FormValue("url"))
		if res.Code >= 400 {
			return c.Status(fiber.StatusInternalServerError).JSON(res)
		}
		return c.JSON(res)
	})
}

func setCookies(app *fiber.App) fiber.Router {
	return app.Post("/network/cookies", func(c *fiber.Ctx) error {
		url := c.FormValue("url")
		cookies := c.FormValue("cookies") // Expecting cookies as a comma-separated string
		cookieList := []string{}
		if cookies != "" {
			cookieList = append(cookieList, cookies)
		}
		res := handler.SetCookies(url, cookieList)
		if res.Code >= 400 {
			return c.Status(fiber.StatusInternalServerError).JSON(res)
		}
		return c.JSON(res)
	})
}
