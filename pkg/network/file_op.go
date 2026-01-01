package network

import (
	"os"
	"path/filepath"
)

func TouchFile(filePath string) (*os.File, error) {

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func SaveFile(filePath string, data *[]byte) error {

	file, err := TouchFile(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write the data to file
	_, err = file.Write(*data)
	if err != nil {
		return err
	}

	return nil
}

func DeleteFile(filePath string) error {
	if err := os.Remove(filePath); err != nil {
		return err
	}
	return nil
}
