package handler

import (
	"encoding/json"

	"github.com/miru-project/miru-core/pkg/anilist"
	"github.com/miru-project/miru-core/pkg/result"
)

func GetAnilistUserData() (*result.Result[any], error) {

	userData, err := anilist.GetUserData()

	if err != nil {
		return result.NewErrorResultAny("Failed to get user data from anilist", 500), err
	}

	// Parse userData into a map
	var parsedData map[string]any
	if err := json.Unmarshal([]byte(userData), &parsedData); err != nil {
		return result.NewErrorResultAny("Failed to parse user data into JSON", 500), err
	}

	return result.NewSuccessResult[any](parsedData), nil
}

// retrieves the user's collection from Anilist.
// UserId can be access from /anilist/user.
// MediaType can be either "ANIME" or "MANGA".

func GetAnilistCollection(userId string, mediaType string) (*result.Result[any], error) {

	collection, err := anilist.GetCollection(userId, mediaType)

	if err != nil {
		return result.NewErrorResultAny("Failed to get collection from anilist", 500), err
	}

	// Parse collection into a map
	var parsedData map[string]any
	if err := json.Unmarshal([]byte(collection), &parsedData); err != nil {
		return result.NewErrorResultAny("Failed to parse collection into JSON", 500), err
	}

	return result.NewSuccessResult[any](parsedData), nil
}

func GetAnilistMediaQuery(page string, searchStr string, mediaType string) (*result.Result[any], error) {
	mediaQuery, err := anilist.MediaQuery(page, searchStr, mediaType)

	if err != nil {
		return result.NewErrorResultAny("Failed to get media query from anilist", 500), err
	}

	// Parse mediaQuery into a map
	var parsedData map[string]any
	if err := json.Unmarshal([]byte(mediaQuery), &parsedData); err != nil {
		return result.NewErrorResultAny("Failed to parse media query into JSON", 500), err
	}

	return result.NewSuccessResult[any](parsedData), nil
}

func EditAnilistList(status string, mediaId *string, id *string, progress *int, score *float64, startDate *anilist.AnilistDate, endDate *anilist.AnilistDate, isPrivate *bool) (*result.Result[any], error) {

	res, err := anilist.EditList(status, mediaId, id, progress, score, startDate, endDate, isPrivate)
	if err != nil {
		return result.NewErrorResultAny("Failed to edit list", 500), err
	}

	var parsedData map[string]any
	if err := json.Unmarshal([]byte(res), &parsedData); err != nil {
		return result.NewErrorResultAny("Failed to parse media query into JSON", 500), err
	}

	return result.NewSuccessResult[any](parsedData), nil
}
