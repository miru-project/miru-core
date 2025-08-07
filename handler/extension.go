package handler

import (
	"strconv"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/pkg/jsExtension"
	"github.com/miru-project/miru-core/pkg/result"
)

// handle Latest when receiving a request
func Latest(page string, pkg string) (*result.Result[any], error) {

	intPage, err := strconv.Atoi(page)
	if err != nil {
		return result.NewErrorResult("Invalid page number", 400, nil), err
	}

	res, e := jsExtension.Latest(pkg, intPage)

	if res == nil && e == nil {
		return result.NewErrorResult("No results found", 404, nil), nil
	}
	return result.NewSuccessResult(res), e

}

// handle Search when receiving a request
func Search(page string, pkg string, kw string, filter string) (*result.Result[any], error) {

	intPage, err := strconv.Atoi(page)
	if err != nil {
		return result.NewErrorResult("Invalid page number", 400, nil), err
	}

	res, e := jsExtension.Search(pkg, intPage, kw, filter)

	if res == nil {
		return result.NewErrorResult("No results found", 404, nil), nil
	}
	return result.NewSuccessResult(res), e

}

// handle Watch when receiving a request
func Watch(pkg string, url string) (*result.Result[any], error) {

	res, e := jsExtension.Watch(pkg, url)

	if res == nil {
		return result.NewErrorResult("No results found", 404, nil), nil
	}
	return result.NewSuccessResult(res), e
}

// handle Detail when receiving a request
func Detail(pkg string, url string) (*result.Result[any], error) {

	res, e := jsExtension.Detail(pkg, url)

	if res.Title == "" {
		return result.NewErrorResult("No results found", 404, nil), nil
	}
	return result.NewSuccessResult(res), e
}

// fetch the extension repository
func FetchExtensionRepo() (map[string][]jsExtension.GithubExtension, map[string]error, error) {
	return jsExtension.FetchExtensionRepo()
}

func SetExtensionRepo(repoUrl string, name string) error {
	return jsExtension.SaveExtensionRepo(repoUrl, name)
}

func GetExtensionRepo() ([]*ent.ExtensionRepo, error) {
	return jsExtension.LoadExtensionRepo()
}

// Download the extension by the given repository and package name
func DownloadExtension(repoUrl string, pkg string) error {
	return jsExtension.DownloadExtension(repoUrl, pkg)
}
