// Package server provides the HTTP server for GoPage.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/hazyhaar/gopage/pkg/db"
	"github.com/hazyhaar/gopage/pkg/engine"
	"github.com/hazyhaar/gopage/pkg/render"
	"github.com/hazyhaar/gopage/pkg/sse"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// Server is the GoPage HTTP server.
type Server struct {
	router   *chi.Mux
	db       *db.DB
	parser   *engine.Parser
	executor *engine.Executor
	renderer *render.Renderer
	sqlDir   string
	logger   *slog.Logger
}

// Config holds server configuration.
type Config struct {
	DB       *db.DB
	Renderer *render.Renderer
	SQLDir   string
	Logger   *slog.Logger
}

// New creates a new server.
func New(cfg Config) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	s := &Server{
		router:   chi.NewRouter(),
		db:       cfg.DB,
		parser:   engine.NewParser(),
		executor: engine.NewExecutor(),
		renderer: cfg.Renderer,
		sqlDir:   cfg.SQLDir,
		logger:   cfg.Logger,
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures the router.
func (s *Server) setupRoutes() {
	r := s.router

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Static assets
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// SSE endpoint for real-time events
	sseHub := sse.NewHub(s.logger)
	sse.SetGlobalHub(sseHub)
	r.Get("/events", sseHub.ServeHTTP)

	// SQL page handler - catch all
	r.HandleFunc("/*", s.handlePage)
}

// handlePage handles SQL page requests.
func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	path := r.URL.Path

	// Normalize path
	if path == "/" {
		path = "/index"
	}
	path = strings.TrimSuffix(path, "/")
	path = strings.TrimSuffix(path, ".sql")

	// Find SQL file
	sqlPath := filepath.Join(s.sqlDir, path+".sql")
	if _, err := os.Stat(sqlPath); os.IsNotExist(err) {
		s.renderError(w, r, http.StatusNotFound, "Page not found")
		return
	}

	// Parse SQL file
	file, err := s.parser.ParseFile(sqlPath)
	if err != nil {
		s.logger.Error("parse error", "path", sqlPath, "error", err)
		s.renderError(w, r, http.StatusInternalServerError, "Failed to parse SQL file")
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
		if err := r.ParseForm(); err == nil {
			for key, values := range r.Form {
				if len(values) > 0 {
					params[key] = values[0]
				}
			}
		}
	}

	// Get appropriate connection
	var conn *sqlite.Conn
	var release func()

	if r.Method == http.MethodPost {
		c, rel, err := s.db.Writer(ctx)
		if err != nil {
			s.logger.Error("get writer", "error", err)
			s.renderError(w, r, http.StatusServiceUnavailable, "Database unavailable")
			return
		}
		conn = c
		release = rel
	} else {
		c, rel, err := s.db.Reader(ctx)
		if err != nil {
			s.logger.Error("get reader", "error", err)
			s.renderError(w, r, http.StatusServiceUnavailable, "Database unavailable")
			return
		}
		conn = c
		release = rel
	}
	defer release()

	// For POST requests, wrap execution in a transaction to ensure writes persist
	// zombiezen.com/go/sqlite requires explicit transactions for data to be committed
	if r.Method == http.MethodPost {
		if err := sqlitex.ExecuteTransient(conn, "BEGIN IMMEDIATE", nil); err != nil {
			s.logger.Error("begin transaction", "error", err)
			s.renderError(w, r, http.StatusInternalServerError, "Database error")
			return
		}
	}

	// Execute queries
	results, err := s.executor.ExecuteFile(ctx, conn, file, params)
	if err != nil {
		// Rollback on error for POST requests
		if r.Method == http.MethodPost {
			_ = sqlitex.ExecuteTransient(conn, "ROLLBACK", nil)
		}
		s.logger.Error("execute error", "error", err)
		s.renderError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	// Commit transaction for POST requests
	if r.Method == http.MethodPost {
		if err := sqlitex.ExecuteTransient(conn, "COMMIT", nil); err != nil {
			s.logger.Error("commit transaction", "error", err)
			s.renderError(w, r, http.StatusInternalServerError, "Failed to save changes")
			return
		}
	}

	// Check for HTMX request
	isHTMX := r.Header.Get("HX-Request") == "true"

	// Process special components (redirect, refresh, headers)
	var filteredResults []*engine.Result
	for _, result := range results {
		switch result.Query.Component {
		case "redirect":
			// Handle redirect
			target := result.Query.Options["target"]
			if target == "" && len(result.Rows) > 0 {
				if t, ok := result.Rows[0]["target"].(string); ok {
					target = t
				} else if t, ok := result.Rows[0]["url"].(string); ok {
					target = t
				}
			}
			if target != "" {
				if isHTMX {
					w.Header().Set("HX-Redirect", target)
					w.WriteHeader(http.StatusOK)
					return
				}
				http.Redirect(w, r, target, http.StatusSeeOther)
				return
			}

		case "refresh":
			// Trigger a page refresh via HTMX
			if isHTMX {
				w.Header().Set("HX-Refresh", "true")
			}

		case "trigger":
			// Trigger custom HTMX events
			if event := result.Query.Options["event"]; event != "" {
				w.Header().Set("HX-Trigger", event)
			}

		case "header":
			// Set custom headers
			for key, val := range result.Query.Options {
				if key != "component" {
					w.Header().Set(key, val)
				}
			}

		default:
			filteredResults = append(filteredResults, result)
		}
	}

	// Build page data with filtered results
	pageData := &render.PageData{
		Title:       "GoPage",
		Results:     filteredResults,
		CurrentPath: r.URL.Path,
		IsHTMX:      isHTMX,
	}

	// Extract title from shell component if present
	for _, result := range filteredResults {
		if result.Query.Component == "shell" {
			if title, ok := result.Query.Options["title"]; ok {
				pageData.Title = title
			}
		}
	}

	// Render page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderer.RenderPage(w, pageData); err != nil {
		s.logger.Error("render error", "error", err)
	}
}

// renderError renders an error page.
func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	pageData := &render.PageData{
		Title:       "Error",
		CurrentPath: r.URL.Path,
		IsHTMX:      r.Header.Get("HX-Request") == "true",
		Error:       &PageError{Status: status, Message: message},
	}

	if err := s.renderer.RenderError(w, pageData); err != nil {
		s.logger.Error("render error page failed", "error", err)
		http.Error(w, message, status)
	}
}

// PageError represents a page error.
type PageError struct {
	Status  int
	Message string
}

func (e *PageError) Error() string {
	return e.Message
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// ListenAndServe starts the server.
func (s *Server) ListenAndServe(addr string) error {
	s.logger.Info("starting server", "addr", addr)
	return http.ListenAndServe(addr, s)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return nil // Chi doesn't have built-in shutdown
}
