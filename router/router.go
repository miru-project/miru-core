package router

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/config"
	errorhandle "github.com/miru-project/miru-core/errorHandle"
	"github.com/miru-project/miru-core/ext"
	"github.com/miru-project/miru-core/handler"
	"github.com/miru-project/miru-core/pkg/result"
	// fiberSwagger "github.com/swaggo/fiber-swagger"
	// _ "github.com/miru-project/miru-core/docs" // Import generated docs
)

//	@title			Miru Core API
//	@version		1.0
//	@description	Miru Core API documentation

//	@contact.name	API Support
//	@contact.url	https://github.com/miru-project

//	@license.name	MIT
//	@license.url	https://github.com/miru-project/miru-core/blob/main/LICENSE

//	@host		127.127.127.127:12777
//	@BasePath	/
//	@schemes	http https

// InitRouter initializes all API routes for the application
//
//	@Summary		Initialize API routes
//	@Description	Sets up all available API endpoints for the Miru application
func InitRouter(app *fiber.App) {
	// Initialize Swagger docs

	RootEndpoint(app)
	initExtensionRouter(app)
	initWebDavRouter(app)
	initAnilistRouter(app)
	initDownloadRouter(app)
	initNetworkRouter(app)
	initDBRouter(app)
	GetAppSetting(app)
	SetAppSetting(app)
	// app.Get("/swagger/*", fiberSwagger.WrapHandler)
	startListening(app, config.Global.Address+":"+config.Global.Port)

}

func startListening(app *fiber.App, host string) {
	if e := app.Listen(host); e != nil {
		errorhandle.PanicF("Can't listen on host %q: %s", host, e)
	}
}

// @Summary		Root endpoint
// @Description	Returns basic information about Miru
// @Tags			root
// @Produce		json
// @Success		200	{object}	result.Result[string]
// @Router			/ [get]
func RootEndpoint(app *fiber.App) fiber.Router {
	return app.Get("/", func(c *fiber.Ctx) error {
		result, err := handler.HelloMiru()
		if err != nil {
			return err
		}
		return c.JSON(result)
	})
}

// @Summary		Get application settings
// @Description	Retrieves the current application settings
// @Tags			settings
// @Produce		json
// @Success		200	{object}	result.Result[[]ent.AppSetting]
// @Router			/appSetting [get]
func GetAppSetting(app *fiber.App) fiber.Router {
	return app.Get("/appSetting", func(c *fiber.Ctx) error {
		result, err := handler.GetAppSetting()
		if err != nil {
			return err
		}
		return c.JSON(result)
	})
}

// @Summary		Update application settings
// @Description	Updates the application settings with new values
// @Tags			settings
// @Accept			json
// @Produce		json
// @Param			settings	body	[]ext.AppSettingJson	true	"Application settings"
// @Success		200 {object}	result.Result[string]
// @Failure		400	{object}	result.Result[string]	"Invalid JSON"
// @Failure		500	{object}	result.Result[string]	"Server error"
// @Router			/appSetting [put]
func SetAppSetting(app *fiber.App) fiber.Router {
	return app.Put("/appSetting", func(c *fiber.Ctx) error {
		var jsonUrl *[]ext.AppSettingJson

		if e := json.Unmarshal(c.Body(), &jsonUrl); e != nil {
			return c.JSON(result.NewErrorResult("Invalid JSON in request body sent to miru_core", 400, nil))
		}

		if jsonUrl == nil {
			return c.JSON(result.NewErrorResult("Invalid JSON in request body sent to miru_core", 400, nil))
		}

		if err := handler.SetAppSetting(jsonUrl); err != nil {

			// Generate error string
			errStr := "Failed to set app settings"
			for _, e := range err {
				errStr += e.Error() + ","
			}

			return c.JSON(result.NewErrorResult(errStr, 500, nil))
		}

		return c.JSON(result.NewSuccessResult("Settings updated successfully"))
	})
}

type WebDavLoginJson struct {
	Host   string `json:"host"`
	Passwd string `json:"passwd"`
	User   string `json:"user"`
}
