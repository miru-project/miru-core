package binary

import (
	"embed"
)

import "C"

//go:embed assets/*
var f embed.FS

type (
	AndroidLib struct{}
)

func (a *AndroidLib) InitAAR(configPath string) {

	go InitProgram(&configPath)
}
