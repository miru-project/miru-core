package handler

import (
	"strconv"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/pkg/jsExtension"
	"github.com/miru-project/miru-core/pkg/result"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

// handle Latest when receiving a request
func Latest(page string, pkg string) *result.Result[[]*proto.ExtensionListItem] {

	intPage, err := strconv.Atoi(page)
	if err != nil {
		return result.NewErrorResult[[]*proto.ExtensionListItem]("Invalid page number", 400, nil)
	}
	res, e := jsExtension.Latest[proto.ExtensionListItem](pkg, intPage)
	return handleResult(res, e)
}

// handle Search when receiving a request
func Search(page string, pkg string, kw string, filter string) *result.Result[[]*proto.ExtensionListItem] {

	intPage, err := strconv.Atoi(page)
	if err != nil {
		return result.NewErrorResult[[]*proto.ExtensionListItem]("Invalid page number", 400, nil)
	}

	res, e := jsExtension.Search[proto.ExtensionListItem](pkg, intPage, kw, filter)
	return handleResult(res, e)
}

// handle CreateFilter when receiving a request
func CreateFilter(pkg string, filter string) *result.Result[map[string]*proto.ExtensionFilter] {
	res, e := jsExtension.CreateFilter(pkg, filter)
	return handleResult(res, e)
}

// handle Watch when receiving a request
func Watch(pkg string, url string) (*result.Result[any], *jsExtension.ExtApi) {

	res, e, api := jsExtension.Watch(pkg, url)
	return handleResult(res, e), api
}

// handle Detail when receiving a request
func Detail(pkg string, url string) *result.Result[*proto.ExtensionDetail] {

	res, e := jsExtension.Detail[proto.ExtensionDetail](pkg, url)
	return handleResult(res, e)
}

func handleResult[T any](res T, e error) *result.Result[T] {
	if e != nil {
		var zero T
		return result.NewErrorResult(e.Error(), 404, zero)
	}

	var zero T
	if any(res) == nil {
		return result.NewErrorResult("No results found", 404, zero)
	}

	return result.NewSuccessResult(res)
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
func DownloadExtension(repoUrl string, pkg string) *result.Result[string] {
	if repoUrl == "" || pkg == "" {
		return result.NewErrorResult("Repository URL and package name are required", 400, "")
	}

	if e := jsExtension.DownloadExtension(repoUrl, pkg); e != nil {
		return result.NewErrorResult(e.Error(), 500, "")
	}

	return result.NewSuccessResult("Extension download initiated successfully")
}

// Remove  the extension repository by the given url
func RemoveExtensionRepo(url string) (*result.Result[string], error) {
	if url == "" {
		return result.NewErrorResult("Repository URL is required", 400, ""), nil
	}
	if err := jsExtension.RemoveExtensionRepo(url); err != nil {
		return result.NewErrorResult(err.Error(), 500, ""), nil
	}
	return result.NewSuccessResult("Repository removed successfully"), nil
}

// Remove the extension by the given package name
func RemoveExtension(pkg string) (*result.Result[string], error) {
	if pkg == "" {
		return result.NewErrorResult("Package name is required", 400, ""), nil
	}
	if e := jsExtension.RemoveExtension(pkg); e != nil {
		return result.NewErrorResult(e.Error(), 500, ""), nil
	}
	return result.NewSuccessResult("Extension removal initiated successfully"), nil
}
