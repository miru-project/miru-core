package router

import (
	fasthttp_router "github.com/fasthttp/router"
	"github.com/miru-project/miru-core/config"
	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
	"github.com/valyala/fasthttp"
)

// InitRouter initializes all API routes for the application
//
//	@Summary		Initialize API routes
//	@Description	Sets up all available API endpoints for the Miru application
func InitRouter(app *fasthttp_router.Router) {
	// Initialize Swagger docs

	initWebDavRouter(app)
	initAnilistRouter(app)
	initTorrentRouter(app)
	// app.Get("/swagger/*", fiberSwagger.WrapHandler)
	go StartGRPCServer()
	startListening(app, config.Global.Address+":"+config.Global.Port)

}

func startListening(app *fasthttp_router.Router, host string) {
	if e := fasthttp.ListenAndServe(host, app.Handler); e != nil {
		errorhandle.PanicF("Can't listen on host %q: %s", host, e)
	}
}

// WebDavLoginJson defines the structure for WebDAV login requests
type WebDavLoginJson struct {
	Host   string `json:"host"`
	Passwd string `json:"passwd"`
	User   string `json:"user"`
}
