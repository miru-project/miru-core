package jsExtension

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"path/filepath"

	log "github.com/miru-project/miru-core/pkg/logger"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/ext"
	"github.com/miru-project/miru-core/pkg/network"
)

type GithubExtension struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	License     string  `json:"license"`
	Version     string  `json:"version"`
	Author      string  `json:"author"`
	Icon        *string `json:"icon,omitempty"`
	Type        string  `json:"type"`
	Language    string  `json:"lang"`
	Website     string  `json:"webSite"`
	IsNsfw      string  `json:"nsfw,omitempty"`
	Package     string  `json:"package"`
}

var fetchedExtensionRepo map[string][]GithubExtension

func LoadExtensionRepo() ([]*ent.ExtensionRepoSetting, error) {
	repo, e := ext.GetAllRepositories()
	if e != nil {
		return nil, e
	}
	if len(repo) == 0 {
		// Create a default repository if none exists
		ext.SetDefaultRepository()
		repo, _ = ext.GetAllRepositories()
	}
	return repo, nil
}

func SaveExtensionRepo(repoUrl string, name string) error {
	if _, err := url.Parse(repoUrl); err != nil {
		return fmt.Errorf("invalid repository URL: %s", repoUrl)
	}
	return ext.SetRepository(name, repoUrl)
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
		if e := json.Unmarshal([]byte(req), &ex); e != nil {
			log.Println("Failed to parse extension repository", rep.Link, ":", e)
			err[rep.Link] = e
			continue
		}
		fetchedExtensionRepo[rep.Link] = ex
	}
	return fetchedExtensionRepo, err, nil
}

func DownloadExtension(repoUrl string, pkg string) error {
	if len(fetchedExtensionRepo) == 0 {
		FetchExtensionRepo()
	}
	repo, ok := fetchedExtensionRepo[repoUrl]
	if !ok {
		return fmt.Errorf("package %s not found in %s", pkg, repoUrl)
	}
	link, e := url.Parse(repoUrl)
	if e != nil {
		return fmt.Errorf("invalid repository URL: %s", repoUrl)
	}
	for _, ext := range repo {

		if ext.Package == pkg {
			link.Path = path.Join(path.Dir(link.Path), "repo", ext.Package+".js")
			fileName := path.Base(link.Path)
			res, e := network.Request[[]byte](link.String(), &network.RequestOptions{Method: "GET"}, network.ReadAll)
			if e != nil {
				return fmt.Errorf("failed to download package %s from %s: %v", pkg, link.String(), e)
			}
			if e := network.SaveFile(filepath.Join(ExtPath, fileName), &res); e != nil {
				return fmt.Errorf("failed to save js extension %s to %s: %v", pkg, ExtPath, e)
			}
			log.Println("Downloaded package:", ext.Package, "from", link.String())

			// ex, e := ParseExtMetadata(string(res), fileName)
			// if e != nil {
			// 	return fmt.Errorf("failed to parse metadata for package %s: %v", pkg, e)
			// }

			// ReloadExtension(ex, res)
			return nil
		}
	}
	return fmt.Errorf("package %s not found in repository %s", pkg, repoUrl)
}

func RemoveExtensionRepo(id string) error {
	return ext.RemoveExtensionRepo(id)

}

func RemoveExtension(pkg string) error {
	loc := filepath.Join(ExtPath, pkg+".js")
	if e := network.DeleteFile(loc); e != nil {
		return fmt.Errorf("failed to delete extension file %s: %v", loc, e)
	}
	log.Println("Deleted extension file:", loc)
	return nil
}
