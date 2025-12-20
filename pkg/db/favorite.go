package db

import (
	"context"
	"time"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/ent/favorite"
	"github.com/miru-project/miru-core/ent/favoritegroup"
	"github.com/miru-project/miru-core/ext"
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

// PutFavoriteByIndex upserts many favorite groups (bulk). It uses OnConflict to update values.
func PutFavoriteByIndex(groups []*ent.FavoriteGroup) error {
	client := ext.EntClient()
	ctx := context.Background()
	// Build create builders
	builders := make([]*ent.FavoriteGroupCreate, 0, len(groups))
	for _, g := range groups {
		c := client.FavoriteGroup.Create().SetName(g.Name).SetDate(g.Date)
		builders = append(builders, c)
	}
	// CreateBulk(...).OnConflict().UpdateNewValues().Exec(ctx) returns error.
	err := client.FavoriteGroup.CreateBulk(builders...).OnConflict().UpdateNewValues().Exec(ctx)
	return err
}

// PutFavorite creates or upserts a favorite. Returns the created Favorite entity.
func PutFavorite(detailUrl string, cover *string, pkg string, t string) (*ent.Favorite, error) {
	client := ext.EntClient()
	ctx := context.Background()

	// Use OnConflict to update new values (upsert)
	create := client.Favorite.Create().SetURL(detailUrl).SetPackage(pkg).SetType(t).SetTitle("").SetDate(time.Now())
	if cover != nil {
		create = create.SetCover(*cover)
	}
	// UpdateNewValues will update mutable fields if conflict
	favID, err := create.OnConflict().UpdateNewValues().ID(ctx)
	if err != nil {
		return nil, err
	}
	return client.Favorite.Get(ctx, favID)
}

// DeleteFavorite deletes a favorite by its url and package.
func DeleteFavorite(detailUrl, pkg string) error {
	client := ext.EntClient()
	ctx := context.Background()
	_, err := client.Favorite.Delete().Where(favorite.URLEQ(detailUrl), favorite.PackageEQ(pkg)).Exec(ctx)
	return err
}

// GetFavoriteGroupsByFavorite returns favorite groups that contain a favorite with the provided package and url.
func GetFavoriteGroupsByFavorite(pkg, url string) ([]*ent.FavoriteGroup, error) {
	client := ext.EntClient()
	ctx := context.Background()
	return client.FavoriteGroup.Query().
		Where(favoritegroup.HasFavoritesWith(
			favorite.PackageEQ(pkg),
			favorite.URLEQ(url),
		)).
		All(ctx)
}
