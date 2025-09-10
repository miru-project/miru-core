package handler

import (
	"net/http"

	"github.com/miru-project/miru-core/pkg/network"
)

func GetCookies(url string) ([]*http.Cookie, error) {
	// network.ReadAll()
	return network.GetCookies(url)
}
