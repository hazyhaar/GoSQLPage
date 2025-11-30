// GoSQLPage v2.1 - Block-based SQL-driven web application server
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/hazyhaar/gopage/internal/templates"
	"github.com/hazyhaar/gopage/pkg/db"
	"github.com/hazyhaar/gopage/pkg/engine"
	"github.com/hazyhaar/gopage/pkg/funcs"
	"github.com/hazyhaar/gopage/pkg/render"
	"github.com/hazyhaar/gopage/pkg/sse"
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
	"zombiezen.com/go/sqlite"
)

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
		BackupDir: *backupDir,
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

	// Open content.db with v1 db package for SQL page rendering
	// (v1 uses zombiezen.com/go/sqlite, v2 uses modernc.org/sqlite)
	sqlPageDB, err := db.Open(db.Config{
		Path:        contentDBPath,
		ReaderCount: 4,
	})
	if err != nil {
		logger.Error("failed to open content.db for SQL pages", "error", err)
		os.Exit(1)
	}
	defer sqlPageDB.Close()

	// Register custom SQL functions
	funcRegistry := funcs.New()
	if err := sqlPageDB.SetConnInit(funcRegistry.Apply); err != nil {
		logger.Error("failed to register SQL functions", "error", err)
		os.Exit(1)
	}
	logger.Info("SQL page engine ready", "sql_dir", *sqlDir)

	// Create SQL page handler
	sqlParser := engine.NewParser()
	sqlExecutor := engine.NewExecutor()
	sqlPageHandler := &SQLPageHandler{
		db:       sqlPageDB,
		parser:   sqlParser,
		executor: sqlExecutor,
		renderer: renderer,
		sqlDir:   *sqlDir,
		logger:   logger,
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

	// SSE endpoint for real-time events
	sseHub := sse.NewHub(logger)
	sse.SetGlobalHub(sseHub)
	router.Get("/events", sseHub.ServeHTTP)

	// SQL pages - catch all (GET and POST)
	router.HandleFunc("/*", sqlPageHandler.ServeHTTP)

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

	// Schema files are in v2/data/
	schemaDir := filepath.Join(filepath.Dir(dataDir), "data")

	// Initialize content.db
	contentSchema, err := os.ReadFile(filepath.Join(schemaDir, "schema_content.sql"))
	if err != nil {
		return fmt.Errorf("read content schema: %w", err)
	}
	if err := initDB(contentPath, string(contentSchema), logger); err != nil {
		return fmt.Errorf("init content.db: %w", err)
	}

	// Initialize schema.db
	schemaSchema, err := os.ReadFile(filepath.Join(schemaDir, "schema_schema.sql"))
	if err != nil {
		return fmt.Errorf("read schema schema: %w", err)
	}
	if err := initDB(schemaPath, string(schemaSchema), logger); err != nil {
		return fmt.Errorf("init schema.db: %w", err)
	}

	// Initialize users.db
	usersSchema, err := os.ReadFile(filepath.Join(schemaDir, "schema_users.sql"))
	if err != nil {
		return fmt.Errorf("read users schema: %w", err)
	}
	if err := initDB(usersPath, string(usersSchema), logger); err != nil {
		return fmt.Errorf("init users.db: %w", err)
	}

	// Initialize audit.db
	auditSchema, err := os.ReadFile(filepath.Join(schemaDir, "schema_audit.sql"))
	if err != nil {
		return fmt.Errorf("read audit schema: %w", err)
	}
	if err := initDB(auditPath, string(auditSchema), logger); err != nil {
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

// SQLPageHandler handles SQL page requests using the v1 engine.
type SQLPageHandler struct {
	db       *db.DB
	parser   *engine.Parser
	executor *engine.Executor
	renderer *render.Renderer
	sqlDir   string
	logger   *slog.Logger
}

// ServeHTTP handles SQL page requests.
func (h *SQLPageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	path := r.URL.Path

	// Normalize path
	if path == "/" {
		path = "/index"
	}
	path = strings.TrimSuffix(path, "/")
	path = strings.TrimSuffix(path, ".sql")

	// Security: Clean path and validate it's within sqlDir
	cleanPath := filepath.Clean(path)
	sqlPath := filepath.Join(h.sqlDir, cleanPath+".sql")
	absPath, err := filepath.Abs(sqlPath)
	if err != nil {
		h.renderError(w, r, http.StatusBadRequest, "Invalid path")
		return
	}
	absSqlDir, _ := filepath.Abs(h.sqlDir)
	if !strings.HasPrefix(absPath, absSqlDir+string(filepath.Separator)) && absPath != absSqlDir {
		h.logger.Warn("path traversal attempt", "path", path, "resolved", absPath)
		h.renderError(w, r, http.StatusForbidden, "Access denied")
		return
	}

	// Check if file exists
	if _, err := os.Stat(sqlPath); err != nil {
		if os.IsNotExist(err) {
			h.renderError(w, r, http.StatusNotFound, "Page not found")
		} else {
			h.logger.Error("stat error", "path", sqlPath, "error", err)
			h.renderError(w, r, http.StatusInternalServerError, "Internal error")
		}
		return
	}

	// Parse SQL file
	file, err := h.parser.ParseFile(sqlPath)
	if err != nil {
		h.logger.Error("parse error", "path", sqlPath, "error", err)
		h.renderError(w, r, http.StatusInternalServerError, "Failed to parse SQL file")
		return
	}

	// Build params from URL query and form
	params := make(engine.Params)
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}

	// Parse form for POST requests
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			h.logger.Warn("parse form error", "error", err)
			// Continue anyway - form might be empty or malformed but we still process
		}
		for key, values := range r.Form {
			if len(values) > 0 {
				params[key] = values[0]
			}
		}
	}

	// Get appropriate connection
	var sqlConn *sqlite.Conn
	var release func()

	if r.Method == http.MethodPost {
		c, rel, err := h.db.Writer(ctx)
		if err != nil {
			h.logger.Error("get writer", "error", err)
			h.renderError(w, r, http.StatusServiceUnavailable, "Database unavailable")
			return
		}
		sqlConn = c
		release = rel
	} else {
		c, rel, err := h.db.Reader(ctx)
		if err != nil {
			h.logger.Error("get reader", "error", err)
			h.renderError(w, r, http.StatusServiceUnavailable, "Database unavailable")
			return
		}
		sqlConn = c
		release = rel
	}
	defer release()

	// Execute all queries
	var results []*engine.Result
	for _, query := range file.Queries {
		result, err := h.executor.Execute(ctx, sqlConn, query, params)
		if err != nil {
			h.logger.Error("execute error", "query", query.Component, "error", err)
			h.renderError(w, r, http.StatusInternalServerError, "Query failed: "+err.Error())
			return
		}
		results = append(results, result)
	}

	// Render page
	isHTMX := r.Header.Get("HX-Request") == "true"
	pageTitle := strings.TrimSuffix(filepath.Base(file.Path), ".sql")
	pageData := &render.PageData{
		Title:       pageTitle,
		Results:     results,
		CurrentPath: r.URL.Path,
		IsHTMX:      isHTMX,
	}

	if err := h.renderer.RenderPage(w, pageData); err != nil {
		h.logger.Error("render error", "error", err)
		// Don't try to render error page if rendering already failed
		http.Error(w, "Render failed", http.StatusInternalServerError)
	}
}

// renderError renders an error page.
func (h *SQLPageHandler) renderError(w http.ResponseWriter, r *http.Request, status int, message string) {
	w.WriteHeader(status)
	isHTMX := r.Header.Get("HX-Request") == "true"
	pageData := &render.PageData{
		Title:       "Error",
		CurrentPath: r.URL.Path,
		IsHTMX:      isHTMX,
		Error:       fmt.Errorf(message),
	}
	if err := h.renderer.RenderError(w, pageData); err != nil {
		http.Error(w, message, status)
	}
}
