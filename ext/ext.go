package ext

import (
	"context"
	"database/sql" // Required to manually map the "sqlite3" engine name
	"fmt"

	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
	log "github.com/miru-project/miru-core/pkg/logger"

	entsql "entgo.io/ent/dialect/sql" // Aliased to avoid naming collisions
	"github.com/miru-project/miru-core/config"
	"github.com/miru-project/miru-core/ent"
	_ "github.com/miru-project/miru-core/ent/runtime"

	"modernc.org/sqlite"
)

var (
	entClient *ent.Client
)

func init() {
	// Start memory usage monitoring goroutine every 10 seconds
	// go func() {
	// 	ticker := time.NewTicker(10 * time.Second)
	// 	for range ticker.C {
	// 		var m runtime.MemStats
	// 		runtime.ReadMemStats(&m)
	// 		log.Printf("[Memory] Alloc = %d MB, TotalAlloc = %d MB, Sys = %d MB, NumGC = %d",
	// 			m.Alloc/1024/1024, m.TotalAlloc/1024/1024, m.Sys/1024/1024, m.NumGC)
	// 	}
	// }()

	// 1. Set up your global connection hook for PRAGMAs.
	// This turns on foreign key validation on the actual SQLite connection pool.
	sqlite.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, dsn string) error {
		_, err := conn.ExecContext(context.Background(), "PRAGMA foreign_keys = ON;", nil)
		return err
	})

	// 2. Map modernc's pure-Go driver instance onto the "sqlite3" key name.
	// We wrap it in a recover block so it doesn't panic if init runs multiple times during testing.
	func() {
		defer func() { recover() }()
		sql.Register("sqlite3", &sqlite.Driver{})
	}()
}

func EntClient() *ent.Client {
	if entClient != nil {
		return entClient
	}

	var client *ent.Client
	dbCfg := config.Global.Database

	var dsn string
	switch dbCfg.Driver {

	case "sqlite3":
		dsn = dbCfg.DBName
		log.Println("Using SQLite3 database at:", dsn)

		// 3. Open using the standard "sqlite3" dialect name.
		// Using standard "file://" URI formatting guarantees modernc parses the
		// parameters correctly so Ent's validation check doesn't trip.
		drv, err := entsql.Open("sqlite3", fmt.Sprintf("file://%s?cache=shared&_fk=1&_pragma=foreign_keys(1)", dsn))
		if err != nil {
			errorhandle.PanicF("failed opening connection to database: %s", err)
			return nil
		}
		client = ent.NewClient(ent.Driver(drv))

	default:
		errorhandle.PanicF("unsupported database driver: %s", dbCfg.Driver)
		return nil
	}

	if err := client.Schema.Create(context.Background()); err != nil {
		errorhandle.PanicF("failed creating schema resources: %s", err)
		return nil
	}

	entClient = client
	return client
}
