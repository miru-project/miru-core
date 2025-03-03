package handler

import "github.com/miru-project/miru-core/pkg/result"

func HelloMiru() (*result.Result, error) {

	return result.NewSuccessResult("Hello Miru!!"), nil
}
