package ext

import (
	"context"
	"fmt"

	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
	log "github.com/miru-project/miru-core/pkg/logger"

	"entgo.io/ent/dialect"
	"github.com/miru-project/miru-core/config"
	"github.com/miru-project/miru-core/ent"
	"github.com/miru-project/miru-core/ent/appsetting"
	_ "github.com/miru-project/miru-core/ent/runtime"

	_ "github.com/sqlite3ent/sqlite3"
)

var (
	entClient *ent.Client
)

func EntClient() *ent.Client {

	if entClient != nil {
		return entClient
	}

	var err error
	var client *ent.Client
	dbCfg := config.Global.Database

	var dsn string
	switch dbCfg.Driver {

	case "sqlite3":
		dsn = dbCfg.DBName
		log.Println("Using SQLite3 database at:", dsn)
		client, err = ent.Open(dialect.SQLite, fmt.Sprintf("file:%s?cache=shared&_fk=1", dsn))

	case "postgres":
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			dbCfg.Host, dbCfg.Port, dbCfg.User, dbCfg.Password, dbCfg.DBName, dbCfg.SSLMode)
		client, err = ent.Open(dialect.Postgres, dsn)

	case "mysql":
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=True",
			dbCfg.User, dbCfg.Password, dbCfg.Host, dbCfg.Port, dbCfg.DBName)
		client, err = ent.Open(dialect.MySQL, dsn)

	default:
		errorhandle.PanicF("unsupported database driver: %s", dbCfg.Driver)
		return nil
	}

	if err != nil {
		errorhandle.PanicF("failed opening connection to database: %s", err)
		return nil
	}

	if err := client.Schema.Create(context.Background()); err != nil {
		errorhandle.PanicF("failed creating schema resources: %s", err)
		return nil
	}

	entClient = client
	Initialize()
	return client
}

func Initialize() {

	// Init official Miru extension
	if _, e := entClient.ExtensionRepoSetting.Query().First(context.Background()); e != nil {
		log.Println("No extension repositories found, initializing...")

		e := SetDefaultRepository()
		if e != nil {
			log.Println("Failed to set default extension repository:", e)
		}
	}
}
func GetAllRepositories() ([]*ent.ExtensionRepoSetting, error) {
	return entClient.ExtensionRepoSetting.Query().All(context.Background())
}
func SetDefaultRepository() error {
	return SetRepository("Official Miru Extension", "https://raw.githubusercontent.com/appdevelpo/repo/refs/heads/miru_alpha/index.json")
}

func SetRepository(name string, url string) error {
	return entClient.ExtensionRepoSetting.Create().SetName(name).
		SetLink(url).OnConflict().UpdateNewValues().
		Exec(context.Background())
}

func RemoveExtensionRepo(url string) error {
	_, e := entClient.ExtensionRepoSetting.Delete().
		Where().
		Exec(context.Background())
	return e
}

func GetAllSettings() ([]*ent.AppSetting, error) {
	return entClient.AppSetting.Query().All(context.Background())
}

func GetSetting(key string) (*ent.AppSetting, error) {

	setting, err := entClient.AppSetting.Query().Where(appsetting.Key(key)).Only(context.Background())
	if err != nil {
		return nil, err
	}

	return setting, nil

}
func SetAppSettings(settings *[]AppSettingJson) []error {

	err := make([]error, 0)

	for _, setting := range *settings {

		// Check if the key already exists
		existing, e := entClient.AppSetting.Query().Where(appsetting.Key(setting.Key)).Only(context.Background())
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
			_, e = entClient.AppSetting.Create().SetKey(setting.Key).SetValue(setting.Value).Save(context.Background())
			if e != nil {
				err = append(err, e)
			}
		}
	}

	return err
}

type AppSettingJson struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
