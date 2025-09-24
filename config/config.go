package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config represents the application configuration
type Config struct {
	Database struct {
		Driver         string `json:"driver"`
		Host           string `json:"host"`
		Port           int    `json:"port"`
		User           string `json:"user"`
		Password       string `json:"password"`
		DBName         string `json:"dbname"`
		SSLMode        string `json:"sslmode"`
		CookieLocation string `json:"cookieLocation"`
	} `json:"database"`
	ExtensionPath string `json:"extensionPath"`
	Debug         bool   `json:"debug"`
}

var (
	// Global is the global configuration instance
	Global Config
)

// Load loads configuration from a file
func Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &Global)
	if err != nil {
		return err
	}

	// Ensure extension path is absolute
	if !filepath.IsAbs(Global.ExtensionPath) {
		Global.ExtensionPath, err = filepath.Abs(Global.ExtensionPath)
		if err != nil {
			return err
		}
	}

	return nil
}

// Save saves the current configuration to a file
func Save(path string) error {
	data, err := json.MarshalIndent(Global, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetDefaultConfig returns a default configuration
func GetDefaultConfig() Config {
	cfg := Config{}
	cfg.Database.Driver = "sqlite3"
	cfg.Database.Host = "localhost"
	cfg.Database.Port = 5432
	cfg.Database.User = "miru"
	cfg.Database.DBName = "miru"
	cfg.Database.SSLMode = "disable"
	cfg.ExtensionPath = "./extensions"
	cfg.Debug = false
	return cfg
}
