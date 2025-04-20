package router

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/handler"
	"github.com/miru-project/miru-core/pkg/download"
	"github.com/miru-project/miru-core/pkg/result"
)

func initDownloadRouter(app *fiber.App) {

	app.Get("/download/status", func(c *fiber.Ctx) error {
		res := handler.DownloadStatus()
		return c.JSON(res)
	})

	app.Post("/download/cancel/:id", func(c *fiber.Ctx) error {

		res, e := handler.CancelTask(c.Params("id"))
		if e != nil {
			return e
		}

		return c.JSON(res)
	})

	app.Post("/download/resume/:id", func(c *fiber.Ctx) error {

		res, e := handler.ResumeTask(c.Params("id"))
		if e != nil {
			return e
		}

		return c.JSON(res)

	})

	app.Post("/download/pause/:id", func(c *fiber.Ctx) error {

		res, e := handler.PauseTask(c.Params("id"))
		if e != nil {
			return e
		}

		return c.JSON(res)
	})

	app.Post("/download/bangumi", func(c *fiber.Ctx) error {

		var jsonReq *download.DownloadOptions

		if e := json.Unmarshal(c.Body(), &jsonReq); e != nil {
			return c.JSON(result.NewErrorResult("Invalid JSON in request body sent to miru_core", 400))
		}

		res, err := handler.DownloadBangumi(jsonReq.DownloadPath, jsonReq.Url, jsonReq.Header, jsonReq.IsHls)

		if err != nil {
			return err
		}

		return c.JSON(res)

	})
}
