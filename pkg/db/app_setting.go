package db

import (
	"context"
	"log"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/ent/appsetting"
	"github.com/miru-project/miru-core/ext"
)

func GetAllRepositories() ([]*ent.ExtensionRepoSetting, error) {
	return ext.EntClient().ExtensionRepoSetting.Query().All(context.Background())
}
func SetDefaultRepository() error {
	return SetRepository("Official Miru Extension", "https://raw.githubusercontent.com/appdevelpo/repo/refs/heads/miru_alpha/index.json")
}

func SetRepository(name string, url string) error {
	return ext.EntClient().ExtensionRepoSetting.Create().SetName(name).
		SetLink(url).OnConflict().UpdateNewValues().
		Exec(context.Background())
}

func RemoveExtensionRepo(url string) error {
	_, e := ext.EntClient().ExtensionRepoSetting.Delete().
		Where().
		Exec(context.Background())
	return e
}

func GetAllAPPSettings() ([]*ent.AppSetting, error) {
	return ext.EntClient().AppSetting.Query().All(context.Background())
}

func GetAPPSetting(key string) (*ent.AppSetting, error) {

	setting, err := ext.EntClient().AppSetting.Query().Where(appsetting.Key(key)).Only(context.Background())
	if err != nil {
		return nil, err
	}

	return setting, nil

}
func SetAppSettings(settings *[]AppSettingJson) []error {

	err := make([]error, 0)

	for _, setting := range *settings {

		// Check if the key already exists
		existing, e := ext.EntClient().AppSetting.Query().Where(appsetting.Key(setting.Key)).Only(context.Background())
		if e != nil && !ent.IsNotFound(e) {
			err = append(err, e)
			continue
		}

		if existing != nil {

			// Update the existing record
			_, e = existing.Update().SetValue(setting.Value).Save(context.Background())
			if e != nil {
				err = append(err, e)
			}

		} else {

			// Create a new record
			_, e = ext.EntClient().AppSetting.Create().SetKey(setting.Key).SetValue(setting.Value).Save(context.Background())
			if e != nil {
				err = append(err, e)
			}
		}
	}

	return err
}

func Initialize() {

	// Init official Miru extension
	if _, e := ext.EntClient().ExtensionRepoSetting.Query().First(context.Background()); e != nil {
		log.Println("No extension repositories found, initializing...")

		e := SetDefaultRepository()
		if e != nil {
			log.Println("Failed to set default extension repository:", e)
		}
	}
}

type AppSettingJson struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
