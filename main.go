package main

import (
	"embed"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/config"
	"github.com/miru-project/miru-core/ext"
	"github.com/miru-project/miru-core/pkg/anilist"
	jsext "github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/router"
)
import "C"

//go:embed  assets/*
var f embed.FS

func main() {
	// Parse command line arguments
	configPath := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()

	// Load configuration
	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		log.Printf("Configuration file not found at %s, creating with default settings", *configPath)
		defaultConfig := config.GetDefaultConfig()
		config.Global = defaultConfig
		if err := config.Save(*configPath); err != nil {
			log.Fatalf("Failed to create default configuration: %v", err)
		}
	} else if err != nil {
		log.Fatalf("Error checking configuration file: %v", err)
	} else {
		if err := config.Load(*configPath); err != nil {
			log.Fatalf("Failed to load configuration: %v", err)
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
	jsext.InitRuntime(config.Global.ExtensionPath, f)

	log.Println("Miru Core initialized successfully!")
	app := fiber.New()

	router.InitRouter(app)
	app.Listen("127.127.127.127:12777")
}

//export initDyLib
func initDyLib() {
	go main()
}
