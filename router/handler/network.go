package handler

import (
	"net/http"
	"strings"

	"github.com/miru-project/miru-core/pkg/network"
	"github.com/miru-project/miru-core/pkg/result"
)

func GetCookies(url string) *result.Result[any] {
	cookie, e := network.GetCookies(url)
	if e != nil {
		return result.NewErrorResultAny(e.Error(), http.StatusInternalServerError)
	}
	return result.NewSuccessResult[any](cookie)
}

func SetCookies(url string, cookies string) *result.Result[any] {
	cookieList := strings.Split(cookies, ";")
	e := network.SetCookies(url, cookieList)
	if e != nil {
		return result.NewErrorResultAny(e.Error(), http.StatusInternalServerError)
	}
	return result.NewSuccessResult[any]("Cookies set successfully")
}
