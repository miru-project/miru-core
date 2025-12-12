package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/router/handler"
)

func initDBRouter(app *fiber.App) {
	GetFavorite(app)
	ModFavorite(app)
	GetHistory(app)
	ModHistory(app)
	GetFavoriteGroup(app)
	ModFavoriteGroup(app)
}

func GetFavorite(app *fiber.App) fiber.Router {
	return app.Get("/db/favorite", func(c *fiber.Ctx) error {
		res, e := handler.GetFavorite(c.Query("function"), c)
		if e != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(res)
		}
		return c.JSON(res)
	})
}

func ModFavorite(app *fiber.App) fiber.Router {
	return app.Post("/db/favorite", func(c *fiber.Ctx) error {
		res, e := handler.ModFavorite(c.Query("function"), c)
		if e != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(res)
		}
		return c.JSON(res)
	})

}

func GetHistory(app *fiber.App) fiber.Router {
	return app.Get("/db/history", func(c *fiber.Ctx) error {
		res, e := handler.GetHistory(c.Query("function"), c)
		if e != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(res)
		}
		return c.JSON(res)
	})
}

func ModHistory(app *fiber.App) fiber.Router {
	return app.Post("/db/history", func(c *fiber.Ctx) error {
		res, e := handler.ModHistory(c.Query("function"), c)
		if e != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(res)
		}
		return c.JSON(res)
	})

}
func GetFavoriteGroup(app *fiber.App) fiber.Router {
	return app.Get("/db/favoriteGroup", func(c *fiber.Ctx) error {
		res, e := handler.GetFavoriteGroup(c.Query("function"), c)
		if e != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(res)
		}
		return c.JSON(res)
	})
}
func ModFavoriteGroup(app *fiber.App) fiber.Router {
	return app.Post("/db/favoriteGroup", func(c *fiber.Ctx) error {
		res, e := handler.ModFavoriteGroup(c.Query("function"), c)
		if e != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(res)
		}
		return c.JSON(res)
	})
}
