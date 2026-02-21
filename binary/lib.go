package binary

import (
	"embed"
	"fmt"

	_ "golang.org/x/mobile/bind"
)

//go:embed assets/*
var f embed.FS

type (
	AndroidLib struct{}
)

func (a *AndroidLib) InitAAR(configPath string) string {
	var result string = "Ok"
	defer func() {
		if r := recover(); r != nil {
			result = "Error: " + fmt.Sprint(r)
		}
	}()
	if result == "" {
		return result
	}
	go InitProgram(&configPath)
	return result
}
