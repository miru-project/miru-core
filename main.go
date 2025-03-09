package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/ext"
	"github.com/miru-project/miru-core/router"
)

func main() {
	app := fiber.New()
	runExt()

	router.InitRouter(app)

	app.Listen(":3000")
}

// temporty
func runExt() {

	ext.InitRuntime()

}
