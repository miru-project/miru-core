package handler

import (
	"strconv"
	"strings"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/pkg/result"
)

func HelloMiru() (*result.Result, error) {

	return result.NewSuccessResult("Hello Miru!!"), nil
}
func Latest(page string, rawPkg string) (*result.Result, error) {
	intPage, err := strconv.Atoi(page)
	if err != nil {
		return result.NewErrorResult("Invalid page number", 400), err
	}
	pkg := strings.TrimSuffix(rawPkg, ".js")
	if err != nil {
		return result.NewErrorResult("Invalid package name", 400), err
	}
	res, ok := extension.Latest(pkg, intPage)
	if !ok {
		return result.NewErrorResult(res, 500), nil
	}
	return result.NewSuccessResult(res), nil

}
