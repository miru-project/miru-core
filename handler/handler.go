package handler

import (
	"strconv"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/pkg/result"
)

func HelloMiru() (*result.Result, error) {

	return result.NewSuccessResult("Hello Miru!!"), nil
}
func Latest(page string, pkg string) (*result.Result, error) {

	intPage, err := strconv.Atoi(page)
	if err != nil {
		return result.NewErrorResult("Invalid page number", 400), err
	}

	res, e := extension.Latest(pkg, intPage)
	return result.NewSuccessResult(res), e

}
func Search(page string, pkg string, kw string, body string) (*result.Result, error) {

	intPage, err := strconv.Atoi(page)
	if err != nil {
		return result.NewErrorResult("Invalid page number", 400), err
	}

	res, e := extension.Search(pkg, intPage, kw, body)
	return result.NewSuccessResult(res), e

}

func Watch(pkg string, url string) (*result.Result, error) {

	res, e := extension.Watch(pkg, url)

	return result.NewSuccessResult(res), e
}

func Detail(pkg string, url string) (*result.Result, error) {

	res, e := extension.Detail(pkg, url)

	return result.NewSuccessResult(res), e
}
