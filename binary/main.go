package binary

import (
	"log"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/config"
	"github.com/miru-project/miru-core/ext"
	"github.com/miru-project/miru-core/pkg/anilist"
	jsext "github.com/miru-project/miru-core/pkg/jsExtension"
	"github.com/miru-project/miru-core/pkg/network"
	"github.com/miru-project/miru-core/router"
	_ "golang.org/x/mobile/bind"
)

func InitProgram(configPath *string) {

	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		log.Printf("Configuration file not found at %s, creating with default settings", *configPath)
		defaultConfig := config.GetDefaultConfig()
		config.Global = defaultConfig
		if err := config.Save(*configPath); err != nil {
			log.Fatalf("Failed to create default configuration (%s): %v", *configPath, err)
		}
	} else if err != nil {
		log.Fatalf("Error checking configuration file: %v", err)
	} else {
		if err := config.Load(*configPath); err != nil {
			log.Fatalf("Failed to load configuration (%s): %v", *configPath, err)
		}
	}

	// Make extension path absolute if needed
	if !filepath.IsAbs(config.Global.ExtensionPath) {
		absPath, err := filepath.Abs(config.Global.ExtensionPath)
		if err != nil {
			log.Fatalf("Failed to resolve absolute path for extensions: %v", err)
		}
		config.Global.ExtensionPath = absPath
	}

	ext.EntClient()
	anilist.InitToken()
	network.InitCookieJar()
	go jsext.InitRuntime(config.Global.ExtensionPath, f)
	log.Println("Miru Core initialized successfully!")
	app := fiber.New()

	router.InitRouter(app)

}
