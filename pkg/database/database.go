package database

import (
	"context"
	"log"
	"time"

	// "database/sql"
	"path/filepath"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/miru-project/miru-core/ent"
	miru_path "github.com/miru-project/miru-core/pkg/path"
)

// Ent client
var Client *ent.Client

// Config represents database configuration
type Config struct {
	Path string // Path to the database file
}

func config() Config {
	return Config{
		Path: filepath.Join(miru_path.MiruDir, "miru_core.db"),
	}
}

// open opens the database connection with Ent
func open(config Config) error {

	// Open SQLite database driver
	drv, err := sql.Open(dialect.SQLite, "file:"+config.Path+"?_fk=1&mode=rwc")
	if err != nil {
		return err
	}
	db := drv.DB()
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(100)
	db.SetConnMaxLifetime(time.Hour)
	Client = ent.NewClient(ent.Driver(drv))
	return nil

}

// Close closes the database connection
func Close() error {
	if Client != nil {
		return Client.Close()
	}
	return nil
}

// Start initializes the database
func Start() error {

	config := config()
	e := open(config)
	if err := Client.Schema.Create(context.Background()); err != nil {
		log.Fatalf("failed creating schema resources: %v", err)
	}
	return e
}

// // GetHistoryByType fetches history entries by type
// func GetHistoryByType(ctx context.Context, historyType string) ([]*ent.History, error) {
// 	return Client.History.
// 		Query().
// 		Where(history.TypeEQ(historyType)).
// 		Order(ent.Desc(history.FieldCreatedAt)).
// 		Limit(100).
// 		All(ctx)
// }
