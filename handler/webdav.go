package handler

import (
	"github.com/miru-project/miru-core/pkg/result"
	webdav "github.com/miru-project/miru-core/pkg/webDav"
)

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
