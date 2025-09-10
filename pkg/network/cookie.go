package network

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"go.nhat.io/cookiejar"
	"golang.org/x/net/publicsuffix"
)

var jar *cookiejar.PersistentJar

func SetCookies(u string, cookies []string) error {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return err
	}
	parsedCookies := []*http.Cookie{}
	for _, c := range cookies {
		parts := strings.SplitN(c, "=", 2)
		if len(parts) != 2 {
			return errors.New("invalid cookie format")
		}
		parsedCookies = append(parsedCookies, &http.Cookie{
			Name:  strings.TrimSpace(parts[0]),
			Value: strings.TrimSpace(parts[1]),
		})
	}
	jar.SetCookies(parsedURL, parsedCookies)
	return nil
}

func GetCookies(u string) ([]*http.Cookie, error) {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	return jar.Cookies(parsedURL), nil
}

func InitCookieJar() {
	tempDir, e := os.MkdirTemp(os.TempDir(), "miru")
	if e != nil {
		panic(e)
	}
	cookiesFile := filepath.Join(tempDir, "cookies")
	jar = cookiejar.NewPersistentJar(
		cookiejar.WithFilePath(cookiesFile),
		cookiejar.WithAutoSync(true),
		cookiejar.WithPublicSuffixList(publicsuffix.List),
	)
}

func getHeadersFromJar(url *url.URL) string {
	cookies := jar.Cookies(url)
	var cookieStrs []string
	for _, cookie := range cookies {
		cookieStrs = append(cookieStrs, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(cookieStrs, "; ")
}
