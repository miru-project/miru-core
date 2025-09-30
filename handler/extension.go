package handler

import (
	"strconv"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/pkg/jsExtension"
	"github.com/miru-project/miru-core/pkg/result"
)

// handle Latest when receiving a request
func Latest(page string, pkg string) *result.Result[any] {

	intPage, err := strconv.Atoi(page)
	if err != nil {
		return result.NewErrorResult("Invalid page number", 400, nil)
	}

	res, e := jsExtension.Latest(pkg, intPage)

	if res == nil && e != nil {
		return result.NewErrorResult(e.Error(), 404, nil)
	}
	return result.NewSuccessResult(res)

}

// handle Search when receiving a request
func Search(page string, pkg string, kw string, filter string) *result.Result[any] {

	intPage, err := strconv.Atoi(page)
	if err != nil {
		return result.NewErrorResult("Invalid page number", 400, nil)
	}

	res, e := jsExtension.Search(pkg, intPage, kw, filter)

	if res == nil {
		return result.NewErrorResult(e.Error(), 404, nil)
	}
	return result.NewSuccessResult(res)

}

// handle Watch when receiving a request
func Watch(pkg string, url string) *result.Result[any] {

	res, e := jsExtension.Watch(pkg, url)

	if res == nil {
		return result.NewErrorResult(e.Error(), 404, nil)
	}
	return result.NewSuccessResult(res)
}

// handle Detail when receiving a request
func Detail(pkg string, url string) (*result.Result[any], error) {

	res, e := jsExtension.Detail(pkg, url)

	if res == nil {
		return result.NewErrorResult(e.Error(), 404, nil), nil
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

func GetExtensionRepo() ([]*ent.ExtensionRepoSetting, error) {
	return jsExtension.LoadExtensionRepo()
}

// Download the extension by the given repository and package name
func DownloadExtension(repoUrl string, pkg string) *result.Result[any] {
	if repoUrl == "" || pkg == "" {
		return result.NewErrorResult("Repository URL and package name are required", 400, nil)
	}

	if e := jsExtension.DownloadExtension(repoUrl, pkg); e != nil {
		return result.NewErrorResult(e.Error(), 500, nil)
	}

	return result.NewSuccessResult("Extension download initiated successfully")
}

// Remove  the extension repository by the given url
func RemoveExtensionRepo(url string) (*result.Result[any], error) {
	if url == "" {
		return result.NewErrorResult("Repository URL is required", 400, nil), nil
	}
	if err := jsExtension.RemoveExtensionRepo(url); err != nil {
		return result.NewErrorResult(err.Error(), 500, nil), nil
	}
	return result.NewSuccessResult("Repository removed successfully"), nil
}

// Remove the extension by the given package name
func RemoveExtension(pkg string) (*result.Result[any], error) {
	if pkg == "" {
		return result.NewErrorResult("Package name is required", 400, nil), nil
	}
	if e := jsExtension.RemoveExtension(pkg); e != nil {
		return result.NewErrorResult(e.Error(), 500, nil), nil
	}
	return result.NewSuccessResult("Extension removal initiated successfully"), nil
}
