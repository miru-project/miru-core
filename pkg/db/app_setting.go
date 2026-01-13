package db

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/ent/appsetting"
	"github.com/miru-project/miru-core/ext"
)

var appSettings AppSetting

type AppSetting struct {
	setting sync.Map
}

func (a *AppSetting) init() {
	settings, _ := ext.EntClient().AppSetting.Query().All(context.Background())
	for _, setting := range settings {
		a.setting.Store(setting.Key, setting.Value)
	}
}

// Load a setting from setting cache
func (a *AppSetting) load(key string) (string, error) {
	setting, ok := a.setting.Load(key)
	if !ok {
		return "", errors.New("setting not found")
	}
	return setting.(string), nil
}

// Load all settings from setting cache
func (a *AppSetting) loadAll() ([]*ent.AppSetting, error) {
	entries := make([]*ent.AppSetting, 0)
	a.setting.Range(func(key, value any) bool {
		entries = append(entries, &ent.AppSetting{
			Key:   key.(string),
			Value: value.(string),
		})
		return true
	})
	return entries, nil
}

func (a *AppSetting) save(key string, value string) error {
	a.setting.Store(key, value)
	return a.saveToDb(key, value)
}

func (a *AppSetting) saveToDb(key string, value string) error {
	// Check if the key already exists
	existing, e := ext.EntClient().AppSetting.Query().Where(appsetting.Key(key)).Only(context.Background())
	if e != nil && !ent.IsNotFound(e) {
		return e
	}

	if existing != nil {
		// Update the existing record
		if _, e = existing.Update().SetValue(value).Save(context.Background()); e != nil {
			return e
		}

	} else {
		// Create a new record
		if _, e = ext.EntClient().AppSetting.Create().SetKey(key).SetValue(value).Save(context.Background()); e != nil {
			return e
		}
	}
	return nil
}

func (a *AppSetting) saveAll(settings map[string]string) []error {
	err := make([]error, 0)
	for key, value := range settings {
		a.setting.Store(key, value)
		if e := a.saveToDb(key, value); e != nil {
			err = append(err, e)
		}
	}
	return err
}

func (a *AppSetting) Delete(key string) error {
	a.setting.Delete(key)
	return nil
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
	return appSettings.loadAll()
}

func GetAPPSetting(key string) (string, error) {
	return appSettings.load(key)
}

func SetAppSettings(settings map[string]string) []error {
	return appSettings.saveAll(settings)
}
func SetAppSetting(key string, value string) error {
	return appSettings.save(key, value)
}

func GetAllRepositories() ([]*ent.ExtensionRepoSetting, error) {
	return ext.EntClient().ExtensionRepoSetting.Query().All(context.Background())
}

func Initialize() {
	appSettings.init()

	// Init official Miru extension
	if _, e := ext.EntClient().ExtensionRepoSetting.Query().First(context.Background()); e != nil {
		log.Println("No extension repositories found, initializing...")

		e := SetDefaultRepository()
		if e != nil {
			log.Println("Failed to set default extension repository:", e)
		}
	}
}
