package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/handler"
	_ "github.com/miru-project/miru-core/pkg/jsExtension"
	"github.com/miru-project/miru-core/pkg/result"
)

// initExtensionRouter handles all extension related routes
func initExtensionRouter(app *fiber.App) {
	GetLatestContent(app)
	SearchContent(app)
	WatchContent(app)
	GetContentDetail(app)
	FetchExtensionRepo(app)
	SetExtensionRepo(app)
	DownloadExtension(app)
	GetExtensionRepo(app)
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

func DownloadExtension(app *fiber.App) fiber.Router {
	return app.Post("/download/extension", func(c *fiber.Ctx) error {
		repoUrl := c.FormValue("repoUrl")
		pkg := c.FormValue("pkg")

		if repoUrl == "" || pkg == "" {
			return c.Status(fiber.StatusBadRequest).
				JSON(result.NewErrorResult("Repository URL and package name are required", fiber.StatusBadRequest, nil))
		}

		if e := handler.DownloadExtension(repoUrl, pkg); e != nil {
			return c.Status(fiber.StatusInternalServerError).
				JSON(result.NewErrorResult(e.Error(), fiber.StatusInternalServerError, nil))
		}

		// Return a success response
		return c.JSON(result.NewSuccessResult("Extension download initiated successfully"))
	})

}

func FetchExtensionRepo(app *fiber.App) fiber.Router {
	return app.Get("/ext/repo", func(c *fiber.Ctx) error {
		repo, fetchErr, err := handler.FetchExtensionRepo()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).
				JSON(result.NewErrorResult(err.Error(), fiber.StatusInternalServerError, nil))
		}
		if len(fetchErr) != 0 {
			return c.Status(fiber.StatusInternalServerError).
				JSON(result.NewErrorResult("Fetch Repo Error!", fiber.StatusInternalServerError, fetchErr))
		}
		return c.JSON(result.NewSuccessResult(repo))
	})
}

func SetExtensionRepo(app *fiber.App) fiber.Router {
	return app.Post("/ext/repo/set", func(c *fiber.Ctx) error {
		repoUrl := c.FormValue("repoUrl")
		name := c.FormValue("name")
		if repoUrl == "" || name == "" {
			return c.Status(fiber.StatusBadRequest).
				JSON(result.NewErrorResult("Repository URL and name are required", fiber.StatusBadRequest, nil))
		}
		err := handler.SetExtensionRepo(repoUrl, name)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).
				JSON(result.NewErrorResult(err.Error(), fiber.StatusInternalServerError, nil))
		}
		return c.JSON(result.NewSuccessResult("Repository set successfully"))
	})
}
func GetExtensionRepo(app *fiber.App) fiber.Router {
	return app.Get("/ext/repo/get", func(c *fiber.Ctx) error {
		repo, err := handler.GetExtensionRepo()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).
				JSON(result.NewErrorResult(err.Error(), fiber.StatusInternalServerError, nil))
		}
		return c.JSON(result.NewSuccessResult(repo))
	})
}
