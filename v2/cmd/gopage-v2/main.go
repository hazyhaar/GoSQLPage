// GoSQLPage v2.1 - Block-based SQL-driven web application server
package main

import (
	"context"
	"database/sql"
	_ "embed"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/hazyhaar/gopage/internal/templates"
	"github.com/hazyhaar/gopage/pkg/render"
	"github.com/hazyhaar/gopage/v2/pkg/api"
	"github.com/hazyhaar/gopage/v2/pkg/audit"
	"github.com/hazyhaar/gopage/v2/pkg/backup"
	"github.com/hazyhaar/gopage/v2/pkg/bot"
	"github.com/hazyhaar/gopage/v2/pkg/cache"
	"github.com/hazyhaar/gopage/v2/pkg/gc"
	"github.com/hazyhaar/gopage/v2/pkg/merger"
	"github.com/hazyhaar/gopage/v2/pkg/metrics"
	"github.com/hazyhaar/gopage/v2/pkg/session"
	_ "modernc.org/sqlite"
)

//go:embed ../../data/schema_content.sql
var contentSchema string

//go:embed ../../data/schema_schema.sql
var schemaSchema string

//go:embed ../../data/schema_users.sql
var usersSchema string

//go:embed ../../data/schema_audit.sql
var auditSchema string

func main() {
	// Parse flags
	var (
		dataDir     = flag.String("data", "./v2/data", "Data directory")
		sessionsDir = flag.String("sessions", "./v2/sessions", "Sessions directory")
		queueDir    = flag.String("queue", "./v2/queue", "Queue directory")
		cacheDir    = flag.String("cache", "./v2/cache/pages", "Page cache directory")
		backupDir   = flag.String("backup", "./v2/backup", "Backup directory")
		sqlDir      = flag.String("sql", "./sql", "SQL pages directory")
		port        = flag.String("port", "8080", "HTTP port")
		metricsPort = flag.String("metrics-port", "9090", "Prometheus metrics port (0 to disable)")
		debug       = flag.Bool("debug", false, "Enable debug logging")
		initDB      = flag.Bool("init", false, "Initialize databases if they don't exist")
		enableCache = flag.Bool("cache-enabled", true, "Enable page caching")
		enableBot   = flag.Bool("bot-enabled", false, "Enable bot worker")
	)
	flag.Parse()

	// Setup logger
	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Database paths
	contentDBPath := filepath.Join(*dataDir, "content.db")
	schemaDBPath := filepath.Join(*dataDir, "schema.db")
	usersDBPath := filepath.Join(*dataDir, "users.db")
	auditDBPath := filepath.Join(*dataDir, "audit.db")

	// Initialize databases if needed
	if *initDB {
		if err := initDatabases(*dataDir, contentDBPath, schemaDBPath, usersDBPath, auditDBPath, logger); err != nil {
			logger.Error("failed to initialize databases", "error", err)
			os.Exit(1)
		}
	}

	// Check databases exist
	for _, path := range []string{contentDBPath, schemaDBPath, usersDBPath, auditDBPath} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			logger.Error("database not found, run with -init flag", "path", path)
			os.Exit(1)
		}
	}

	// Create directories
	for _, dir := range []string{
		*sessionsDir,
		filepath.Join(*queueDir, "pending"),
		filepath.Join(*queueDir, "processing"),
		filepath.Join(*queueDir, "done"),
		filepath.Join(*queueDir, "failed"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			logger.Error("failed to create directory", "path", dir, "error", err)
			os.Exit(1)
		}
	}

	// Create session manager
	sessionMgr, err := session.NewManager(session.ManagerConfig{
		SessionsDir:      *sessionsDir,
		ContentDBPath:    contentDBPath,
		SchemaDBPath:     schemaDBPath,
		MaxInactiveHours: 24,
		Logger:           logger,
	})
	if err != nil {
		logger.Error("failed to create session manager", "error", err)
		os.Exit(1)
	}
	defer sessionMgr.Close()
	logger.Info("session manager started")

	// Create merger daemon
	mergerDaemon, err := merger.New(merger.Config{
		ContentDBPath:    contentDBPath,
		SchemaDBPath:     schemaDBPath,
		AuditDBPath:      auditDBPath,
		PendingDir:       filepath.Join(*queueDir, "pending"),
		ProcessingDir:    filepath.Join(*queueDir, "processing"),
		DoneDir:          filepath.Join(*queueDir, "done"),
		FailedDir:        filepath.Join(*queueDir, "failed"),
		PollIntervalMS:   500,
		MaxRetries:       3,
		RecoverOnStartup: true,
		Logger:           logger,
	})
	if err != nil {
		logger.Error("failed to create merger", "error", err)
		os.Exit(1)
	}
	defer mergerDaemon.Close()
	logger.Info("merger daemon created")

	// Create GC
	gcDaemon := gc.New(gc.Config{
		SessionsDir:        *sessionsDir,
		AuditDBPath:        auditDBPath,
		ContentDBPath:      contentDBPath,
		FailedDir:          filepath.Join(*queueDir, "failed"),
		DoneDir:            filepath.Join(*queueDir, "done"),
		IntervalHours:      6,
		AbandonedDays:      7,
		MergedDays:         1,
		AuditRetentionDays: 90,
		Logger:             logger,
	})
	logger.Info("GC created")

	// Create audit logger
	auditLogger, err := audit.NewLogger(audit.LoggerConfig{
		DBPath: auditDBPath,
		Config: audit.Config{
			StoreContent:      false,
			StoreContentTypes: []string{"code", "definition", "procedure"},
			RetentionDays:     90,
		},
		Logger: logger,
	})
	if err != nil {
		logger.Error("failed to create audit logger", "error", err)
		os.Exit(1)
	}
	defer auditLogger.Close()
	logger.Info("audit logger created")

	// Create page cache
	pageCache, err := cache.New(cache.Config{
		Dir:       *cacheDir,
		MaxSizeMB: 100,
		TTLHours:  24,
		Enabled:   *enableCache,
		Logger:    logger,
	})
	if err != nil {
		logger.Error("failed to create page cache", "error", err)
		os.Exit(1)
	}
	logger.Info("page cache created", "enabled", *enableCache)

	// Create backup manager
	backupMgr, err := backup.New(backup.Config{
		BackupDir:     *backupDir,
		Databases: []backup.DatabaseConfig{
			{Name: "content", Path: contentDBPath},
			{Name: "schema", Path: schemaDBPath},
			{Name: "users", Path: usersDBPath},
			{Name: "audit", Path: auditDBPath},
		},
		IntervalHours: 24,
		RetentionDays: 30,
		MaxBackups:    10,
		Logger:        logger,
	})
	if err != nil {
		logger.Error("failed to create backup manager", "error", err)
		os.Exit(1)
	}
	logger.Info("backup manager created")

	// Create metrics registry
	metricsRegistry := metrics.NewRegistry()
	requestMetrics := metrics.NewRequestMetrics(metricsRegistry)
	sessionMetrics := metrics.NewSessionMetrics(metricsRegistry)
	mergerMetrics := metrics.NewMergerMetrics(metricsRegistry)
	cacheMetrics := metrics.NewCacheMetrics(metricsRegistry)
	_ = requestMetrics // Will be used in middleware
	_ = sessionMetrics
	_ = mergerMetrics
	_ = cacheMetrics
	logger.Info("metrics registry created")

	// Create bot worker (optional)
	var botWorker *bot.Worker
	if *enableBot {
		botWorker, err = bot.NewWorker(bot.WorkerConfig{
			ContentDBPath:  contentDBPath,
			SessionManager: sessionMgr,
			Provider:       &bot.MockProvider{}, // Replace with real provider
			BotUserID:      "bot_system",
			PollIntervalMS: 1000,
			Logger:         logger,
		})
		if err != nil {
			logger.Warn("failed to create bot worker", "error", err)
		} else {
			logger.Info("bot worker created")
		}
	}

	// Create API handler
	apiHandler, err := api.New(api.Config{
		SessionManager: sessionMgr,
		Merger:         mergerDaemon,
		GC:             gcDaemon,
		AuditLogger:    auditLogger,
		ContentDBPath:  contentDBPath,
		SchemaDBPath:   schemaDBPath,
		UsersDBPath:    usersDBPath,
		Logger:         logger,
	})
	if err != nil {
		logger.Error("failed to create API handler", "error", err)
		os.Exit(1)
	}
	defer apiHandler.Close()
	logger.Info("API handler created")

	// Load templates
	templateFS, err := fs.Sub(templates.FS, "files")
	if err != nil {
		logger.Error("failed to load templates", "error", err)
		os.Exit(1)
	}

	// Create renderer
	renderer, err := render.New(render.Config{
		TemplatesFS: templateFS,
		Logger:      logger,
	})
	if err != nil {
		logger.Error("failed to create renderer", "error", err)
		os.Exit(1)
	}

	// Create router
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Compress(5))

	// Mount API routes
	router.Mount("/api", apiHandler.Routes())

	// Static assets
	router.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))

	// Health check
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK - GoSQLPage v2.1"))
	})

	// Prometheus metrics endpoint
	if *metricsPort != "0" {
		router.Handle("/metrics", metricsRegistry.Handler())
	}

	// Cache stats endpoint
	router.Get("/api/cache/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		stats := pageCache.GetStats()
		fmt.Fprintf(w, `{"enabled":%v,"entries":%d,"size_mb":%.2f,"hit_ratio":%.2f}`,
			stats.Enabled, stats.Entries, stats.SizeMB, stats.HitRatio)
	})

	// Backup endpoints
	router.Get("/api/backup/list", func(w http.ResponseWriter, r *http.Request) {
		backups, err := backupMgr.ListBackups()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"backups":%d}`, len(backups))
	})
	router.Post("/api/backup/now", func(w http.ResponseWriter, r *http.Request) {
		go backupMgr.RunNow(context.Background())
		w.Write([]byte(`{"status":"backup_started"}`))
	})

	// Placeholder for SQL pages (to be integrated with existing engine)
	router.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		// For now, render a simple page showing v2.1 is running
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>GoSQLPage v2.1</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        h1 { color: #333; }
        .status { background: #e8f5e9; padding: 20px; border-radius: 8px; margin: 20px 0; }
        code { background: #f5f5f5; padding: 2px 6px; border-radius: 4px; }
        .endpoints { background: #f5f5f5; padding: 20px; border-radius: 8px; }
        .endpoints h3 { margin-top: 0; }
        ul { line-height: 2; }
    </style>
</head>
<body>
    <h1>GoSQLPage v2.1</h1>
    <div class="status">
        <strong>Status:</strong> Running<br>
        <strong>Architecture:</strong> Block-based with isolated sessions
    </div>

    <div class="endpoints">
        <h3>API Endpoints</h3>
        <ul>
            <li><code>GET /api/health</code> - Health check</li>
            <li><code>GET /api/blocks</code> - List blocks</li>
            <li><code>GET /api/blocks/{id}</code> - Get block</li>
            <li><code>GET /api/search?q=...</code> - Search blocks</li>
            <li><code>POST /api/session</code> - Create editing session</li>
            <li><code>POST /api/session/blocks</code> - Add block to session</li>
            <li><code>POST /api/session/submit</code> - Submit for merge</li>
            <li><code>GET /api/admin/schema</code> - Get block types</li>
        </ul>
    </div>

    <p>SQL pages from <code>%s</code> will be served from this endpoint once integrated.</p>
</body>
</html>`, *sqlDir)
	})

	_ = renderer // Will be used when integrating SQL pages

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start merger daemon
	go mergerDaemon.Start(ctx)
	logger.Info("merger daemon started")

	// Start GC daemon
	go gcDaemon.Start(ctx)
	logger.Info("GC daemon started")

	// Start backup manager
	go backupMgr.Start(ctx)
	logger.Info("backup manager started")

	// Start bot worker if enabled
	if botWorker != nil {
		go botWorker.Start(ctx)
		logger.Info("bot worker started")
	}

	// Handle signals
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("shutting down...")
		cancel()
	}()

	// Start server
	addr := ":" + *port
	logger.Info("starting GoSQLPage v2.1 server", "addr", addr)

	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			cancel()
		}
	}()

	<-ctx.Done()

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
	}

	logger.Info("goodbye!")
}

// initDatabases initializes all databases with their schemas.
func initDatabases(dataDir, contentPath, schemaPath, usersPath, auditPath string, logger *slog.Logger) error {
	// Create data directory
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	// Initialize content.db
	if err := initDB(contentPath, contentSchema, logger); err != nil {
		return fmt.Errorf("init content.db: %w", err)
	}

	// Initialize schema.db
	if err := initDB(schemaPath, schemaSchema, logger); err != nil {
		return fmt.Errorf("init schema.db: %w", err)
	}

	// Initialize users.db
	if err := initDB(usersPath, usersSchema, logger); err != nil {
		return fmt.Errorf("init users.db: %w", err)
	}

	// Initialize audit.db
	if err := initDB(auditPath, auditSchema, logger); err != nil {
		return fmt.Errorf("init audit.db: %w", err)
	}

	logger.Info("databases initialized successfully")
	return nil
}

// initDB initializes a single database with schema.
func initDB(path, schema string, logger *slog.Logger) error {
	if _, err := os.Stat(path); err == nil {
		logger.Info("database already exists", "path", path)
		return nil
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()

	// Enable WAL mode
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("enable WAL: %w", err)
	}

	// Execute schema
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("execute schema: %w", err)
	}

	logger.Info("database created", "path", path)
	return nil
}
