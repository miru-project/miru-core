package binary

import (
	"os"
	"path/filepath"

	fasthttp_router "github.com/fasthttp/router"
	"github.com/miru-project/miru-core/config"
	"github.com/miru-project/miru-core/ext"
	golang "github.com/miru-project/miru-core/pkg/extension/golang"
	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/download"
	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
	jsext "github.com/miru-project/miru-core/pkg/extension/js"
	log "github.com/miru-project/miru-core/pkg/logger"
	"github.com/miru-project/miru-core/pkg/network"
	"github.com/miru-project/miru-core/pkg/torrent"
	"github.com/miru-project/miru-core/router"
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

	app := fasthttp_router.New()
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

	network.Init()
	ext.EntClient()
	db.Initialize()
	torrent.Init()
	download.Init()
	jsext.InitRuntime(config.Global.ExtensionPath, f)
	// The Go/Scriggo extension runtime resolves packages by stat-ing
	// <ExtensionDir>/<pkg>.go. Unlike js.ExtPath (set inside
	// InitRuntime), golang.ExtensionDir is never initialized, so .go
	// extensions could never be found. Set it here so .go extensions are
	// discoverable and resolvable.
	golang.ExtensionDir = config.Global.ExtensionPath
	// Eagerly load Go/Scriggo extensions at startup (mirrors jsext.InitRuntime)
	// so they are visibly loaded and any compile error surfaces early.
	golang.LoadExtensions()
	log.Println("Miru Core initialized successfully!")
}
