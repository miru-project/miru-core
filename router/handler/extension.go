package handler

import (
	"encoding/json"
	"strconv"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/pkg/extension/endpoint"
	js "github.com/miru-project/miru-core/pkg/extension/js"
	"github.com/miru-project/miru-core/pkg/result"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

// handle Latest when receiving a request
func Latest(page string, pkg string) *result.Result[[]*proto.ExtensionListItem] {

	intPage, err := strconv.Atoi(page)
	if err != nil {
		return result.NewErrorResult[[]*proto.ExtensionListItem]("Invalid page number", 400, nil)
	}

	rt, e := endpoint.GetRuntime(pkg)
	if e != nil {
		return result.NewErrorResult[[]*proto.ExtensionListItem](e.Error(), 500, nil)
	}

	res, e := rt.Latest(pkg, intPage)
	return handleResult(res, e)
}

// handle Search when receiving a request
func Search(page string, pkg string, kw string, filter string) *result.Result[[]*proto.ExtensionListItem] {

	intPage, err := strconv.Atoi(page)
	if err != nil {
		return result.NewErrorResult[[]*proto.ExtensionListItem]("Invalid page number", 400, nil)
	}

	rt, e := endpoint.GetRuntime(pkg)
	if e != nil {
		return result.NewErrorResult[[]*proto.ExtensionListItem](e.Error(), 500, nil)
	}

	res, e := rt.Search(pkg, intPage, kw, filter)
	return handleResult(res, e)
}

// handle CreateFilter when receiving a request
func CreateFilter(pkg string, filter string) *result.Result[map[string]*proto.ExtensionFilter] {
	rt, e := endpoint.GetRuntime(pkg)
	if e != nil {
		return result.NewErrorResult[map[string]*proto.ExtensionFilter](e.Error(), 500, nil)
	}

	res, e := rt.CreateFilter(pkg, filter)
	return handleResult(res, e)
}

// handle Watch when receiving a request
func Watch(pkg string, url string) (*result.Result[any], *extension.Extension) {
	rt, e := endpoint.GetRuntime(pkg)
	if e != nil {
		return result.NewErrorResult[any](e.Error(), 500, nil), nil
	}

	res, meta, e := rt.Watch(pkg, url)
	return handleResult(res, e), meta
}

// handle Mirror when receiving a request
func Mirror(pkg string, url string) *result.Result[string] {
	rt, e := endpoint.GetRuntime(pkg)
	if e != nil {
		return result.NewErrorResult(e.Error(), 500, "")
	}

	res, e := rt.Mirror(pkg, url)
	if e != nil {
		return result.NewErrorResult(e.Error(), 500, "")
	}

	switch v := res.(type) {
	case string:
		return result.NewSuccessResult(v)
	case map[string]any:
		if link, ok := v["url"].(string); ok {
			return result.NewSuccessResult(link)
		}
	}
	b, _ := json.Marshal(res)
	return result.NewSuccessResult(string(b))
}

// handle Detail when receiving a request
func Detail(pkg string, url string) *result.Result[*proto.ExtensionDetail] {
	rt, e := endpoint.GetRuntime(pkg)
	if e != nil {
		return result.NewErrorResult[*proto.ExtensionDetail](e.Error(), 404, nil)
	}

	res, e := rt.Detail(pkg, url)
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
func FetchExtensionRepo() (map[string][]js.GithubExtension, map[string]error, error) {
	return js.FetchExtensionRepo()
}

func SetExtensionRepo(repoUrl string, name string) error {
	return js.SaveExtensionRepo(repoUrl, name)
}

func GetExtensionRepo() ([]*ent.ExtensionRepoSetting, error) {
	return js.LoadExtensionRepo()
}

// Download the extension by the given repository and package name
func DownloadExtension(repoUrl string, pkg string) *result.Result[string] {
	if repoUrl == "" || pkg == "" {
		return result.NewErrorResult("Repository URL and package name are required", 400, "")
	}

	if e := js.DownloadExtension(repoUrl, pkg); e != nil {
		return result.NewErrorResult(e.Error(), 500, "")
	}

	return result.NewSuccessResult("Extension download initiated successfully")
}

// Remove  the extension repository by the given url
func RemoveExtensionRepo(url string) (*result.Result[string], error) {
	if url == "" {
		return result.NewErrorResult("Repository URL is required", 400, ""), nil
	}
	if err := js.RemoveExtensionRepo(url); err != nil {
		return result.NewErrorResult(err.Error(), 500, ""), nil
	}
	return result.NewSuccessResult("Repository removed successfully"), nil
}

// Remove the extension by the given package name
func RemoveExtension(pkg string) (*result.Result[string], error) {
	if pkg == "" {
		return result.NewErrorResult("Package name is required", 400, ""), nil
	}
	if e := js.RemoveExtension(pkg); e != nil {
		return result.NewErrorResult(e.Error(), 500, ""), nil
	}
	return result.NewSuccessResult("Extension removal initiated successfully"), nil
}
