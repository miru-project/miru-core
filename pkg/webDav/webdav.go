package webdav

import (
	"errors"
	"os"
	"strings"

	"github.com/miru-project/miru-core/config"
	"github.com/studio-b12/gowebdav"
)

var client = &gowebdav.Client{}
var file_name string

// Authenticate connects to the WebDAV server using the provided host, user, and password
func Authenticate(host string, user string, password string) error {

	client = gowebdav.NewClient(host, user, password)

	// Trim and get the fileName
	file_name = strings.TrimRight(config.Global.Database.DBName, "/")
	file_name = strings.Split(file_name, "/")[len(strings.Split(file_name, "/"))-1]

	return client.Connect()
}

func checkIsLoggedIn() error {

	if file_name == "" {
		return errors.New("WebDAV client have not logged in yet")
	}
	return nil
}

func Backup() error {
	var foundDir = false

	if err := checkIsLoggedIn(); err != nil {
		return err
	}

	files, err := client.ReadDir("/")

	if err != nil {
		return err
	}

	// Check if the Miru directory exists , if not create one
	for _, f := range files {
		if f.Name() == "Miru" || f.IsDir() {
			foundDir = true
			break
		}
	}

	if !foundDir {
		err := client.Mkdir("Miru", 0644)
		if err != nil {
			return err
		}
	}

	file, err := os.ReadFile(config.Global.Database.DBName)

	if err != nil {
		return err
	}

	// Write to the WebDAV server
	client.Write("/Miru/"+file_name, file, 0644)
	return nil
}

func Restore() error {

	if err := checkIsLoggedIn(); err != nil {
		return err
	}

	file, err := client.Read("/Miru/" + file_name)

	if err != nil {
		return err
	}

	err = os.WriteFile(config.Global.Database.DBName, file, 0644)

	if err != nil {
		return err
	}

	return nil
}
