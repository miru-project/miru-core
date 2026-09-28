package js

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/miru-project/miru-core/pkg/db"
	log "github.com/miru-project/miru-core/pkg/logger"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/pkg/network"
)

type GithubExtension struct {
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	License     string   `json:"license"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Icon        *string  `json:"icon,omitempty"`
	Type        string   `json:"type"`
	Language    string   `json:"lang"`
	Website     string   `json:"webSite"`
	IsNsfw      FlexBool `json:"nsfw,omitempty"`
	Package     string   `json:"package"`
	// URL is the entry's source path inside the repo (js/<pkg>.js or
	// golang/<pkg>.go). It is the only thing that tells the two runtimes apart,
	// so a download must follow it instead of assuming <pkg>.js.
	URL string `json:"url,omitempty"`
}

// FlexBool accepts the two shapes repositories use for the nsfw flag: a JSON
// boolean or the string "true"/"false". It always marshals back as a boolean
// so clients never receive a string where they expect a bool.
type FlexBool bool

// UnmarshalJSON decodes a JSON bool or string into FlexBool.
func (b *FlexBool) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	*b = FlexBool(strings.EqualFold(s, "true") || s == "1")
	return nil
}

// MarshalJSON emits the flag as a plain JSON boolean.
func (b FlexBool) MarshalJSON() ([]byte, error) {
	if b {
		return []byte("true"), nil
	}
	return []byte("false"), nil
}

var fetchedExtensionRepo map[string][]GithubExtension

func LoadExtensionRepo() ([]*ent.ExtensionRepoSetting, error) {
	repo, e := db.GetAllRepositories()
	if e != nil {
		return nil, e
	}
	if len(repo) == 0 {
		// Create a default repository if none exists
		db.SetDefaultRepository()
		repo, _ = db.GetAllRepositories()
	}
	return repo, nil
}

func SaveExtensionRepo(repoUrl string, name string) error {
	if _, err := url.Parse(repoUrl); err != nil {
		return fmt.Errorf("invalid repository URL: %s", repoUrl)
	}
	return db.SetRepository(name, repoUrl)
}

func FetchExtensionRepo() (map[string][]GithubExtension, map[string]error, error) {
	repo, e := LoadExtensionRepo()
	if e != nil {
		return nil, nil, e
	}
	fetchedExtensionRepo = make(map[string][]GithubExtension)
	err := make(map[string]error)
	for _, rep := range repo {
		req, e := network.Request[string](rep.Link, &network.RequestOptions{Method: "GET"}, network.ReadAll)
		if e != nil {
			log.Println("Failed to fetch extension repository", rep.Link, ":", e)
			err[rep.Link] = e
			continue
		}
		var ex []GithubExtension
		if e := json.Unmarshal([]byte(req.Body), &ex); e != nil {
			log.Println("Failed to parse extension repository", rep.Link, ":", e)
			err[rep.Link] = e
			continue
		}
		fetchedExtensionRepo[rep.Link] = ex
	}
	return fetchedExtensionRepo, err, nil
}

// extensionSourceURL resolves the absolute URL of an extension's source file.
// The repo index publishes the path per entry (js/<pkg>.js or
// golang/<pkg>.go), so it is followed verbatim: a golang extension requested as
// <pkg>.js 404s. Repos that omit the field fall back to the historical js
// layout.
func extensionSourceURL(repoUrl string, ext GithubExtension) (string, error) {
	rel := strings.TrimSpace(ext.URL)
	if rel == "" {
		rel = path.Join("js", ext.Package+string(extension.LanguageJS))
	}
	// A repo may publish a fully qualified link instead of a relative path.
	if abs, e := url.Parse(rel); e == nil && abs.IsAbs() {
		return abs.String(), nil
	}
	link, e := url.Parse(repoUrl)
	if e != nil {
		return "", fmt.Errorf("invalid repository URL: %s", repoUrl)
	}
	rel = strings.TrimPrefix(rel, "/")
	// Tolerate indexes that spell the path with the repo/ directory included.
	rel = strings.TrimPrefix(rel, "repo/")
	link.Path = path.Join(path.Dir(link.Path), "repo", rel)
	return link.String(), nil
}

func DownloadExtension(repoUrl string, pkg string) error {
	if len(fetchedExtensionRepo) == 0 {
		FetchExtensionRepo()
	}
	repo, ok := fetchedExtensionRepo[repoUrl]
	if !ok {
		return fmt.Errorf("package %s not found in %s", pkg, repoUrl)
	}
	for _, ext := range repo {
		if ext.Package != pkg {
			continue
		}
		sourceUrl, e := extensionSourceURL(repoUrl, ext)
		if e != nil {
			return e
		}
		res, e := network.Request[[]byte](sourceUrl, &network.RequestOptions{Method: "GET"}, network.ReadAll)
		if e != nil {
			return fmt.Errorf("failed to download package %s from %s: %v", pkg, sourceUrl, e)
		}
		// network.Request resolves error responses instead of failing, so an
		// error page body would otherwise be written out as the extension.
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return fmt.Errorf("failed to download package %s from %s: status %d", pkg, sourceUrl, res.StatusCode)
		}
		// The suffix follows the published path, so a golang extension lands as
		// <pkg>.go and is picked up by the golang runtime's directory scan.
		fileName := network.SanitizeFilename(path.Base(sourceUrl))
		if e := network.SaveFile(filepath.Join(ExtPath, fileName), &res.Body); e != nil {
			return fmt.Errorf("failed to save extension %s to %s: %v", pkg, ExtPath, e)
		}
		log.Println("Downloaded package:", ext.Package, "from", sourceUrl)
		return nil
	}
	return fmt.Errorf("package %s not found in repository %s", pkg, repoUrl)
}

func RemoveExtensionRepo(id string) error {
	return db.RemoveExtensionRepo(id)

}

// RemoveExtension deletes the extension source of pkg. Both runtimes share one
// directory, so the file suffix is looked up instead of assumed: a golang
// extension is <pkg>.go and would otherwise survive an uninstall.
func RemoveExtension(pkg string) error {
	var lastErr error
	for _, lang := range []extension.Language{extension.LanguageJS, extension.LanguageGolang} {
		loc := filepath.Join(ExtPath, pkg+string(lang))
		if _, e := os.Stat(loc); e != nil {
			continue
		}
		if e := network.DeleteFile(loc); e != nil {
			lastErr = e
			continue
		}
		log.Println("Deleted extension file:", loc)
		ApiPkgCache.Remove(pkg)
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("failed to delete extension file %s: %v", filepath.Join(ExtPath, pkg), lastErr)
	}
	return fmt.Errorf("extension file for %s not found in %s", pkg, ExtPath)
}
