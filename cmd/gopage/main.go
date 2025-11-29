// GoPage - SQL-driven web application server
package main

import (
	"context"
	"flag"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hazyhaar/gopage/internal/templates"
	"github.com/hazyhaar/gopage/pkg/db"
	"github.com/hazyhaar/gopage/pkg/render"
	"github.com/hazyhaar/gopage/pkg/server"
)

func main() {
	// Parse flags
	var (
		dbPath = flag.String("db", "gopage.db", "SQLite database path")
		sqlDir = flag.String("sql", "./sql", "SQL files directory")
		port   = flag.String("port", "8080", "HTTP port")
		debug  = flag.Bool("debug", false, "Enable debug logging")
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

	// Open database
	database, err := db.Open(db.Config{
		Path:        *dbPath,
		ReaderCount: 4,
	})
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	logger.Info("database opened", "path", *dbPath)

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

	// Create server
	srv := server.New(server.Config{
		DB:       database,
		Renderer: renderer,
		SQLDir:   *sqlDir,
		Logger:   logger,
	})

	// Handle shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("shutting down...")
		cancel()
	}()

	// Start server
	addr := ":" + *port
	logger.Info("starting GoPage server", "addr", addr, "sql_dir", *sqlDir)

	go func() {
		if err := srv.ListenAndServe(addr); err != nil {
			logger.Error("server error", "error", err)
			cancel()
		}
	}()

	<-ctx.Done()
	logger.Info("goodbye!")
}
