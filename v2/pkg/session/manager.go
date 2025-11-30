// Package session provides session management for isolated editing in GoSQLPage v2.1.
package session

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hazyhaar/gopage/v2/pkg/blocks"
	_ "modernc.org/sqlite"
)

//go:embed ../../../data/schema_session.sql
var sessionSchema string

// ManagerConfig holds configuration for the session manager.
type ManagerConfig struct {
	SessionsDir      string
	ContentDBPath    string
	SchemaDBPath     string
	MaxInactiveHours int
	Logger           *slog.Logger
}

// Manager manages editing sessions.
type Manager struct {
	cfg           ManagerConfig
	sessions      map[string]*Session
	mu            sync.RWMutex
	contentDB     *sql.DB
	schemaDB      *sql.DB
	schemaVersion int
	schemaHash    string
	logger        *slog.Logger
	idGen         *blocks.IDGenerator
}

// NewManager creates a new session manager.
func NewManager(cfg ManagerConfig) (*Manager, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.MaxInactiveHours <= 0 {
		cfg.MaxInactiveHours = 24
	}

	// Ensure sessions directory exists
	if err := os.MkdirAll(cfg.SessionsDir, 0755); err != nil {
		return nil, fmt.Errorf("create sessions dir: %w", err)
	}

	// Open content.db for reading
	contentDB, err := sql.Open("sqlite", cfg.ContentDBPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open content.db: %w", err)
	}

	// Open schema.db for reading
	schemaDB, err := sql.Open("sqlite", cfg.SchemaDBPath+"?mode=ro")
	if err != nil {
		contentDB.Close()
		return nil, fmt.Errorf("open schema.db: %w", err)
	}

	m := &Manager{
		cfg:       cfg,
		sessions:  make(map[string]*Session),
		contentDB: contentDB,
		schemaDB:  schemaDB,
		logger:    cfg.Logger,
		idGen:     blocks.NewIDGenerator(),
	}

	// Load schema version
	if err := m.loadSchemaVersion(); err != nil {
		contentDB.Close()
		schemaDB.Close()
		return nil, fmt.Errorf("load schema version: %w", err)
	}

	// Load existing sessions from disk
	if err := m.loadExistingSessions(); err != nil {
		cfg.Logger.Warn("failed to load existing sessions", "error", err)
	}

	return m, nil
}

// loadSchemaVersion loads the current schema version and hash.
func (m *Manager) loadSchemaVersion() error {
	row := m.schemaDB.QueryRow("SELECT version, hash FROM schema_version WHERE id = 1")
	return row.Scan(&m.schemaVersion, &m.schemaHash)
}

// loadExistingSessions loads existing session files from disk.
func (m *Manager) loadExistingSessions() error {
	entries, err := os.ReadDir(m.cfg.SessionsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".db" {
			continue
		}

		path := filepath.Join(m.cfg.SessionsDir, entry.Name())
		session, err := m.loadSessionFromFile(path)
		if err != nil {
			m.logger.Warn("failed to load session", "path", path, "error", err)
			continue
		}

		m.mu.Lock()
		m.sessions[session.ID] = session
		m.mu.Unlock()
	}

	return nil
}

// loadSessionFromFile loads a session from its database file.
func (m *Manager) loadSessionFromFile(path string) (*Session, error) {
	db, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var meta SessionMeta
	row := db.QueryRow(`SELECT session_id, user_id, user_type, created_at, last_activity,
		base_snapshot, schema_version, schema_hash, status FROM _session_meta LIMIT 1`)
	err = row.Scan(&meta.SessionID, &meta.UserID, &meta.UserType, &meta.CreatedAt,
		&meta.LastActivity, &meta.BaseSnapshot, &meta.SchemaVersion, &meta.SchemaHash, &meta.Status)
	if err != nil {
		return nil, err
	}

	createdAt, _ := time.Parse(time.RFC3339, meta.CreatedAt)
	lastActivity, _ := time.Parse(time.RFC3339, meta.LastActivity)

	return &Session{
		ID:            meta.SessionID,
		UserID:        meta.UserID,
		UserType:      meta.UserType,
		CreatedAt:     createdAt,
		LastActivity:  lastActivity,
		BaseSnapshot:  meta.BaseSnapshot,
		SchemaVersion: meta.SchemaVersion,
		SchemaHash:    meta.SchemaHash,
		Status:        Status(meta.Status),
		DBPath:        path,
	}, nil
}

// Create creates a new editing session for a user.
func (m *Manager) Create(ctx context.Context, userID, userType string) (*Session, error) {
	sessionID := m.idGen.SessionID(userID)
	dbPath := filepath.Join(m.cfg.SessionsDir, sessionID+".db")

	// Get content.db snapshot hash
	baseSnapshot, err := m.getContentSnapshot()
	if err != nil {
		return nil, fmt.Errorf("get content snapshot: %w", err)
	}

	// Create session database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("create session db: %w", err)
	}
	defer db.Close()

	// Initialize schema
	if _, err := db.ExecContext(ctx, sessionSchema); err != nil {
		os.Remove(dbPath)
		return nil, fmt.Errorf("init session schema: %w", err)
	}

	// Insert session metadata
	now := time.Now().UTC()
	_, err = db.ExecContext(ctx, `INSERT INTO _session_meta
		(session_id, user_id, user_type, created_at, last_activity, base_snapshot, schema_version, schema_hash, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID, userID, userType, now.Format(time.RFC3339), now.Format(time.RFC3339),
		baseSnapshot, m.schemaVersion, m.schemaHash, StatusActive)
	if err != nil {
		os.Remove(dbPath)
		return nil, fmt.Errorf("insert session meta: %w", err)
	}

	session := &Session{
		ID:            sessionID,
		UserID:        userID,
		UserType:      userType,
		CreatedAt:     now,
		LastActivity:  now,
		BaseSnapshot:  baseSnapshot,
		SchemaVersion: m.schemaVersion,
		SchemaHash:    m.schemaHash,
		Status:        StatusActive,
		DBPath:        dbPath,
	}

	m.mu.Lock()
	m.sessions[sessionID] = session
	m.mu.Unlock()

	m.logger.Info("created session", "session_id", sessionID, "user_id", userID)
	return session, nil
}

// Get returns a session by ID.
func (m *Manager) Get(sessionID string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[sessionID]
	return session, ok
}

// GetOrCreate gets an existing active session for a user or creates a new one.
func (m *Manager) GetOrCreate(ctx context.Context, userID, userType string) (*Session, error) {
	m.mu.RLock()
	for _, s := range m.sessions {
		if s.UserID == userID && s.Status == StatusActive {
			m.mu.RUnlock()
			return s, nil
		}
	}
	m.mu.RUnlock()

	return m.Create(ctx, userID, userType)
}

// getContentSnapshot returns a hash representing the current content.db state.
func (m *Manager) getContentSnapshot() (string, error) {
	// Use the count and max updated_at as a simple snapshot
	var count int
	var maxUpdated sql.NullString
	row := m.contentDB.QueryRow("SELECT COUNT(*), MAX(updated_at) FROM blocks")
	if err := row.Scan(&count, &maxUpdated); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%s", count, maxUpdated.String), nil
}

// CopyBlock copies a block from content.db to a session for editing.
func (m *Manager) CopyBlock(ctx context.Context, sessionID, blockID string) (*blocks.Block, error) {
	session, ok := m.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	if session.Status != StatusActive {
		return nil, fmt.Errorf("session is not active: %s", session.Status)
	}

	// Read block from content.db
	var block blocks.Block
	var parentID sql.NullString
	var contentHTML, deletedAt sql.NullString

	row := m.contentDB.QueryRowContext(ctx, `
		SELECT id, parent_id, type, content, content_html, position, hash,
		       created_at, updated_at, created_by, published, deleted_at
		FROM blocks WHERE id = ?`, blockID)

	err := row.Scan(&block.ID, &parentID, &block.Type, &block.Content, &contentHTML,
		&block.Position, &block.Hash, &block.CreatedAt, &block.UpdatedAt,
		&block.CreatedBy, &block.Published, &deletedAt)
	if err != nil {
		return nil, fmt.Errorf("read block: %w", err)
	}

	if parentID.Valid {
		block.ParentID = &parentID.String
	}
	if contentHTML.Valid {
		block.ContentHTML = contentHTML.String
	}

	// Open session db and copy the block
	sessionDB, err := sql.Open("sqlite", session.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open session db: %w", err)
	}
	defer sessionDB.Close()

	_, err = sessionDB.ExecContext(ctx, `
		INSERT OR REPLACE INTO blocks
		(id, parent_id, type, content, content_html, position, hash, created_at, updated_at, created_by, published, deleted_at, _dirty, _source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 'copy')`,
		block.ID, parentID, block.Type, block.Content, contentHTML,
		block.Position, block.Hash, block.CreatedAt, block.UpdatedAt,
		block.CreatedBy, block.Published, deletedAt)
	if err != nil {
		return nil, fmt.Errorf("copy to session: %w", err)
	}

	// Record structural dependency
	deps := []string{}
	if parentID.Valid {
		deps = append(deps, parentID.String)
	}
	depsJSON, _ := json.Marshal(deps)
	hashesJSON, _ := json.Marshal(map[string]string{blockID: block.Hash})

	_, err = sessionDB.ExecContext(ctx, `
		INSERT OR REPLACE INTO _structural_deps (block_id, depends_on, snapshot_hashes)
		VALUES (?, ?, ?)`, blockID, string(depsJSON), string(hashesJSON))
	if err != nil {
		return nil, fmt.Errorf("record deps: %w", err)
	}

	// Update last activity
	m.updateLastActivity(ctx, session)

	block.Source = "copy"
	return &block, nil
}

// UpdateBlock updates a block in a session.
func (m *Manager) UpdateBlock(ctx context.Context, sessionID string, block *blocks.Block) error {
	session, ok := m.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if session.Status != StatusActive {
		return fmt.Errorf("session is not active: %s", session.Status)
	}

	sessionDB, err := sql.Open("sqlite", session.DBPath)
	if err != nil {
		return fmt.Errorf("open session db: %w", err)
	}
	defer sessionDB.Close()

	// Get current state for change log
	var beforeJSON string
	row := sessionDB.QueryRowContext(ctx, `
		SELECT json_object('id', id, 'content', content, 'type', type, 'position', position)
		FROM blocks WHERE id = ?`, block.ID)
	row.Scan(&beforeJSON)

	// Update the block
	block.UpdateHash()
	block.UpdatedAt = time.Now().UTC()

	_, err = sessionDB.ExecContext(ctx, `
		UPDATE blocks SET
			content = ?, content_html = ?, position = ?, hash = ?,
			updated_at = ?, published = ?, _dirty = 1
		WHERE id = ?`,
		block.Content, block.ContentHTML, block.Position, block.Hash,
		block.UpdatedAt, block.Published, block.ID)
	if err != nil {
		return fmt.Errorf("update block: %w", err)
	}

	// Log the change
	afterJSON, _ := json.Marshal(block)
	_, err = sessionDB.ExecContext(ctx, `
		INSERT INTO _changes (operation, block_id, before, after)
		VALUES ('update', ?, ?, ?)`, block.ID, beforeJSON, string(afterJSON))
	if err != nil {
		return fmt.Errorf("log change: %w", err)
	}

	m.updateLastActivity(ctx, session)
	return nil
}

// InsertBlock creates a new block in a session.
func (m *Manager) InsertBlock(ctx context.Context, sessionID string, block *blocks.Block) error {
	session, ok := m.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if session.Status != StatusActive {
		return fmt.Errorf("session is not active: %s", session.Status)
	}

	sessionDB, err := sql.Open("sqlite", session.DBPath)
	if err != nil {
		return fmt.Errorf("open session db: %w", err)
	}
	defer sessionDB.Close()

	// Generate ID if needed
	if block.ID == "" {
		block.ID = blocks.NewBlockID()
	}

	block.UpdateHash()
	now := time.Now().UTC()
	block.CreatedAt = now
	block.UpdatedAt = now

	_, err = sessionDB.ExecContext(ctx, `
		INSERT INTO blocks
		(id, parent_id, type, content, content_html, position, hash, created_at, updated_at, created_by, published, _dirty, _source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 'new')`,
		block.ID, block.ParentID, block.Type, block.Content, block.ContentHTML,
		block.Position, block.Hash, block.CreatedAt, block.UpdatedAt,
		block.CreatedBy, block.Published)
	if err != nil {
		return fmt.Errorf("insert block: %w", err)
	}

	// Log the change
	afterJSON, _ := json.Marshal(block)
	_, err = sessionDB.ExecContext(ctx, `
		INSERT INTO _changes (operation, block_id, after)
		VALUES ('insert', ?, ?)`, block.ID, string(afterJSON))
	if err != nil {
		return fmt.Errorf("log change: %w", err)
	}

	m.updateLastActivity(ctx, session)
	return nil
}

// DeleteBlock marks a block as deleted in a session.
func (m *Manager) DeleteBlock(ctx context.Context, sessionID, blockID string) error {
	session, ok := m.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if session.Status != StatusActive {
		return fmt.Errorf("session is not active: %s", session.Status)
	}

	sessionDB, err := sql.Open("sqlite", session.DBPath)
	if err != nil {
		return fmt.Errorf("open session db: %w", err)
	}
	defer sessionDB.Close()

	// Get current state
	var beforeJSON string
	row := sessionDB.QueryRowContext(ctx, `
		SELECT json_object('id', id, 'content', content, 'type', type)
		FROM blocks WHERE id = ?`, blockID)
	row.Scan(&beforeJSON)

	now := time.Now().UTC()
	_, err = sessionDB.ExecContext(ctx, `
		UPDATE blocks SET deleted_at = ?, _dirty = 1 WHERE id = ?`,
		now, blockID)
	if err != nil {
		return fmt.Errorf("delete block: %w", err)
	}

	// Log the change
	_, err = sessionDB.ExecContext(ctx, `
		INSERT INTO _changes (operation, block_id, before)
		VALUES ('delete', ?, ?)`, blockID, beforeJSON)
	if err != nil {
		return fmt.Errorf("log change: %w", err)
	}

	m.updateLastActivity(ctx, session)
	return nil
}

// Submit submits a session for merging.
func (m *Manager) Submit(ctx context.Context, sessionID string) error {
	session, ok := m.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if session.Status != StatusActive {
		return fmt.Errorf("session is not active: %s", session.Status)
	}

	sessionDB, err := sql.Open("sqlite", session.DBPath)
	if err != nil {
		return fmt.Errorf("open session db: %w", err)
	}
	defer sessionDB.Close()

	_, err = sessionDB.ExecContext(ctx, `
		UPDATE _session_meta SET status = ? WHERE session_id = ?`,
		StatusSubmitted, sessionID)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	m.mu.Lock()
	session.Status = StatusSubmitted
	m.mu.Unlock()

	m.logger.Info("submitted session", "session_id", sessionID)
	return nil
}

// Abandon abandons a session.
func (m *Manager) Abandon(ctx context.Context, sessionID string) error {
	session, ok := m.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	sessionDB, err := sql.Open("sqlite", session.DBPath)
	if err != nil {
		return fmt.Errorf("open session db: %w", err)
	}
	defer sessionDB.Close()

	_, err = sessionDB.ExecContext(ctx, `
		UPDATE _session_meta SET status = ? WHERE session_id = ?`,
		StatusAbandoned, sessionID)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	m.mu.Lock()
	session.Status = StatusAbandoned
	m.mu.Unlock()

	m.logger.Info("abandoned session", "session_id", sessionID)
	return nil
}

// GetDiff returns the differences between session and content.db.
func (m *Manager) GetDiff(ctx context.Context, sessionID string) (*Diff, error) {
	session, ok := m.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	sessionDB, err := sql.Open("sqlite", session.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open session db: %w", err)
	}
	defer sessionDB.Close()

	diff := &Diff{SessionID: sessionID}

	// Get all dirty blocks
	rows, err := sessionDB.QueryContext(ctx, `
		SELECT id, parent_id, type, content, position, hash, _source
		FROM blocks WHERE _dirty = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var block blocks.Block
		var parentID sql.NullString
		var source string

		err := rows.Scan(&block.ID, &parentID, &block.Type, &block.Content,
			&block.Position, &block.Hash, &source)
		if err != nil {
			continue
		}

		if parentID.Valid {
			block.ParentID = &parentID.String
		}

		if source == "new" {
			diff.Inserts = append(diff.Inserts, &BlockDiff{
				BlockID: block.ID,
				Type:    block.Type,
				After:   &block,
			})
		} else {
			// Get original from content.db
			var original blocks.Block
			row := m.contentDB.QueryRowContext(ctx, `
				SELECT id, type, content, position, hash FROM blocks WHERE id = ?`, block.ID)
			if err := row.Scan(&original.ID, &original.Type, &original.Content,
				&original.Position, &original.Hash); err == nil {
				diff.Updates = append(diff.Updates, &BlockDiff{
					BlockID: block.ID,
					Type:    block.Type,
					Before:  &original,
					After:   &block,
				})
			}
		}
	}

	return diff, nil
}

// updateLastActivity updates the session's last activity timestamp.
func (m *Manager) updateLastActivity(ctx context.Context, session *Session) {
	now := time.Now().UTC()

	sessionDB, err := sql.Open("sqlite", session.DBPath)
	if err != nil {
		return
	}
	defer sessionDB.Close()

	sessionDB.ExecContext(ctx, `
		UPDATE _session_meta SET last_activity = ? WHERE session_id = ?`,
		now.Format(time.RFC3339), session.ID)

	m.mu.Lock()
	session.LastActivity = now
	m.mu.Unlock()
}

// ListSessions returns all sessions.
func (m *Manager) ListSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// ListSessionsByUser returns sessions for a specific user.
func (m *Manager) ListSessionsByUser(userID string) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []*Session
	for _, s := range m.sessions {
		if s.UserID == userID {
			sessions = append(sessions, s)
		}
	}
	return sessions
}

// Close closes the session manager.
func (m *Manager) Close() error {
	var errs []error
	if err := m.contentDB.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := m.schemaDB.Close(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}
