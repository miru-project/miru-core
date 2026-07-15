package binary

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	fasthttp_router "github.com/fasthttp/router"
	"github.com/miru-project/miru-core/config"
	"github.com/miru-project/miru-core/ext"
	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/download"
	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
	golang "github.com/miru-project/miru-core/pkg/extension/golang"
	jsext "github.com/miru-project/miru-core/pkg/extension/js"
	log "github.com/miru-project/miru-core/pkg/logger"
	"github.com/miru-project/miru-core/pkg/network"
	"github.com/miru-project/miru-core/pkg/torrent"
	"github.com/miru-project/miru-core/router"
)

// memoryMonitorSampleInterval is how often the program logs its memory usage
// and GC behaviour while running.
const memoryMonitorSampleInterval = 10 * time.Second

// startMemoryMonitor launches a background goroutine that prints the process
// heap/GC statistics every memoryMonitorSampleInterval seconds. It starts once
// (guarded by startMemoryMonitorOnce) when the program boots so the output is
// emitted for the whole lifetime of the app. Each sample also records the
// cumulative number of GC cycles since the previous sample, which is the
// closest signal to "GC triggering" available from runtime.MemStats.
func startMemoryMonitor() {
	var started int32
	if !atomic.CompareAndSwapInt32(&started, 0, 1) {
		return
	}
	go func() {
		var prevNumGC uint32
		var prevPauseTotalNs uint64
		ticker := time.NewTicker(memoryMonitorSampleInterval)
		defer ticker.Stop()
		for range ticker.C {
			var m runtime.MemStats
			// Read the latest GC counters first so LastGC and the
			// delta counters below reflect the most recent collections.
			runtime.ReadMemStats(&m)

			gcSinceLast := m.NumGC - prevNumGC
			pauseSinceLastNs := m.PauseTotalNs - prevPauseTotalNs
			prevNumGC = m.NumGC
			prevPauseTotalNs = m.PauseTotalNs

			lastGCAgo := time.Duration(0)
			if m.LastGC != 0 {
				lastGCAgo = time.Since(time.Unix(0, int64(m.LastGC)))
			}
			lastPause := time.Duration(0)
			if m.NumGC > 0 {
				lastPause = time.Duration(m.PauseNs[(m.NumGC+255)%256])
			}

			log.Printf(
				"[mem] alloc=%s sys=%s heapInuse=%s heapIdle=%s heapReleased=%s "+
					"heapObjects=%d frees(total)=%d numGC(total)=%d gcTriggered(last %ds)=%d "+
					"lastGC=%s pause(last)=%s pause(last %ds)=%s nextGC=%s gcPause%%=%.4f",
				formatBytes(m.Alloc),
				formatBytes(m.Sys),
				formatBytes(m.HeapInuse),
				formatBytes(m.HeapIdle),
				formatBytes(m.HeapReleased),
				m.HeapObjects,
				m.Frees,
				m.NumGC,
				int(memoryMonitorSampleInterval.Seconds()),
				gcSinceLast,
				lastGCAgo.Round(time.Millisecond),
				lastPause,
				int(memoryMonitorSampleInterval.Seconds()),
				time.Duration(pauseSinceLastNs),
				formatBytes(m.NextGC),
				percent(pauseSinceLastNs, uint64(memoryMonitorSampleInterval)),
			)
		}
	}()
}

// formatBytes renders a byte count in a human readable unit.
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// percent returns x as a percentage of total, guarded against divide-by-zero.
func percent(x, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(x) / float64(total) * 100
}

func InitProgram(configPath *string) {

	// Start the memory/GC monitor as early as possible so it tracks the whole
	// lifetime of the program.
	startMemoryMonitor()

	// Initialize logger
	log.InitLog(filepath.Dir(*configPath))

	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		log.Printf("Configuration file not found at %s, creating with default settings", *configPath)
		defaultConfig := config.GetDefaultConfig()
		config.Global = defaultConfig
		if err := config.Save(*configPath); err != nil {
			errorhandle.PanicF("Failed to create default configuration (%s): %v", *configPath, err)
		}
	} else if err != nil {
		errorhandle.PanicF("Error checking configuration file: %v", err)
	} else {
		if err := config.Load(*configPath); err != nil {
			errorhandle.PanicF("Failed to load configuration (%s): %v", *configPath, err)
		}
	}

	app := fasthttp_router.New()
	Init()
	router.InitRouter(app)

}

func Init() {
	// Make extension path absolute if needed
	if !filepath.IsAbs(config.Global.ExtensionPath) {
		absPath, err := filepath.Abs(config.Global.ExtensionPath)
		if err != nil {
			errorhandle.PanicF("failed to get absolute path for extension path: %v", err)
		}
		config.Global.ExtensionPath = absPath
	}

	network.Init()
	ext.EntClient()
	db.Initialize()
	torrent.Init()
	download.Init()
	jsext.InitRuntime(config.Global.ExtensionPath, jsext.AssetsFS)
	// The Go/Scriggo extension runtime resolves packages by stat-ing
	// <ExtensionDir>/<pkg>.go. Unlike js.ExtPath (set inside
	// InitRuntime), golang.ExtensionDir is never initialized, so .go
	// extensions could never be found. Set it here so .go extensions are
	// discoverable and resolvable.
	golang.ExtensionDir = config.Global.ExtensionPath
	// Eagerly load Go/Scriggo extensions at startup (mirrors jsext.InitRuntime)
	// so they are visibly loaded and any compile error surfaces early.
	golang.LoadExtensions()
	log.Println("Miru Core initialized successfully!")
}
