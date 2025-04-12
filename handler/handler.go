package handler

import (
	"encoding/json"
	"strconv"

	"github.com/miru-project/miru-core/ext"
	"github.com/miru-project/miru-core/pkg/anilist"
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
func Search(page string, pkg string, kw string, filter string) (*result.Result, error) {

	intPage, err := strconv.Atoi(page)
	if err != nil {
		return result.NewErrorResult("Invalid page number", 400), err
	}

	res, e := extension.Search(pkg, intPage, kw, filter)
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
func GetAppSetting() (*result.Result, error) {
	// Get all settings
	settings, err := ext.GetAllSettings()
	if err != nil {
		return result.NewErrorResult("Failed to get settings", 500), err
	}

	return result.NewSuccessResult(settings), nil
}

func SetAppSetting(settings *[]ext.AppSettingJson) []error {

	if e := ext.SetAppSettings(settings); len(e) != 0 {
		return e
	}
	return nil
}

func GetAnilistUserData() (*result.Result, error) {

	userData, err := anilist.GetUserData()

	if err != nil {
		return result.NewErrorResult("Failed to get user data from anilist", 500), err
	}

	// Parse userData into a map
	var parsedData map[string]any
	if err := json.Unmarshal([]byte(userData), &parsedData); err != nil {
		return result.NewErrorResult("Failed to parse user data into JSON", 500), err
	}

	return result.NewSuccessResult(parsedData), nil
}

// retrieves the user's collection from Anilist.
// UserId can be access from /anilist/user.
// MediaType can be either "ANIME" or "MANGA".

func GetAnilistCollection(userId string, mediaType string) (*result.Result, error) {

	collection, err := anilist.GetCollection(userId, mediaType)

	if err != nil {
		return result.NewErrorResult("Failed to get collection from anilist", 500), err
	}

	// Parse collection into a map
	var parsedData map[string]any
	if err := json.Unmarshal([]byte(collection), &parsedData); err != nil {
		return result.NewErrorResult("Failed to parse collection into JSON", 500), err
	}

	return result.NewSuccessResult(parsedData), nil
}

func GetAnilistMediaQuery(page string, searchStr string, mediaType string) (*result.Result, error) {
	mediaQuery, err := anilist.MediaQuery(page, searchStr, mediaType)

	if err != nil {
		return result.NewErrorResult("Failed to get media query from anilist", 500), err
	}

	// Parse mediaQuery into a map
	var parsedData map[string]any
	if err := json.Unmarshal([]byte(mediaQuery), &parsedData); err != nil {
		return result.NewErrorResult("Failed to parse media query into JSON", 500), err
	}

	return result.NewSuccessResult(parsedData), nil
}

func EditAnilistList(status string, mediaId *string, id *string, progress *int, score *float64, startDate *anilist.AnilistDate, endDate *anilist.AnilistDate, isPrivate *bool) (*result.Result, error) {

	res, err := anilist.EditList(status, mediaId, id, progress, score, startDate, endDate, isPrivate)
	if err != nil {
		return result.NewErrorResult("Failed to edit list", 500), err
	}

	var parsedData map[string]any
	if err := json.Unmarshal([]byte(res), &parsedData); err != nil {
		return result.NewErrorResult("Failed to parse media query into JSON", 500), err
	}

	return result.NewSuccessResult(parsedData), nil
}
