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

// func storeLog(fileDir string, fileName string, msg string) {
// 	msgByte := []byte(msg)

// 	file, err := os.Create(fileDir + fileName)

// 	if err != nil {
// 		fmt.Println(err)
// 	}
// 	defer file.Close()
// 	numB, err := file.Write(msgByte)

// 	if err != nil {
// 		fmt.Println(err)
// 	}

// 	fmt.Printf("wrote %d bytes\n", numB)
// }

func (a *AndroidLib) InitAAR(configPath string) {

	go InitProgram(&configPath)
}
