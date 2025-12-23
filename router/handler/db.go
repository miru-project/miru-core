package handler

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/ent/favorite"
	"github.com/miru-project/miru-core/ent/favoritegroup"
	"github.com/miru-project/miru-core/ext"
	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/result"
)

// GetFavoriteGroupsById returns favorite groups that link to a favorite with the provided id.
func GetFavoriteGroupsById(id int) ([]*ent.FavoriteGroup, error) {
	client := ext.EntClient()
	ctx := context.Background()

	groups, err := client.FavoriteGroup.Query().Where(favoritegroup.HasFavoritesWith(favorite.IDEQ(id))).All(ctx)
	return groups, err
}

// GetAllFavoriteGroup returns all favorite groups.
func GetAllFavoriteGroup() ([]*ent.FavoriteGroup, error) {
	client := ext.EntClient()
	ctx := context.Background()
	return client.FavoriteGroup.Query().All(ctx)
}

// DeleteFavoriteGroup deletes favorite groups with any of the provided names.
func DeleteFavoriteGroup(names []string) error {
	client := ext.EntClient()
	ctx := context.Background()
	_, err := client.FavoriteGroup.Delete().Where(favoritegroup.NameIn(names...)).Exec(ctx)
	return err
}

// RenameFavoriteGroup renames a favorite group from oldName to newName.
// Returns an error if the group is not found.
func RenameFavoriteGroup(oldName, newName string) error {
	client := ext.EntClient()
	ctx := context.Background()

	grp, err := client.FavoriteGroup.Query().Where(favoritegroup.NameEQ(oldName)).First(ctx)
	if err != nil {
		return err
	}
	_, err = grp.Update().SetName(newName).Save(ctx)
	return err
}

// PutFavoriteGroup creates a FavoriteGroup with the given name and attaches favorites by IDs.
func PutFavoriteGroup(name string, items []int) (*ent.FavoriteGroup, error) {
	client := ext.EntClient()
	ctx := context.Background()

	create := client.FavoriteGroup.Create().SetName(name).SetDate(time.Now())
	if len(items) > 0 {
		create = create.AddFavoriteIDs(items...)
	}
	return create.Save(ctx)
}

// GetFavoriteByPackageAndUrl returns the favorite matching package and url, or (nil, nil) if not found.
func GetFavoriteByPackageAndUrl(pkg, url string) (*ent.Favorite, error) {
	client := ext.EntClient()
	ctx := context.Background()
	f, err := client.Favorite.Query().Where(favorite.PackageEQ(pkg), favorite.URLEQ(url)).First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return f, nil
}

// GetAllFavorite returns all favorites.
func GetAllFavorite() ([]*ent.Favorite, error) {
	client := ext.EntClient()
	ctx := context.Background()
	return client.Favorite.Query().All(ctx)
}

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

	case "GetHistoriesByType":
		t := c.FormValue("type")
		o, e = db.GetHistoriesByType(&t)
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
