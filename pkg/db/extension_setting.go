package db

import (
	"context"
	"encoding/json"
	"errors"
	"log"

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

func RemoveSetting(pkg string, key string) error {
	client := ext.EntClient()
	ctx := context.Background()
	_, err := client.ExtensionSetting.Delete().Where(extensionsetting.PackageEQ(pkg), extensionsetting.KeyEQ(key)).Exec(ctx)
	return err
}

func GetSetting(pkg string, key string) (*ent.ExtensionSetting, error) {
	client := ext.EntClient()
	ctx := context.Background()
	return client.ExtensionSetting.Query().Where(extensionsetting.PackageEQ(pkg), extensionsetting.KeyEQ(key)).First(ctx)
}

func GetSettingsByPackage(pkg string) ([]*ent.ExtensionSetting, error) {
	client := ext.EntClient()
	ctx := context.Background()
	return client.ExtensionSetting.Query().Where(extensionsetting.PackageEQ(pkg)).All(ctx)
}

func RegisterSetting(setting map[string]any, pkg string) error {
	log.Printf("[RegisterSetting] pkg: %s, setting: %+v", pkg, setting)

	key := nilableObj[string](setting["key"])
	title := nilableObj[string](setting["title"])
	if pkg == "" || key == nil || title == nil {
		return errors.New("package name or key cannot be empty")
	}
	set, _ := GetSetting(pkg, *key)
	dbType := nilableObj[extensionsetting.DbType](setting["type"])
	if set != nil && extensionsetting.DbType(set.DbType.String()) != setting["type"] {
		return nil
	}
	client := ext.EntClient()
	ctx := context.Background()

	if dbType == nil {
		if t := nilableObj[string](setting["type"]); t != nil {
			val := extensionsetting.DbType(*t)
			dbType = &val
		}
	}

	_, err := client.ExtensionSetting.Create().
		SetPackage(pkg).
		SetTitle(*title).
		SetKey(*key).
		SetNillableDbType(dbType).
		SetNillableValue(nilableObj[string](setting["value"])).
		SetNillableDefaultValue(nilableObj[string](setting["defaultValue"])).
		SetNillableDescription(nilableObj[string](setting["description"])).
		SetNillableOptions(jsonString(setting["options"])).
		Save(ctx)
	if err != nil {
		log.Printf("[RegisterSetting] Error saving setting: %v", err)
	}
	return err
}
func jsonString(obj any) *string {
	if obj == nil {
		return nil
	}
	if s, ok := obj.(string); ok {
		return &s
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return nil
	}
	res := string(b)
	return &res
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
