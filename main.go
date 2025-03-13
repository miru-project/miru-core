package main

import (
	"github.com/gofiber/fiber/v2"
	ext "github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/router"
)

func main() {
	runExt()
	app := fiber.New()

	router.InitRouter(app)

	app.Listen(":3000")
}

// temporty
func runExt() {

	ext.InitRuntime()

}
