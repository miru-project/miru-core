package handler

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/result"
)

func GetFavorite(function string, c *fiber.Ctx) (any, error) {
	var e error
	var o any

	switch function {
	case "GetAllFavorite":
		o, e = db.GetAllFavorite()
		if e != nil {
			break
		}
	case "GetFavoriteByPackageAndUrl":
		pkg := c.FormValue("package")
		url := c.FormValue("url")
		o, e = db.GetFavoriteByPackageAndUrl(pkg, url)
		if e != nil {
			break
		}

	default:
		e = errors.New("unknown function")
	}

	if e != nil {
		return result.NewErrorResult(e.Error(), fiber.StatusInternalServerError, nil), e
	}
	return result.NewSuccessResult(o), nil
}

func GetFavoriteGroup(function string, c *fiber.Ctx) (any, error) {
	var e error
	var o any

	switch function {
	case "GetFavoriteGroupsById":
		id, e := strconv.Atoi(c.FormValue("id"))
		if e != nil {
			break
		}
		o, e = db.GetFavoriteGroupsById(id)
		if e != nil {
			break
		}
	case "GetAllFavoriteGroup":
		o, e = db.GetAllFavoriteGroup()
		if e != nil {
			break
		}
	default:
		e = errors.New("unknown function")
	}

	if e != nil {
		return result.NewErrorResult(e.Error(), fiber.StatusInternalServerError, nil), e
	}
	return result.NewSuccessResult(o), nil
}

func ModFavorite(function string, c *fiber.Ctx) (*result.Result[any], error) {
	var e error
	switch function {
	case "PutFavoriteByIndex":
		var groups []*ent.FavoriteGroup
		e = c.BodyParser(&groups)
		if e != nil {
			break
		}
		e = db.PutFavoriteByIndex(groups)
		if e != nil {
			break
		}

	default:
		e = errors.New("unknown function")
	}
	var res *result.Result[any]
	if e != nil {
		res = result.NewErrorResult(e.Error(), fiber.StatusInternalServerError, nil)
		return res, e
	}
	res = result.NewSuccessResult(nil)
	return res, e
}

func ModFavoriteGroup(function string, c *fiber.Ctx) (any, error) {
	var e error
	var o any
	switch function {
	case "PutFavoriteGroup":
		var grp struct {
			Name  string `json:"name"`
			Items []int  `json:"items"`
		}
		e = c.BodyParser(&grp)
		if e != nil {
			break
		}
		o, e = db.PutFavoriteGroup(grp.Name, grp.Items)
		if e != nil {
			break
		}

	case "RenameFavoriteGroup":
		oldName := c.FormValue("oldName")
		newName := c.FormValue("newName")
		e = db.RenameFavoriteGroup(oldName, newName)
		if e != nil {
			break
		}

	case "DeleteFavoriteGroup":
		var names struct {
			Names []string `json:"names"`
		}
		e = c.BodyParser(&names)
		if e != nil {
			break
		}
		e = db.DeleteFavoriteGroup(names.Names)
		if e != nil {
			break
		}

	default:
		e = errors.New("unknown function")
	}

	if e != nil {
		return result.NewErrorResult(e.Error(), fiber.StatusInternalServerError, nil), e
	}
	return result.NewSuccessResult(o), nil
}

func GetHistory(function string, c *fiber.Ctx) (any, error) {
	var e error
	var o any
	switch function {
	case "GetHistoryByPackageAndUrl":
		pkg := c.FormValue("package")
		url := c.FormValue("url")
		o, e = db.GetHistoryByPackageAndUrl(pkg, url)
		if e != nil {
			break
		}

	case "GetHistorysByType":
		t := c.FormValue("type")
		o, e = db.GetHistorysByType(&t)
		if e != nil {
			break
		}

	default:
		e = errors.New("unknown function")
	}

	if e != nil {
		return result.NewErrorResult(e.Error(), fiber.StatusInternalServerError, nil), e
	}
	return result.NewSuccessResult(o), nil
}

func ModHistory(function string, c *fiber.Ctx) (any, error) {
	var e error
	var o any
	switch function {
	case "PutHistory":
		var history *ent.History
		e = c.BodyParser(&history)
		if e != nil {
			break
		}
		o, e = db.PutHistory(history)
		if e != nil {
			break
		}

	case "DeleteHistoryByPackageAndUrl":
		pkg := c.FormValue("package")
		url := c.FormValue("url")
		e = db.DeleteHistoryByPackageAndUrl(pkg, url)
		if e != nil {
			break
		}

	case "DeleteAllHistory":
		o, e = db.DeleteAllHistory()
		if e != nil {
			break
		}

	default:
		e = errors.New("unknown function")
	}

	if e != nil {
		return result.NewErrorResult(e.Error(), fiber.StatusInternalServerError, nil), e
	}
	return result.NewSuccessResult(o), nil
}
