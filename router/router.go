package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/config"
	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
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

	initWebDavRouter(app)
	initAnilistRouter(app)
	initTorrentRouter(app)
	// app.Get("/swagger/*", fiberSwagger.WrapHandler)
	go StartGRPCServer()
	startListening(app, config.Global.Address+":"+config.Global.Port)

}

func startListening(app *fiber.App, host string) {
	if e := app.Listen(host); e != nil {
		errorhandle.PanicF("Can't listen on host %q: %s", host, e)
	}
}

// WebDavLoginJson defines the structure for WebDAV login requests
type WebDavLoginJson struct {
	Host   string `json:"host"`
	Passwd string `json:"passwd"`
	User   string `json:"user"`
}
