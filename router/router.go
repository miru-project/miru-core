package router

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/ext"
	"github.com/miru-project/miru-core/handler"
	"github.com/miru-project/miru-core/pkg/result"
)

func InitRouter(app *fiber.App) {

	app.Get("/", func(c *fiber.Ctx) error {
		result, err := handler.HelloMiru()
		if err != nil {
			return err
		}
		return c.JSON(result)
	})

	initExtensionRouter(app)
	initWebDavRouter(app)
	initAnilistRouter(app)
	initDownloadRouter(app)

	app.Get("/appSetting", func(c *fiber.Ctx) error {
		result, err := handler.GetAppSetting()
		if err != nil {
			return err
		}
		return c.JSON(result)

	})

	app.Put("/appSetting", func(c *fiber.Ctx) error {
		var jsonUrl *[]ext.AppSettingJson

		if e := json.Unmarshal(c.Body(), &jsonUrl); e != nil {
			return c.JSON(result.NewErrorResult("Invalid JSON in request body sent to miru_core", 400))
		}

		if jsonUrl == nil {
			return c.JSON(result.NewErrorResult("Invalid JSON in request body sent to miru_core", 400))
		}

		if err := handler.SetAppSetting(jsonUrl); err != nil {

			// Generate error string
			errStr := "Failed to set app settings"
			for _, e := range err {
				errStr += e.Error() + ","
			}

			return c.JSON(result.NewErrorResult(errStr, 500))
		}

		return c.SendStatus(200)

	})

}

type WebDavLoginJson struct {
	Host   string `json:"host"`
	Passwd string `json:"passwd"`
	User   string `json:"user"`
}
