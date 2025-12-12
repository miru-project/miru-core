package router

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/router/handler"
	"github.com/miru-project/miru-core/pkg/download"
	"github.com/miru-project/miru-core/pkg/result"
)

// initDownloadRouter handles all download related routes
//
//	@Summary		Download management API
//	@Description	API endpoints for managing downloads in Miru
//	@Tags			download
func initDownloadRouter(app *fiber.App) {
	GetDownloadStatus(app)
	CancelDownload(app)
	ResumeDownload(app)
	PauseDownload(app)
	DownloadBangumiContent(app)
}

//	@Summary		Get download status
//	@Description	Get the status of all download tasks
//	@Tags			download
//	@Produce		json
//	@Success		200	{object}	result.Result[map[int]*download.Progress]
//	@Router			/download/status [get]

func GetDownloadStatus(app *fiber.App) fiber.Router {
	return app.Get("/download/status", func(c *fiber.Ctx) error {
		res := handler.DownloadStatus()
		return c.JSON(res)
	})
}

// @Summary		Cancel a download task
// @Description	Cancel a specific download task by ID
// @Tags			download
// @Produce		json
// @Param			id	path		string	true	"Task ID"
// @Success		200	{object}	result.Result[string]
// @Failure		400	{object}	result.Result[string]
// @Router			/download/cancel/{id} [post]
func CancelDownload(app *fiber.App) fiber.Router {
	return app.Post("/download/cancel/:id", func(c *fiber.Ctx) error {
		res, e := handler.CancelTask(c.Params("id"))
		if e != nil {
			return e
		}

		return c.JSON(res)
	})
}

// @Summary		Resume a download task
// @Description	Resume a paused download task by ID
// @Tags			download
// @Produce		json
// @Param			id	path		string	true	"Task ID"
// @Success		200	{object}	result.Result[string]
// @Failure		400	{object}	result.Result[string]
// @Router			/download/resume/{id} [post]
func ResumeDownload(app *fiber.App) fiber.Router {
	return app.Post("/download/resume/:id", func(c *fiber.Ctx) error {
		res, e := handler.ResumeTask(c.Params("id"))
		if e != nil {
			return e
		}

		return c.JSON(res)
	})
}

// @Summary		Pause a download task
// @Description	Pause an active download task by ID
// @Tags			download
// @Produce		json
// @Param			id	path		string	true	"Task ID"
// @Success		200	{object}	result.Result[string]
// @Failure		400	{object}	result.Result[string]
// @Router			/download/pause/{id} [post]
func PauseDownload(app *fiber.App) fiber.Router {
	return app.Post("/download/pause/:id", func(c *fiber.Ctx) error {
		res, e := handler.PauseTask(c.Params("id"))
		if e != nil {
			return e
		}

		return c.JSON(res)
	})
}

// @Summary		Download bangumi content
// @Description	Start downloading bangumi content with specified options
// @Tags			download
// @Accept			json
// @Produce		json
// @Param			options	body		download.DownloadOptions	true	"Download Options"
// @Success		200		{object}	result.Result[download.MultipleLinkJson]
// @Failure		400		{object}	result.Result[string]	"Invalid JSON"
// @Router			/download/bangumi [post]
func DownloadBangumiContent(app *fiber.App) fiber.Router {
	return app.Post("/download/bangumi", func(c *fiber.Ctx) error {
		var jsonReq *download.DownloadOptions

		if e := json.Unmarshal(c.Body(), &jsonReq); e != nil {
			return c.JSON(result.NewErrorResult("Invalid JSON in request body sent to miru_core", 400, nil))
		}

		res, err := handler.DownloadBangumi(jsonReq.DownloadPath, jsonReq.Url, jsonReq.Header, jsonReq.IsHls)

		if err != nil {
			return err
		}

		return c.JSON(res)
	})
}
