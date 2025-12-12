package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config represents the application configuration
type Config struct {
	Database struct {
		Driver   string `json:"driver"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		DBName   string `json:"dbname"`
		SSLMode  string `json:"sslmode"`
	} `json:"database"`
	ExtensionPath  string `json:"extensionPath"`
	CookieStoreLoc string `json:"cookieStoreLocation"`
	Address        string `json:"address"`
	Port           string `json:"port"`
	BTDataDir      string `json:"btDataDir"`
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

	// Apply defaults for empty/nil values
	applyConfigDefaults(&Global)

	// Ensure extension path is absolute
	if !filepath.IsAbs(Global.ExtensionPath) {
		Global.ExtensionPath, err = filepath.Abs(Global.ExtensionPath)
		if err != nil {
			return err
		}
	}
	if !filepath.IsAbs(Global.BTDataDir) {
		base := filepath.Dir(Global.ExtensionPath)
		Global.BTDataDir = filepath.Join(base, "bt-data")
	}
	return nil
}

// applyConfigDefaults applies default values to nil/empty fields
func applyConfigDefaults(cfg *Config) {
	// Database defaults
	if cfg.Database.Driver == "" {
		cfg.Database.Driver = "sqlite3"
	}
	if cfg.Database.Host == "" {
		cfg.Database.Host = "localhost"
	}
	if cfg.Database.Port == 0 {
		cfg.Database.Port = 5432
	}
	if cfg.Database.User == "" {
		cfg.Database.User = "miru"
	}
	if cfg.Database.DBName == "" {
		cfg.Database.DBName = "miru"
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}

	// Top-level defaults
	if cfg.ExtensionPath == "" {
		cfg.ExtensionPath = "./extensions"
	}
	if cfg.Address == "" {
		cfg.Address = "127.0.0.1"
	}
	if cfg.Port == "" {
		cfg.Port = "3000"
	}
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
	cfg.Address = "127.0.0.1"
	cfg.Port = "3000"
	return cfg
}
