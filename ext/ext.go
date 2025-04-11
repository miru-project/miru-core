package ext

import (
	"context"
	"fmt"
	"log"

	"entgo.io/ent/dialect"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/miru-project/miru-core/config"
	"github.com/miru-project/miru-core/ent"
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
		log.Printf("unsupported database driver: %s", dbCfg.Driver)
		return nil
	}

	if err != nil {
		log.Printf("failed opening connection to database: %s", err)
		return nil
	}

	if err := client.Schema.Create(context.Background()); err != nil {
		log.Printf("failed creating schema resources: %s", err)
		return nil
	}

	entClient = client

	return client
}

func GetAllSettings() (*ent.AppSetting, error) {
	return entClient.AppSetting.Query().First(context.Background())
}

func SetAppSettings(settings *[]AppSettingJson) []error {

	err := make([]error, 0)

	for _, setting := range *settings {
		if _, e := entClient.AppSetting.Create().SetKey(setting.Key).SetValue(setting.Value).Save(context.Background()); e != nil {
			err = append(err, e)
		}

	}

	return err
}

type AppSettingJson struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
