// Package tenant provides multi-tenant support for GoSQLPage v2.1.
package tenant

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

// Tenant represents a tenant in the system.
type Tenant struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Subdomain   string `json:"subdomain,omitempty"`
	PathPrefix  string `json:"path_prefix,omitempty"`
	Status      string `json:"status"` // active, suspended, deleted
	Config      string `json:"config,omitempty"` // JSON config
	CreatedAt   string `json:"created_at"`
	DataDir     string `json:"-"` // runtime: path to tenant data
}

// Manager manages multi-tenant operations.
type Manager struct {
	cfg       Config
	tenants   map[string]*Tenant
	mu        sync.RWMutex
	globalDB  *sql.DB
	logger    *slog.Logger
}

// Config holds tenant manager configuration.
type Config struct {
	// Base directory for tenant data
	BaseDataDir string
	// Global database path (shared schema.db, users.db)
	GlobalDBPath string
	// Routing mode: "subdomain", "path", "header"
	RoutingMode string
	// Header name for header-based routing
	HeaderName string
	// Default tenant ID for requests without tenant context
	DefaultTenant string
	Logger        *slog.Logger
}

// NewManager creates a new tenant manager.
func NewManager(cfg Config) (*Manager, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.RoutingMode == "" {
		cfg.RoutingMode = "subdomain"
	}
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-Tenant-ID"
	}

	// Open global database
	globalDB, err := sql.Open("sqlite", cfg.GlobalDBPath)
	if err != nil {
		return nil, fmt.Errorf("open global db: %w", err)
	}

	m := &Manager{
		cfg:      cfg,
		tenants:  make(map[string]*Tenant),
		globalDB: globalDB,
		logger:   cfg.Logger,
	}

	// Load existing tenants
	if err := m.loadTenants(); err != nil {
		globalDB.Close()
		return nil, fmt.Errorf("load tenants: %w", err)
	}

	return m, nil
}

// loadTenants loads tenants from the global database.
func (m *Manager) loadTenants() error {
	rows, err := m.globalDB.Query(`
		SELECT id, name, subdomain, path_prefix, status, config, created_at
		FROM tenants WHERE status != 'deleted'`)
	if err != nil {
		// Table might not exist yet
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var t Tenant
		var subdomain, pathPrefix, config sql.NullString
		err := rows.Scan(&t.ID, &t.Name, &subdomain, &pathPrefix, &t.Status, &config, &t.CreatedAt)
		if err != nil {
			continue
		}
		if subdomain.Valid {
			t.Subdomain = subdomain.String
		}
		if pathPrefix.Valid {
			t.PathPrefix = pathPrefix.String
		}
		if config.Valid {
			t.Config = config.String
		}
		t.DataDir = filepath.Join(m.cfg.BaseDataDir, t.ID)
		m.tenants[t.ID] = &t
	}

	m.logger.Info("loaded tenants", "count", len(m.tenants))
	return nil
}

// Create creates a new tenant.
func (m *Manager) Create(ctx context.Context, tenant *Tenant) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate
	if _, exists := m.tenants[tenant.ID]; exists {
		return fmt.Errorf("tenant already exists: %s", tenant.ID)
	}

	// Create tenant data directory
	dataDir := filepath.Join(m.cfg.BaseDataDir, tenant.ID)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	// Create tenant databases
	if err := m.initTenantDatabases(dataDir); err != nil {
		os.RemoveAll(dataDir)
		return fmt.Errorf("init databases: %w", err)
	}

	// Insert into global database
	_, err := m.globalDB.ExecContext(ctx, `
		INSERT INTO tenants (id, name, subdomain, path_prefix, status, config, created_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'))`,
		tenant.ID, tenant.Name, tenant.Subdomain, tenant.PathPrefix, "active", tenant.Config)
	if err != nil {
		os.RemoveAll(dataDir)
		return fmt.Errorf("insert tenant: %w", err)
	}

	tenant.Status = "active"
	tenant.DataDir = dataDir
	m.tenants[tenant.ID] = tenant

	m.logger.Info("created tenant", "id", tenant.ID, "name", tenant.Name)
	return nil
}

// initTenantDatabases initializes databases for a tenant.
func (m *Manager) initTenantDatabases(dataDir string) error {
	databases := []string{"content.db", "audit.db"}

	for _, dbName := range databases {
		dbPath := filepath.Join(dataDir, dbName)
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			return err
		}
		db.Exec("PRAGMA journal_mode=WAL")
		db.Close()
	}

	// Create sessions directory
	os.MkdirAll(filepath.Join(dataDir, "sessions"), 0755)

	return nil
}

// Get returns a tenant by ID.
func (m *Manager) Get(id string) (*Tenant, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tenants[id]
	return t, ok
}

// GetBySubdomain returns a tenant by subdomain.
func (m *Manager) GetBySubdomain(subdomain string) (*Tenant, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, t := range m.tenants {
		if t.Subdomain == subdomain {
			return t, true
		}
	}
	return nil, false
}

// GetByPathPrefix returns a tenant by path prefix.
func (m *Manager) GetByPathPrefix(path string) (*Tenant, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, t := range m.tenants {
		if t.PathPrefix != "" && strings.HasPrefix(path, t.PathPrefix) {
			return t, true
		}
	}
	return nil, false
}

// List returns all active tenants.
func (m *Manager) List() []*Tenant {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tenants := make([]*Tenant, 0, len(m.tenants))
	for _, t := range m.tenants {
		if t.Status == "active" {
			tenants = append(tenants, t)
		}
	}
	return tenants
}

// Suspend suspends a tenant.
func (m *Manager) Suspend(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tenants[id]
	if !ok {
		return fmt.Errorf("tenant not found: %s", id)
	}

	_, err := m.globalDB.ExecContext(ctx, `
		UPDATE tenants SET status = 'suspended' WHERE id = ?`, id)
	if err != nil {
		return err
	}

	t.Status = "suspended"
	m.logger.Info("suspended tenant", "id", id)
	return nil
}

// Activate activates a suspended tenant.
func (m *Manager) Activate(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tenants[id]
	if !ok {
		return fmt.Errorf("tenant not found: %s", id)
	}

	_, err := m.globalDB.ExecContext(ctx, `
		UPDATE tenants SET status = 'active' WHERE id = ?`, id)
	if err != nil {
		return err
	}

	t.Status = "active"
	m.logger.Info("activated tenant", "id", id)
	return nil
}

// Delete soft-deletes a tenant.
func (m *Manager) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.tenants[id]
	if !ok {
		return fmt.Errorf("tenant not found: %s", id)
	}

	_, err := m.globalDB.ExecContext(ctx, `
		UPDATE tenants SET status = 'deleted' WHERE id = ?`, id)
	if err != nil {
		return err
	}

	delete(m.tenants, id)
	m.logger.Info("deleted tenant", "id", id)
	return nil
}

// TenantContext holds tenant information for a request.
type TenantContext struct {
	Tenant   *Tenant
	DataDir  string
	ContentDB string
	AuditDB   string
	SessionsDir string
}

// contextKey is the key type for context values.
type contextKey string

const tenantContextKey contextKey = "tenant"

// FromContext extracts tenant context from request context.
func FromContext(ctx context.Context) (*TenantContext, bool) {
	tc, ok := ctx.Value(tenantContextKey).(*TenantContext)
	return tc, ok
}

// Middleware returns HTTP middleware for tenant resolution.
func (m *Manager) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant := m.resolveTenant(r)
			if tenant == nil {
				// Use default tenant if configured
				if m.cfg.DefaultTenant != "" {
					if t, ok := m.Get(m.cfg.DefaultTenant); ok {
						tenant = t
					}
				}
			}

			if tenant == nil {
				http.Error(w, "Tenant not found", http.StatusNotFound)
				return
			}

			if tenant.Status != "active" {
				http.Error(w, "Tenant is not active", http.StatusForbidden)
				return
			}

			// Create tenant context
			tc := &TenantContext{
				Tenant:     tenant,
				DataDir:    tenant.DataDir,
				ContentDB:  filepath.Join(tenant.DataDir, "content.db"),
				AuditDB:    filepath.Join(tenant.DataDir, "audit.db"),
				SessionsDir: filepath.Join(tenant.DataDir, "sessions"),
			}

			ctx := context.WithValue(r.Context(), tenantContextKey, tc)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// resolveTenant resolves tenant from the request.
func (m *Manager) resolveTenant(r *http.Request) *Tenant {
	switch m.cfg.RoutingMode {
	case "subdomain":
		return m.resolveBySubdomain(r)
	case "path":
		return m.resolveByPath(r)
	case "header":
		return m.resolveByHeader(r)
	default:
		return nil
	}
}

// resolveBySubdomain resolves tenant by subdomain.
func (m *Manager) resolveBySubdomain(r *http.Request) *Tenant {
	host := r.Host
	// Remove port if present
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}
	// Extract subdomain (first part before first dot)
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return nil
	}
	subdomain := parts[0]
	t, _ := m.GetBySubdomain(subdomain)
	return t
}

// resolveByPath resolves tenant by path prefix.
func (m *Manager) resolveByPath(r *http.Request) *Tenant {
	t, _ := m.GetByPathPrefix(r.URL.Path)
	return t
}

// resolveByHeader resolves tenant by header.
func (m *Manager) resolveByHeader(r *http.Request) *Tenant {
	tenantID := r.Header.Get(m.cfg.HeaderName)
	if tenantID == "" {
		return nil
	}
	t, _ := m.Get(tenantID)
	return t
}

// Close closes the tenant manager.
func (m *Manager) Close() error {
	return m.globalDB.Close()
}

// Stats returns tenant statistics.
type Stats struct {
	TotalTenants   int `json:"total_tenants"`
	ActiveTenants  int `json:"active_tenants"`
	SuspendedTenants int `json:"suspended_tenants"`
}

// GetStats returns tenant statistics.
func (m *Manager) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{TotalTenants: len(m.tenants)}
	for _, t := range m.tenants {
		switch t.Status {
		case "active":
			stats.ActiveTenants++
		case "suspended":
			stats.SuspendedTenants++
		}
	}
	return stats
}

// InitSchema creates the tenants table in the global database.
func InitSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS tenants (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			subdomain TEXT UNIQUE,
			path_prefix TEXT,
			status TEXT NOT NULL DEFAULT 'active',
			config TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		CREATE INDEX IF NOT EXISTS idx_tenants_subdomain ON tenants(subdomain);
		CREATE INDEX IF NOT EXISTS idx_tenants_status ON tenants(status);
	`)
	return err
}
