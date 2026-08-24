package router

import (
	"fmt"

	fasthttp_router "github.com/fasthttp/router"
	"github.com/miru-project/miru-core/config"
	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
	"github.com/miru-project/miru-core/pkg/grpc"
	"github.com/miru-project/miru-core/pkg/logger"
	"github.com/miru-project/miru-core/pkg/network"
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
	initProxy(app)
	// Guard the gRPC server: a panic here must not take down the process.
	go func() {
		defer errorhandle.RecoverLog("grpc.StartServer")
		grpc.StartServer()
	}()
	startListening(app, config.Global.Address+":"+config.Global.Port)

}

func initProxy(app *fasthttp_router.Router) {
	app.ANY("/proxy/{path:*}", network.Proxy)
}
func startListening(app *fasthttp_router.Router, host string) {
	logger.Println("HTTP Server started on ", host)
	// Per-request panic recovery: a bad request must not crash the server.
	// Captured panics are written to the miru_core crash log and answered
	// with HTTP 500 instead of terminating the process.
	handler := func(ctx *fasthttp.RequestCtx) {
		defer func() {
			if r := recover(); r != nil {
				errorhandle.LogCrash(
					r,
					fmt.Sprintf(
						"%s %s",
						string(ctx.Method()),
						string(ctx.RequestURI()),
					),
				)
				ctx.SetStatusCode(fasthttp.StatusInternalServerError)
				ctx.SetBodyString("internal server error")
			}
		}()
		app.Handler(ctx)
	}
	if e := fasthttp.ListenAndServe(host, handler); e != nil {
		errorhandle.PanicF("Can't listen on host %q: %s", host, e)
	}
}

// WebDavLoginJson defines the structure for WebDAV login requests
type WebDavLoginJson struct {
	Host   string `json:"host"`
	Passwd string `json:"passwd"`
	User   string `json:"user"`
}
