package db

import (
	"context"
	"errors"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/ent/extensionsetting"
	"github.com/miru-project/miru-core/ext"
)

func SetSetting(pkg string, key string, value string) error {
	setting, e := GetSetting(pkg, key)
	if e != nil {
		return e
	}
	if setting != nil {
		client := ext.EntClient()
		ctx := context.Background()
		_, err := client.ExtensionSetting.Update().Where(extensionsetting.PackageEQ(pkg), extensionsetting.KeyEQ(key)).SetValue(value).Save(ctx)
		return err
	}
	return errors.New("setting keys not found")
}
func GetSetting(pkg string, key string) (*ent.ExtensionSetting, error) {
	client := ext.EntClient()
	ctx := context.Background()
	return client.ExtensionSetting.Query().Where(extensionsetting.PackageEQ(pkg), extensionsetting.KeyEQ(key)).First(ctx)
}

func RegisterSetting(setting map[string]any, pkg string) error {

	key := nilableObj[string](setting["key"])
	title := nilableObj[string](setting["title"])
	if pkg == "" || key == nil || title == nil {
		return errors.New("package name or key cannot be empty")
	}
	set, _ := GetSetting(pkg, *key)
	if set != nil {
		return nil
	}
	client := ext.EntClient()
	ctx := context.Background()

	_, err := client.ExtensionSetting.Create().
		SetPackage(pkg).
		SetTitle(*title).
		SetKey(*key).
		SetNillableDbType(nilableObj[extensionsetting.DbType](setting["type"])).
		SetNillableValue(nilableObj[string](setting["value"])).
		SetNillableDefaultValue(nilableObj[string](setting["defaultValue"])).
		SetNillableDescription(nilableObj[string](setting["description"])).
		SetNillableOptions(nilableObj[string](setting["options"])).
		Save(ctx)
	return err
}
func nilableObj[T any](obj any) *T {
	if obj == nil {
		return nil
	}
	if val, ok := obj.(T); ok {
		return &val
	}
	if valp, ok := obj.(*T); ok {
		return valp
	}
	return nil

}
