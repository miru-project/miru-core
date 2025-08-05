package handler

import (
	"strconv"

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

func FetchExtensionRepo() (map[string][]jsExtension.GithubExtension, error, map[string]error) {
	repo, e := jsExtension.LoadExtensionRepo()
	if e != nil {
		return nil, e, nil
	}
	r, err := jsExtension.FetchExtensionRepo(repo)
	return r, nil, err
}
