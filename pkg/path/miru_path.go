package miru_path

import (
	"os"

	"github.com/adrg/xdg"
)

var MiruDir string

func InitPath() {
	MiruDir = xdg.UserDirs.Documents + "/miru"
	if _, err := os.Stat(MiruDir); os.IsNotExist(err) {
		os.Mkdir(MiruDir, 0755)
	}
}
