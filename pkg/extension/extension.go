package extension

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// Language identifies the runtime that backs an extension source file. The
// language is decided purely by the source file's extension; this is the single,
// centralized routing decision used across the whole codebase.
type Language string

const (
	// LanguageJS is the JavaScript (goja) runtime, backed by .js files.
	LanguageJS Language = ".js"
	// LanguageGolang is the Golang (Scriggo) runtime, backed by .go files.
	LanguageGolang Language = ".go"
	// LanguageUnknown is returned when a file's extension is not a known runtime.
	LanguageUnknown Language = ""
)

// DetectLanguage returns the runtime Language for a source file, based solely
// on its extension. This is the centralized router: a file's suffix decides
// whether it is handled by the JavaScript or Golang runtime.
func DetectLanguage(path string) Language {
	switch filepath.Ext(path) {
	case string(LanguageJS):
		return LanguageJS
	case string(LanguageGolang):
		return LanguageGolang
	default:
		return LanguageUnknown
	}
}

var metadataRegexp = regexp.MustCompile(`@(\w+)\s+(.*)`)

// ParseExtensionMetadata parses the // ==MiruExtension== metadata header shared
// by both runtimes (the same @key value format) into a shared Extension value.
// It is universal: it does not depend on any particular runtime, only on the
// file's name/extension.
//
// The parsed source is stored in Extension.Context so runtimes that need the raw
// source (e.g. the JS runtime compiling into goja) can access it.
//
// Two per-runtime quirks are preserved so callers get exactly the previous
// result:
//   - The JS runtime requires <package>.<ext> to equal the file name; the Golang
//     runtime historically does not validate the file name, so validation is
//     skipped for .go files.
//   - The JS runtime defaults a missing/non-"2" apiVersion to "1"; the Golang
//     runtime keeps the raw value (it is v2-only and the caller supplies it).
func ParseExtensionMetadata(content string, fileName string) (*Extension, error) {
	lang := DetectLanguage(fileName)
	if lang == LanguageUnknown {
		return nil, fmt.Errorf("unsupported extension file: %s", fileName)
	}

	ext := &Extension{Name: fileName, FileLang: lang}
	for _, match := range metadataRegexp.FindAllStringSubmatch(content, -1) {
		key := match[1]
		value := strings.TrimSpace(match[2])
		switch key {
		case "name":
			ext.Name = value
		case "version":
			ext.Version = value
		case "author":
			ext.Author = value
		case "license":
			ext.License = value
		case "lang":
			ext.Lang = value
		case "icon":
			ext.Icon = value
		case "package":
			ext.Pkg = value
		case "webSite":
			ext.Website = value
		case "description":
			ext.Description = value
		case "apiVersion":
			if lang == LanguageJS {
				if value == "2" {
					ext.ApiVersion = "2"
				} else {
					ext.ApiVersion = "1"
				}
			} else {
				ext.ApiVersion = value
			}
		case "type":
			ext.WatchType = value
		case "tags":
			tagList := strings.Split(value, ",")
			for i, tag := range tagList {
				tagList[i] = strings.TrimSpace(tag)
			}
			ext.Tags = tagList
		}
	}

	if lang != LanguageGolang {
		// The JS runtime historically requires the package name plus the file
		// extension to match the file name exactly.
		if ext.Pkg+string(lang) != fileName {
			return ext, fmt.Errorf("package name does not match the file name\r\n file name: %s\r\n package name: %s", fileName, ext.Pkg)
		}
	}

	ext.Context = &content
	return ext, nil
}

// FilterExtensions scans a directory and returns the parsed metadata of every
// supported extension source file, plus a map of invalid file -> error message
// (so runtimes can surface load failures). It is universal: each file's
// language is detected by extension and its metadata parsed accordingly.
func FilterExtensions(dir string) ([]*Extension, map[string]string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.Mkdir(dir, os.ModePerm); err != nil {
			log.Println("Failed to create directory:", dir)
			return nil, nil
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}

	var exts []*Extension
	invalid := map[string]string{}
	for _, file := range entries {
		if file.IsDir() {
			continue
		}
		if DetectLanguage(file.Name()) == LanguageUnknown {
			continue
		}
		f, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			invalid[file.Name()] = err.Error()
			continue
		}
		ext, err := ParseExtensionMetadata(string(f), file.Name())
		if err != nil {
			invalid[file.Name()] = err.Error()
		} else {
			exts = append(exts, ext)
		}
	}
	return exts, invalid
}

// WatchExtensions watches the given directories and invokes onChange whenever a
// supported extension source file is created, written, removed or renamed. The
// language passed to onChange is decided by the changed file's extension, so
// callers can route the event to the correct runtime. The returned watcher can
// be closed by the caller to stop watching.
func WatchExtensions(dirs []string, onChange func(lang Language, pkg string)) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	for _, dir := range dirs {
		if err := watcher.Add(dir); err != nil {
			watcher.Close()
			return nil, err
		}
	}

	go func() {
		for event := range watcher.Events {
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			lang := DetectLanguage(event.Name)
			if lang == LanguageUnknown {
				continue
			}
			name := filepath.Base(event.Name)
			pkg := strings.TrimSuffix(name, string(lang))
			onChange(lang, pkg)
		}
	}()

	return watcher, nil
}
