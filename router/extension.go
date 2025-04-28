package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/handler"
	_ "github.com/miru-project/miru-core/pkg/jsExtension"
	_ "github.com/miru-project/miru-core/pkg/result"
)

// initExtensionRouter handles all extension related routes
func initExtensionRouter(app *fiber.App) {
	GetLatestContent(app)
	SearchContent(app)
	WatchContent(app)
	GetContentDetail(app)
}

// @Summary		Get latest content from extension
// @Description	Retrieves the latest content from a specific extension package
// @Tags			extension
// @Produce		json
// @Param			pkg		path		string	true	"Package identifier"
// @Param			page	path		string	true	"Page number"
// @Success		200		{object}	result.Result[jsExtension.ExtensionListItems]
// @Failure		500		{object}	result.Result[string]	"Server error"
// @Router			/ext/latest/{pkg}/{page} [get]
func GetLatestContent(app *fiber.App) fiber.Router {
	return app.Get("/ext/latest/:pkg/:page", func(c *fiber.Ctx) error {
		result, err := handler.Latest(c.Params("page"), c.Params("pkg"))
		if err != nil {
			return err
		}
		return c.JSON(result)
	})
}

// @Summary		Search content via extension
// @Description	Search for content in a specific extension package
// @Tags			extension
// @Accept			json
// @Produce		json
// @Param			pkg		path		string	true	"Package identifier"
// @Param			page	path		string	true	"Page number"
// @Param			kw		path		string	true	"Search keyword"
// @Param			filter	body		string	false	"Filter options in JSON format"
// @Success		200		{object}	result.Result[jsExtension.ExtensionListItems]
// @Failure		500		{object}	result.Result[string]	"Server error"
// @Router			/ext/search/{pkg}/{page}/{kw} [get]
func SearchContent(app *fiber.App) fiber.Router {
	return app.Get("/ext/search/:pkg/:page/:kw", func(c *fiber.Ctx) error {
		result, err := handler.Search(c.Params("page"), c.Params("pkg"), c.Params("kw"), string(c.Body()))
		if err != nil {
			return err
		}
		return c.JSON(result)
	})
}

// @Summary		Watch content via extension
// @Description	Get watch information for content from a specific package
// @Tags			extension
// @Accept			json
// @Produce		json
// @Param			pkg	path		string	true	"Package identifier"
// @Param			url	body		string	true	"URL encoded content path"
// @Success		200	{object}	result.Result[jsExtension.ExtensionMangaWatch]
// @Success		200	{object}	result.Result[jsExtension.ExtensionBangumiWatch]
// @Failure		500	{object}	result.Result[jsExtension.ExtensionFikushonWatch]	"Server error"
// @Router			/ext/watch/{pkg} [get]
func WatchContent(app *fiber.App) fiber.Router {
	return app.Get("/ext/watch/:pkg", func(c *fiber.Ctx) error {
		result, err := handler.Watch(c.Params("pkg"), string(c.Body()))
		if err != nil {
			return err
		}
		return c.JSON(result)
	})
}

// @Summary		Get content details
// @Description	Get detailed information about content from a specific package
// @Tags			extension
// @Accept			json
// @Produce		json
// @Param			pkg	path		string	true	"Package identifier"
// @Param			url	body		string	true	"URL encoded content path"
// @Success		200	{object}	result.Result[jsExtension.ExtensionDetail]
// @Failure		500	{object}	result.Result[string]	"Server error"
// @Router			/ext/detail/{pkg} [get]
func GetContentDetail(app *fiber.App) fiber.Router {
	return app.Get("/ext/detail/:pkg", func(c *fiber.Ctx) error {
		result, err := handler.Detail(c.Params("pkg"), string(c.Body()))
		if err != nil {
			return err
		}
		return c.JSON(result)
	})
}
