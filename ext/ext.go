package ext

import (
	"context"
	"fmt"

	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
	log "github.com/miru-project/miru-core/pkg/logger"

	"entgo.io/ent/dialect"
	"github.com/miru-project/miru-core/config"
	"github.com/miru-project/miru-core/ent"
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

	// case "postgres":
	// 	dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
	// 		dbCfg.Host, dbCfg.Port, dbCfg.User, dbCfg.Password, dbCfg.DBName, dbCfg.SSLMode)
	// 	client, err = ent.Open(dialect.Postgres, dsn)

	// case "mysql":
	// 	dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=True",
	// 		dbCfg.User, dbCfg.Password, dbCfg.Host, dbCfg.Port, dbCfg.DBName)
	// 	client, err = ent.Open(dialect.MySQL, dsn)

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
	return client
}
