package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/router/handler"
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
	RemoveExtensionRepo(app)
	RemoveExtension(app)
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
	return app.Get("/ext/latest", func(c *fiber.Ctx) error {
		res := handler.Latest(c.FormValue("page"), c.FormValue("pkg"))
		return c.Status(res.Code).JSON(res)
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
	return app.Get("/ext/search", func(c *fiber.Ctx) error {
		res := handler.Search(c.FormValue("page"), c.FormValue("pkg"), c.FormValue("kw"), c.FormValue("filter"))
		return c.Status(res.Code).JSON(res)
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
	return app.Get("/ext/watch", func(c *fiber.Ctx) error {
		res := handler.Watch(c.FormValue("pkg"), c.FormValue("url"))
		return c.Status(res.Code).JSON(res)
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
	return app.Get("/ext/detail", func(c *fiber.Ctx) error {
		result := handler.Detail(c.FormValue("pkg"), c.FormValue("url"))
		return c.Status(result.Code).JSON(result)
	})
}

func DownloadExtension(app *fiber.App) fiber.Router {
	return app.Post("/download/extension", func(c *fiber.Ctx) error {
		repoUrl := c.FormValue("repoUrl")
		pkg := c.FormValue("pkg")
		res := handler.DownloadExtension(repoUrl, pkg)
		if res.Code >= 400 {
			return c.Status(res.Code).JSON(res)
		}
		return c.JSON(res)
	})

}

func FetchExtensionRepo(app *fiber.App) fiber.Router {
	return app.Get("/ext/repolist", func(c *fiber.Ctx) error {
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
func RemoveExtensionRepo(app *fiber.App) fiber.Router {
	return app.Delete("/ext/repo", func(c *fiber.Ctx) error {
		repoUrl := c.FormValue("repoUrl")
		res, err := handler.RemoveExtensionRepo(repoUrl)
		if err != nil {
			return err
		}
		if res.Code >= 400 {
			return c.Status(res.Code).JSON(res)
		}
		return c.JSON(res)
	})
}

func RemoveExtension(app *fiber.App) fiber.Router {
	return app.Delete("/rm/extension", func(c *fiber.Ctx) error {
		pkg := c.FormValue("pkg")
		res, err := handler.RemoveExtension(pkg)
		if err != nil {
			return err
		}
		if res.Code >= 400 {
			return c.Status(res.Code).JSON(res)
		}
		return c.JSON(res)
	})

}

// @Summary		Add or update an extension repository
// @Description	Adds a new extension repository or updates an existing one
// @Tags			extension
// @Accept			json
// @Produce		json
// @Param			repoUrl	formData	string	true	"Repository URL"
// @Param			name	formData	string	true	"Repository name"
// @Success		200	{object}	result.Result[string]
// @Failure		400	{object}	result.Result[string]	"Invalid input"
// @Failure		500	{object}	result.Result[string]	"Server error"
// @Router			/ext/repo [post]
func SetExtensionRepo(app *fiber.App) fiber.Router {
	return app.Post("/ext/repo", func(c *fiber.Ctx) error {
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
	return app.Get("/ext/repo", func(c *fiber.Ctx) error {
		repo, err := handler.GetExtensionRepo()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).
				JSON(result.NewErrorResult(err.Error(), fiber.StatusInternalServerError, nil))
		}
		return c.JSON(result.NewSuccessResult(repo))
	})
}
