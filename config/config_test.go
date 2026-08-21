package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadAppliesGRPCPortDefault ensures that a config file created by older
// app versions (which lacked the "gRPCPort" field entirely) still resolves to
// the documented default 3001. An empty/invalid gRPCPort would otherwise make
// the gRPC server bind to a random ephemeral port (e.g. 44458) that the
// Flutter client can't predict, causing "Connection refused".
func TestLoadAppliesGRPCPortDefault(t *testing.T) {
	dir := t.TempDir()
	// Simulate the legacy config written by the Flutter app: no "gRPCPort",
	// and a top-level "port" but no explicit gRPC port.
	cfgPath := filepath.Join(dir, "config.json")
	legacy := `{
  "database": {"driver":"sqlite3","host":"localhost","port":5432,"user":"miru","dbname":"miru.db","sslmode":"disable"},
  "cookieStoreLocation": "",
  "extensionPath": "./extensions",
  "address": "127.0.0.1",
  "port": "3000"
}`
	if err := os.WriteFile(cfgPath, []byte(legacy), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	if err := Load(cfgPath); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if Global.Port != "3000" {
		t.Errorf("Port = %q, want %q", Global.Port, "3000")
	}
	if Global.Address != "127.0.0.1" {
		t.Errorf("Address = %q, want %q", Global.Address, "127.0.0.1")
	}
	if Global.GRPCPort != "3001" {
		t.Errorf("GRPCPort = %q, want %q", Global.GRPCPort, "3001")
	}
}

// TestGetDefaultConfigIncludesGRPCPort guards GetDefaultConfig, which the
// backend uses when writing a fresh config.json, so it must carry gRPCPort.
func TestGetDefaultConfigIncludesGRPCPort(t *testing.T) {
	cfg := GetDefaultConfig()
	if cfg.GRPCPort == "" {
		t.Error("GetDefaultConfig().GRPCPort = empty, want non-empty default")
	}
	if cfg.Port == "" || cfg.Address == "" {
		t.Errorf("GetDefaultConfig() missing base fields: %+v", cfg)
	}
}
