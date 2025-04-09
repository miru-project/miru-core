package handler

import (
	"strconv"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/pkg/result"
	webdav "github.com/miru-project/miru-core/pkg/webDav"
)

func HelloMiru() (*result.Result, error) {

	return result.NewSuccessResult("Hello Miru!!"), nil
}

// handle Latest when receiving a request
func Latest(page string, pkg string) (*result.Result, error) {

	intPage, err := strconv.Atoi(page)
	if err != nil {
		return result.NewErrorResult("Invalid page number", 400), err
	}

	res, e := extension.Latest(pkg, intPage)
	return result.NewSuccessResult(res), e

}

// handle Search when receiving a request
func Search(page string, pkg string, kw string, body string) (*result.Result, error) {

	intPage, err := strconv.Atoi(page)
	if err != nil {
		return result.NewErrorResult("Invalid page number", 400), err
	}

	res, e := extension.Search(pkg, intPage, kw, body)
	return result.NewSuccessResult(res), e

}

// handle Watch when receiving a request
func Watch(pkg string, url string) (*result.Result, error) {

	res, e := extension.Watch(pkg, url)

	return result.NewSuccessResult(res), e
}

// handle Detail when receiving a request
func Detail(pkg string, url string) (*result.Result, error) {

	res, e := extension.Detail(pkg, url)

	return result.NewSuccessResult(res), e
}

// handle WebDav login
func Login(host string, user string, passwd string) (*result.Result, error) {

	err := webdav.Authenticate(host, user, passwd)
	if err != nil {
		return result.NewErrorResult("Failed to login WebDav server", 500), err
	}

	return result.NewSuccessResult("ok"), err
}

// handle WebDav backup
func Backup() (*result.Result, error) {

	err := webdav.Backup()
	if err != nil {
		return result.NewErrorResult("Failed to backup WebDav server", 500), err
	}

	return result.NewSuccessResult("ok"), err
}

func Restore() (*result.Result, error) {
	err := webdav.Restore()
	if err != nil {
		return result.NewErrorResult("Failed to restore WebDav server", 500), err
	}

	return result.NewSuccessResult("ok"), err
}
