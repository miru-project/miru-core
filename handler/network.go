package handler

import (
	"net/http"

	"github.com/miru-project/miru-core/pkg/network"
	"github.com/miru-project/miru-core/pkg/result"
)

func GetCookies(url string) *result.Result[any] {
	cookie, e := network.GetCookies(url)
	if e != nil {
		return result.NewErrorResult(e.Error(), http.StatusInternalServerError, nil)
	}
	return result.NewSuccessResult(cookie)
}

func SetCookies(url string, cookies []string) *result.Result[any] {
	e := network.SetCookies(url, cookies)
	if e != nil {
		return result.NewErrorResult(e.Error(), http.StatusInternalServerError, nil)
	}
	return result.NewSuccessResult("Cookies set successfully")
}
