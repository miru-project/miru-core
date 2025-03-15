package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/pkg/database"
	ext "github.com/miru-project/miru-core/pkg/extension"
	miru_path "github.com/miru-project/miru-core/pkg/path"
	"github.com/miru-project/miru-core/router"
)

func main() {
	miru_path.InitPath()
	database.Start()
	runExt()
	app := fiber.New()

	router.InitRouter(app)

	app.Listen(":3000")
}

// temporty
func runExt() {

	ext.InitRuntime()

}
