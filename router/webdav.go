package router

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/router/handler"
	"github.com/miru-project/miru-core/pkg/result"
)

// initWebDavRouter handles all WebDav related routes
//
//	@Summary		WebDav integration API
//	@Description	API endpoints for WebDav integration with Miru
//	@Tags			webdav
func initWebDavRouter(app *fiber.App) {
	WebDavLogin(app)
	WebDavBackup(app)
	WebDavRestore(app)
}

// @Summary		Login to WebDav server
// @Description	Authenticate with a WebDav server
// @Tags			webdav
// @Accept			json
// @Produce		json
// @Param			credentials	body		WebDavLoginJson	true	"WebDav login credentials"
// @Success		200			{object}	result.Result[string]
// @Failure		400			{object}	result.Result[string]	"Invalid JSON or missing required fields"
// @Router			/drive/login [post]
func WebDavLogin(app *fiber.App) fiber.Router {
	return app.Post("/drive/login", func(c *fiber.Ctx) error {
		var jsonReq *WebDavLoginJson

		if e := json.Unmarshal(c.Body(), &jsonReq); e != nil {
			return c.JSON(result.NewErrorResult("Invalid JSON in request body sent to miru_core", 400, nil))
		}

		host, user, passwd := jsonReq.Host, jsonReq.User, jsonReq.Passwd

		if host == "" || user == "" || passwd == "" {
			return c.JSON(result.NewErrorResult("Invalid URL in resuest body sent to miru_core", 400, nil))
		}

		result, err := handler.Login(host, user, passwd)
		if err != nil {
			return err
		}
		return c.JSON(result)
	})
}

// @Summary		Backup database to WebDav
// @Description	Backup the Miru database to a connected WebDav server
// @Tags			webdav
// @Produce		json
// @Success		200	{object}	result.Result[string]
// @Failure		500	{object}	result.Result[string]	"Backup failed"
// @Router			/drive/backup [get]
func WebDavBackup(app *fiber.App) fiber.Router {
	return app.Get("/drive/backup", func(c *fiber.Ctx) error {
		result, err := handler.Backup()
		if err != nil {
			return err
		}
		return c.JSON(result)
	})
}

// @Summary		Restore database from WebDav
// @Description	Restore the Miru database from a backup on a connected WebDav server
// @Tags			webdav
// @Produce		json
// @Success		200	{object}	result.Result[string]
// @Failure		500	{object}	result.Result[string]	"Restore failed"
// @Router			/drive/restore [get]
func WebDavRestore(app *fiber.App) fiber.Router {
	return app.Get("/drive/restore", func(c *fiber.Ctx) error {
		result, err := handler.Restore()
		if err != nil {
			return err
		}
		return c.JSON(result)
	})
}
