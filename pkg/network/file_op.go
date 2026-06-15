package network

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func SanitizeFilename(name string) string {
	// Remove illegal characters
	illegalChars := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	safeName := illegalChars.ReplaceAllString(name, " ")

	// Windows specific: Filenames cannot end in a space or a period
	safeName = strings.TrimRight(safeName, " .")

	// Windows specific: Block reserved system names
	reservedNames := map[string]bool{
		"CON": true, "PRN": true, "AUX": true, "NUL": true,
		"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
		"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	}

	baseName := strings.ToUpper(safeName)
	if reservedNames[baseName] || (strings.Contains(baseName, ".") && reservedNames[strings.Split(baseName, ".")[0]]) {
		safeName = "miru_" + safeName
	}

	// Fallback for completely empty names
	if len(strings.TrimSpace(safeName)) == 0 {
		safeName = "unnamed_file"
	}

	// Clean up accidental double spaces left behind by replacements
	safeName = strings.Join(strings.Fields(safeName), " ")

	return safeName
}

func SanitizeFolderName(name string) string {
	// Remove illegal characters
	illegalChars := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	safeName := illegalChars.ReplaceAllString(name, " ")

	// Windows specific: Folder names cannot end in a space or a period
	safeName = strings.TrimRight(safeName, " .")

	// Windows specific: Block reserved system names
	reservedNames := map[string]bool{
		"CON": true, "PRN": true, "AUX": true, "NUL": true,
		"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
		"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	}

	baseName := strings.ToUpper(safeName)
	if reservedNames[baseName] || (strings.Contains(baseName, ".") && reservedNames[strings.Split(baseName, ".")[0]]) {
		safeName = "miru_" + safeName
	}

	// Fallback for completely empty names
	if len(strings.TrimSpace(safeName)) == 0 {
		safeName = "unnamed_folder"
	}

	// Clean up accidental double spaces left behind by replacements
	safeName = strings.Join(strings.Fields(safeName), " ")

	return safeName
}

func SanitizeFolderPath(dir string) string {
	// Clean and split directory path
	dir = filepath.Clean(dir)
	vol := filepath.VolumeName(dir)
	dirWithoutVol := dir[len(vol):]

	// Split by separator (slash or backslash)
	parts := strings.FieldsFunc(dirWithoutVol, func(r rune) bool {
		return r == '/' || r == '\\'
	})

	var sanitizedParts []string
	for _, part := range parts {
		if part == "." {
			continue
		}
		if part == ".." {
			sanitizedParts = append(sanitizedParts, part)
			continue
		}
		sanitizedParts = append(sanitizedParts, SanitizeFolderName(part))
	}

	// Reconstruct sanitized directory path
	var sanitizedDir string
	if len(sanitizedParts) > 0 {
		base := vol
		if len(dirWithoutVol) > 0 && (dirWithoutVol[0] == '/' || dirWithoutVol[0] == '\\') {
			base = vol + string(filepath.Separator)
		}
		sanitizedDir = filepath.Join(append([]string{base}, sanitizedParts...)...)
	} else {
		if vol != "" {
			if len(dirWithoutVol) > 0 && (dirWithoutVol[0] == '/' || dirWithoutVol[0] == '\\') {
				sanitizedDir = vol + string(filepath.Separator)
			} else {
				sanitizedDir = vol
			}
		} else {
			if len(dirWithoutVol) > 0 && (dirWithoutVol[0] == '/' || dirWithoutVol[0] == '\\') {
				sanitizedDir = string(filepath.Separator)
			} else {
				sanitizedDir = "."
			}
		}
	}

	// Localize the path if possible to ensure it fits system conventions
	if local, err := filepath.Localize(sanitizedDir); err == nil {
		sanitizedDir = local
	}

	return sanitizedDir
}

func TouchFile(filePath string) (*os.File, error) {
	dir := filepath.Dir(filePath)
	fileName := filepath.Base(filePath)

	// Sanitize folder path and filename
	sanitizedDir := SanitizeFolderPath(dir)
	safeFileName := SanitizeFilename(fileName)

	// Create directories
	if err := os.MkdirAll(sanitizedDir, 0755); err != nil {
		return nil, err
	}

	// Construct full sanitized file path
	safeFilePath := filepath.Join(sanitizedDir, safeFileName)

	file, err := os.Create(safeFilePath)
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
