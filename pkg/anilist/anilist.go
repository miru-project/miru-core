package anilist

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/network"
)

var token string

const anilistAPI = "https://graphql.anilist.co"

func GetUserData() (string, error) {

	query := `
	{
		Viewer {
			name
			id
			avatar { medium }
			statistics {
				anime { episodesWatched }
				manga { chaptersRead }
			}
		}
	}
	`
	res, err := request(query)

	if err != nil {
		return "", err
	}

	return res, nil

}

func GetCollection(userId string, mediaType string) (string, error) {

	query := `
	{
        MediaListCollection(userId: ` + userId + `, type : ` + mediaType + `) {
          lists {
            status
            entries {
              status
              progress
              score
              media {
                id
                status
                chapters
                episodes
                meanScore
                isFavourite
                nextAiringEpisode {
                  episode
                }
                coverImage {
                  large
                }
                title {
                  userPreferred
                }
              }
            }
          }
        }
      }
	`
	res, err := request(query)

	if err != nil {
		return "", err
	}

	return res, nil
}

func MediaQuery(page string, searchStr string, mediaType string) (string, error) {

	query := `
		{Page(page:` + page + `){
   		 media(search:"` + searchStr + `",type: ` + mediaType + `){
        id
        type
        seasonYear
        isAdult
        description
        status
        season
        startDate{
            year
            month
            day
        }
        endDate{
            year
            month
            day
        }
        coverImage{
            large
        }
        title{
            romaji
            english
            native
            userPreferred 
        }
    }
  }}
	`

	res, err := request(query)

	if err != nil {
		return "", err
	}

	return res, nil
}

func EditList(
	status string,
	mediaId *string,
	id *string,
	progress *int,
	score *float64,
	startDate *AnilistDate,
	endDate *AnilistDate,
	isPrivate *bool,
) (string, error) {

	// Build the query list dynamically
	queryList := []string{}
	if id == nil {
		queryList = append(queryList, "mediaId:"+*mediaId)
	} else {
		queryList = append(queryList, "id:"+*id)
	}

	if score != nil {
		queryList = append(queryList, fmt.Sprintf("score:%f", *score))
	}

	if progress != nil {
		queryList = append(queryList, fmt.Sprintf("progress:%d", *progress))
	}

	if startDate != nil {
		queryList = append(queryList, fmt.Sprintf(
			"startedAt:{year:%d,month:%d,day:%d}",
			startDate.Year, startDate.Month, startDate.Day,
		))
	}

	if endDate != nil {
		queryList = append(queryList, fmt.Sprintf(
			"completedAt:{year:%d,month:%d,day:%d}",
			endDate.Year, endDate.Month, endDate.Day,
		))
	}

	// Join the query list into a single string
	queryStr := strings.Join(queryList, ",")

	// Construct the GraphQL mutation query
	query := fmt.Sprintf(`mutation{
        SaveMediaListEntry(status:%s,private:%t,%s){
            id
        }
    }`, status, isPrivate != nil && *isPrivate, queryStr)

	// Debug print the query string for troubleshooting
	fmt.Println("Generated Query:", query)

	// Send the GraphQL request
	res, err := request(query)

	if err != nil {
		return "", err
	}

	return res, nil
}

func request(query string) (string, error) {

	q, e := json.Marshal(&AnilistQuery{Query: query})

	if e != nil {
		return "", e
	}

	option := &network.RequestOptions{
		Method: "POST",
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Accept":        "application/json",
			"Authorization": "Bearer " + token,
		}, RequestBodyRaw: q}

	return network.Request[string](anilistAPI, option, network.ReadAll)
}

func InitToken() error {

	tok, e := db.GetAPPSetting("anilist_token")

	if e != nil {
		return e
	}

	token = tok

	return nil
}

type AnilistQuery struct {
	Query string `json:"query"`
}

type AnilistDate struct{ Year, Month, Day int }

type AnilistEditListJson struct {
	Status    string       `json:"status"`
	MediaId   *string      `json:"mediaId"`
	Id        *string      `json:"id"`
	Progress  *int         `json:"progress"`
	Score     *float64     `json:"score"`
	StartDate *AnilistDate `json:"startDate"`
	EndDate   *AnilistDate `json:"endDate"`
	IsPrivate *bool        `json:"isPrivate"`
}
