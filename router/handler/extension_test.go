package handler

import (
	"os"
	"path/filepath"
	"testing"

	endpoint "github.com/miru-project/miru-core/pkg/extension/endpoint"
	golang "github.com/miru-project/miru-core/pkg/extension/golang"
	js "github.com/miru-project/miru-core/pkg/extension/js"
	"github.com/stretchr/testify/assert"
)

func TestGetRuntimeUnknownReturnsError(t *testing.T) {
	_, err := endpoint.GetRuntime("missing-extension")
	assert.Error(t, err)
}

func TestGetRuntimeDetectsGolang(t *testing.T) {
	golangDir, err := filepath.Abs(filepath.Join("..", "..", "pkg", "extension", "golang", "extensions", "example"))
	assert.NoError(t, err)
	origDir := golang.ExtensionDir
	defer func() { golang.ExtensionDir = origDir }()

	golang.ExtensionDir = golangDir
	rt, err := endpoint.GetRuntime("example")
	assert.NoError(t, err)
	assert.IsType(t, golang.ExtensionRuntime{}, rt)
}

func TestGetRuntimeDetectsJavaScript(t *testing.T) {
	jsDir, err := os.MkdirTemp("", "miru-js-ext")
	assert.NoError(t, err)
	defer os.RemoveAll(jsDir)

	origPath := js.ExtPath
	defer func() { js.ExtPath = origPath }()

	js.ExtPath = jsDir
	assert.NoError(t, os.WriteFile(filepath.Join(jsDir, "example.js"), []byte("// ==MiruExtension==\n// @package example\n// ==/MiruExtension=="), 0644))
	rt, err := endpoint.GetRuntime("example")
	assert.NoError(t, err)
	assert.IsType(t, js.Runtime{}, rt)
}
