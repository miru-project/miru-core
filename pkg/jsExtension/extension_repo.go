package jsExtension

import (
	"encoding/json"
	"log"

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

func LoadExtensionRepo() ([]*ent.ExtensionRepo, error) {
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

func FetchExtensionRepo(repo []*ent.ExtensionRepo) (map[string][]GithubExtension, map[string]error) {
	result := make(map[string][]GithubExtension)
	err := make(map[string]error)
	for _, rep := range repo {
		req, e := network.Request[string](rep.URL, &network.RequestOptions{Method: "GET"}, network.ReadAll)
		if e != nil {
			log.Println("Failed to fetch extension repository", rep.URL, ":", e)
			err[rep.URL] = e
			continue
		}
		var ex []GithubExtension
		if e := json.Unmarshal([]byte(req), &ex); e != nil {
			log.Println("Failed to parse extension repository", rep.URL, ":", e)
			err[rep.URL] = e
			continue
		}
		result[rep.URL] = ex
	}
	return result, err
}
