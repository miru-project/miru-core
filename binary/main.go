package binary

import (
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/config"
	errorhandle "github.com/miru-project/miru-core/errorHandle"
	"github.com/miru-project/miru-core/ext"
	"github.com/miru-project/miru-core/pkg/anilist"
	jsext "github.com/miru-project/miru-core/pkg/jsExtension"
	log "github.com/miru-project/miru-core/pkg/logger"
	"github.com/miru-project/miru-core/pkg/network"
	"github.com/miru-project/miru-core/router"
	_ "golang.org/x/mobile/bind"
)

func InitProgram(configPath *string) {

	// Initialize logger
	log.InitLog(filepath.Dir(*configPath))

	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		log.Printf("Configuration file not found at %s, creating with default settings", *configPath)
		defaultConfig := config.GetDefaultConfig()
		config.Global = defaultConfig
		if err := config.Save(*configPath); err != nil {
			errorhandle.PanicF("Failed to create default configuration (%s): %v", *configPath, err)
		}
	} else if err != nil {
		errorhandle.PanicF("Error checking configuration file: %v", err)
	} else {
		if err := config.Load(*configPath); err != nil {
			errorhandle.PanicF("Failed to load configuration (%s): %v", *configPath, err)
		}
	}

	app := fiber.New()
	Init()
	router.InitRouter(app)

}

func Init() {
	// Make extension path absolute if needed
	if !filepath.IsAbs(config.Global.ExtensionPath) {
		absPath, err := filepath.Abs(config.Global.ExtensionPath)
		if err != nil {
			errorhandle.PanicF("failed to get absolute path for extension path: %v", err)
		}
		config.Global.ExtensionPath = absPath
	}

	ext.EntClient()
	anilist.InitToken()
	network.InitCookieJar()
	jsext.InitRuntime(config.Global.ExtensionPath, f)
	log.Println("Miru Core initialized successfully!")
}
