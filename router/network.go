package router

import "github.com/gofiber/fiber/v2"

func initNetworkRouter(app *fiber.App) {
	listCookies(app)
	setCookies(app)
}

func listCookies(app *fiber.App) fiber.Router {
	return app.Get("/network/cookies", func(c *fiber.Ctx) error {
		// Currently a placeholder for future implementation
		return c.SendStatus(fiber.StatusNotImplemented)
	})
}
func setCookies(app *fiber.App) fiber.Router {
	return app.Post("/network/cookies", func(c *fiber.Ctx) error {
		// Currently a placeholder for future implementation
		return c.SendStatus(fiber.StatusNotImplemented)
	})
}
