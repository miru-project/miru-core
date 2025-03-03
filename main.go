package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/router"
)

func main() {
	app := fiber.New()

	router.InitRouter(app)

	app.Listen(":3000")
}
